// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testblobs

import (
	"context"

	"go.uber.org/zap"

	"storj.io/storj/storage"
	"storj.io/storj/storagenode"
)

// BadDB implements bad storage node DB.
type BadDB struct {
	storagenode.DB
	blobs *BadBlobs
	log   *zap.Logger
}

// NewBadDB creates a new bad storage node DB.
// Use SetError to manually configure the error returned by all piece operations.
func NewBadDB(log *zap.Logger, db storagenode.DB) *BadDB {
	return &BadDB{
		DB:    db,
		blobs: newBadBlobs(log, db.Pieces()),
		log:   log,
	}
}

// Pieces returns the blob store.
func (bad *BadDB) Pieces() storage.Blobs {
	return bad.blobs
}

// SetError sets an error to be returned for all piece operations.
func (bad *BadDB) SetError(err error) {
	bad.blobs.SetError(err)
}

// BadBlobs implements a bad blob store.
type BadBlobs struct {
	err   error
	blobs storage.Blobs
	log   *zap.Logger
}

// newBadBlobs creates a new bad blob store wrapping the provided blobs.
// Use SetError to manually configure the error returned by all operations.
func newBadBlobs(log *zap.Logger, blobs storage.Blobs) *BadBlobs {
	return &BadBlobs{
		log:   log,
		blobs: blobs,
	}
}

// Create creates a new blob that can be written optionally takes a size
// argument for performance improvements, -1 is unknown size.
func (bad *BadBlobs) Create(ctx context.Context, ref storage.BlobRef, size int64) (storage.BlobWriter, error) {
	if bad.err != nil {
		return nil, bad.err
	}
	return bad.blobs.Create(ctx, ref, size)
}

// Close closes the blob store and any resources associated with it.
func (bad *BadBlobs) Close() error {
	if bad.err != nil {
		return bad.err
	}
	return bad.blobs.Close()
}

// Open opens a reader with the specified namespace and key.
func (bad *BadBlobs) Open(ctx context.Context, ref storage.BlobRef) (storage.BlobReader, error) {
	if bad.err != nil {
		return nil, bad.err
	}
	return bad.blobs.Open(ctx, ref)
}

// OpenWithStorageFormat opens a reader for the already-located blob, avoiding the potential need
// to check multiple storage formats to find the blob.
func (bad *BadBlobs) OpenWithStorageFormat(ctx context.Context, ref storage.BlobRef, formatVer storage.FormatVersion) (storage.BlobReader, error) {
	if bad.err != nil {
		return nil, bad.err
	}
	return bad.blobs.OpenWithStorageFormat(ctx, ref, formatVer)
}

// Trash deletes the blob with the namespace and key.
func (bad *BadBlobs) Trash(ctx context.Context, ref storage.BlobRef) error {
	if bad.err != nil {
		return bad.err
	}
	return bad.blobs.Trash(ctx, ref)
}

// RestoreTrash restores all files in the trash.
func (bad *BadBlobs) RestoreTrash(ctx context.Context, namespace []byte) error {
	if bad.err != nil {
		return bad.err
	}
	return bad.blobs.RestoreTrash(ctx, namespace)
}

// Delete deletes the blob with the namespace and key.
func (bad *BadBlobs) Delete(ctx context.Context, ref storage.BlobRef) error {
	if bad.err != nil {
		return bad.err
	}
	return bad.blobs.Delete(ctx, ref)
}

// DeleteWithStorageFormat deletes the blob with the namespace, key, and format version.
func (bad *BadBlobs) DeleteWithStorageFormat(ctx context.Context, ref storage.BlobRef, formatVer storage.FormatVersion) error {
	if bad.err != nil {
		return bad.err
	}
	return bad.blobs.DeleteWithStorageFormat(ctx, ref, formatVer)
}

// Stat looks up disk metadata on the blob file.
func (bad *BadBlobs) Stat(ctx context.Context, ref storage.BlobRef) (storage.BlobInfo, error) {
	if bad.err != nil {
		return nil, bad.err
	}
	return bad.blobs.Stat(ctx, ref)
}

// StatWithStorageFormat looks up disk metadata for the blob file with the given storage format
// version. This avoids the potential need to check multiple storage formats for the blob
// when the format is already known.
func (bad *BadBlobs) StatWithStorageFormat(ctx context.Context, ref storage.BlobRef, formatVer storage.FormatVersion) (storage.BlobInfo, error) {
	if bad.err != nil {
		return nil, bad.err
	}
	return bad.blobs.StatWithStorageFormat(ctx, ref, formatVer)
}

// WalkNamespace executes walkFunc for each locally stored blob in the given namespace.
// If walkFunc returns a non-nil error, WalkNamespace will stop iterating and return the
// error immediately.
func (bad *BadBlobs) WalkNamespace(ctx context.Context, namespace []byte, walkFunc func(storage.BlobInfo) error) error {
	if bad.err != nil {
		return bad.err
	}
	return bad.blobs.WalkNamespace(ctx, namespace, walkFunc)
}

// ListNamespaces returns all namespaces that might be storing data.
func (bad *BadBlobs) ListNamespaces(ctx context.Context) ([][]byte, error) {
	if bad.err != nil {
		return make([][]byte, 0), bad.err
	}
	return bad.blobs.ListNamespaces(ctx)
}

// FreeSpace return how much free space left for writing.
func (bad *BadBlobs) FreeSpace() (int64, error) {
	if bad.err != nil {
		return 0, bad.err
	}
	return bad.blobs.FreeSpace()
}

// SpaceUsed adds up how much is used in all namespaces.
func (bad *BadBlobs) SpaceUsed(ctx context.Context) (int64, error) {
	if bad.err != nil {
		return 0, bad.err
	}
	return bad.blobs.SpaceUsed(ctx)
}

// SpaceUsedInNamespace adds up how much is used in the given namespace.
func (bad *BadBlobs) SpaceUsedInNamespace(ctx context.Context, namespace []byte) (int64, error) {
	if bad.err != nil {
		return 0, bad.err
	}
	return bad.blobs.SpaceUsedInNamespace(ctx, namespace)
}

// SetError configures the blob store to return a specific error for all operations.
func (bad *BadBlobs) SetError(err error) {
	bad.err = err
}
