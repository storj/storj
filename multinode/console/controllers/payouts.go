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

// Earned handles retrieval total earned amount .
func (controller *Payouts) Earned(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	earned, err := controller.service.Earned(ctx)
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

// NodeExpectations handles node's estimated and undistributed.
func (controller *Payouts) NodeExpectations(w http.ResponseWriter, r *http.Request) {
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

	expectations, err := controller.service.NodeExpectations(ctx, nodeID)
	if err != nil {
		controller.serveError(w, http.StatusInternalServerError, ErrPayouts.Wrap(err))
		return
	}

	if err = json.NewEncoder(w).Encode(expectations); err != nil {
		controller.log.Error("failed to write json response", zap.Error(err))
		return
	}
}

// Expectations handles nodes estimated and undistributed earnings.
func (controller *Payouts) Expectations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Add("Content-Type", "application/json")

	expectations, err := controller.service.Expectations(ctx)
	if err != nil {
		controller.serveError(w, http.StatusInternalServerError, ErrPayouts.Wrap(err))
		return
	}

	if err = json.NewEncoder(w).Encode(expectations); err != nil {
		controller.log.Error("failed to write json response", zap.Error(err))
		return
	}
}

// SummaryPeriod handles retrieval from nodes for specific period.
func (controller *Payouts) SummaryPeriod(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Add("Content-Type", "application/json")
	segmentParams := mux.Vars(r)

	period, ok := segmentParams["period"]
	if !ok {
		controller.serveError(w, http.StatusBadRequest, ErrPayouts.New("couldn't receive route variable period"))
		return
	}

	summary, err := controller.service.SummaryPeriod(ctx, period)
	if err != nil {
		controller.serveError(w, http.StatusInternalServerError, ErrPayouts.Wrap(err))
		return
	}

	if err = json.NewEncoder(w).Encode(summary); err != nil {
		controller.log.Error("failed to write json response", zap.Error(err))
		return
	}
}

// Summary handles retrieval from nodes.
func (controller *Payouts) Summary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Add("Content-Type", "application/json")

	summary, err := controller.service.Summary(ctx)
	if err != nil {
		controller.serveError(w, http.StatusInternalServerError, ErrPayouts.Wrap(err))
		return
	}

	if err = json.NewEncoder(w).Encode(summary); err != nil {
		controller.log.Error("failed to write json response", zap.Error(err))
		return
	}
}

// SummarySatellitePeriod handles retrieval from nodes from specific satellite for specific period.
func (controller *Payouts) SummarySatellitePeriod(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Add("Content-Type", "application/json")
	segmentParams := mux.Vars(r)

	period, ok := segmentParams["period"]
	if !ok {
		controller.serveError(w, http.StatusBadRequest, ErrPayouts.New("couldn't receive route variable period"))
		return
	}

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

	summary, err := controller.service.SummarySatellitePeriod(ctx, satelliteID, period)
	if err != nil {
		controller.serveError(w, http.StatusInternalServerError, ErrPayouts.Wrap(err))
		return
	}

	if err = json.NewEncoder(w).Encode(summary); err != nil {
		controller.log.Error("failed to write json response", zap.Error(err))
		return
	}
}

// SummarySatellite handles retrieval from nodes from specific satellite.
func (controller *Payouts) SummarySatellite(w http.ResponseWriter, r *http.Request) {
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

	summary, err := controller.service.SummarySatellite(ctx, satelliteID)
	if err != nil {
		controller.serveError(w, http.StatusInternalServerError, ErrPayouts.Wrap(err))
		return
	}

	if err = json.NewEncoder(w).Encode(summary); err != nil {
		controller.log.Error("failed to write json response", zap.Error(err))
		return
	}
}

// Paystub returns all summed paystubs.
func (controller *Payouts) Paystub(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Add("Content-Type", "application/json")
	segmentParams := mux.Vars(r)

	nodeIDstring, ok := segmentParams["nodeID"]
	if !ok {
		controller.serveError(w, http.StatusBadRequest, ErrPayouts.New("couldn't receive route variable nodeID"))
		return
	}

	nodeID, err := storj.NodeIDFromString(nodeIDstring)
	if err != nil {
		controller.serveError(w, http.StatusBadRequest, ErrPayouts.Wrap(err))
		return
	}

	paystub, err := controller.service.Paystub(ctx, nodeID)
	if err != nil {
		controller.serveError(w, http.StatusInternalServerError, ErrPayouts.Wrap(err))
		return
	}

	if err = json.NewEncoder(w).Encode(paystub); err != nil {
		controller.log.Error("failed to write json response", zap.Error(err))
		return
	}
}

// PaystubSatellite returns all summed paystubs from specific satellite.
func (controller *Payouts) PaystubSatellite(w http.ResponseWriter, r *http.Request) {
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

	nodeIDstring, ok := segmentParams["nodeID"]
	if !ok {
		controller.serveError(w, http.StatusBadRequest, ErrPayouts.New("couldn't receive route variable nodeID"))
		return
	}

	nodeID, err := storj.NodeIDFromString(nodeIDstring)
	if err != nil {
		controller.serveError(w, http.StatusBadRequest, ErrPayouts.Wrap(err))
		return
	}

	paystub, err := controller.service.PaystubSatellite(ctx, nodeID, satelliteID)
	if err != nil {
		controller.serveError(w, http.StatusInternalServerError, ErrPayouts.Wrap(err))
		return
	}

	if err = json.NewEncoder(w).Encode(paystub); err != nil {
		controller.log.Error("failed to write json response", zap.Error(err))
		return
	}
}

// PaystubSatellitePeriod returns satellite summed paystubs for period.
func (controller *Payouts) PaystubSatellitePeriod(w http.ResponseWriter, r *http.Request) {
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

	nodeIDstring, ok := segmentParams["nodeID"]
	if !ok {
		controller.serveError(w, http.StatusBadRequest, ErrPayouts.New("couldn't receive route variable nodeID"))
		return
	}

	nodeID, err := storj.NodeIDFromString(nodeIDstring)
	if err != nil {
		controller.serveError(w, http.StatusBadRequest, ErrPayouts.Wrap(err))
		return
	}

	period, ok := segmentParams["period"]
	if !ok {
		controller.serveError(w, http.StatusBadRequest, ErrPayouts.New("couldn't receive route variable period"))
		return
	}

	paystub, err := controller.service.PaystubSatellitePeriod(ctx, period, nodeID, satelliteID)
	if err != nil {
		controller.serveError(w, http.StatusInternalServerError, ErrPayouts.Wrap(err))
		return
	}

	if err = json.NewEncoder(w).Encode(paystub); err != nil {
		controller.log.Error("failed to write json response", zap.Error(err))
		return
	}
}

// PaystubPeriod returns all satellites summed paystubs for period.
func (controller *Payouts) PaystubPeriod(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Add("Content-Type", "application/json")
	segmentParams := mux.Vars(r)

	nodeIDstring, ok := segmentParams["nodeID"]
	if !ok {
		controller.serveError(w, http.StatusBadRequest, ErrPayouts.New("couldn't receive route variable nodeID"))
		return
	}

	nodeID, err := storj.NodeIDFromString(nodeIDstring)
	if err != nil {
		controller.serveError(w, http.StatusBadRequest, ErrPayouts.Wrap(err))
		return
	}

	period, ok := segmentParams["period"]
	if !ok {
		controller.serveError(w, http.StatusBadRequest, ErrPayouts.New("couldn't receive route variable period"))
		return
	}

	paystub, err := controller.service.PaystubPeriod(ctx, period, nodeID)
	if err != nil {
		controller.serveError(w, http.StatusInternalServerError, ErrPayouts.Wrap(err))
		return
	}

	if err = json.NewEncoder(w).Encode(paystub); err != nil {
		controller.log.Error("failed to write json response", zap.Error(err))
		return
	}
}

// HeldAmountSummary handles retrieving held amount history for a node.
func (controller *Payouts) HeldAmountSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	vars := mux.Vars(r)

	idString, ok := vars["nodeID"]
	if !ok {
		controller.serveError(w, http.StatusBadRequest, ErrPayouts.New("node id segment parameter is missing"))
		return
	}
	nodeID, err := storj.NodeIDFromString(idString)
	if err != nil {
		controller.serveError(w, http.StatusBadRequest, ErrPayouts.Wrap(err))
		return
	}

	heldSummary, err := controller.service.HeldAmountSummary(ctx, nodeID)
	if err != nil {
		if nodes.ErrNoNode.Has(err) {
			controller.serveError(w, http.StatusNotFound, ErrPayouts.Wrap(err))
			return
		}

		controller.log.Error("held amount history internal error", zap.Error(ErrPayouts.Wrap(err)))
		controller.serveError(w, http.StatusInternalServerError, ErrPayouts.Wrap(err))
		return
	}

	if err = json.NewEncoder(w).Encode(heldSummary); err != nil {
		controller.log.Error("failed to write json response", zap.Error(ErrPayouts.Wrap(err)))
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
