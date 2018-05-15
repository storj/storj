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
	filePaths   [][]byte
}

func (m *mockDB) Put(f boltdb.File) error {
	m.timesCalled++
	m.puts = append(m.puts, f)
	return nil
}

func (m *mockDB) Get(path []byte) ([]byte, error) {
	m.timesCalled++

	for _, file := range m.puts {
		if bytes.Equal(path, file.Path) {
			return file.Value, nil
		}
	}
	panic("failed to get the given file")
}

func (m *mockDB) List() ([][]byte, error) {
	m.timesCalled++

	for _, file := range m.puts {
		m.filePaths = append(m.filePaths, file.Path)
	}

	return m.filePaths, nil
}

func (m *mockDB) Delete(path []byte) error {
	m.timesCalled++

	for i, file := range m.puts {
		if bytes.Equal(path, file.Path) {
			m.puts = append(m.puts[:i], m.puts[i+1:]...)
		}
	}

	return nil
}
