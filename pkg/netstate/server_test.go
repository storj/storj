// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package netstate

import (
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"storj.io/storj/storage/boltdb"
)

type mockDB struct {
	timesCalled int
}

func TestNetStateServer(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 8080))
	assert.NoError(t, err)

	srv := NewServer(logger, mockDB{
		timesCalled: 0,
	})
	assert.NotNil(t, srv)

	go srv.Serve(lis)
	srv.Stop()
}

func (m mockDB) Put(boltdb.File) error {
	m.timesCalled++
	return nil
}

func (m mockDB) Get(path []byte) (boltdb.File, error) {
	m.timesCalled++

	f := boltdb.File{
		Path:  string(path),
		Value: []byte("here is a value"),
	}

	return f, nil
}

func (m mockDB) List() ([]string, error) {
	m.timesCalled++
	return nil, nil
}

func (m mockDB) Delete([]byte) error {
	m.timesCalled++
	return nil
}
