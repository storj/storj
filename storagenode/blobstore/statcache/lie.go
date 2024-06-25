// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package statcache

import (
	"context"
	"time"

	"storj.io/storj/storagenode/blobstore"
)

// Lie is a cache implementation with returns with fake values. Do not use in production. Only for performance testing.
type Lie struct {
}

var _ Cache = &Lie{}

// Get implements Cache.
func (l Lie) Get(ctx context.Context, namespace []byte, key []byte) (blobstore.FileInfo, bool, error) {
	return FileInfo{
		modTime: time.Now(),
		size:    1,
	}, true, nil
}

// Set implements Cache.
func (l Lie) Set(ctx context.Context, namespace []byte, key []byte, value blobstore.FileInfo) error {
	return nil
}

// Delete implements Cache.
func (l Lie) Delete(ctx context.Context, namespace []byte, key []byte) error {
	return nil
}

// Close implements Cache.
func (l Lie) Close() error {
	return nil
}
