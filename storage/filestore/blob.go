// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package filestore

import (
	"io"
	"os"

	"github.com/zeebo/errs"

	"storj.io/storj/storage"
)

// blobWriter implements reader that is offset by blob header size
type blobReader struct {
	*os.File
}

func newBlobReader(file *os.File) *blobReader {
	return &blobReader{file}
}

// Size returns how large is the blob.
func (blob *blobReader) Size() int64 {
	stat, _ := blob.Stat()
	return stat.Size()
}

// blobWriter implements reader that is offset by blob header size
type blobWriter struct {
	ref   storage.BlobRef
	store *Store

	*os.File
}

func newBlobWriter(ref storage.BlobRef, store *Store, file *os.File) *blobWriter {
	return &blobWriter{ref, store, file}
}

// Cancel discards the blob.
func (blob *blobWriter) Cancel() error {
	err := blob.File.Close()
	removeErr := os.Remove(blob.File.Name())
	return Error.Wrap(errs.Combine(err, removeErr))
}

// Commit moves the file to the target location.
func (blob *blobWriter) Commit() error {
	err := blob.store.dir.Commit(blob.File, blob.ref)
	return Error.Wrap(err)
}

// Size returns how much has been written so far.
func (blob *blobWriter) Size() int64 {
	p, _ := blob.Seek(0, io.SeekCurrent)
	return p
}
