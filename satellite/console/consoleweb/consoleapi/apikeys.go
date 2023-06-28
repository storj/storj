// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/private/web"
	"storj.io/storj/satellite/console"
)

var (
	// ErrAPIKeysAPI - console api keys api error type.
	ErrAPIKeysAPI = errs.Class("console api keys")
)

// APIKeys is an api controller that exposes all api keys related functionality.
type APIKeys struct {
	log     *zap.Logger
	service *console.Service
}

// NewAPIKeys is a constructor for api api keys controller.
func NewAPIKeys(log *zap.Logger, service *console.Service) *APIKeys {
	return &APIKeys{
		log:     log,
		service: service,
	}
}

// GetAllAPIKeyNames returns all api key names by project ID.
func (keys *APIKeys) GetAllAPIKeyNames(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	projectIDString := r.URL.Query().Get("projectID")
	if projectIDString == "" {
		keys.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("Project ID was not provided."))
		return
	}

	projectID, err := uuid.FromString(projectIDString)
	if err != nil {
		keys.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	apiKeyNames, err := keys.service.GetAllAPIKeyNamesByProjectID(ctx, projectID)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			keys.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		keys.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	err = json.NewEncoder(w).Encode(apiKeyNames)
	if err != nil {
		keys.log.Error("failed to write json all api key names response", zap.Error(ErrAPIKeysAPI.Wrap(err)))
	}
}

// DeleteByNameAndProjectID deletes specific api key by it's name and project ID.
// ID here may be project.publicID or project.ID.
func (keys *APIKeys) DeleteByNameAndProjectID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	name := r.URL.Query().Get("name")
	projectIDString := r.URL.Query().Get("projectID")
	publicIDString := r.URL.Query().Get("publicID")

	if name == "" {
		keys.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	var projectID uuid.UUID
	if projectIDString != "" {
		projectID, err = uuid.FromString(projectIDString)
		if err != nil {
			keys.serveJSONError(ctx, w, http.StatusBadRequest, err)
			return
		}
	} else if publicIDString != "" {
		projectID, err = uuid.FromString(publicIDString)
		if err != nil {
			keys.serveJSONError(ctx, w, http.StatusBadRequest, err)
			return
		}
	} else {
		keys.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("Project ID was not provided."))
		return
	}

	err = keys.service.DeleteAPIKeyByNameAndProjectID(ctx, name, projectID)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			keys.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		if console.ErrNoAPIKey.Has(err) {
			keys.serveJSONError(ctx, w, http.StatusNoContent, err)
			return
		}

		keys.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}
}

// serveJSONError writes JSON error to response output stream.
func (keys *APIKeys) serveJSONError(ctx context.Context, w http.ResponseWriter, status int, err error) {
	web.ServeJSONError(ctx, keys.log, w, status, err)
}
