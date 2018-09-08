// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package streams

import "io"

// EOFAwareLimitReader holds reader and status of EOF
type EOFAwareLimitReader struct {
	reader io.Reader
	eof    bool
	err    error
}

// EOFAwareReader keeps track of the state, has the internal reader reached EOF
func EOFAwareReader(r io.Reader) *EOFAwareLimitReader {
	return &EOFAwareLimitReader{reader: r}
}

func (r *EOFAwareLimitReader) Read(p []byte) (n int, err error) {
	n, err = r.reader.Read(p)
	if err == io.EOF {
		r.eof = true
	} else if err != nil && r.err == nil {
		r.err = err
	}
	return n, err
}

func (r *EOFAwareLimitReader) isEOF() bool {
	return r.eof
}

func (r *EOFAwareLimitReader) hasError() bool {
	return r.err != nil
}
