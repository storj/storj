// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/private/web"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/restapikeys"
)

// RestAPIKeys is an api controller that exposes REST API related functionality.
type RestAPIKeys struct {
	log     *zap.Logger
	service restapikeys.Service
}

// NewRestAPIKeys is a constructor for rest api keys controller.
func NewRestAPIKeys(log *zap.Logger, service restapikeys.Service) *RestAPIKeys {
	return &RestAPIKeys{
		log:     log,
		service: service,
	}
}

// GetUserRestAPIKeys returns the user's REST API keys.
func (rA *RestAPIKeys) GetUserRestAPIKeys(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	keys, err := rA.service.GetAll(ctx)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			rA.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}
		rA.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	err = json.NewEncoder(w).Encode(keys)
	if err != nil {
		rA.serveJSONError(ctx, w, http.StatusInternalServerError, err)
	}
}

// CreateRestKey handles creating REST API keys.
func (rA *RestAPIKeys) CreateRestKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	var data struct {
		Name       string         `json:"name"`
		Expiration *time.Duration `json:"expiration"`
	}
	err = json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		rA.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	key, _, err := rA.service.Create(ctx, data.Name, data.Expiration)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			rA.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}
		rA.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	err = json.NewEncoder(w).Encode(key)
	if err != nil {
		rA.serveJSONError(ctx, w, http.StatusInternalServerError, err)
	}
}

// RevokeRestKeys handles revoking REST API keys.
func (rA *RestAPIKeys) RevokeRestKeys(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	var data struct {
		IDs []string `json:"ids"`
	}

	err = json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		rA.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	ids := make([]uuid.UUID, len(data.IDs))
	for i, id := range data.IDs {
		ids[i], err = uuid.FromString(id)
		if err != nil {
			rA.serveJSONError(ctx, w, http.StatusBadRequest, err)
			return
		}
	}

	err = rA.service.RevokeByIDs(ctx, ids)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			rA.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}
		rA.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}
}

// serveJSONError writes JSON error to response output stream.
func (rA *RestAPIKeys) serveJSONError(ctx context.Context, w http.ResponseWriter, status int, err error) {
	web.ServeJSONError(ctx, rA.log, w, status, err)
}
