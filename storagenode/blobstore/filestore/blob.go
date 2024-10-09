// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package filestore

import (
	"bufio"
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
	flushed       bool
	formatVersion blobstore.FormatVersion
	buffer        *bufio.Writer
	file          *lazyFile
	sync          bool

	track leak.Ref
}

func newBlobWriter(track leak.Ref, ref blobstore.BlobRef, store *blobStore, formatVersion blobstore.FormatVersion, file *lazyFile, bufferSize int, sync bool) *blobWriter {
	return &blobWriter{
		ref:           ref,
		store:         store,
		closed:        false,
		formatVersion: formatVersion,
		buffer:        bufio.NewWriterSize(file, bufferSize),
		file:          file,
		sync:          sync,

		track: track,
	}
}

// Write adds data to the blob.
func (blob *blobWriter) Write(p []byte) (int, error) {
	return blob.buffer.Write(p)
}

// Cancel discards the blob.
func (blob *blobWriter) Cancel(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if blob.closed {
		return nil
	}
	blob.closed = true

	// buffer was not flushed to disk yet
	if blob.file.fh == nil {
		return blob.track.Close()
	}

	err = blob.file.fh.Close()
	removeErr := os.Remove(blob.file.fh.Name())
	return Error.Wrap(errs.Combine(err, removeErr, blob.track.Close()))
}

// Commit moves the file to the target location.
func (blob *blobWriter) Commit(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if blob.closed {
		return Error.New("already closed")
	}
	blob.closed = true

	if err := blob.buffer.Flush(); err != nil {
		// TODO: when flush fails, it looks like we don't close the file handle
		return err
	}

	blob.flushed = true

	if blob.file.fh == nil {
		if err := blob.file.createFile(); err != nil {
			return err
		}
	}

	err = blob.store.dir.Commit(ctx, blob.file.fh, blob.sync, blob.ref, blob.formatVersion)
	return Error.Wrap(errs.Combine(err, blob.track.Close()))
}

// Seek flushes any buffer and seeks the underlying file.
func (blob *blobWriter) Seek(offset int64, whence int) (int64, error) {
	if err := blob.buffer.Flush(); err != nil {
		return 0, err
	}

	blob.flushed = true

	if blob.file.fh == nil {
		if err := blob.file.createFile(); err != nil {
			return 0, err
		}
	}

	return blob.file.fh.Seek(offset, whence)
}

// Size returns how much has been written so far.
func (blob *blobWriter) Size() (int64, error) {
	pos, err := blob.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}

	return pos, err
}

// allocation to match V1PieceHeaderReservedArea but we cannot import it here.
var headerAllocation = [512]byte{}

// ReserveHeader reserves header area at the beginning of the blob.
func (blob *blobWriter) ReserveHeader(size int64) error {
	if blob.flushed || blob.buffer.Buffered() > 0 {
		return Error.New("cannot reserve header after writing data")
	}

	if size == 0 {
		return nil
	}

	// small optimization to avoid allocation for mostly used header size
	var allocation []byte
	if size <= int64(len(headerAllocation)) {
		allocation = headerAllocation[:size]
	} else {
		allocation = make([]byte, size)
	}

	_, err := blob.buffer.Write(allocation)
	return err
}

// StorageFormatVersion indicates what storage format version the blob is using.
func (blob *blobWriter) StorageFormatVersion() blobstore.FormatVersion {
	return blob.formatVersion
}
