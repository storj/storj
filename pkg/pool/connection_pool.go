// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package pool

import (
	"context"
	"sync"
)

// ConnectionPool is the in memory implementation of a connection Pool
type ConnectionPool struct {
	mu    sync.RWMutex
	cache map[string]interface{}
}

// NewConnectionPool initializes a new in memory pool
func NewConnectionPool() Pool {
	return &ConnectionPool{}
}

// Add takes a node client and
func (mp *ConnectionPool) Add(ctx context.Context, key string, value interface{}) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.cache[key] = value

	return nil
}

// Get (TODO)
func (mp *ConnectionPool) Get(ctx context.Context, key string) (interface{}, error) {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	return mp.cache[key], nil
}

// Remove (TODO)
func (mp *ConnectionPool) Remove(ctx context.Context, key string) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.cache[key] = nil

	return nil
}
