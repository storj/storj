// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcclient

import (
	"sync"

	"storj.io/storj/drpc/drpcwire"
)

type lockedBuffer struct {
	buf *drpcwire.Buffer
	mu  sync.Mutex
}

func (lb *lockedBuffer) Write(pkt drpcwire.Packet) error {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.buf.Write(pkt)
}

func (lb *lockedBuffer) Flush() error {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.buf.Flush()
}
