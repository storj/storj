// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"sync"

	"google.golang.org/grpc"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/transport"
)

type Pool struct {
	transport transport.Client
	size      int

	limit  sync2.Semaphore
	mu     sync.Mutex
	closed bool
	recent []*poolConn
}

type poolConn struct {
	refcount int32 // only modify when holding pool.mu

	mu     sync.Mutex
	err    error
	addr   string
	conn   *grpc.ClientConn
	client pb.NodesClient
}

func NewPool(transport transport.Client) *Pool {
	pool := &Pool{
		transport: transport,
		size:      10,
	}
	pool.limit.Init(10)
	return pool
}

func (pool *Pool) Close() error {
	pool.limit.Close()
	pool.disconnectAll()
	return nil
}

func (pool *Pool) disconnectAll() {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pool.closed = true

	// hide all connections to prevent new connections
	recent := pool.recent
	pool.recent = nil

	for _, conn := range recent {
		conn.conn.Close()
	}
}

func (pool *Pool) forceSize(ctx context.Context) error {
	n := len(pool.recent)
	if n > pool.size {
		conn := pool.recent[n-1]
		pool.recent[n-1] = nil
		pool.recent = pool.recent[:n-1]
		conn.client.Close()
	}
}

func (pool *Pool) release(conn *poolConn) error {
	pool.mu.Lock()
	conn.refcount--
	pool.mu.Unlock()
	return err
}

func (pool *Pool) releaser(conn *poolConn) func() error {
	return func() error { return pool.release(conn) }
}

func (pool *Pool) connect(ctx context.Context, to pb.Node) (*poolConn, func() error, error) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	if pool.closed {
		return nil, func() error { return nil }, context.Canceled
	}

	for i, conn := range pool.recent {
		// TODO: verify also other properties
		if conn.addr == to.GetAddress().Address {
			conn.refcount++
			k := i / 2
			pool.recent[k], pool.recent[i] = pool.recent[i], pool.recent[k]
			return conn, pool.releaser(conn), err
		}
	}
	pool.forceSize()

	grpcconn, err := pool.transport.Dial(ctx, to)
	conn := &poolConn{
		mu:       sync.Mutex{},
		refcount: 1,
		addr:     to.GetAddress().Address,
		conn:     grpcconn,
		client:   pb.NewNodesClient(conn),
	}
	pool.recent = append(pool.recent, conn)

	return conn, pool.releaser(conn), err
}

func (pool *Pool) Lookup(ctx context.Context, self pb.Node, ask pb.Node, find pb.Node) ([]*pb.Node, error) {
	if !pool.limit.Lock() {
		return nil, context.Canceled
	}
	defer pool.limit.Unlock()

	conn, release, err := pool.connect(ask)
	defer release()
	if err != nil {
		return nil, err
	}

	resp, err := conn.Query(ctx, &pb.QueryRequest{
		Limit:    20,
		Sender:   &self,
		Target:   &find,
		Pingback: true,
	})

	if err != nil {
		return nil, err
	}

	// notify kademlia about success/failure

	return resp.Response, nil
}

func (pool *Pool) Ping(ctx context.Context, ask pb.Node, find pb.Node) ([]*pb.Node, error) {
	if !pool.limit.Lock() {
		return nil, context.Canceled
	}
	defer pool.limit.Unlock()

	conn, release, err := pool.connect(ask)
	defer release()
	if err != nil {
		return nil, err
	}

	_, err = conn.Ping(ctx, &pb.PingRequest{})

	// notify kademlia about success/failure

	return err == nil, err
}
