// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package filestore

import (
	"io"
	"os"

	"github.com/zeebo/errs"

	"storj.io/storj/storage"
)

// blobReader implements reading blobs
type blobReader struct {
	*os.File
}

func newBlobReader(file *os.File) *blobReader {
	return &blobReader{file}
}

// Size returns how large is the blob.
func (blob *blobReader) Size() (int64, error) {
	stat, err := blob.Stat()
	if err != nil {
		return 0, err
	}
	return stat.Size(), err
}

// blobWriter implements writing blobs
type blobWriter struct {
	ref    storage.BlobRef
	store  *Store
	closed bool

	*os.File
}

func newBlobWriter(ref storage.BlobRef, store *Store, file *os.File) *blobWriter {
	return &blobWriter{ref, store, false, file}
}

// Cancel discards the blob.
func (blob *blobWriter) Cancel() error {
	if blob.closed {
		return nil
	}
	blob.closed = true
	err := blob.File.Close()
	removeErr := os.Remove(blob.File.Name())
	return Error.Wrap(errs.Combine(err, removeErr))
}

// Commit moves the file to the target location.
func (blob *blobWriter) Commit() error {
	if blob.closed {
		return nil
	}
	blob.closed = true
	err := blob.store.dir.Commit(blob.File, blob.ref)
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
