// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package readcloser

import "io"

// MultiReadCloser is a MultiReader extension that returns a ReaderCloser
// that's the logical concatenation of the provided input readers.
// They're read sequentially. Once all inputs have returned EOF,
// Read will return EOF.  If any of the readers return a non-nil,
// non-EOF error, Read will return that error.
func MultiReadCloser(readers ...io.ReadCloser) io.ReadCloser {
	r := make([]io.Reader, len(readers))
	for i := range readers {
		r[i] = readers[i]
	}
	c := make([]io.Closer, len(readers))
	for i := range readers {
		c[i] = readers[i]
	}
	return &multiReadCloser{io.MultiReader(r...), c}
}

type multiReadCloser struct {
	multireader io.Reader
	closers     []io.Closer
}

func (l *multiReadCloser) Read(p []byte) (n int, err error) {
	return l.multireader.Read(p)
}

func (l *multiReadCloser) Close() error {
	var firstErr error
	for _, c := range l.closers {
		err := c.Close()
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
