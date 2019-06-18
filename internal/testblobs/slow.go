// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testblobs

import (
	"context"
	"time"

	"storj.io/storj/storage"
)

// SlowBlobs implements a slow blob store.
type SlowBlobs struct {
	blobs storage.Blobs
	delay time.Duration
}

// NewSlowBlobs creates a new slow blob store wrapping the provided blobs.
// All operations are delayed with the provided duration.
func NewSlowBlobs(blobs storage.Blobs, delay time.Duration) *SlowBlobs {
	return &SlowBlobs{
		blobs: blobs,
		delay: delay,
	}
}

// Create creates a new blob that can be written
// optionally takes a size argument for performance improvements, -1 is unknown size
func (slow *SlowBlobs) Create(ctx context.Context, ref storage.BlobRef, size int64) (storage.BlobWriter, error) {
	time.Sleep(slow.delay)
	return slow.blobs.Create(ctx, ref, size)
}

// Open opens a reader with the specified namespace and key
func (slow *SlowBlobs) Open(ctx context.Context, ref storage.BlobRef) (storage.BlobReader, error) {
	time.Sleep(slow.delay)
	return slow.blobs.Open(ctx, ref)
}

// Delete deletes the blob with the namespace and key
func (slow *SlowBlobs) Delete(ctx context.Context, ref storage.BlobRef) error {
	time.Sleep(slow.delay)
	return slow.blobs.Delete(ctx, ref)
}

// FreeSpace return how much free space left for writing
func (slow *SlowBlobs) FreeSpace() (int64, error) {
	time.Sleep(slow.delay)
	return slow.blobs.FreeSpace()
}
