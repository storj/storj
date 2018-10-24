// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package pool

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"storj.io/storj/pkg/provider"
)

type TestFoo struct {
	called string
}

func TestGet(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		pool          *ConnectionPool
		key           string
		expected      TestFoo
		expectedError error
	}{
		{
			pool: func() *ConnectionPool {
				p := NewConnectionPool(newTestIdentity(t))
				assert.NoError(t, p.Add(ctx, "foo", TestFoo{called: "hoot"}))
				return p
			}(),
			key:           "foo",
			expected:      TestFoo{called: "hoot"},
			expectedError: nil,
		},
	}

	for i := range cases {
		v := &cases[i]
		test, err := v.pool.Get(ctx, v.key)
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
				items: map[string]*Conn{}},
			key:           "foo",
			value:         TestFoo{called: "hoot"},
			expected:      TestFoo{called: "hoot"},
			expectedError: nil,
		},
	}

	for i := range cases {
		v := &cases[i]
		err := v.pool.Add(context.Background(), v.key, v.value)
		assert.Equal(t, v.expectedError, err)

		test, err := v.pool.Get(context.Background(), v.key)
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
				items: map[string]*Conn{}},
			key:           "foo",
			expected:      nil,
			expectedError: nil,
		},
	}

	for i := range cases {
		v := &cases[i]
		err := v.pool.Remove(context.Background(), v.key)
		assert.Equal(t, v.expectedError, err)

		test, err := v.pool.Get(context.Background(), v.key)
		assert.Equal(t, v.expectedError, err)

		assert.Equal(t, v.expected, test)
	}
}

func newTestIdentity(t *testing.T) *provider.FullIdentity {
	ctx := context.Background()
	ca, err := provider.NewCA(ctx, 12, 4)
	assert.NoError(t, err)
	identity, err := ca.NewIdentity()
	assert.NoError(t, err)

	return identity
}
