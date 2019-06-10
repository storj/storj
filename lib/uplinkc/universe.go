// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"sync"
)

var universe = NewUniverse()

type Ref uint64

type Universe struct {
	lock    sync.Mutex
	nextid  Ref
	values  map[Ref]interface{}
}

func NewUniverse() *Universe {
	return &Universe{
		values: make(map[Ref]interface{}),
	}
}

func (m *Universe) Add(x interface{}) Ref {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.nextid++
	m.values[m.nextid] = x
	return m.nextid
}

func (m *Universe) Get(x Ref) interface{} {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.values[x]
}

func (m *Universe) Del(x Ref) {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.values, x)
}
