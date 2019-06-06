// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"sync"
)

// #include <stdlib.h>
import "C"

type mapping struct {
	lock   sync.Mutex
	values map[token]interface{}
}

func newMapping() *mapping {
	return &mapping{
		values: make(map[token]interface{}),
	}
}

type token uintptr

func (m *mapping) Add(x interface{}) token {
	res := token(C.malloc(1))

	m.lock.Lock()
	m.values[res] = x
	m.lock.Unlock()

	return res
}

func (m *mapping) Get(x token) interface{} {
	m.lock.Lock()
	res := m.values[x]
	m.lock.Unlock()

	return res
}

func (m *mapping) Del(x token) {
	m.lock.Lock()
	delete(m.values, x)
	m.lock.Unlock()
}