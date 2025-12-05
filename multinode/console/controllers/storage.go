// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package controllers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/multinode/nodes"
	"storj.io/storj/multinode/storage"
	"storj.io/storj/private/compensation"
)

var (
	// ErrStorage is an internal error type for storage web api controller.
	ErrStorage = errs.Class("storage web api controller")
)

// Storage is a storage web api controller.
type Storage struct {
	log     *zap.Logger
	service *storage.Service
}

// NewStorage is a constructor of Storage controller.
func NewStorage(log *zap.Logger, service *storage.Service) *Storage {
	return &Storage{
		log:     log,
		service: service,
	}
}

// Usage handles retrieval of a node storage usage for a period interval.
func (controller *Storage) Usage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Add("Content-Type", "application/json")
	segments := mux.Vars(r)

	nodeIDEnc, ok := segments["nodeID"]
	if !ok {
		controller.serveError(w, http.StatusBadRequest, ErrStorage.New("could not receive node id segment"))
		return
	}
	nodeID, err := storj.NodeIDFromString(nodeIDEnc)
	if err != nil {
		controller.serveError(w, http.StatusBadRequest, ErrStorage.Wrap(err))
		return
	}

	var from time.Time
	to := time.Now()

	if periodParam := r.URL.Query().Get("period"); periodParam != "" {
		period, err := compensation.PeriodFromString(periodParam)
		if err != nil {
			controller.serveError(w, http.StatusBadRequest, ErrStorage.Wrap(err))
			return
		}

		from = period.StartDate()
		to = period.EndDateExclusive()
	}

	usage, err := controller.service.Usage(ctx, nodeID, from, to)
	if err != nil {
		if nodes.ErrNoNode.Has(err) {
			controller.serveError(w, http.StatusNotFound, ErrStorage.Wrap(err))
			return
		}

		controller.log.Error("usage internal error", zap.Error(ErrStorage.Wrap(err)))
		controller.serveError(w, http.StatusInternalServerError, ErrStorage.Wrap(err))
		return
	}

	// return empty slice instead of nil
	if usage.Stamps == nil {
		usage.Stamps = make([]storage.UsageStamp, 0)
	}
	if err = json.NewEncoder(w).Encode(usage); err != nil {
		controller.log.Error("failed to write json response", zap.Error(ErrStorage.Wrap(err)))
		return
	}
}

// UsageSatellite handles retrieval of a node storage usage for a satellite and period interval.
func (controller *Storage) UsageSatellite(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Add("Content-Type", "application/json")
	segments := mux.Vars(r)

	nodeIDEnc, ok := segments["nodeID"]
	if !ok {
		controller.serveError(w, http.StatusBadRequest, ErrStorage.New("could not receive node id segment"))
		return
	}
	satelliteIDEnc, ok := segments["satelliteID"]
	if !ok {
		controller.serveError(w, http.StatusBadRequest, ErrStorage.New("could not receive satellite id segment"))
		return
	}

	nodeID, err := storj.NodeIDFromString(nodeIDEnc)
	if err != nil {
		controller.serveError(w, http.StatusBadRequest, ErrStorage.Wrap(err))
		return
	}
	satelliteID, err := storj.NodeIDFromString(satelliteIDEnc)
	if err != nil {
		controller.serveError(w, http.StatusBadRequest, ErrStorage.Wrap(err))
		return
	}

	var from time.Time
	to := time.Now()

	if periodParam := r.URL.Query().Get("period"); periodParam != "" {
		period, err := compensation.PeriodFromString(periodParam)
		if err != nil {
			controller.serveError(w, http.StatusBadRequest, ErrStorage.Wrap(err))
			return
		}

		from = period.StartDate()
		to = period.EndDateExclusive()
	}

	usage, err := controller.service.UsageSatellite(ctx, nodeID, satelliteID, from, to)
	if err != nil {
		if nodes.ErrNoNode.Has(err) {
			controller.serveError(w, http.StatusNotFound, ErrStorage.Wrap(err))
			return
		}

		controller.log.Error("usage satellite internal error", zap.Error(ErrStorage.Wrap(err)))
		controller.serveError(w, http.StatusInternalServerError, ErrStorage.Wrap(err))
		return
	}

	// return empty slice instead of nil
	if usage.Stamps == nil {
		usage.Stamps = make([]storage.UsageStamp, 0)
	}
	if err = json.NewEncoder(w).Encode(usage); err != nil {
		controller.log.Error("failed to write json response", zap.Error(ErrStorage.Wrap(err)))
		return
	}
}

// TotalUsage handles retrieval of aggregated storage usage for a period interval.
func (controller *Storage) TotalUsage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Add("Content-Type", "application/json")

	var from time.Time
	to := time.Now()

	if periodParam := r.URL.Query().Get("period"); periodParam != "" {
		period, err := compensation.PeriodFromString(periodParam)
		if err != nil {
			controller.serveError(w, http.StatusBadRequest, ErrStorage.Wrap(err))
			return
		}

		from = period.StartDate()
		to = period.EndDateExclusive()
	}

	usage, err := controller.service.TotalUsage(ctx, from, to)
	if err != nil {
		if nodes.ErrNoNode.Has(err) {
			controller.serveError(w, http.StatusNotFound, ErrStorage.Wrap(err))
			return
		}

		controller.log.Error("total usage internal error", zap.Error(ErrStorage.Wrap(err)))
		controller.serveError(w, http.StatusInternalServerError, ErrStorage.Wrap(err))
		return
	}

	// return empty slice instead of nil
	if usage.Stamps == nil {
		usage.Stamps = make([]storage.UsageStamp, 0)
	}
	if err = json.NewEncoder(w).Encode(usage); err != nil {
		controller.log.Error("failed to write json response", zap.Error(ErrStorage.Wrap(err)))
		return
	}
}

// TotalUsageSatellite handles retrieval of aggregated storage usage for a satellite and period interval.
func (controller *Storage) TotalUsageSatellite(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Add("Content-Type", "application/json")
	segments := mux.Vars(r)

	satelliteIDEnc, ok := segments["satelliteID"]
	if !ok {
		controller.serveError(w, http.StatusBadRequest, ErrStorage.New("could not receive satellite id segment"))
		return
	}
	satelliteID, err := storj.NodeIDFromString(satelliteIDEnc)
	if err != nil {
		controller.serveError(w, http.StatusBadRequest, ErrStorage.Wrap(err))
		return
	}

	var from time.Time
	to := time.Now()

	if periodParam := r.URL.Query().Get("period"); periodParam != "" {
		period, err := compensation.PeriodFromString(periodParam)
		if err != nil {
			controller.serveError(w, http.StatusBadRequest, ErrStorage.Wrap(err))
			return
		}

		from = period.StartDate()
		to = period.EndDateExclusive()
	}

	usage, err := controller.service.TotalUsageSatellite(ctx, satelliteID, from, to)
	if err != nil {
		if nodes.ErrNoNode.Has(err) {
			controller.serveError(w, http.StatusNotFound, ErrStorage.Wrap(err))
			return
		}

		controller.log.Error("usage satellite internal error", zap.Error(ErrStorage.Wrap(err)))
		controller.serveError(w, http.StatusInternalServerError, ErrStorage.Wrap(err))
		return
	}

	// return empty slice instead of nil
	if usage.Stamps == nil {
		usage.Stamps = make([]storage.UsageStamp, 0)
	}
	if err = json.NewEncoder(w).Encode(usage); err != nil {
		controller.log.Error("failed to write json response", zap.Error(ErrStorage.Wrap(err)))
		return
	}
}

// TotalDiskSpace returns all info about all storagenodes disk space usage.
func (controller *Storage) TotalDiskSpace(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Add("Content-Type", "application/json")

	totalDiskSpace, err := controller.service.TotalDiskSpace(ctx)
	if err != nil {
		controller.log.Error("could not get total disk space", zap.Error(err))
		controller.serveError(w, http.StatusInternalServerError, ErrStorage.Wrap(err))
		return
	}

	if err = json.NewEncoder(w).Encode(totalDiskSpace); err != nil {
		controller.log.Error("failed to write json response", zap.Error(err))
		return
	}
}

// DiskSpace returns all info about concrete storagenode disk space usage.
func (controller *Storage) DiskSpace(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Add("Content-Type", "application/json")
	segments := mux.Vars(r)

	nodeIDparam, ok := segments["nodeID"]
	if !ok {
		controller.serveError(w, http.StatusBadRequest, ErrStorage.New("node id is missing"))
		return
	}
	nodeID, err := storj.NodeIDFromString(nodeIDparam)
	if err != nil {
		controller.serveError(w, http.StatusBadRequest, ErrStorage.Wrap(err))
		return
	}

	diskSpace, err := controller.service.DiskSpace(ctx, nodeID)
	if err != nil {
		if nodes.ErrNoNode.Has(err) {
			controller.serveError(w, http.StatusNotFound, ErrStorage.Wrap(err))
			return
		}

		controller.log.Error("could not get disk space", zap.Error(err))
		controller.serveError(w, http.StatusInternalServerError, ErrStorage.Wrap(err))
		return
	}

	if err = json.NewEncoder(w).Encode(diskSpace); err != nil {
		controller.log.Error("failed to write json response", zap.Error(err))
		return
	}
}

// serveError set http statuses and send json error.
func (controller *Storage) serveError(w http.ResponseWriter, status int, err error) {
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
