// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package node

import (
	"sync"

	"github.com/zeebo/errs"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/utils"
)

// Error defines a connection pool error
var Error = errs.Class("connection pool error")

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

// Disconnect closes the connection to the node and removes it from the connection pool
func (mp *ConnectionPool) Disconnect(ctx context.Context) error {
	var err error
	var errs []error
	for k, v := range mp.cache {
		conn, ok := v.(interface{ Close() error })
		if !ok {
			err = Error.New("connection pool value not a grpc client connection")
			errs = append(errs, err)
			continue
		}
		err = conn.Close()
		if err != nil {
			errs = append(errs, Error.Wrap(err))
			continue
		}
		err = mp.Remove(ctx, k)
		if err != nil {
			errs = append(errs, Error.Wrap(err))
			continue
		}
	}
	return utils.CombineErrors(errs...)
}
