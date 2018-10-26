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
func (p *ConnectionPool) Get(key string) (interface{}, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	i, ok := p.items[key]
	if !ok {
		return nil, nil
	}

	return i, nil
}

// Disconnect deletes a connection associated with the provided NodeID
func (p *ConnectionPool) Disconnect(key string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	i, ok := p.items[key]
	if !ok {
		return nil
	}

	delete(p.items, key)

	return i.grpc.Close()
}

// Dial connects to the node with the given ID and Address returning a gRPC Node Client
func (p *ConnectionPool) Dial(ctx context.Context, n *pb.Node) (pb.NodesClient, error) {
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

		conn.client = pb.NewNodesClient(conn.grpc)
	})

	if conn.err != nil {
		return nil, conn.err
	}

	return conn.client, nil
}

// DisconnectAll closes all connections nodes and removes them from the connection pool
func (p *ConnectionPool) DisconnectAll() error {
	errs := []error{}
	for k := range p.items {
		if err := p.Disconnect(k); err != nil {
			errs = append(errs, Error.Wrap(err))
			continue
		}
	}

	return utils.CombineErrors(errs...)
}

// Init initializes the cache
func (p *ConnectionPool) Init() {
	p.items = make(map[string]*Conn)
}
