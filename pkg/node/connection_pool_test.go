// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package node

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

type TestFoo struct {
	called string
}

func TestGet(t *testing.T) {
	cases := []struct {
		pool          *ConnectionPool
		key           string
		expected      TestFoo
		expectedError error
	}{
		{
			pool: func() *ConnectionPool {
				p := NewConnectionPool()
				p.Init()
				assert.NoError(t, p.Add("foo", TestFoo{called: "hoot"}))
				return p
			}(),
			key:           "foo",
			expected:      TestFoo{called: "hoot"},
			expectedError: nil,
		},
	}

	for i := range cases {
		v := &cases[i]
		test, err := v.pool.Get(v.key)
		assert.Equal(t, v.expectedError, err)
		assert.Equal(t, v.expected, test)
	}
}

func TestAdd(t *testing.T) {
	cases := []struct {
		pool          ConnectionPool
		key           string
		value         TestFoo
		expected      TestFoo
		expectedError error
	}{
		{
			pool: ConnectionPool{
				mu:    sync.RWMutex{},
				cache: map[string]interface{}{}},
			key:           "foo",
			value:         TestFoo{called: "hoot"},
			expected:      TestFoo{called: "hoot"},
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
	cases := []struct {
		pool          ConnectionPool
		key           string
		expected      interface{}
		expectedError error
	}{
		{
			pool: ConnectionPool{
				mu:    sync.RWMutex{},
				cache: map[string]interface{}{"foo": TestFoo{called: "hoot"}}},
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
