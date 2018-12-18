// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package sync2

import (
	"bytes"
	"io"
	"sync"
)

// MemoryBuffer is a synchronized variable-sized memory buffer. Read will
// block until new data is added or the buffer is closed. If the buffer is
// closed, Read will continue reading as long as there is data in the buffer.
type MemoryBuffer struct {
	buf    *bytes.Buffer
	cond   *sync.Cond
	closed bool
	err    error
}

// NewMemoryBuffer creates a new memory buffer initialized with buf.
func NewMemoryBuffer(buf []byte) *MemoryBuffer {
	return &MemoryBuffer{
		buf:  bytes.NewBuffer(buf),
		cond: sync.NewCond(&sync.Mutex{}),
	}
}

// Read reads from buffer into p. This call will block if there is no data
// available and the buffer is not closed.
func (b *MemoryBuffer) Read(p []byte) (n int, err error) {
	b.cond.L.Lock()
	defer b.cond.L.Unlock()

	for b.buf.Len() == 0 {
		if b.closed {
			if b.err != nil {
				return 0, b.err
			}
			return 0, io.EOF
		}
		b.cond.Wait()
	}

	return b.buf.Read(p)
}

// Write writes p to the buffer.
func (b *MemoryBuffer) Write(p []byte) (n int, err error) {
	b.cond.L.Lock()
	defer b.cond.L.Unlock()
	defer b.cond.Signal()

	if b.closed {
		return 0, io.ErrClosedPipe
	}

	return b.buf.Write(p)
}

// Close closes the buffer.
func (b *MemoryBuffer) Close() error {
	return b.CloseWithError(nil)
}

// CloseWithError closes the buffer with error.
func (b *MemoryBuffer) CloseWithError(err error) error {
	b.cond.L.Lock()
	defer b.cond.L.Unlock()
	defer b.cond.Signal()

	b.closed = true
	b.err = err

	return nil
}
