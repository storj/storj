// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi

import (
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
)

// Projects is an api controller that exposes projects related functionality.
type Projects struct {
	log     *zap.Logger
	service *console.Service
}

// NewProjects is a constructor for api analytics controller.
func NewProjects(log *zap.Logger, service *console.Service) *Projects {
	return &Projects{
		log:     log,
		service: service,
	}
}

// GetSalt returns the project's salt.
func (p *Projects) GetSalt(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	idParam, ok := mux.Vars(r)["id"]
	if !ok {
		p.serveJSONError(w, http.StatusBadRequest, errs.New("missing id route param"))
		return
	}

	id, err := uuid.FromString(idParam)
	if err != nil {
		p.serveJSONError(w, http.StatusBadRequest, err)
	}

	salt, err := p.service.GetSalt(ctx, id)
	if err != nil {
		p.serveJSONError(w, http.StatusUnauthorized, err)
		return
	}

	b64SaltString := base64.StdEncoding.EncodeToString(salt)

	err = json.NewEncoder(w).Encode(b64SaltString)
	if err != nil {
		p.serveJSONError(w, http.StatusInternalServerError, err)
	}
}

// serveJSONError writes JSON error to response output stream.
func (p *Projects) serveJSONError(w http.ResponseWriter, status int, err error) {
	ServeJSONError(p.log, w, status, err)
}
