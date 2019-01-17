// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package sync2

type Semaphore struct {
	queue chan struct{}
}

func NewSemaphore(size int) *Semaphore {
	sema := &Semaphore{}
	sema.Init(size)
	return sema
}
func (sema *Semaphore) Init(size int) {
	sema.queue = make(chan struct{}, size)
	for i := 0; i < size; i++ {
		sema.queue <- struct{}{}
	}
}

func (sema *Semaphore) Close() {
	close(sema.queue)
}

func (sema *Semaphore) Lock() bool {
	_, ok := <-sema.queue
	return ok
}

func (sema *Semaphore) Unlock() {
	select {
	case sema.queue <- struct{}{}:
	default:
		// this will only fail when the semaphore has been misused
		// or the semaphore is closed
	}
}
