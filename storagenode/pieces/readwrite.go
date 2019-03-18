// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"bufio"
	"hash"
	"io"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/storage"
)

// Writer implements a piece writer that writes content to blob store and calculates a hash.
type Writer struct {
	buf  bufio.Writer
	hash hash.Hash
	blob storage.BlobWriter
	size int64

	closed bool
}

// NewWriter creates a new writer for storage.BlobWriter.
func NewWriter(blob storage.BlobWriter, bufferSize int) (*Writer, error) {
	w := &Writer{}
	w.buf = *bufio.NewWriterSize(blob, bufferSize)
	w.blob = blob
	w.hash = pkcrypto.NewHash()
	return w, nil
}

// Write writes data to the blob and calculates the hash.
func (w *Writer) Write(data []byte) (int, error) {
	n, err := w.buf.Write(data)
	w.size += int64(n)
	_, _ = w.hash.Write(data[:n]) // guaranteed not to return an error
	return n, Error.Wrap(err)
}

// Size returns the amount of data written so far.
func (w *Writer) Size() int64 { return w.size }

// Hash returns the hash of data written so far.
func (w *Writer) Hash() []byte { return w.hash.Sum(nil) }

// Commit commits piece to permanent storage.
func (w *Writer) Commit() error {
	if w.closed {
		return nil
	}
	w.closed = true

	if err := w.buf.Flush(); err != nil {
		return Error.Wrap(errs.Combine(err, w.Cancel()))
	}
	return Error.Wrap(w.blob.Commit())
}

// Cancel deletes any temporarily written data.
func (w *Writer) Cancel() error {
	if w.closed {
		return nil
	}
	w.closed = true

	w.buf.Reset(nil)
	return Error.Wrap(w.blob.Cancel())
}

// Reader implements a piece writer that writes content to blob store and calculates a hash.
type Reader struct {
	buf  bufio.Reader
	blob storage.BlobReader
	pos  int64
	size int64
}

// NewReader creates a new reader for storage.BlobReader.
func NewReader(blob storage.BlobReader, bufferSize int) (*Reader, error) {
	size, err := blob.Size()
	if err != nil {
		return nil, Error.Wrap(err)
	}

	reader := &Reader{}
	reader.buf = *bufio.NewReaderSize(blob, bufferSize)
	reader.blob = blob
	reader.size = size

	return reader, nil
}

// Read reads data from the underlying blob, buffering as necessary.
func (r *Reader) Read(data []byte) (int, error) {
	n, err := r.blob.Read(data)
	r.pos += int64(n)
	return n, Error.Wrap(err)
}

// Seek seeks to the specified location.
func (r *Reader) Seek(offset int64, whence int) (int64, error) {
	if whence == io.SeekStart && r.pos == offset {
		return r.pos, nil
	}

	r.buf.Reset(r.blob)
	pos, err := r.blob.Seek(offset, whence)
	r.pos = pos
	return pos, Error.Wrap(err)
}

// ReadAt reads data at the specified offset
func (r *Reader) ReadAt(data []byte, offset int64) (int, error) {
	n, err := r.blob.ReadAt(data, offset)
	return n, Error.Wrap(err)
}

// Size returns the amount of data written so far.
func (r *Reader) Size() int64 { return r.size }

// Close closes the reader.
func (r *Reader) Close() error {
	r.buf.Reset(nil)
	return Error.Wrap(r.blob.Close())
}
