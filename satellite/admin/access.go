// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"context"
	"net/http"

	"github.com/zeebo/errs"

	"storj.io/common/grant"
	"storj.io/common/macaroon"
	"storj.io/common/uuid"
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

// AccessRevokeRequest contains access data to revoke.
type AccessRevokeRequest struct {
	Tail     []byte `json:"tail"`
	APIKeyID string `json:"apiKeyID"`
	Reason   string `json:"reason"`
}

// RevokeAccess revokes access based on the provided tail and API key ID.
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
	if request.Tail == nil || request.APIKeyID == "" {
		return apiError(http.StatusBadRequest, errs.New("tail and API key ID are required"))
	}

	keyID, err := uuid.FromString(request.APIKeyID)
	if err != nil {
		return apiError(http.StatusBadRequest, errs.New("could not parse API key ID: %+v", err))
	}

	keyInfo, err := s.consoleDB.APIKeys().Get(ctx, keyID)
	if err != nil {
		return apiError(http.StatusInternalServerError, errs.New("could not get API key info: %+v", err))
	}
	project, err := s.consoleDB.Projects().Get(ctx, keyInfo.ProjectID)
	if err != nil {
		return apiError(http.StatusInternalServerError, errs.New("could not get project info: %+v", err))
	}

	err = s.revocationDB.Revoke(ctx, request.Tail, keyInfo.ID.Bytes())
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
		After:        map[string]any{"revoked_tail": request.Tail},
		Timestamp:    s.nowFn(),
	})

	return api.HTTPError{}
}
