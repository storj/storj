// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package auth

import (
	"context"
)

// The key type is unexported to prevent collisions with context keys defined in
// other packages.
type apikey struct{}

// WithAPIKey creates context with api key
func WithAPIKey(ctx context.Context, key []byte) context.Context {
	return context.WithValue(ctx, apikey{}, key)
}

// GetAPIKey returns api key from context is exists
func GetAPIKey(ctx context.Context) ([]byte, bool) {
	key, ok := ctx.Value(apikey{}).([]byte)
	return key, ok
}
