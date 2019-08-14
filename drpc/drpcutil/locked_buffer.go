// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcutil

import (
	"sync"

	"storj.io/storj/drpc/drpcwire"
)

type LockedBuffer struct {
	buf *drpcwire.Buffer
	mu  sync.Mutex
}

func NewLockedBuffer(buf *drpcwire.Buffer) *LockedBuffer {
	return &LockedBuffer{buf: buf}
}

func (lb *LockedBuffer) Write(pkt drpcwire.Packet) error {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.buf.Write(pkt)
}

func (lb *LockedBuffer) Flush() error {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.buf.Flush()
}
