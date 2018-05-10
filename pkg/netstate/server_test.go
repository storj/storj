// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package netstate

import (
	"bytes"

	"storj.io/storj/storage/boltdb"
)

type mockDB struct {
	timesCalled int
	puts        []boltdb.File
}

func (m *mockDB) Put(f boltdb.File) error {
	m.timesCalled++
	m.puts = append(m.puts, f)
	return nil
}

func (m *mockDB) Get(path []byte) (boltdb.File, error) {
	m.timesCalled++

	for i := range m.puts {
		if bytes.Equal(path, m.puts[i].Path) {
			return m.puts[i], nil
		}
	}
	panic("Failed to get the given file")
}

func (m *mockDB) List() ([][]byte, error) {
	m.timesCalled++
	return nil, nil
}

func (m *mockDB) Delete([]byte) error {
	m.timesCalled++
	return nil
}
