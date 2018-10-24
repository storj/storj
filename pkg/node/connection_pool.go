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
func (mp *ConnectionPool) Add(key string, value interface{}) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.cache[key] = value
	return nil
}

// Get retrieves a node connection with the provided nodeID
// nil is returned if the NodeID is not in the connection pool
func (mp *ConnectionPool) Get(key string) (interface{}, error) {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	return mp.cache[key], nil
}

// Remove deletes a connection associated with the provided NodeID
func (mp *ConnectionPool) Remove(key string) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.cache[key] = nil
	return nil
}
