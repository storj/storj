// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/private/web"
	"storj.io/storj/satellite/console"
)

// Valdi is an api controller that exposes valdi related functionality.
type Valdi struct {
	log     *zap.Logger
	service *console.Service
}

// NewValdi is a constructor for api valdi controller.
func NewValdi(log *zap.Logger, service *console.Service) *Valdi {
	return &Valdi{
		log:     log,
		service: service,
	}
}

// GetAPIKey gets a valdi API key for a project. If valdi user doesn't exist, it is created.
func (v *Valdi) GetAPIKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var ok bool
	var idParam string

	if idParam, ok = mux.Vars(r)["project-id"]; !ok {
		v.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing project id route param"))
		return
	}

	id, err := uuid.FromString(idParam)
	if err != nil {
		v.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("invalid project id: %s", idParam))
		return
	}

	apiKey, status, err := v.service.GetValdiAPIKey(ctx, id)
	if err != nil {
		v.serveJSONError(ctx, w, status, err)
		return
	}

	err = json.NewEncoder(w).Encode(apiKey)
	if err != nil {
		v.serveJSONError(ctx, w, http.StatusInternalServerError, err)
	}
}

// serveJSONError writes JSON error to response output stream.
func (v *Valdi) serveJSONError(ctx context.Context, w http.ResponseWriter, status int, err error) {
	web.ServeJSONError(ctx, v.log, w, status, err)
}
