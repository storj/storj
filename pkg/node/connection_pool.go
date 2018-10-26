// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package node

import (
	"context"
	"sync"

	"github.com/zeebo/errs"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/pkg/utils"
)

// Error defines a connection pool error
var Error = errs.Class("connection pool error")

// ConnectionPool is the in memory pool of node connections
type ConnectionPool struct {
	tc    transport.Client
	mu    sync.RWMutex
	items map[string]*Conn
}

// Conn is the connection that is stored in the connection pool
type Conn struct {
	addr string

	dial   sync.Once
	client pb.NodesClient
	grpc   *grpc.ClientConn
	err    error
}

// NewConn intitalizes a new Conn struct with the provided address, but does not iniate a connection
func NewConn(addr string) *Conn { return &Conn{addr: addr} }

// NewConnectionPool initializes a new in memory pool
func NewConnectionPool(identity *provider.FullIdentity) *ConnectionPool {
	return &ConnectionPool{
		tc:    transport.NewClient(identity),
		items: make(map[string]*Conn),
		mu:    sync.RWMutex{},
	}
}

// Get retrieves a node connection with the provided nodeID
// nil is returned if the NodeID is not in the connection pool
func (pool *ConnectionPool) Get(key string) (interface{}, error) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	i, ok := pool.items[key]
	if !ok {
		return nil, nil
	}

	return i, nil
}

// Disconnect deletes a connection associated with the provided NodeID
func (pool *ConnectionPool) Disconnect(key string) error {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	i, ok := pool.items[key]
	if !ok {
		return nil
	}

	delete(pool.items, key)

	return i.grpc.Close()
}

// Dial connects to the node with the given ID and Address returning a gRPC Node Client
func (pool *ConnectionPool) Dial(ctx context.Context, n *pb.Node) (pb.NodesClient, error) {
	id := n.GetId()
	pool.mu.Lock()
	conn, ok := pool.items[id]
	if !ok {
		conn = NewConn(n.GetAddress().Address)
		pool.items[id] = conn
	}
	pool.mu.Unlock()

	conn.dial.Do(func() {
		conn.grpc, conn.err = pool.tc.DialNode(ctx, n)
		if conn.err != nil {
			return
		}

		conn.client = pb.NewNodesClient(conn.grpc)
	})

	if conn.err != nil {
		return nil, conn.err
	}

	return conn.client, nil
}

// DisconnectAll closes all connections nodes and removes them from the connection pool
func (pool *ConnectionPool) DisconnectAll() error {
	errs := []error{}
	for k := range pool.items {
		if err := pool.Disconnect(k); err != nil {
			errs = append(errs, Error.Wrap(err))
			continue
		}
	}

	return utils.CombineErrors(errs...)
}

// Init initializes the cache
func (pool *ConnectionPool) Init() {
	pool.items = make(map[string]*Conn)
}
