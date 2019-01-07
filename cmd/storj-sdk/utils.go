// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// Fence is a synchronization primitive.
// TODO: move to sync2
type Fence struct {
	start  sync.Once
	done   sync.Once
	closed int64
	ch     chan struct{}
}

func (fence *Fence) init() {
	fence.start.Do(func() { fence.ch = make(chan struct{}) })
}

// Wait waits for the fence to be released
func (fence *Fence) Wait() {
	fence.init()
	<-fence.ch
}

// Blocked checks whether the fence is not yet complete
func (fence *Fence) Blocked() bool {
	return atomic.LoadInt64(&fence.closed) == 0
}

// Release releases the Fence
func (fence *Fence) Release() {
	fence.init()
	atomic.StoreInt64(&fence.closed, 1)
	fence.done.Do(func() { close(fence.ch) })
}

// TryConnect tries to connect to addr, returns true when successful
func TryConnect(addr string) bool {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return false
	}
	_, err = conn.Write([]byte{})
	// ignoring error, because we only care about being able to connect
	_ = conn.Close()
	return true
}

func Sleep(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
