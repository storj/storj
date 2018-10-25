// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package node

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

type TestFoo struct {
	called string
}

func TestGet(t *testing.T) {
	cases := []struct {
		pool          *ConnectionPool
		key           string
		expected      Conn
		expectedError error
	}{
		{
			pool: func() *ConnectionPool {
				p := NewConnectionPool(newTestIdentity(t))
				assert.NoError(t, p.Add("foo", &Conn{addr: "foo"}))
				return p
			}(),
			key:           "foo",
			expected:      Conn{addr: "foo"},
			expectedError: nil,
		},
	}

	for i := range cases {
		v := &cases[i]
		test, err := v.pool.Get(v.key)
		assert.Equal(t, v.expectedError, err)

		assert.Equal(t, v.expected.addr, test.(*Conn).addr)
	}
}

func TestAdd(t *testing.T) {
	cases := []struct {
		pool          ConnectionPool
		key           string
		value         *Conn
		expected      *Conn
		expectedError error
	}{
		{
			pool: ConnectionPool{
				mu:    sync.RWMutex{},
				items: map[string]*Conn{}},
			key:           "foo",
			value:         &Conn{addr: "hoot"},
			expected:      &Conn{addr: "hoot"},
			expectedError: nil,
		},
	}

	for i := range cases {
		v := &cases[i]
		err := v.pool.Add(v.key, v.value)
		assert.Equal(t, v.expectedError, err)

		test, err := v.pool.Get(v.key)
		assert.Equal(t, v.expectedError, err)

		assert.Equal(t, v.expected, test)
	}
}

func TestRemove(t *testing.T) {

	conn, err := grpc.Dial("127.0.0.1:0", grpc.WithInsecure())
	assert.NoError(t, err)
	// gc.Close = func() error { return nil }
	cases := []struct {
		pool          ConnectionPool
		key           string
		expected      interface{}
		expectedError error
	}{
		{
			pool: ConnectionPool{
				mu:    sync.RWMutex{},
				items: map[string]*Conn{"foo": &Conn{grpc: conn}},
			},
			key:           "foo",
			expected:      nil,
			expectedError: nil,
		},
	}

	for i := range cases {
		v := &cases[i]
		err := v.pool.Remove(v.key)
		assert.Equal(t, v.expectedError, err)

		test, err := v.pool.Get(v.key)
		assert.Equal(t, v.expectedError, err)

		assert.Equal(t, v.expected, test)
	}
}
