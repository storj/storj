// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package routes

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"

	"go.uber.org/zap"
	"storj.io/storj/storage/boltdb"
)

type MockDB struct {
	timesCalled int
	puts        []boltdb.PointerEntry
	pathKeys    [][]byte
}

func (m *MockDB) Put(f boltdb.PointerEntry) error {
	m.timesCalled++
	m.puts = append(m.puts, f)
	return nil
}

func (m *MockDB) Get(path []byte) ([]byte, error) {
	m.timesCalled++

	for _, pointerEntry := range m.puts {
		if bytes.Equal(path, pointerEntry.Path) {
			return pointerEntry.Pointer, nil
		}
	}
	panic("failed to get the given file")
}

func (m *MockDB) List() ([][]byte, error) {
	m.timesCalled++

	for _, putReq := range m.puts {
		m.pathKeys = append(m.pathKeys, putReq.Path)
	}

	return m.pathKeys, nil
}

func (m *MockDB) Delete(path []byte) error {
	m.timesCalled++

	for i, pointerEntry := range m.puts {
		if bytes.Equal(path, pointerEntry.Path) {
			m.puts = append(m.puts[:i], m.puts[i+1:]...)
		}
	}

	return nil
}

func TestHttpNetState(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	mdb := &MockDB{
		timesCalled: 0,
	}

	ns := NewNetStateRoutes(logger, mdb)
	err := http.ListenAndServe(fmt.Sprintf(":%d", 10000), Start(ns))
	if err != nil {
		t.Error("Failed to listen and serve test")
	}
}
