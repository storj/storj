// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package sync2

import "sync"

// Semaphore implements a closable semaphore
type Semaphore struct {
	close sync.Once
	queue chan struct{}
}

// NewSemaphore creates a semaphore with the specified size.
func NewSemaphore(size int) *Semaphore {
	sema := &Semaphore{}
	sema.Init(size)
	return sema
}

// Init initializes semaphore to the specified size.
func (sema *Semaphore) Init(size int) {
	sema.queue = make(chan struct{}, size)
}

// Close closes the semaphore from further use.
func (sema *Semaphore) Close() {
	sema.close.Do(func() {
		close(sema.queue)
	})
}

// Lock locks the semaphore.
func (sema *Semaphore) Lock() bool {
	defer func() {
		_ = recover()
	}()

	sema.queue <- struct{}{}
	return true
}

// Unlock unlocks the semaphore.
func (sema *Semaphore) Unlock() {
	select {
	case <-sema.queue:
	default:
		// this will only fail when the semaphore has been misused
		// or the semaphore is closed
	}
}
