// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package auth

import "context"

// The key type is unexported to prevent collisions with context keys defined in
// other packages.
type key int

// apiKey is the context key for the user API Key
const apiKey key = 0

// WithAPIKey creates context with api key
func WithAPIKey(ctx context.Context, key []byte) context.Context {
	return context.WithValue(ctx, apiKey, key)
}

// GetAPIKey returns api key from context is exists
func GetAPIKey(ctx context.Context) ([]byte, bool) {
	key, ok := ctx.Value(apiKey).([]byte)
	return key, ok
}
