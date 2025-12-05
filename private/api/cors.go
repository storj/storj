// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package api

import "net/http"

// CORS exposes methods to control CORS (Cross-Origin Resource Sharing) for each endpoint.
type CORS interface {
	// Handle sets the necessary CORS headers for the request and checks if the request is an OPTIONS preflight request.
	Handle(w http.ResponseWriter, r *http.Request) bool
}
