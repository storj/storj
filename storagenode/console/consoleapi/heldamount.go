// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/heldamount"
)

// ErrHeldAmountAPI - console heldAmount api error type.
var ErrHeldAmountAPI = errs.Class("heldAmount console web error")

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

// PayStubMonthly returns heldamount, storage holding and prices data for specific month from all satellites or specified satellite by query parameter id.
func (heldAmount *HeldAmount) PayStubMonthly(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set(contentType, applicationJSON)

	segmentParams := mux.Vars(r)
	queryParams := r.URL.Query()

	period, ok := segmentParams["period"]
	if !ok {
		heldAmount.serveJSONError(w, http.StatusBadRequest, ErrNotificationsAPI.Wrap(err))
		return
	}

	id := queryParams.Get("id")
	if id == "" {
		payStubs, err := heldAmount.service.AllPayStubsMonthlyCached(ctx, period)
		if err != nil {
			heldAmount.serveJSONError(w, http.StatusInternalServerError, ErrHeldAmountAPI.Wrap(err))
			return
		}

		if err := json.NewEncoder(w).Encode(payStubs); err != nil {
			heldAmount.log.Error("failed to encode json response", zap.Error(ErrHeldAmountAPI.Wrap(err)))
			return
		}
	} else {
		satelliteID, err := storj.NodeIDFromString(id)
		if err != nil {
			heldAmount.serveJSONError(w, http.StatusBadRequest, ErrHeldAmountAPI.Wrap(err))
			return
		}

		payStub, err := heldAmount.service.SatellitePayStubMonthlyCached(ctx, satelliteID, period)
		if err != nil {
			if heldamount.ErrNoPayStubForPeriod.Has(err) {
				heldAmount.serveJSONError(w, http.StatusNotFound, ErrHeldAmountAPI.Wrap(err))
				return
			}

			heldAmount.serveJSONError(w, http.StatusInternalServerError, ErrHeldAmountAPI.Wrap(err))
			return
		}

		if err := json.NewEncoder(w).Encode([]*heldamount.PayStub{payStub}); err != nil {
			heldAmount.log.Error("failed to encode json response", zap.Error(ErrHeldAmountAPI.Wrap(err)))
			return
		}
	}
}

// PayStubPeriod retrieves held amount for selected range of months from storagenode database for all satellites or specified satellite by query parameter id.
func (heldAmount *HeldAmount) PayStubPeriod(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set(contentType, applicationJSON)

	segmentParams := mux.Vars(r)
	queryParams := r.URL.Query()

	start, ok := segmentParams["start"]
	if !ok {
		heldAmount.serveJSONError(w, http.StatusBadRequest, ErrNotificationsAPI.Wrap(err))
		return
	}

	end, ok := segmentParams["end"]
	if !ok {
		heldAmount.serveJSONError(w, http.StatusBadRequest, ErrNotificationsAPI.Wrap(err))
		return
	}

	id := queryParams.Get("id")
	if id == "" {
		payStubs, err := heldAmount.service.AllPayStubsPeriodCached(ctx, start, end)
		if err != nil {
			if heldamount.ErrBadPeriod.Has(err) {
				heldAmount.serveJSONError(w, http.StatusBadRequest, ErrHeldAmountAPI.Wrap(err))
				return
			}

			heldAmount.serveJSONError(w, http.StatusInternalServerError, ErrHeldAmountAPI.Wrap(err))
			return
		}

		if err := json.NewEncoder(w).Encode(payStubs); err != nil {
			heldAmount.log.Error("failed to encode json response", zap.Error(ErrHeldAmountAPI.Wrap(err)))
			return
		}
	} else {
		satelliteID, err := storj.NodeIDFromString(id)
		if err != nil {
			heldAmount.serveJSONError(w, http.StatusBadRequest, ErrHeldAmountAPI.Wrap(err))
			return
		}

		payStubs, err := heldAmount.service.SatellitePayStubPeriodCached(ctx, satelliteID, start, end)
		if err != nil {
			if heldamount.ErrBadPeriod.Has(err) {
				heldAmount.serveJSONError(w, http.StatusBadRequest, ErrHeldAmountAPI.Wrap(err))
				return
			}

			heldAmount.serveJSONError(w, http.StatusInternalServerError, ErrHeldAmountAPI.Wrap(err))
			return
		}

		if err := json.NewEncoder(w).Encode(payStubs); err != nil {
			heldAmount.log.Error("failed to encode json response", zap.Error(ErrHeldAmountAPI.Wrap(err)))
			return
		}
	}
}

// HeldbackHistory returns heldback for each % period for specific satellite.
func (heldAmount *HeldAmount) HeldbackHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set(contentType, applicationJSON)

	segmentParams := mux.Vars(r)
	id, ok := segmentParams["id"]
	if !ok {
		heldAmount.serveJSONError(w, http.StatusBadRequest, ErrNotificationsAPI.Wrap(err))
		return
	}

	satelliteID, err := storj.NodeIDFromString(id)
	if err != nil {
		heldAmount.serveJSONError(w, http.StatusBadRequest, ErrHeldAmountAPI.Wrap(err))
		return
	}

	heldbackHistory, err := heldAmount.service.AllHeldbackHistory(ctx, satelliteID)
	if err != nil {
		heldAmount.serveJSONError(w, http.StatusInternalServerError, ErrHeldAmountAPI.Wrap(err))
		return
	}

	if err := json.NewEncoder(w).Encode(heldbackHistory); err != nil {
		heldAmount.log.Error("failed to encode json response", zap.Error(ErrHeldAmountAPI.Wrap(err)))
		return
	}
}

// serveJSONError writes JSON error to response output stream.
func (heldAmount *HeldAmount) serveJSONError(w http.ResponseWriter, status int, err error) {
	w.WriteHeader(status)

	var response struct {
		Error string `json:"error"`
	}

	response.Error = err.Error()

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		heldAmount.log.Error("failed to write json error response", zap.Error(ErrHeldAmountAPI.Wrap(err)))
		return
	}
}
