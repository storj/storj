// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi

import (
	"encoding/json"
	"net/http"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/heldamount"
)

// ErrHeldAmountPI - console heldAmount api error type.
var ErrHeldAmountPI = errs.Class("heldAmount console web error")

// HeldAmount is an api controller that exposes all held amount related api.
type HeldAmount struct {
	service *heldamount.Service

	log *zap.Logger
}

// NewHeldAmount is a constructor for heldAmount controller.
func NewHeldAmount(log *zap.Logger, service *heldamount.Service) *HeldAmount {
	return &HeldAmount{
		log:     log,
		service: service,
	}
}

// GetMonthlyHeldAmount returns heldamount, storage holding and prices data for specific month from satellite.
func (heldamount *HeldAmount) GetMonthlyHeldAmount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set(contentType, applicationJSON)

	period := r.URL.Query().Get("period")
	id := r.URL.Query().Get("satelliteID")

	satelliteID, err := storj.NodeIDFromString(id)
	if err != nil {
		heldamount.serveJSONError(w, http.StatusBadRequest, ErrHeldAmountPI.Wrap(err))
		return
	}

	paystubData, err := heldamount.service.GetPaystubStatsCached(ctx, satelliteID, period)
	if err != nil {
		heldamount.serveJSONError(w, http.StatusInternalServerError, ErrHeldAmountPI.Wrap(err))
		return
	}

	if err := json.NewEncoder(w).Encode(paystubData); err != nil {
		heldamount.log.Error("failed to encode json response", zap.Error(ErrHeldAmountPI.Wrap(err)))
		return
	}
}

// GetMonthlyPayment returns payment data from satellite for specific month.
func (heldamount *HeldAmount) GetMonthlyPayment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set(contentType, applicationJSON)

	period := r.URL.Query().Get("period")
	id := r.URL.Query().Get("satelliteID")

	satelliteID, err := storj.NodeIDFromString(id)
	if err != nil {
		heldamount.serveJSONError(w, http.StatusBadRequest, ErrHeldAmountPI.Wrap(err))
		return
	}

	paymentData, err := heldamount.service.GetPaymentCached(ctx, satelliteID, period)
	if err != nil {
		heldamount.serveJSONError(w, http.StatusInternalServerError, ErrHeldAmountPI.Wrap(err))
		return
	}

	if err := json.NewEncoder(w).Encode(paymentData); err != nil {
		heldamount.log.Error("failed to encode json response", zap.Error(ErrHeldAmountPI.Wrap(err)))
		return
	}
}

// serveJSONError writes JSON error to response output stream.
func (heldamount *HeldAmount) serveJSONError(w http.ResponseWriter, status int, err error) {
	w.WriteHeader(status)

	var response struct {
		Error string `json:"error"`
	}

	response.Error = err.Error()

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		heldamount.log.Error("failed to write json error response", zap.Error(ErrHeldAmountPI.Wrap(err)))
		return
	}
}
