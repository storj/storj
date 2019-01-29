// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package utils

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
