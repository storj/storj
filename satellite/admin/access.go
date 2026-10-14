// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/zeebo/errs"

	"storj.io/common/grant"
	"storj.io/common/macaroon"
	"storj.io/storj/private/api"
	"storj.io/storj/satellite/admin/auditlogger"
	"storj.io/storj/satellite/admin/changehistory"
)

// AccessInspectRequest contains an access string to inspect.
type AccessInspectRequest struct {
	Access string `json:"access"`
}

// AccessInspectResult contains all info about access inspection that should be presented.
type AccessInspectResult struct {
	SatelliteAddr     string   `json:"satelliteAddr"`
	DefaultPathCipher string   `json:"defaultPathCipher"`
	APIKey            string   `json:"apiKey"`
	APIKeyID          string   `json:"apiKeyID"`
	Macaroon          Macaroon `json:"macaroon"`
	Revoked           bool     `json:"revoked"`

	PublicProjectID   string `json:"publicProjectID"`
	ProjectOwnerID    string `json:"projectOwnerID"`
	ProjectOwnerEmail string `json:"projectOwnerEmail"`
	CreatorID         string `json:"creatorID"`
}

// Macaroon contains all info about access macaroon that should be presented.
type Macaroon struct {
	Head    []byte            `json:"-"`
	Caveats []macaroon.Caveat `json:"caveats"`
	Tail    []byte            `json:"tail"`
}

// InspectAccess inspects the provided access string and returns its metadata.
func (s *Service) InspectAccess(ctx context.Context, request AccessInspectRequest) (*AccessInspectResult, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	if request.Access == "" {
		return nil, api.HTTPError{
			Status: http.StatusBadRequest,
			Err:    errs.New("access string is required"),
		}
	}

	// The provided access value might be the S3 public access key ID.
	if utf8.RuneCountInString(request.Access) == 28 {
		access, err := s.fetchAccessGrant(ctx, request.Access)
		if err != nil {
			return nil, api.HTTPError{
				Status: http.StatusInternalServerError,
				Err:    errs.New("could not fetch access grant: %+v", err),
			}
		}
		request.Access = access
	}

	access, err := grant.ParseAccess(request.Access)
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusBadRequest,
			Err:    errs.New("could not parse access: %+v", err),
		}
	}

	m, err := macaroon.ParseMacaroon(access.APIKey.SerializeRaw())
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusBadRequest,
			Err:    errs.New("could not parse api key macaroon: %+v", err),
		}
	}

	result := &AccessInspectResult{}

	for _, cb := range m.Caveats() {
		var c macaroon.Caveat

		err := c.UnmarshalBinary(cb)
		if err != nil {
			return nil, api.HTTPError{
				Status: http.StatusBadRequest,
				Err:    errs.New("could not parse macaroon caveat: %+v", err),
			}
		}

		result.Macaroon.Caveats = append(result.Macaroon.Caveats, c)
	}

	result.Revoked, err = s.revocationDB.Check(ctx, [][]byte{m.Tail()})
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusInternalServerError,
			Err:    errs.New("could not check revocation status: %+v", err),
		}
	}

	keyInfo, err := s.consoleDB.APIKeys().GetByHead(ctx, m.Head())
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusInternalServerError,
			Err:    errs.New("could not get API key info: %+v", err),
		}
	}

	project, err := s.consoleDB.Projects().GetByPublicID(ctx, keyInfo.ProjectPublicID)
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusInternalServerError,
			Err:    errs.New("could not get project info: %+v", err),
		}
	}

	projectOwner, err := s.consoleDB.Users().Get(ctx, project.OwnerID)
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusInternalServerError,
			Err:    errs.New("could not get project owner info: %+v", err),
		}
	}

	result.SatelliteAddr = access.SatelliteAddress
	result.APIKey = access.APIKey.Serialize()
	result.APIKeyID = keyInfo.ID.String()
	result.DefaultPathCipher = access.EncAccess.Store.GetDefaultPathCipher().String()
	result.Macaroon.Head = m.Head()
	result.Macaroon.Tail = m.Tail()

	result.PublicProjectID = project.PublicID.String()
	result.ProjectOwnerID = projectOwner.ID.String()
	result.ProjectOwnerEmail = projectOwner.Email
	result.CreatorID = keyInfo.CreatedBy.String()

	return result, api.HTTPError{}
}

func (s *Service) fetchAccessGrant(ctx context.Context, access string) (string, error) {
	var err error
	defer mon.Task()(&ctx)(&err)

	endpoint, err := url.JoinPath(s.adminConfig.AuthServiceAccessEndpoint, access)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+s.adminConfig.AuthServiceAccessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { err = errs.Combine(err, resp.Body.Close()) }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", errs.New("auth service returned status %d and body could not be read: %+v", resp.StatusCode, err)
		}

		return "", errs.New("auth service returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var data struct {
		AccessGrant string `json:"access_grant"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	if data.AccessGrant == "" {
		return "", errs.New("empty access grant received from auth service")
	}

	return data.AccessGrant, nil
}

// AccessRevokeRequest contains access data to revoke.
type AccessRevokeRequest struct {
	// APIKey is the serialized API key whose tail is revoked, as returned by InspectAccess.
	APIKey string `json:"apiKey"`
	Reason string `json:"reason"`
}

// RevokeAccess revokes the provided API key.
//
// The tail to revoke is taken from the API key after validating it against the secret of
// its root key, so a caller cannot pair a tail with a key of a different project.
func (s *Service) RevokeAccess(ctx context.Context, authInfo *AuthInfo, request AccessRevokeRequest) api.HTTPError {
	var err error
	defer mon.Task()(&ctx)(&err)

	apiError := func(status int, err error) api.HTTPError {
		return api.HTTPError{
			Status: status, Err: Error.Wrap(err),
		}
	}

	if !s.authorizer.IsAuthorized(authInfo) {
		return apiError(http.StatusUnauthorized, errs.New("not authorized"))
	}

	if request.Reason == "" {
		return apiError(http.StatusBadRequest, errs.New("reason for revocation is required"))
	}
	if request.APIKey == "" {
		return apiError(http.StatusBadRequest, errs.New("API key is required"))
	}

	apiKey, err := macaroon.ParseAPIKey(request.APIKey)
	if err != nil {
		return apiError(http.StatusBadRequest, errs.New("could not parse API key: %+v", err))
	}
	m, err := macaroon.ParseMacaroon(apiKey.SerializeRaw())
	if err != nil {
		return apiError(http.StatusBadRequest, errs.New("could not parse api key macaroon: %+v", err))
	}

	keyInfo, err := s.consoleDB.APIKeys().GetByHead(ctx, m.Head())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return apiError(http.StatusNotFound, errs.New("API key not found"))
		}
		return apiError(http.StatusInternalServerError, errs.New("could not get API key info: %+v", err))
	}
	project, err := s.consoleDB.Projects().Get(ctx, keyInfo.ProjectID)
	if err != nil {
		return apiError(http.StatusInternalServerError, errs.New("could not get project info: %+v", err))
	}

	if apiErr := s.checkProjectOwnerTenant(ctx, project.OwnerID); apiErr.Err != nil {
		return apiErr
	}

	if !m.Validate(keyInfo.Secret) {
		return apiError(http.StatusBadRequest, errs.New("API key does not match its root key"))
	}
	tail := m.Tail()

	err = s.revocationDB.Revoke(ctx, tail, keyInfo.ID.Bytes())
	if err != nil {
		return apiError(http.StatusInternalServerError, errs.New("could not revoke access: %+v", err))
	}

	s.auditLogger.EnqueueChangeEvent(auditlogger.Event{
		UserID:       project.OwnerID,
		ProjectID:    &project.PublicID,
		RootAPIKeyID: &keyInfo.ID,
		Action:       "revoke_access",
		AdminEmail:   authInfo.Email,
		ItemType:     changehistory.ItemTypeAccess,
		Reason:       request.Reason,
		Before:       map[string]any{"revoked_tail": nil},
		After:        map[string]any{"revoked_tail": tail},
		Timestamp:    s.nowFn(),
	})

	return api.HTTPError{}
}
