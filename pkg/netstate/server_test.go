// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package netstate

import (
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

	f := boltdb.File{
		Path:  string(path),
		Value: []byte("here is a value"),
	}

	return f, nil
}

func (m *mockDB) List() ([]string, error) {
	m.timesCalled++
	return nil, nil
}

func (m *mockDB) Delete([]byte) error {
	m.timesCalled++
	return nil
}
