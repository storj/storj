// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package pool

import (
	"context"
	"sync"

	"google.golang.org/grpc"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/transport"
)

// ConnectionPool is the in memory implementation of a connection Pool
type ConnectionPool struct {
	tc    transport.Client
	mu    sync.RWMutex
	items map[string]*Conn
}

// Conn is the connection that is stored in the connection pool
type Conn struct {
	Client pb.NodesClient
	grpc   *grpc.ClientConn
	addr   string
	dial   sync.Once
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

// Add takes a node ID as the key and a node client as the value to store
func (p *ConnectionPool) Add(ctx context.Context, key string, value interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	c, ok := value.(*Conn)
	if !ok {
		return PoolError.New("invalid value for connection pool")
	}

	p.items[key] = c

	return nil
}

// Get retrieves a node connection with the provided nodeID
// nil is returned if the NodeID is not in the connection pool
func (p *ConnectionPool) Get(ctx context.Context, key string) (interface{}, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	i, ok := p.items[key]
	if !ok {
		return nil, nil
	}

	return i, nil
}

// Remove deletes a connection associated with the provided NodeID
func (p *ConnectionPool) Remove(ctx context.Context, key string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	i, ok := p.items[key]
	if !ok {
		return nil
	}

	delete(p.items, key)

	return i.grpc.Close()
}

// Dial connects to the node with the given ID and Address
// Needs to only be called once, connection is left open
func (p *ConnectionPool) Dial(ctx context.Context, n *pb.Node) (*Conn, error) {
	id := n.GetId()
	p.mu.Lock()
	conn, ok := p.items[id]
	if !ok {
		conn = NewConn(n.GetAddress().Address)
		p.items[id] = conn
	}
	p.mu.Unlock()

	conn.dial.Do(func() {
		conn.grpc, conn.err = p.tc.DialNode(ctx, n)
		if conn.err != nil {
			return
		}

		conn.Client = pb.NewNodesClient(conn.grpc)
	})

	if conn.err != nil {
		return nil, conn.err
	}

	return conn, nil
}
