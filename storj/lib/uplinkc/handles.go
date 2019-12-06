// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"sync"
)

import "C"

// handle is a generic handle.
type handle = C.long

// handles stores different Go values that need to be accessed from Go side.
type handles struct {
	lock   sync.Mutex
	nextid handle
	values map[handle]interface{}
}

// newHandles creates a place to store go files by handle.
func newHandles() *handles {
	return &handles{
		values: make(map[handle]interface{}),
	}
}

// Add adds a value to the table.
func (m *handles) Add(x interface{}) handle {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.nextid++
	m.values[m.nextid] = x
	return m.nextid
}

// Get gets a value.
func (m *handles) Get(x handle) interface{} {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.values[x]
}

// Del deletes the value
func (m *handles) Del(x handle) {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.values, x)
}

// Empty returns whether the handles is empty.
func (m *handles) Empty() bool {
	m.lock.Lock()
	defer m.lock.Unlock()
	return len(m.values) == 0
}
