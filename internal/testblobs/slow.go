// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testblobs

import (
	"context"
	"sync/atomic"
	"time"

	"storj.io/storj/storage"
	"storj.io/storj/storagenode"
)

// SlowDB implements slow storage node DB.
type SlowDB struct {
	storagenode.DB
	blobs *SlowBlobs
}

// NewSlowDB creates a new slow storage node DB wrapping the provided db.
// When the Slow method is called, all piece operations are delayed with the
// provided duration.
func NewSlowDB(db storagenode.DB, latency time.Duration) *SlowDB {
	return &SlowDB{
		DB:    db,
		blobs: NewSlowBlobs(db.Pieces(), latency),
	}
}

// Pieces returns the blob store.
func (slow *SlowDB) Pieces() storage.Blobs {
	return slow.blobs
}

// Slow enables the latency in the piece operations.
func (slow *SlowDB) Slow() {
	slow.blobs.Slow()
}

// Fast disables the latency in the piece operations.
func (slow *SlowDB) Fast() {
	slow.blobs.Fast()
}

// SlowBlobs implements a slow blob store.
type SlowBlobs struct {
	blobs   storage.Blobs
	delay   time.Duration
	enabled int32
}

// NewSlowBlobs creates a new slow blob store wrapping the provided blobs.
// When the Slow method is called, all operations are delayed with the provided
// duration.
func NewSlowBlobs(blobs storage.Blobs, delay time.Duration) *SlowBlobs {
	return &SlowBlobs{
		blobs: blobs,
		delay: delay,
	}
}

// Create creates a new blob that can be written optionally takes a size
// argument for performance improvements, -1 is unknown size.
func (slow *SlowBlobs) Create(ctx context.Context, ref storage.BlobRef, size int64) (storage.BlobWriter, error) {
	if atomic.LoadInt32(&slow.enabled) == 1 {
		time.Sleep(slow.delay)
	}
	return slow.blobs.Create(ctx, ref, size)
}

// Open opens a reader with the specified namespace and key.
func (slow *SlowBlobs) Open(ctx context.Context, ref storage.BlobRef) (storage.BlobReader, error) {
	if atomic.LoadInt32(&slow.enabled) == 1 {
		time.Sleep(slow.delay)
	}
	return slow.blobs.Open(ctx, ref)
}

// Delete deletes the blob with the namespace and key.
func (slow *SlowBlobs) Delete(ctx context.Context, ref storage.BlobRef) error {
	if atomic.LoadInt32(&slow.enabled) == 1 {
		time.Sleep(slow.delay)
	}
	return slow.blobs.Delete(ctx, ref)
}

// FreeSpace return how much free space left for writing.
func (slow *SlowBlobs) FreeSpace() (int64, error) {
	if atomic.LoadInt32(&slow.enabled) == 1 {
		time.Sleep(slow.delay)
	}
	return slow.blobs.FreeSpace()
}

// Slow enables the latency in the blob store.
func (slow *SlowBlobs) Slow() {
	atomic.CompareAndSwapInt32(&slow.enabled, 0, 1)
}

// Fast disables the latency in the blob store.
func (slow *SlowBlobs) Fast() {
	atomic.CompareAndSwapInt32(&slow.enabled, 1, 0)
}
