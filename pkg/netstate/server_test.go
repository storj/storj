// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package netstate

import (
	"bytes"
	"os"

	"storj.io/storj/storage/boltdb"
)

// MockDB mocks db functionality for testing
type MockDB struct {
	timesCalled int
	puts        []boltdb.PointerEntry
	pathKeys    [][]byte
}

<<<<<<< HEAD
func (m *MockDB) Put(f boltdb.PointerEntry) error {
=======
func setEnv() {
	os.Setenv("API_KEY", "abc123")
}

func (m *mockDB) Put(f boltdb.File) error {
>>>>>>> work in progress for modifying test suite to accomodate credentials
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
