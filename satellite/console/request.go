// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"net/http"
)

// requestKey is context key for Requests.
const requestKey key = 1

// WithRequest creates new context with *http.Request.
func WithRequest(ctx context.Context, req *http.Request) context.Context {
	return context.WithValue(ctx, requestKey, req)
}

// GetRequest gets *http.Request from context.
func GetRequest(ctx context.Context) *http.Request {
	if req, ok := ctx.Value(requestKey).(*http.Request); ok {
		return req
	}
	return nil
}
