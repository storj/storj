// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package controllers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/multinode/nodes"
	"storj.io/storj/multinode/reputation"
)

var (
	// ErrReputation is an error type for reputation web api controller.
	ErrReputation = errs.Class("reputation web api controller")
)

// Reputation is a reputation web api controller.
type Reputation struct {
	log     *zap.Logger
	service *reputation.Service
}

// NewReputation is a constructor of reputation controller.
func NewReputation(log *zap.Logger, service *reputation.Service) *Reputation {
	return &Reputation{
		log:     log,
		service: service,
	}
}

// Stats handles retrieval of a node reputation for particular satellite.
func (controller *Reputation) Stats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Add("Content-Type", "application/json")
	segments := mux.Vars(r)

	satelliteIDEnc, ok := segments["satelliteID"]
	if !ok {
		controller.serveError(w, http.StatusBadRequest, ErrReputation.New("could not retrieve satellite id segment"))
		return
	}
	satelliteID, err := storj.NodeIDFromString(satelliteIDEnc)
	if err != nil {
		controller.serveError(w, http.StatusBadRequest, ErrReputation.Wrap(err))
		return
	}

	stats, err := controller.service.Stats(ctx, satelliteID)
	if err != nil {
		if nodes.ErrNoNode.Has(err) {
			controller.serveError(w, http.StatusNotFound, ErrReputation.Wrap(err))
			return
		}

		controller.log.Error("reputation stats internal error", zap.Error(ErrReputation.Wrap(err)))
		controller.serveError(w, http.StatusInternalServerError, ErrReputation.Wrap(err))
		return
	}

	if len(stats) == 0 {
		stats = make([]reputation.Stats, 0)
	}
	if err = json.NewEncoder(w).Encode(stats); err != nil {
		controller.log.Error("failed to write json response", zap.Error(ErrReputation.Wrap(err)))
		return
	}
}

// serveError set http statuses and send json error.
func (controller *Reputation) serveError(w http.ResponseWriter, status int, err error) {
	w.WriteHeader(status)

	var response struct {
		Error string `json:"error"`
	}
	response.Error = err.Error()

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		controller.log.Error("failed to write json error response", zap.Error(err))
	}
}
