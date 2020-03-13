// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleheldamount

import (
	"encoding/json"
	"net/http"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/heldamount"
)

const (
	contentType = "Content-Type"

	applicationJSON = "application/json"
)

var mon = monkit.Package()

// Error is error type of storagenode web console.
var Error = errs.Class("heldamount console web error")

// HeldAmount represents heldmount service.
// architecture: Service
type HeldAmount struct {
	service *heldamount.Service

	log *zap.Logger
}

// jsonOutput defines json structure of api response data.
type jsonOutput struct {
	Data  interface{} `json:"data"`
	Error string      `json:"error"`
}

// NewHeldAmount creates new instance of heldamount service.
func NewHeldAmount(log *zap.Logger, service *heldamount.Service) *HeldAmount {
	return &HeldAmount{
		log:     log,
		service: service,
	}
}

// GetMonthlyHeldAmount returns heldamount, storage holding and prices data for specific month from satellite.
func (heldamount *HeldAmount) GetMonthlyHeldAmount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer mon.Task()(&ctx)(nil)

	period := r.URL.Query().Get("period")
	id := r.URL.Query().Get("satelliteID")

	satelliteID, err := storj.NodeIDFromString(id)
	if err != nil {
		heldamount.writeError(w, http.StatusBadRequest, Error.Wrap(err))
		return
	}

	paystubData, err := heldamount.service.GetPaystubStats(ctx, satelliteID, period)
	if err != nil {
		heldamount.writeError(w, http.StatusInternalServerError, Error.Wrap(err))
		return
	}

	heldamount.writeData(w, paystubData)
}

// writeData is helper method to write JSON to http.ResponseWriter and log encoding error.
func (heldamount *HeldAmount) writeData(w http.ResponseWriter, data interface{}) {
	w.Header().Set(contentType, applicationJSON)
	w.WriteHeader(http.StatusOK)

	output := jsonOutput{Data: data}

	if err := json.NewEncoder(w).Encode(output); err != nil {
		heldamount.log.Error("json encoder error", zap.Error(err))
	}
}

// writeError writes a JSON error payload to http.ResponseWriter log encoding error.
func (heldamount *HeldAmount) writeError(w http.ResponseWriter, status int, err error) {
	if status >= http.StatusInternalServerError {
		heldamount.log.Error("api handler server error", zap.Int("status code", status), zap.Error(err))
	}

	w.Header().Set(contentType, applicationJSON)
	w.WriteHeader(status)

	output := jsonOutput{Error: err.Error()}

	if err := json.NewEncoder(w).Encode(output); err != nil {
		heldamount.log.Error("json encoder error", zap.Error(err))
	}
}
