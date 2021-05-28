// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi

import (
	"encoding/json"
	"net/http"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
)

var (
	// ErrAPIKeysAPI - console api keys api error type.
	ErrAPIKeysAPI = errs.Class("consoleapi keys")
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

// DeleteByNameAndProjectID deletes specific api key by it's name and project ID.
func (keys *APIKeys) DeleteByNameAndProjectID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	name := r.URL.Query().Get("name")
	projectIDString := r.URL.Query().Get("projectID")

	if name == "" {
		keys.serveJSONError(w, http.StatusBadRequest, err)
		return
	}

	projectID, err := uuid.FromString(projectIDString)
	if err != nil {
		keys.serveJSONError(w, http.StatusBadRequest, err)
		return
	}

	err = keys.service.DeleteAPIKeyByNameAndProjectID(ctx, name, projectID)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			keys.serveJSONError(w, http.StatusUnauthorized, err)
			return
		}

		if console.ErrNoAPIKey.Has(err) {
			keys.serveJSONError(w, http.StatusNoContent, err)
			return
		}

		keys.serveJSONError(w, http.StatusInternalServerError, err)
		return
	}
}

// serveJSONError writes JSON error to response output stream.
func (keys *APIKeys) serveJSONError(w http.ResponseWriter, status int, err error) {
	w.WriteHeader(status)

	if status == http.StatusNoContent {
		return
	}

	if status == http.StatusInternalServerError {
		keys.log.Error("returning internal server error to client", zap.Int("code", status), zap.Error(err))
	} else {
		keys.log.Debug("returning error to client", zap.Int("code", status), zap.Error(err))
	}

	var response struct {
		Error string `json:"error"`
	}

	response.Error = err.Error()

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		keys.log.Error("failed to write json error response", zap.Error(ErrAPIKeysAPI.Wrap(err)))
	}
}
