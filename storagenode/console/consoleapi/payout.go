// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/storagenode/payout"
)

// ErrPayoutAPI - console payout api error type.
var ErrPayoutAPI = errs.Class("payout console web error")

// Payout is an api controller that exposes all payout related api.
type Payout struct {
	service *payout.Service

	log *zap.Logger
}

// NewPayout is a constructor for payout controller.
func NewPayout(log *zap.Logger, service *payout.Service) *Payout {
	return &Payout{
		log:     log,
		service: service,
	}
}

// PayStubMonthly returns payout, storage holding and prices data for specific month from all satellites or specified satellite by query parameter id.
func (payouts *Payout) PayStubMonthly(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set(contentType, applicationJSON)

	segmentParams := mux.Vars(r)
	queryParams := r.URL.Query()

	period, ok := segmentParams["period"]
	if !ok {
		payouts.serveJSONError(w, http.StatusBadRequest, ErrNotificationsAPI.Wrap(err))
		return
	}

	id := queryParams.Get("id")
	if id == "" {
		payStubs, err := payouts.service.AllPayStubsMonthly(ctx, period)
		if err != nil {
			payouts.serveJSONError(w, http.StatusInternalServerError, ErrPayoutAPI.Wrap(err))
			return
		}

		if err := json.NewEncoder(w).Encode(payStubs); err != nil {
			payouts.log.Error("failed to encode json response", zap.Error(ErrPayoutAPI.Wrap(err)))
			return
		}
	} else {
		satelliteID, err := storj.NodeIDFromString(id)
		if err != nil {
			payouts.serveJSONError(w, http.StatusBadRequest, ErrPayoutAPI.Wrap(err))
			return
		}

		payStub, err := payouts.service.SatellitePayStubMonthly(ctx, satelliteID, period)
		if err != nil {
			if payout.ErrNoPayStubForPeriod.Has(err) {
				payouts.serveJSONError(w, http.StatusNotFound, ErrPayoutAPI.Wrap(err))
				return
			}

			payouts.serveJSONError(w, http.StatusInternalServerError, ErrPayoutAPI.Wrap(err))
			return
		}

		if err := json.NewEncoder(w).Encode([]*payout.PayStub{payStub}); err != nil {
			payouts.log.Error("failed to encode json response", zap.Error(ErrPayoutAPI.Wrap(err)))
			return
		}
	}
}

// PayStubPeriod retrieves paystubs for selected range of months from storagenode database for all satellites or specified satellite by query parameter id.
func (payouts *Payout) PayStubPeriod(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set(contentType, applicationJSON)

	segmentParams := mux.Vars(r)
	queryParams := r.URL.Query()

	start, ok := segmentParams["start"]
	if !ok {
		payouts.serveJSONError(w, http.StatusBadRequest, ErrNotificationsAPI.Wrap(err))
		return
	}

	end, ok := segmentParams["end"]
	if !ok {
		payouts.serveJSONError(w, http.StatusBadRequest, ErrNotificationsAPI.Wrap(err))
		return
	}

	id := queryParams.Get("id")
	if id == "" {
		payStubs, err := payouts.service.AllPayStubsPeriod(ctx, start, end)
		if err != nil {
			if payout.ErrBadPeriod.Has(err) {
				payouts.serveJSONError(w, http.StatusBadRequest, ErrPayoutAPI.Wrap(err))
				return
			}

			payouts.serveJSONError(w, http.StatusInternalServerError, ErrPayoutAPI.Wrap(err))
			return
		}

		if err := json.NewEncoder(w).Encode(payStubs); err != nil {
			payouts.log.Error("failed to encode json response", zap.Error(ErrPayoutAPI.Wrap(err)))
			return
		}
	} else {
		satelliteID, err := storj.NodeIDFromString(id)
		if err != nil {
			payouts.serveJSONError(w, http.StatusBadRequest, ErrPayoutAPI.Wrap(err))
			return
		}

		payStubs, err := payouts.service.SatellitePayStubPeriod(ctx, satelliteID, start, end)
		if err != nil {
			if payout.ErrBadPeriod.Has(err) {
				payouts.serveJSONError(w, http.StatusBadRequest, ErrPayoutAPI.Wrap(err))
				return
			}

			payouts.serveJSONError(w, http.StatusInternalServerError, ErrPayoutAPI.Wrap(err))
			return
		}

		if err := json.NewEncoder(w).Encode(payStubs); err != nil {
			payouts.log.Error("failed to encode json response", zap.Error(ErrPayoutAPI.Wrap(err)))
			return
		}
	}
}

// HeldHistory returns held amount for each % period for all satellites.
func (payouts *Payout) HeldHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set(contentType, applicationJSON)

	heldbackHistory, err := payouts.service.AllHeldbackHistory(ctx)
	if err != nil {
		payouts.serveJSONError(w, http.StatusInternalServerError, ErrPayoutAPI.Wrap(err))
		return
	}

	if err := json.NewEncoder(w).Encode(heldbackHistory); err != nil {
		payouts.log.Error("failed to encode json response", zap.Error(ErrPayoutAPI.Wrap(err)))
		return
	}
}

// PayoutHistory retrieves paystubs for specific period from all satellites and transaction receipts if exists.
func (payouts *Payout) PayoutHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set(contentType, applicationJSON)

	segmentParams := mux.Vars(r)

	period, ok := segmentParams["period"]
	if !ok {
		payouts.serveJSONError(w, http.StatusBadRequest, ErrNotificationsAPI.Wrap(err))
		return
	}

	payoutHistory, err := payouts.service.AllSatellitesPayoutPeriod(ctx, period)
	if err != nil {
		payouts.serveJSONError(w, http.StatusInternalServerError, ErrPayoutAPI.Wrap(err))
		return
	}

	if err := json.NewEncoder(w).Encode(payoutHistory); err != nil {
		payouts.log.Error("failed to encode json response", zap.Error(ErrPayoutAPI.Wrap(err)))
		return
	}
}

// HeldAmountPeriods retrieves all periods in which we have some payout data.
// Have optional parameter - satelliteID.
// If satelliteID specified - will retrieve periods only for concrete satellite.
func (payouts *Payout) HeldAmountPeriods(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set(contentType, applicationJSON)

	queryParams := r.URL.Query()

	id := queryParams.Get("id")
	if id == "" {
		payStubs, err := payouts.service.AllPeriods(ctx)
		if err != nil {
			payouts.serveJSONError(w, http.StatusInternalServerError, ErrPayoutAPI.Wrap(err))
			return
		}

		if err := json.NewEncoder(w).Encode(payStubs); err != nil {
			payouts.log.Error("failed to encode json response", zap.Error(ErrPayoutAPI.Wrap(err)))
			return
		}
	} else {
		satelliteID, err := storj.NodeIDFromString(id)
		if err != nil {
			payouts.serveJSONError(w, http.StatusBadRequest, ErrPayoutAPI.Wrap(err))
			return
		}

		payStubs, err := payouts.service.SatellitePeriods(ctx, satelliteID)
		if err != nil {
			payouts.serveJSONError(w, http.StatusInternalServerError, ErrPayoutAPI.Wrap(err))
			return
		}

		if err := json.NewEncoder(w).Encode(payStubs); err != nil {
			payouts.log.Error("failed to encode json response", zap.Error(ErrPayoutAPI.Wrap(err)))
			return
		}
	}
}

// serveJSONError writes JSON error to response output stream.
func (payouts *Payout) serveJSONError(w http.ResponseWriter, status int, err error) {
	w.WriteHeader(status)

	var response struct {
		Error string `json:"error"`
	}

	response.Error = err.Error()

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		payouts.log.Error("failed to write json error response", zap.Error(ErrPayoutAPI.Wrap(err)))
		return
	}
}
