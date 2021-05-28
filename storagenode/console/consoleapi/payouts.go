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
	"storj.io/storj/storagenode/payouts"
)

// ErrPayoutAPI - console payouts api error type.
var ErrPayoutAPI = errs.Class("consoleapi payouts")

// Payout is an api controller that exposes all payouts related api.
type Payout struct {
	service *payouts.Service

	log *zap.Logger
}

// NewPayout is a constructor for payouts controller.
func NewPayout(log *zap.Logger, service *payouts.Service) *Payout {
	return &Payout{
		log:     log,
		service: service,
	}
}

// PayStubMonthly returns payouts, storage holding and prices data for specific month from all satellites or specified satellite by query parameter id.
func (payout *Payout) PayStubMonthly(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set(contentType, applicationJSON)

	segmentParams := mux.Vars(r)
	queryParams := r.URL.Query()

	period, ok := segmentParams["period"]
	if !ok {
		payout.serveJSONError(w, http.StatusBadRequest, ErrNotificationsAPI.Wrap(err))
		return
	}

	id := queryParams.Get("id")
	if id == "" {
		payStubs, err := payout.service.AllPayStubsMonthly(ctx, period)
		if err != nil {
			payout.serveJSONError(w, http.StatusInternalServerError, ErrPayoutAPI.Wrap(err))
			return
		}

		if err := json.NewEncoder(w).Encode(payStubs); err != nil {
			payout.log.Error("failed to encode json response", zap.Error(ErrPayoutAPI.Wrap(err)))
			return
		}
	} else {
		satelliteID, err := storj.NodeIDFromString(id)
		if err != nil {
			payout.serveJSONError(w, http.StatusBadRequest, ErrPayoutAPI.Wrap(err))
			return
		}

		payStub, err := payout.service.SatellitePayStubMonthly(ctx, satelliteID, period)
		if err != nil {
			if payouts.ErrNoPayStubForPeriod.Has(err) {
				payout.serveJSONError(w, http.StatusNotFound, ErrPayoutAPI.Wrap(err))
				return
			}

			payout.serveJSONError(w, http.StatusInternalServerError, ErrPayoutAPI.Wrap(err))
			return
		}

		if err := json.NewEncoder(w).Encode(payStub); err != nil {
			payout.log.Error("failed to encode json response", zap.Error(ErrPayoutAPI.Wrap(err)))
			return
		}
	}
}

// PayStubPeriod retrieves paystubs for selected range of months from storagenode database for all satellites or specified satellite by query parameter id.
func (payout *Payout) PayStubPeriod(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set(contentType, applicationJSON)

	segmentParams := mux.Vars(r)
	queryParams := r.URL.Query()

	start, ok := segmentParams["start"]
	if !ok {
		payout.serveJSONError(w, http.StatusBadRequest, ErrNotificationsAPI.Wrap(err))
		return
	}

	end, ok := segmentParams["end"]
	if !ok {
		payout.serveJSONError(w, http.StatusBadRequest, ErrNotificationsAPI.Wrap(err))
		return
	}

	id := queryParams.Get("id")
	if id == "" {
		payStubs, err := payout.service.AllPayStubsPeriod(ctx, start, end)
		if err != nil {
			if payouts.ErrBadPeriod.Has(err) {
				payout.serveJSONError(w, http.StatusBadRequest, ErrPayoutAPI.Wrap(err))
				return
			}

			payout.serveJSONError(w, http.StatusInternalServerError, ErrPayoutAPI.Wrap(err))
			return
		}

		if err := json.NewEncoder(w).Encode(payStubs); err != nil {
			payout.log.Error("failed to encode json response", zap.Error(ErrPayoutAPI.Wrap(err)))
			return
		}
	} else {
		satelliteID, err := storj.NodeIDFromString(id)
		if err != nil {
			payout.serveJSONError(w, http.StatusBadRequest, ErrPayoutAPI.Wrap(err))
			return
		}

		payStubs, err := payout.service.SatellitePayStubPeriod(ctx, satelliteID, start, end)
		if err != nil {
			if payouts.ErrBadPeriod.Has(err) {
				payout.serveJSONError(w, http.StatusBadRequest, ErrPayoutAPI.Wrap(err))
				return
			}

			payout.serveJSONError(w, http.StatusInternalServerError, ErrPayoutAPI.Wrap(err))
			return
		}

		if err := json.NewEncoder(w).Encode(payStubs); err != nil {
			payout.log.Error("failed to encode json response", zap.Error(ErrPayoutAPI.Wrap(err)))
			return
		}
	}
}

// HeldHistory returns held amount for each % period for all satellites.
func (payout *Payout) HeldHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set(contentType, applicationJSON)

	heldbackHistory, err := payout.service.AllHeldbackHistory(ctx)
	if err != nil {
		payout.serveJSONError(w, http.StatusInternalServerError, ErrPayoutAPI.Wrap(err))
		return
	}

	if err := json.NewEncoder(w).Encode(heldbackHistory); err != nil {
		payout.log.Error("failed to encode json response", zap.Error(ErrPayoutAPI.Wrap(err)))
		return
	}
}

// PayoutHistory retrieves paystubs for specific period from all satellites and transaction receipts if exists.
func (payout *Payout) PayoutHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set(contentType, applicationJSON)

	segmentParams := mux.Vars(r)

	period, ok := segmentParams["period"]
	if !ok {
		payout.serveJSONError(w, http.StatusBadRequest, ErrNotificationsAPI.Wrap(err))
		return
	}

	payoutHistory, err := payout.service.AllSatellitesPayoutPeriod(ctx, period)
	if err != nil {
		payout.serveJSONError(w, http.StatusInternalServerError, ErrPayoutAPI.Wrap(err))
		return
	}

	if err := json.NewEncoder(w).Encode(payoutHistory); err != nil {
		payout.log.Error("failed to encode json response", zap.Error(ErrPayoutAPI.Wrap(err)))
		return
	}
}

// HeldAmountPeriods retrieves all periods in which we have some payouts data.
// Have optional parameter - satelliteID.
// If satelliteID specified - will retrieve periods only for concrete satellite.
func (payout *Payout) HeldAmountPeriods(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set(contentType, applicationJSON)

	queryParams := r.URL.Query()

	id := queryParams.Get("id")
	if id == "" {
		payStubs, err := payout.service.AllPeriods(ctx)
		if err != nil {
			payout.serveJSONError(w, http.StatusInternalServerError, ErrPayoutAPI.Wrap(err))
			return
		}

		if err := json.NewEncoder(w).Encode(payStubs); err != nil {
			payout.log.Error("failed to encode json response", zap.Error(ErrPayoutAPI.Wrap(err)))
			return
		}
	} else {
		satelliteID, err := storj.NodeIDFromString(id)
		if err != nil {
			payout.serveJSONError(w, http.StatusBadRequest, ErrPayoutAPI.Wrap(err))
			return
		}

		payStubs, err := payout.service.SatellitePeriods(ctx, satelliteID)
		if err != nil {
			payout.serveJSONError(w, http.StatusInternalServerError, ErrPayoutAPI.Wrap(err))
			return
		}

		if err := json.NewEncoder(w).Encode(payStubs); err != nil {
			payout.log.Error("failed to encode json response", zap.Error(ErrPayoutAPI.Wrap(err)))
			return
		}
	}
}

// serveJSONError writes JSON error to response output stream.
func (payout *Payout) serveJSONError(w http.ResponseWriter, status int, err error) {
	w.WriteHeader(status)

	var response struct {
		Error string `json:"error"`
	}

	response.Error = err.Error()

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		payout.log.Error("failed to write json error response", zap.Error(ErrPayoutAPI.Wrap(err)))
		return
	}
}
