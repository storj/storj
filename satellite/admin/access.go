// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"context"
	"net/http"

	"github.com/zeebo/errs"

	"storj.io/common/grant"
	"storj.io/common/macaroon"
	"storj.io/storj/private/api"
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
	result.DefaultPathCipher = access.EncAccess.Store.GetDefaultPathCipher().String()
	result.Macaroon.Head = m.Head()
	result.Macaroon.Tail = m.Tail()

	result.PublicProjectID = project.PublicID.String()
	result.ProjectOwnerID = projectOwner.ID.String()
	result.ProjectOwnerEmail = projectOwner.Email
	result.CreatorID = keyInfo.CreatedBy.String()

	return result, api.HTTPError{}
}
