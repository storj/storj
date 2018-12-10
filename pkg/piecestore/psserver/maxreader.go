// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package psserver

import (
	"errors"
	"io"
)

// MaxReader behaves like a normal reader, but throws an error if the data
// read in exceeds a maximum amount
type MaxReader struct {
	r     io.Reader
	sofar int64
	max   int64
}

// NewMaxReader creates a new instance of MaxReader
func NewMaxReader(r io.Reader, max int64) (mr *MaxReader) {
	return &MaxReader{
		r:     r,
		sofar: 0,
		max:   max,
	}
}

// Read throws an error if the total data read in exceeds the maximum
// but otherwise acts like io.Reader.Read
func (mr *MaxReader) Read(p []byte) (n int, err error) {
	if mr.sofar > mr.max {
		return n, errors.New("Data read from reader exceeds maximum allowed")
	}

	n, err = mr.r.Read(p)
	if err != nil {
		return n, err
	}

	mr.sofar += int64(n)
	if mr.sofar > mr.max {
		return n, errors.New("Data read from reader exceeds maximum allowed")
	}

	return n, nil
}
