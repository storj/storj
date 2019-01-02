// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package utils

import "sync"

// Event is a synchronization primitive that lets an arbitrary number
// of concurrent waiters to wait for an event to happen 1 or more times.
type Event struct {
	mtx         sync.Mutex
	initialized bool
	fired       bool
	block       sync.Mutex
}

func (e *Event) setup() {
	e.mtx.Lock()
	if !e.initialized {
		e.block.Lock()
		e.initialized = true
	}
	e.mtx.Unlock()
}

// Wait sleeps until Fire is called at least once.
func (e *Event) Wait() {
	e.setup()
	e.block.Lock()
	e.block.Unlock()
}

// Fire wakes up any sleeping waiters and prevents any future waiters from
// waiting.
func (e *Event) Fire() {
	e.setup()
	e.mtx.Lock()
	if !e.fired {
		e.block.Unlock()
		e.fired = true
	}
	e.mtx.Unlock()
}
