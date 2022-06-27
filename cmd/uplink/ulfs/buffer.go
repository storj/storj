// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package ulfs

import (
	"context"
	"errors"
	"io"
	"sync"
)

// BufferedReadHandle wraps a ReadHandler with an in-memory buffer.
type BufferedReadHandle struct {
	ctx    context.Context
	reader ReadHandle
	buf    []byte
	ready  bool
	size   int
	pos    int
}

// NewBufferedReadHandle wraps reader with buf.
func NewBufferedReadHandle(ctx context.Context, reader ReadHandle, buf []byte) ReadHandle {
	return &BufferedReadHandle{
		ctx:    ctx,
		reader: reader,
		buf:    buf,
	}
}

// Read will first read the entire content of the wrapped reader to the
// internal buffer before returning.
func (b *BufferedReadHandle) Read(p []byte) (int, error) {
	// Read out reader to fill up buf before returning the first byte.
	if !b.ready {
		n, err := io.ReadFull(b.reader, b.buf)
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			return 0, err
		}

		b.ready = true
		b.size = n
	}

	n := copy(p, b.buf[b.pos:b.size])
	if n == 0 {
		return 0, io.EOF
	}

	b.pos += n

	return n, nil
}

// Close closes the wrapped ReadHandle.
func (b *BufferedReadHandle) Close() error {
	return b.reader.Close()
}

// Info returns Info of the wrapped ReadHandle.
func (b *BufferedReadHandle) Info() ObjectInfo { return b.reader.Info() }

// BytesPool is a fixed-size pool of []byte.
type BytesPool struct {
	size int
	mu   sync.Mutex
	free [][]byte
}

// NewBytesPool creates a pool for []byte slices of length `size`.
func NewBytesPool(size int) *BytesPool {
	return &BytesPool{
		size: size,
	}
}

// Get returns a new []byte from the pool.
func (pool *BytesPool) Get() []byte {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	if len(pool.free) > 0 {
		n := len(pool.free)
		last := pool.free[n-1]
		pool.free = pool.free[:n-1]
		return last
	}

	return make([]byte, pool.size)
}

// Put releases buf back to the pool.
func (pool *BytesPool) Put(buf []byte) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pool.free = append(pool.free, buf)
}
