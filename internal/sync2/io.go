// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package sync2

import "io"

// ReadAtWriteAtCloser implements all io.ReaderAt, io.WriterAt and io.Closer
type ReadAtWriteAtCloser interface {
	io.ReaderAt
	io.WriterAt
	io.Closer
}

// PipeWriter allows closing the writer with an error
type PipeWriter interface {
	io.WriteCloser
	CloseWithError(reason error) error
}

// PipeReader allows closing the reader with an error
type PipeReader interface {
	io.ReadCloser
	CloseWithError(reason error) error
}
