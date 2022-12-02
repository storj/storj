// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package healthcheck

import (
	"encoding/json"
	"net/http"
)

// Endpoint handles HTTP request for health endpoint.
type Endpoint struct {
	service *Service
}

// NewEndpoint creates a new HTTP endpoint.
func NewEndpoint(service *Service) *Endpoint {
	return &Endpoint{
		service: service,
	}
}

// HandleHTTP manages the HTTP conversion for the function call.
func (e *Endpoint) HandleHTTP(writer http.ResponseWriter, request *http.Request) {
	health, err := e.service.GetHealth(request.Context())
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		_, _ = writer.Write([]byte(err.Error()))
		return
	}

	out, err := json.MarshalIndent(health, "", "  ")
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		_, _ = writer.Write([]byte(err.Error()))
		return
	}

	if health.AllHealthy {
		writer.WriteHeader(http.StatusOK)
	} else {
		writer.WriteHeader(http.StatusServiceUnavailable)
	}

	_, _ = writer.Write(out)
}
