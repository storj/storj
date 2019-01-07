// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package sync2

import (
	"runtime"
	"sync"
	"sync/atomic"
)

// Fence allows to wait for something to happen.
type Fence struct {
	status uint32
	wait   sync.Mutex
}

/*
General flow of the fence:

init:
	first arriver caller to `init` will setup a lock in `wait`

Wait callers:
	try to lock/unlock in Wait, this will block until the initial lock will be released

Release caller:
	first caller will release the initial lock
*/

const (
	statusUninitialized = iota
	statusInitializing
	statusBlocked
	statusReleased
)

// init sets up the initial lock into wait
func (fence *Fence) init() {
	// wait for initialization
	for atomic.LoadUint32(&fence.status) <= statusInitializing {
		// first arriver sets up lock
		if atomic.CompareAndSwapUint32(&fence.status, statusUninitialized, statusInitializing) {
			fence.wait.Lock()
			atomic.StoreUint32(&fence.status, statusBlocked)
		} else {
			runtime.Gosched()
		}
	}
}

// Wait waits for wait to be unlocked
func (fence *Fence) Wait() {
	// fast-path
	if fence.Released() {
		return
	}
	fence.init()
	// start waiting on the initial lock to be released
	fence.wait.Lock()
	// intentionally empty critical section to wait for Release
	//nolint
	fence.wait.Unlock()
}

// Released returns whether the fence has been released.
func (fence *Fence) Released() bool {
	return atomic.LoadUint32(&fence.status) >= statusReleased
}

// Release releases everyone from Wait
func (fence *Fence) Release() {
	fence.init()
	// the first one releases the status
	if atomic.CompareAndSwapUint32(&fence.status, statusBlocked, statusReleased) {
		fence.wait.Unlock()
	}
}
