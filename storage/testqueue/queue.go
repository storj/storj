// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package testqueue

import (
	"container/list"
	"fmt"
	"sync"

	"storj.io/storj/storage"
)

//Queue is a threadsafe FIFO queue implementing storage.Queue
type Queue struct {
	mu sync.Mutex
	s  *list.List
}

//New returns a queue suitable for testing
func New() *Queue {
	return &Queue{s: list.New(), mu: sync.Mutex{}}
}

//Enqueue add a FIFO element
func (q *Queue) Enqueue(value storage.Value) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.s.PushBack(value)
	return nil
}

//Dequeue removes a FIFO element
func (q *Queue) Dequeue() (storage.Value, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for q.s.Len() > 0 {
		e := q.s.Front() // First element
		q.s.Remove(e)    // Dequeue
		return e.Value.(storage.Value), nil
	}
	return nil, fmt.Errorf("queue empty")
}

//Close closes the queue
func (q *Queue) Close() error {
	return nil
}
