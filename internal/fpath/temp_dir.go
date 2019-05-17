// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// TODO maybe there is better place for this

package fpath

import "context"

// The key type is unexported to prevent collisions with context keys defined in
// other packages.
type key int

// tempDir is the context key for temporary directory
const tempDir key = 0

// WithTempDir creates context with api key
func WithTempDir(ctx context.Context, dir string) context.Context {
	return context.WithValue(ctx, tempDir, dir)
}

// GetTempDir returns api key from context is exists
func GetTempDir(ctx context.Context) (string, bool) {
	key, ok := ctx.Value(tempDir).(string)
	return key, ok
}
