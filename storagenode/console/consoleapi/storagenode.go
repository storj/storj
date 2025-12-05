// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/storagenode/console"
)

// ErrStorageNodeAPI - console storagenode api error type.
var ErrStorageNodeAPI = errs.Class("consoleapi storagenode")

// StorageNode is an api controller that exposes all dashboard related api.
type StorageNode struct {
	service *console.Service

	log *zap.Logger
}

// NewStorageNode is a constructor for sno controller.
func NewStorageNode(log *zap.Logger, service *console.Service) *StorageNode {
	return &StorageNode{
		log:     log,
		service: service,
	}
}

// StorageNode handles StorageNode API requests.
func (dashboard *StorageNode) StorageNode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set(contentType, applicationJSON)

	data, err := dashboard.service.GetDashboardData(ctx)
	if err != nil {
		dashboard.serveJSONError(w, http.StatusInternalServerError, ErrStorageNodeAPI.Wrap(err))
		return
	}

	if err := json.NewEncoder(w).Encode(data); err != nil {
		dashboard.log.Error("failed to encode json response", zap.Error(ErrStorageNodeAPI.Wrap(err)))
		return
	}
}

// Satellites handles satellites API request.
func (dashboard *StorageNode) Satellites(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set(contentType, applicationJSON)

	data, err := dashboard.service.GetAllSatellitesData(ctx)
	if err != nil {
		dashboard.serveJSONError(w, http.StatusInternalServerError, ErrStorageNodeAPI.Wrap(err))
		return
	}

	if err := json.NewEncoder(w).Encode(data); err != nil {
		dashboard.log.Error("failed to encode json response", zap.Error(ErrStorageNodeAPI.Wrap(err)))
		return
	}
}

// Satellite handles satellite API requests.
func (dashboard *StorageNode) Satellite(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set(contentType, applicationJSON)

	params := mux.Vars(r)
	id, ok := params["id"]
	if !ok {
		dashboard.serveJSONError(w, http.StatusBadRequest, ErrStorageNodeAPI.Wrap(err))
		return
	}

	satelliteID, err := storj.NodeIDFromString(id)
	if err != nil {
		dashboard.serveJSONError(w, http.StatusBadRequest, ErrStorageNodeAPI.Wrap(err))
		return
	}

	if err = dashboard.service.VerifySatelliteID(ctx, satelliteID); err != nil {
		dashboard.serveJSONError(w, http.StatusNotFound, ErrStorageNodeAPI.Wrap(err))
		return
	}

	data, err := dashboard.service.GetSatelliteData(ctx, satelliteID)
	if err != nil {
		dashboard.serveJSONError(w, http.StatusInternalServerError, ErrStorageNodeAPI.Wrap(err))
		return
	}

	if err := json.NewEncoder(w).Encode(data); err != nil {
		dashboard.log.Error("failed to encode json response", zap.Error(ErrStorageNodeAPI.Wrap(err)))
		return
	}
}

// EstimatedPayout returns estimated payouts from specific satellite or all satellites if current traffic level remains same.
func (dashboard *StorageNode) EstimatedPayout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set(contentType, applicationJSON)

	now := time.Now()

	queryParams := r.URL.Query()
	id := queryParams.Get("id")
	if id == "" {
		data, err := dashboard.service.GetAllSatellitesEstimatedPayout(ctx, now)
		if err != nil {
			dashboard.serveJSONError(w, http.StatusInternalServerError, ErrStorageNodeAPI.Wrap(err))
			return
		}

		if err := json.NewEncoder(w).Encode(data); err != nil {
			dashboard.log.Error("failed to encode json response", zap.Error(ErrPayoutAPI.Wrap(err)))
			return
		}
	} else {
		satelliteID, err := storj.NodeIDFromString(id)
		if err != nil {
			dashboard.serveJSONError(w, http.StatusBadRequest, ErrPayoutAPI.Wrap(err))
			return
		}

		data, err := dashboard.service.GetSatelliteEstimatedPayout(ctx, satelliteID, now)
		if err != nil {
			dashboard.serveJSONError(w, http.StatusInternalServerError, ErrStorageNodeAPI.Wrap(err))
			return
		}

		if err := json.NewEncoder(w).Encode(data); err != nil {
			dashboard.log.Error("failed to encode json response", zap.Error(ErrPayoutAPI.Wrap(err)))
			return
		}
	}
}

// Pricing returns pricing model for specific satellite.
func (dashboard *StorageNode) Pricing(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set(contentType, applicationJSON)

	params := mux.Vars(r)
	id, ok := params["id"]
	if !ok {
		dashboard.serveJSONError(w, http.StatusInternalServerError, ErrStorageNodeAPI.Wrap(err))
		return
	}
	satelliteID, err := storj.NodeIDFromString(id)
	if err != nil {
		dashboard.serveJSONError(w, http.StatusBadRequest, ErrStorageNodeAPI.Wrap(err))
		return
	}

	data, err := dashboard.service.GetSatellitePricingModel(ctx, satelliteID)
	if err != nil {
		dashboard.serveJSONError(w, http.StatusInternalServerError, ErrStorageNodeAPI.Wrap(err))
		return
	}

	if err := json.NewEncoder(w).Encode(data); err != nil {
		dashboard.log.Error("failed to encode json response", zap.Error(ErrStorageNodeAPI.Wrap(err)))
		return
	}
}

// serveJSONError writes JSON error to response output stream.
func (dashboard *StorageNode) serveJSONError(w http.ResponseWriter, status int, err error) {
	w.WriteHeader(status)

	var response struct {
		Error string `json:"error"`
	}

	response.Error = err.Error()

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		dashboard.log.Error("failed to write json error response", zap.Error(ErrStorageNodeAPI.Wrap(err)))
		return
	}
}
