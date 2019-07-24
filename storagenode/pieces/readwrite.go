// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"context"
	"hash"
	"io"

	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/storage"
)

// Writer implements a piece writer that writes content to blob store and calculates a hash.
type Writer struct {
	hash hash.Hash
	blob storage.BlobWriter
	size int64

	closed bool
}

// NewWriter creates a new writer for storage.BlobWriter.
func NewWriter(blob storage.BlobWriter) (*Writer, error) {
	w := &Writer{}
	w.blob = blob
	w.hash = pkcrypto.NewHash()
	return w, nil
}

// Write writes data to the blob and calculates the hash.
func (w *Writer) Write(data []byte) (int, error) {
	n, err := w.blob.Write(data)
	w.size += int64(n)
	_, _ = w.hash.Write(data[:n]) // guaranteed not to return an error
	if err == io.EOF {
		return n, err
	}
	return n, Error.Wrap(err)
}

// Size returns the amount of data written so far.
func (w *Writer) Size() int64 { return w.size }

// Hash returns the hash of data written so far.
func (w *Writer) Hash() []byte { return w.hash.Sum(nil) }

// Commit commits piece to permanent storage.
func (w *Writer) Commit(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	if w.closed {
		return Error.New("already closed")
	}
	w.closed = true
	return Error.Wrap(w.blob.Commit(ctx))
}

// Cancel deletes any temporarily written data.
func (w *Writer) Cancel(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	if w.closed {
		return nil
	}
	w.closed = true
	return Error.Wrap(w.blob.Cancel(ctx))
}

// Reader implements a piece reader that reads content from blob store.
type Reader struct {
	blob storage.BlobReader
	pos  int64
	size int64
}

// NewReader creates a new reader for storage.BlobReader.
func NewReader(blob storage.BlobReader) (*Reader, error) {
	size, err := blob.Size()
	if err != nil {
		return nil, Error.Wrap(err)
	}

	reader := &Reader{}
	reader.blob = blob
	reader.size = size

	return reader, nil
}

// Read reads data from the underlying blob, buffering as necessary.
func (r *Reader) Read(data []byte) (int, error) {
	n, err := r.blob.Read(data)
	r.pos += int64(n)
	if err == io.EOF {
		return n, err
	}
	return n, Error.Wrap(err)
}

// Seek seeks to the specified location.
func (r *Reader) Seek(offset int64, whence int) (int64, error) {
	if whence == io.SeekStart && r.pos == offset {
		return r.pos, nil
	}

	pos, err := r.blob.Seek(offset, whence)
	r.pos = pos
	if err == io.EOF {
		return pos, err
	}
	return pos, Error.Wrap(err)
}

// ReadAt reads data at the specified offset
func (r *Reader) ReadAt(data []byte, offset int64) (int, error) {
	n, err := r.blob.ReadAt(data, offset)
	if err == io.EOF {
		return n, err
	}
	return n, Error.Wrap(err)
}

// Size returns the amount of data written so far.
func (r *Reader) Size() int64 { return r.size }

// Close closes the reader.
func (r *Reader) Close() error {
	return Error.Wrap(r.blob.Close())
}
