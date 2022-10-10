// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package api

import (
	"context"
	"net/http"
)

// Auth exposes methods to control authentication process for each endpoint.
type Auth interface {
	// IsAuthenticated checks if request is performed with all needed authorization credentials.
	IsAuthenticated(ctx context.Context, r *http.Request, isCookieAuth, isKeyAuth bool) (context.Context, error)
	// RemoveAuthCookie indicates to the client that the authentication cookie should be removed.
	RemoveAuthCookie(w http.ResponseWriter)
}
