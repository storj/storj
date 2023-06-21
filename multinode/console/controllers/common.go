// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package controllers

import (
	"encoding/json"
	"net/http"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"
)

var (
	mon = monkit.Package()
)

// NotFound handles API response for not found routes.
type NotFound struct {
	log *zap.Logger
}

// NewNotFound creates new instance of NotFound handler.
func NewNotFound(log *zap.Logger) http.Handler {
	return &NotFound{
		log: log,
	}
}

// ServeHTTP serves 404 response with json error when resource is not found.
func (handler *NotFound) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)

	var response struct {
		Error string `json:"error"`
	}

	response.Error = "resource not found"

	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		handler.log.Error("failed to write json error response", zap.Error(err))
	}
}
