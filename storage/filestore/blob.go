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
