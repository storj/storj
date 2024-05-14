// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package filestore

import (
	"context"
	"io"
	"os"

	"github.com/zeebo/errs"

	"storj.io/common/leak"
	"storj.io/storj/storagenode/blobstore"
)

const (
	// FormatV0 is the identifier for storage format v0, which also corresponds to an absence of
	// format version information.
	FormatV0 blobstore.FormatVersion = 0
	// FormatV1 is the identifier for storage format v1.
	FormatV1 blobstore.FormatVersion = 1

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

	// MinFormatVersionSupportedInTrash is the lowest supported storage format that can be used
	// for storage in the trash.
	MinFormatVersionSupportedInTrash = FormatV1
)

// blobReader implements reading blobs.
type blobReader struct {
	*os.File
	formatVersion blobstore.FormatVersion

	track leak.Ref
}

func newBlobReader(track leak.Ref, file *os.File, formatVersion blobstore.FormatVersion) *blobReader {
	return &blobReader{file, formatVersion, track}
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
func (blob *blobReader) StorageFormatVersion() blobstore.FormatVersion {
	return blob.formatVersion
}

// Close closes the reader.
func (blob *blobReader) Close() error {
	return errs.Combine(blob.File.Close(), blob.track.Close())
}

// blobWriter implements writing blobs.
type blobWriter struct {
	ref           blobstore.BlobRef
	store         *blobStore
	closed        bool
	formatVersion blobstore.FormatVersion
	buffer        []byte
	pos           int
	fh            *os.File
	sync          bool

	track leak.Ref
}

func newBlobWriter(track leak.Ref, ref blobstore.BlobRef, store *blobStore, formatVersion blobstore.FormatVersion, file *os.File, bufferSize int, sync bool) *blobWriter {
	return &blobWriter{
		ref:           ref,
		store:         store,
		closed:        false,
		formatVersion: formatVersion,
		buffer:        make([]byte, 0, bufferSize),
		fh:            file,
		sync:          sync,

		track: track,
	}
}

// Write adds data to the blob.
func (blob *blobWriter) Write(p []byte) (int, error) {
	if blob.pos+len(p) < len(blob.buffer) {
		copy(blob.buffer[blob.pos:], p)
	} else {
		blob.buffer = append(blob.buffer[:blob.pos], p...)
	}
	blob.pos += len(p)
	return len(p), nil
}

// Cancel discards the blob.
func (blob *blobWriter) Cancel(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if blob.closed {
		return nil
	}
	blob.closed = true

	err = blob.fh.Close()
	removeErr := os.Remove(blob.fh.Name())
	return Error.Wrap(errs.Combine(err, removeErr, blob.track.Close()))
}

// Commit moves the file to the target location.
func (blob *blobWriter) Commit(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if blob.closed {
		return Error.New("already closed")
	}
	if _, err := blob.fh.Write(blob.buffer); err != nil {
		return errs.Combine(Error.Wrap(err), blob.Cancel(ctx))
	}
	blob.closed = true

	err = blob.store.dir.Commit(ctx, blob.fh, blob.sync, blob.ref, blob.formatVersion)
	return Error.Wrap(errs.Combine(err, blob.track.Close()))
}

// Seek flushes any buffer and seeks the underlying file.
func (blob *blobWriter) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekCurrent:
		blob.pos += int(offset)
	case io.SeekEnd:
		blob.pos = len(blob.buffer) + int(offset)
	case io.SeekStart:
		blob.pos = int(offset)
	}
	if blob.pos > cap(blob.buffer) {
		// do some geometric growth so that we don't get quadratic behavior
		// from some bozo calling .Seek(1, io.SeekCurrent) over and over
		buffer := make([]byte, blob.pos, 3*blob.pos/2)
		copy(buffer, blob.buffer)
		blob.buffer = buffer
	}
	if blob.pos > len(blob.buffer) {
		blob.buffer = blob.buffer[:blob.pos]
	}
	return int64(blob.pos), nil
}

// Size returns how much has been written so far.
func (blob *blobWriter) Size() (int64, error) {
	return int64(len(blob.buffer)), nil
}

// StorageFormatVersion indicates what storage format version the blob is using.
func (blob *blobWriter) StorageFormatVersion() blobstore.FormatVersion {
	return blob.formatVersion
}
