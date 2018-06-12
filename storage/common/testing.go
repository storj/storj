// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.
package storage

import (
"errors"
)

type MockStorageClient struct {
	Data         map[string][]byte
	GetCalled    int
	PutCalled    int
	ListCalled   int
	DeleteCalled int
	CloseCalled  int
	PingCalled   int
}

// ErrMissingKey is the error returned if a key is not in the mock store
var ErrMissingKey = errors.New("missing")

// ErrForced is the error returned when the forced error flag is passed to mock an error
var ErrForced = errors.New("error forced by using 'error' key in mock")

func (m *MockStorageClient) Get(key []byte) ([]byte, error) {
	m.GetCalled++
	if string(key) == "error" {
		return []byte{}, ErrForced
	}
	v, ok := m.Data[string(key)]
	if !ok {
		return []byte{}, ErrMissingKey
	}

	return v, nil
}

func (m *MockStorageClient) Put(key, value []byte) error {
	m.PutCalled++
	m.Data[string(key)] = value
	return nil
}

func (m *MockStorageClient) Delete(key []byte) error {
	m.DeleteCalled++
	delete(m.Data, string(key))
	return nil
}

func (m *MockStorageClient) List() (_ [][]byte, _ error) {
	m.ListCalled++
	keys := [][]byte{}
	for k := range m.Data {
		keys = append(keys, []byte(k))
	}

	return keys, nil
}

func (m *MockStorageClient) Close() error {
	m.CloseCalled++
	return nil
}

func (m *MockStorageClient) Ping() error {
	m.PingCalled++
	return nil
}

func NewMockStorageClient(d map[string][]byte) *MockStorageClient {
	return &MockStorageClient{
		Data:         d,
		GetCalled:    0,
		PutCalled:    0,
		ListCalled:   0,
		DeleteCalled: 0,
		CloseCalled:  0,
		PingCalled:   0,
	}
}
