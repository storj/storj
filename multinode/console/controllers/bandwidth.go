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
	"storj.io/storj/multinode/bandwidth"
)

var (
	// ErrBandwidth is an internal error type for bandwidth web api controller.
	ErrBandwidth = errs.Class("bandwidth web api controller")
)

// Bandwidth is a web api controller.
type Bandwidth struct {
	log     *zap.Logger
	service *bandwidth.Service
}

// NewBandwidth is a constructor for Bandwidth.
func NewBandwidth(log *zap.Logger, service *bandwidth.Service) *Bandwidth {
	return &Bandwidth{
		log:     log,
		service: service,
	}
}

// Monthly handles all satellites all nodes bandwidth monthly.
func (controller *Bandwidth) Monthly(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Add("Content-Type", "application/json")

	monthly, err := controller.service.Monthly(ctx)
	if err != nil {
		controller.log.Error("get bandwidth monthly error", zap.Error(err))
		controller.serveError(w, http.StatusInternalServerError, ErrBandwidth.Wrap(err))
		return
	}

	if err = json.NewEncoder(w).Encode(monthly); err != nil {
		controller.log.Error("failed to write json response", zap.Error(err))
		return
	}
}

// MonthlyNode handles all satellites single node bandwidth monthly.
func (controller *Bandwidth) MonthlyNode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Add("Content-Type", "application/json")
	segmentParams := mux.Vars(r)

	id, ok := segmentParams["nodeID"]
	if !ok {
		controller.serveError(w, http.StatusBadRequest, ErrPayouts.New("couldn't receive route variable nodeID"))
		return
	}

	nodeID, err := storj.NodeIDFromString(id)
	if err != nil {
		controller.serveError(w, http.StatusBadRequest, ErrPayouts.Wrap(err))
		return
	}

	monthly, err := controller.service.MonthlyNode(ctx, nodeID)
	if err != nil {
		controller.log.Error("get bandwidth monthly for specific node error", zap.Error(err))
		controller.serveError(w, http.StatusInternalServerError, ErrBandwidth.Wrap(err))
		return
	}

	if err = json.NewEncoder(w).Encode(monthly); err != nil {
		controller.log.Error("failed to write json response", zap.Error(err))
		return
	}
}

// MonthlySatellite handles specific satellite all nodes bandwidth monthly.
func (controller *Bandwidth) MonthlySatellite(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Add("Content-Type", "application/json")
	segmentParams := mux.Vars(r)

	id, ok := segmentParams["id"]
	if !ok {
		controller.serveError(w, http.StatusBadRequest, ErrPayouts.New("couldn't receive route variable satellite id"))
		return
	}

	satelliteID, err := storj.NodeIDFromString(id)
	if err != nil {
		controller.serveError(w, http.StatusBadRequest, ErrPayouts.Wrap(err))
		return
	}

	monthly, err := controller.service.MonthlySatellite(ctx, satelliteID)
	if err != nil {
		controller.log.Error("get bandwidth monthly for specific satellite error", zap.Error(err))
		controller.serveError(w, http.StatusInternalServerError, ErrBandwidth.Wrap(err))
		return
	}

	if err = json.NewEncoder(w).Encode(monthly); err != nil {
		controller.log.Error("failed to write json response", zap.Error(err))
		return
	}
}

// MonthlySatelliteNode handles specific satellite single node bandwidth monthly.
func (controller *Bandwidth) MonthlySatelliteNode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Add("Content-Type", "application/json")
	segmentParams := mux.Vars(r)

	id, ok := segmentParams["id"]
	if !ok {
		controller.serveError(w, http.StatusBadRequest, ErrPayouts.New("couldn't receive route variable satellite id"))
		return
	}

	satelliteID, err := storj.NodeIDFromString(id)
	if err != nil {
		controller.serveError(w, http.StatusBadRequest, ErrPayouts.Wrap(err))
		return
	}

	node, ok := segmentParams["nodeID"]
	if !ok {
		controller.serveError(w, http.StatusBadRequest, ErrPayouts.New("couldn't receive route variable satellite id"))
		return
	}

	nodeID, err := storj.NodeIDFromString(node)
	if err != nil {
		controller.serveError(w, http.StatusBadRequest, ErrPayouts.Wrap(err))
		return
	}

	monthly, err := controller.service.MonthlySatelliteNode(ctx, satelliteID, nodeID)
	if err != nil {
		controller.log.Error("get bandwidth monthly for specific satellite and node error", zap.Error(err))
		controller.serveError(w, http.StatusInternalServerError, ErrBandwidth.Wrap(err))
		return
	}

	if err = json.NewEncoder(w).Encode(monthly); err != nil {
		controller.log.Error("failed to write json response", zap.Error(err))
		return
	}
}

// serveError set http statuses and send json error.
func (controller *Bandwidth) serveError(w http.ResponseWriter, status int, err error) {
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
