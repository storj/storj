// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storage

import (
	"context"
	"io"
)

// BlobRef is an unique reference to a blob
type BlobRef [32]byte

// ReadSeekCloser is an interface that groups Read, ReadAt, Seek and Close.
type ReadSeekCloser interface {
	io.Reader
	io.ReaderAt
	io.Seeker
	io.Closer
	Size() int64
}

// Blobs is a blob storage interface
type Blobs interface {
	// Load loads blob with the specified reference
	Load(context.Context, BlobRef) (ReadSeekCloser, error)
	// Delete deletes the blob with the specified reference
	Delete(context.Context, BlobRef) error
	// Store stores blob from reader
	// optionally takes a size argument for improvements, -1 is unknown size
	Store(ctx context.Context, r io.Reader, size int64) (BlobRef, error)
}
