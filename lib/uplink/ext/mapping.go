// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"sync"
)

// #include <stdlib.h>
import "C"

type Token uintptr

type Mapping struct {
	lock   sync.Mutex
	values map[Token]interface{}
}

func NewMapping() *Mapping {
	return &Mapping{
		values: make(map[Token]interface{}),
	}
}

func (m *Mapping) Add(x interface{}) Token {
	res := Token(C.malloc(1))

	m.lock.Lock()
	m.values[res] = x
	m.lock.Unlock()

	return res
}

func (m *Mapping) Get(x Token) interface{} {
	m.lock.Lock()
	res := m.values[x]
	m.lock.Unlock()

	return res
}

func (m *Mapping) Del(x Token) {
	m.lock.Lock()
	delete(m.values, x)
	m.lock.Unlock()
}
