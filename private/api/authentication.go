// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package api

import "net/http"

// Auth exposes methods to control authentication process for each endpoint.
type Auth interface {
	// IsAuthenticated checks if request is performed with all needed authorization credentials.
	IsAuthenticated(r *http.Request) error
}
