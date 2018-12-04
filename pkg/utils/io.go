// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package utils

import (
	"io"
	"os"

	"go.uber.org/zap"
)

// ReaderSource takes a src func and turns it into an io.Reader
type ReaderSource struct {
	src func() ([]byte, error)
	buf []byte
	err error
}

// NewReaderSource makes a new ReaderSource
func NewReaderSource(src func() ([]byte, error)) *ReaderSource {
	return &ReaderSource{src: src}
}

// Read implements io.Reader
func (rs *ReaderSource) Read(p []byte) (n int, err error) {
	if rs.err != nil {
		return 0, rs.err
	}
	if len(rs.buf) == 0 {
		rs.buf, rs.err = rs.src()
	}

	n = copy(p, rs.buf)
	rs.buf = rs.buf[n:]
	return n, rs.err
}

// LogClose closes an io.Closer, logging the error if there is one that isn't
// os.ErrClosed
func LogClose(fh io.Closer) {
	err := fh.Close()
	if err == nil || err == os.ErrClosed {
		return
	}
	if perr, ok := err.(*os.PathError); ok && perr.Err == os.ErrClosed {
		return
	}
	zap.S().Errorf("Failed to close file: %s", err)
}
