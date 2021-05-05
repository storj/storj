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
	"storj.io/storj/multinode/payouts"
)

var (
	// ErrPayouts is an internal error type for payouts web api controller.
	ErrPayouts = errs.Class("payouts web api controller")
)

// Payouts is a web api controller.
type Payouts struct {
	log     *zap.Logger
	service *payouts.Service
}

// NewPayouts is a constructor for Payouts.
func NewPayouts(log *zap.Logger, service *payouts.Service) *Payouts {
	return &Payouts{
		log:     log,
		service: service,
	}
}

// GetAllNodesTotalEarned handles retrieval total earned amount .
func (controller *Payouts) GetAllNodesTotalEarned(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	earned, err := controller.service.GetAllNodesAllTimeEarned(ctx)
	if err != nil {
		controller.log.Error("all node total earned internal error", zap.Error(err))
		controller.serveError(w, http.StatusInternalServerError, ErrPayouts.Wrap(err))
		return
	}

	if err = json.NewEncoder(w).Encode(earned); err != nil {
		controller.log.Error("failed to write json response", zap.Error(err))
		return
	}
}

// SatelliteEstimations handles nodes estimated earnings from satellite.
func (controller *Payouts) SatelliteEstimations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)
	segmentParams := mux.Vars(r)
	id, ok := segmentParams["satelliteID"]
	if !ok {
		controller.serveError(w, http.StatusBadRequest, ErrPayouts.Wrap(err))
		return
	}
	satelliteID, err := storj.NodeIDFromString(id)
	if err != nil {
		controller.serveError(w, http.StatusBadRequest, ErrPayouts.Wrap(err))
		return
	}
	estimatedEarnings, err := controller.service.AllNodesSatelliteEstimations(ctx, satelliteID)
	if err != nil {
		controller.serveError(w, http.StatusInternalServerError, ErrPayouts.Wrap(err))
		return
	}
	if err = json.NewEncoder(w).Encode(estimatedEarnings); err != nil {
		controller.log.Error("failed to write json response", zap.Error(err))
		return
	}
}

// Estimations handles nodes estimated earnings.
func (controller *Payouts) Estimations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)
	estimatedEarnings, err := controller.service.AllNodesEstimations(ctx)
	if err != nil {
		controller.serveError(w, http.StatusInternalServerError, ErrPayouts.Wrap(err))
		return
	}
	if err = json.NewEncoder(w).Encode(estimatedEarnings); err != nil {
		controller.log.Error("failed to write json response", zap.Error(err))
		return
	}
}

// serveError set http statuses and send json error.
func (controller *Payouts) serveError(w http.ResponseWriter, status int, err error) {
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
