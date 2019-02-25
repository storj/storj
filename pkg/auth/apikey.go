// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package auth

import (
	"context"
	"crypto/subtle"
)

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

// ValidateAPIKey compares the context api key with the key passed in as an argument
func ValidateAPIKey(ctx context.Context, actualKey []byte) error {
	expectedKey, ok := GetAPIKey(ctx)
	if !ok {
		return Error.New("Could not get api key from context")
	}

	matches := (1 == subtle.ConstantTimeCompare(actualKey, expectedKey))
	if !matches {
		return Error.New("Invalid API credential")
	}
	return nil
}
