// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcutil

import (
	"io"
	"sync"

	"storj.io/storj/drpc/drpcwire"
)

// Buffer allows one to buffer up writes of many small packets into one
// larger flush, without worrying about partial writes of packets.
type Buffer struct {
	w   io.Writer
	mu  sync.Mutex
	buf []byte
	tmp []byte
}

// NewBuffer constructs a buffer that will write to the provided writer when
// the serialized packets would be larger than cap.
func NewBuffer(w io.Writer, size int) *Buffer {
	return &Buffer{
		w:   w,
		buf: make([]byte, 0, size),
		tmp: make([]byte, 0, drpcwire.MaxPacketSize),
	}
}

// Write appends the frame to the buffer and flushes when necessary. A call
// to Flush must always eventually happen after a call to Write or your packet
// may be buffered indefinitely.
func (b *Buffer) Write(fr drpcwire.Frame) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.tmp = drpcwire.AppendFrame(b.tmp[:0], fr)

	// n.b. we consider a full buffer as "not fitting" to decide when to flush.
	// if it can't fit in the buffer without allocating, flush first.
	if len(b.tmp)+len(b.buf) >= cap(b.buf) {
		if err := b.flush(); err != nil {
			return err
		}
		// if it still can't fit in the buffer without allocating, write it.
		if len(b.tmp) >= cap(b.buf) {
			if _, err := b.w.Write(b.tmp); err != nil {
				return err
			}
			return nil
		}
	}

	// it definitely fits. add it to the buffer.
	b.buf = append(b.buf, b.tmp...)
	return nil
}

// Flush writes the buffer to the writer. A call to Flush must always
// eventually happen after a call to Write or your packet may be buffered
// indefinitely.
func (b *Buffer) Flush() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.flush()
}

func (b *Buffer) flush() error {
	if len(b.buf) == 0 {
		return nil
	}
	if _, err := b.w.Write(b.buf); err != nil {
		return err
	}
	b.buf = b.buf[:0]
	return nil
}
