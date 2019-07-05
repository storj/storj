// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testblobs

import (
	"context"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/storage"
	"storj.io/storj/storagenode"
)

// SlowDB implements slow storage node DB.
type SlowDB struct {
	storagenode.DB
	blobs *SlowBlobs
	log   *zap.Logger
}

// NewSlowDB creates a new slow storage node DB wrapping the provided db.
// Use SetLatency to dynamically configure the latency of all piece operations.
func NewSlowDB(log *zap.Logger, db storagenode.DB) *SlowDB {
	return &SlowDB{
		DB:    db,
		blobs: NewSlowBlobs(log, db.Pieces()),
		log:   log,
	}
}

// Pieces returns the blob store.
func (slow *SlowDB) Pieces() storage.Blobs {
	return slow.blobs
}

// SetLatency enables a sleep for delay duration for all piece operations.
// A zero or negative delay means no sleep.
func (slow *SlowDB) SetLatency(delay time.Duration) {
	slow.blobs.SetLatency(delay)
}

// SlowBlobs implements a slow blob store.
type SlowBlobs struct {
	delay int64 // time.Duration
	blobs storage.Blobs
	log   *zap.Logger
}

// NewSlowBlobs creates a new slow blob store wrapping the provided blobs.
// Use SetLatency to dynamically configure the latency of all operations.
func NewSlowBlobs(log *zap.Logger, blobs storage.Blobs) *SlowBlobs {
	return &SlowBlobs{
		log:   log,
		blobs: blobs,
	}
}

// Create creates a new blob that can be written optionally takes a size
// argument for performance improvements, -1 is unknown size.
func (slow *SlowBlobs) Create(ctx context.Context, ref storage.BlobRef, size int64) (storage.BlobWriter, error) {
	slow.sleep()
	return slow.blobs.Create(ctx, ref, size)
}

// Open opens a reader with the specified namespace and key.
func (slow *SlowBlobs) Open(ctx context.Context, ref storage.BlobRef) (storage.BlobReader, error) {
	slow.sleep()
	return slow.blobs.Open(ctx, ref)
}

// Delete deletes the blob with the namespace and key.
func (slow *SlowBlobs) Delete(ctx context.Context, ref storage.BlobRef) error {
	slow.sleep()
	return slow.blobs.Delete(ctx, ref)
}

// FreeSpace return how much free space left for writing.
func (slow *SlowBlobs) FreeSpace() (int64, error) {
	slow.sleep()
	return slow.blobs.FreeSpace()
}

// SetLatency configures the blob store to sleep for delay duration for all
// operations. A zero or negative delay means no sleep.
func (slow *SlowBlobs) SetLatency(delay time.Duration) {
	atomic.StoreInt64(&slow.delay, int64(delay))
}

// sleep sleeps for the duration set to slow.delay
func (slow *SlowBlobs) sleep() {
	delay := time.Duration(atomic.LoadInt64(&slow.delay))
	time.Sleep(delay)
}
