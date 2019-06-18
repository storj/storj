// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"sync"
)

import "C"

var universe = NewUniverse()

// Handle is a generic handle.
type Handle = C.long

// Universe stores different Go values that need to be accessed from Go side.
type Universe struct {
	lock   sync.Mutex
	nextid Handle
	values map[Handle]interface{}
}

// NewUniverse creates a place to store go files by handle.
func NewUniverse() *Universe {
	return &Universe{
		values: make(map[Handle]interface{}),
	}
}

// Add adds a value to the table.
func (m *Universe) Add(x interface{}) Handle {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.nextid++
	m.values[m.nextid] = x
	return m.nextid
}

// Get gets a value.
func (m *Universe) Get(x Handle) interface{} {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.values[x]
}

// Del deletes the value
func (m *Universe) Del(x Handle) {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.values, x)
}

// Empty returns whether the universe is empty.
func (m *Universe) Empty() bool {
	m.lock.Lock()
	defer m.lock.Unlock()
	return len(m.values) == 0
}

//export internal_UniverseIsEmpty
// internal_UniverseIsEmpty returns true if nothing is stored in the global map.
func internal_UniverseIsEmpty() bool {
	return universe.Empty()
}
