// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package filestore

import (
	"context"
	"io"
	"os"

	"github.com/zeebo/errs"

	"storj.io/storj/storage"
)

const (
	// FormatV0 is the identifier for storage format v0, which also corresponds to an absence of
	// format version information.
	FormatV0 storage.FormatVersion = 0
	// FormatV1 is the identifier for storage format v1
	FormatV1 storage.FormatVersion = 1

	// Note: New FormatVersion values should be consecutive, as certain parts of this blob store
	// iterate over them numerically and check for blobs stored with each version.
)

const (
	// MaxFormatVersionSupported is the highest supported storage format version for reading, and
	// the only supported storage format version for writing. If stored blobs claim a higher
	// storage format version than this, or a caller requests _writing_ a storage format version
	// which is not this, this software will not know how to perform the read or write and an error
	// will be returned.
	MaxFormatVersionSupported = FormatV1

	// MinFormatVersionSupported is the lowest supported storage format version for reading. If
	// stored blobs claim a lower storage format version than this, this software will not know how
	// to perform the read and an error will be returned.
	MinFormatVersionSupported = FormatV0
)

// blobReader implements reading blobs
type blobReader struct {
	*os.File
	formatVersion storage.FormatVersion
}

func newBlobReader(file *os.File, formatVersion storage.FormatVersion) *blobReader {
	return &blobReader{file, formatVersion}
}

// Size returns how large is the blob.
func (blob *blobReader) Size() (int64, error) {
	stat, err := blob.Stat()
	if err != nil {
		return 0, err
	}
	return stat.Size(), err
}

// StorageFormatVersion gets the storage format version being used by the blob.
func (blob *blobReader) StorageFormatVersion() storage.FormatVersion {
	return blob.formatVersion
}

// blobWriter implements writing blobs
type blobWriter struct {
	ref           storage.BlobRef
	store         *Store
	closed        bool
	formatVersion storage.FormatVersion

	*os.File
}

func newBlobWriter(ref storage.BlobRef, store *Store, formatVersion storage.FormatVersion, file *os.File) *blobWriter {
	return &blobWriter{
		ref:           ref,
		store:         store,
		closed:        false,
		formatVersion: formatVersion,
		File:          file,
	}
}

// Cancel discards the blob.
func (blob *blobWriter) Cancel(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	if blob.closed {
		return nil
	}
	blob.closed = true
	err = blob.File.Close()
	removeErr := os.Remove(blob.File.Name())
	return Error.Wrap(errs.Combine(err, removeErr))
}

// Commit moves the file to the target location.
func (blob *blobWriter) Commit(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	if blob.closed {
		return Error.New("already closed")
	}
	blob.closed = true
	err = blob.store.dir.Commit(ctx, blob.File, blob.ref, blob.formatVersion)
	return Error.Wrap(err)
}

// Size returns how much has been written so far.
func (blob *blobWriter) Size() (int64, error) {
	pos, err := blob.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	return pos, err
}

// StorageFormatVersion indicates what storage format version the blob is using.
func (blob *blobWriter) StorageFormatVersion() storage.FormatVersion {
	return blob.formatVersion
}
