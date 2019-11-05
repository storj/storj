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
		blobs: newSlowBlobs(log, db.Pieces()),
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

// newSlowBlobs creates a new slow blob store wrapping the provided blobs.
// Use SetLatency to dynamically configure the latency of all operations.
func newSlowBlobs(log *zap.Logger, blobs storage.Blobs) *SlowBlobs {
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

// OpenWithStorageFormat opens a reader for the already-located blob, avoiding the potential need
// to check multiple storage formats to find the blob.
func (slow *SlowBlobs) OpenWithStorageFormat(ctx context.Context, ref storage.BlobRef, formatVer storage.FormatVersion) (storage.BlobReader, error) {
	slow.sleep()
	return slow.blobs.OpenWithStorageFormat(ctx, ref, formatVer)
}

// Delete deletes the blob with the namespace and key.
func (slow *SlowBlobs) Delete(ctx context.Context, ref storage.BlobRef) error {
	slow.sleep()
	return slow.blobs.Delete(ctx, ref)
}

// DeleteWithStorageFormat deletes the blob with the namespace, key, and format version
func (slow *SlowBlobs) DeleteWithStorageFormat(ctx context.Context, ref storage.BlobRef, formatVer storage.FormatVersion) error {
	slow.sleep()
	return slow.blobs.DeleteWithStorageFormat(ctx, ref, formatVer)
}

// Stat looks up disk metadata on the blob file
func (slow *SlowBlobs) Stat(ctx context.Context, ref storage.BlobRef) (storage.BlobInfo, error) {
	slow.sleep()
	return slow.blobs.Stat(ctx, ref)
}

// StatWithStorageFormat looks up disk metadata for the blob file with the given storage format
// version. This avoids the potential need to check multiple storage formats for the blob
// when the format is already known.
func (slow *SlowBlobs) StatWithStorageFormat(ctx context.Context, ref storage.BlobRef, formatVer storage.FormatVersion) (storage.BlobInfo, error) {
	slow.sleep()
	return slow.blobs.StatWithStorageFormat(ctx, ref, formatVer)
}

// WalkNamespace executes walkFunc for each locally stored blob in the given namespace.
// If walkFunc returns a non-nil error, WalkNamespace will stop iterating and return the
// error immediately.
func (slow *SlowBlobs) WalkNamespace(ctx context.Context, namespace []byte, walkFunc func(storage.BlobInfo) error) error {
	slow.sleep()
	return slow.blobs.WalkNamespace(ctx, namespace, walkFunc)
}

// ListNamespaces returns all namespaces that might be storing data.
func (slow *SlowBlobs) ListNamespaces(ctx context.Context) ([][]byte, error) {
	return slow.blobs.ListNamespaces(ctx)
}

// FreeSpace return how much free space left for writing.
func (slow *SlowBlobs) FreeSpace() (int64, error) {
	slow.sleep()
	return slow.blobs.FreeSpace()
}

// SpaceUsed adds up how much is used in all namespaces
func (slow *SlowBlobs) SpaceUsed(ctx context.Context) (int64, error) {
	slow.sleep()
	return slow.blobs.SpaceUsed(ctx)
}

// SpaceUsedInNamespace adds up how much is used in the given namespace
func (slow *SlowBlobs) SpaceUsedInNamespace(ctx context.Context, namespace []byte) (int64, error) {
	slow.sleep()
	return slow.blobs.SpaceUsedInNamespace(ctx, namespace)
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
