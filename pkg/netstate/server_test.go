// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package netstate

import (
	"bytes"
	"os"
	"testing"

	"github.com/spf13/viper"

	"storj.io/storj/storage/boltdb"
)

const (
	API_KEY = "abc123"
)

func TestMain(m *testing.M) {
	viper.SetEnvPrefix("API")
	os.Setenv("API_KEY", API_KEY)
	viper.AutomaticEnv()
	os.Exit(m.Run())
}

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
