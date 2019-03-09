// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storage

import (
	"context"
	"io"
)

// BlobRef is a reference to a blob
type BlobRef struct {
	Namespace []byte
	Key       []byte
}

// BlobReader is an interface that groups Read, ReadAt, Seek and Close.
type BlobReader interface {
	io.Reader
	io.ReaderAt
	io.Seeker
	io.Closer
	// Size returns the size of the blob
	Size() int64
}

// BlobWriter is an interface that groups Read, ReadAt, Seek and Close.
type BlobWriter interface {
	io.Writer
	// Cancel discards the blob.
	Cancel() error
	// Commit ensures that the blob is readable by others.
	Commit() error
	// Size returns the size of the blob
	Size() int64
}

// Blobs is a blob storage interface
type Blobs interface {
	// Create creates a new blob that can be written
	// optionally takes a size argument for performance improvements, -1 is unknown size
	Create(ctx context.Context, ref BlobRef, size int64) (BlobWriter, error)
	// Open opens a reader with the specified namespace and key
	Open(ctx context.Context, ref BlobRef) (BlobReader, error)
	// Delete deletes the blob with the namespace and key
	Delete(ctx context.Context, ref BlobRef) error
}
