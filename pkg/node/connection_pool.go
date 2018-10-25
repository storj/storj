// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package node

import (
	"sync"
)

// ConnectionPool is the in memory implementation of a connection Pool
type ConnectionPool struct {
	mu    sync.RWMutex
	cache map[string]interface{}
}

// NewConnectionPool initializes a new in memory pool
func NewConnectionPool() *ConnectionPool {
	return &ConnectionPool{
		cache: make(map[string]interface{}),
		mu:    sync.RWMutex{},
	}
}

// Add takes a node ID as the key and a node client as the value to store
func (pool *ConnectionPool) Add(key string, value interface{}) error {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	pool.cache[key] = value
	return nil
}

// Get retrieves a node connection with the provided nodeID
// nil is returned if the NodeID is not in the connection pool
func (pool *ConnectionPool) Get(key string) (interface{}, error) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	return pool.cache[key], nil
}

// Remove deletes a connection associated with the provided NodeID
func (pool *ConnectionPool) Remove(key string) error {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	pool.cache[key] = nil
	return nil
}
