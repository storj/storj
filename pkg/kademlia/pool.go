// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"sync"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/transport"
)

// Pool is a kademlia connection pool.
type Pool struct {
	log       *zap.Logger
	transport transport.Client
	size      int

	limit  sync2.Semaphore
	mu     sync.Mutex
	closed bool
	recent []*Conn
}

// Conn represents a cached kademlia cache.
type Conn struct {
	pool     *Pool
	refcount int32 // only modify when holding pool.mu

	mu     sync.Mutex
	addr   string
	conn   *grpc.ClientConn
	client pb.NodesClient
}

// NewPool creates a connection pool for kademlia.
func NewPool(log *zap.Logger, transport transport.Client) *Pool {
	pool := &Pool{
		log:       log,
		transport: transport,
		size:      10,
	}
	pool.limit.Init(10)
	return pool
}

// Close closes the pool resources and prevents new connections to be made.
func (pool *Pool) Close() error {
	pool.limit.Close()
	return pool.disconnectAll()
}

// disconnectAll disconnects all the active and non-active connections.
func (pool *Pool) disconnectAll() error {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pool.closed = true

	// hide all connections to prevent new connections
	recent := pool.recent
	pool.recent = nil

	var errgroup errs.Group
	for _, conn := range recent {
		errgroup.Add(conn.disconnect())
	}

	return errgroup.Err()
}

// disconnectOne disconnects one connection
func (pool *Pool) disconnectOne() {
	n := len(pool.recent)
	for i := n - 1; i >= 0; i-- {
		conn := pool.recent[i]
		if conn.refcount == 0 {
			pool.recent = append(pool.recent[:i], pool.recent[i+1:]...)
			err := conn.disconnect()
			if err != nil {
				pool.log.Debug("error during closing connection", zap.Error(err))
			}
			return
		}
	}
	pool.log.Debug("did not find connection to drop, shouldn't happen")
}

// Lookup queries ask about find, and also sends information about self.
func (pool *Pool) Lookup(ctx context.Context, self pb.Node, ask pb.Node, find pb.Node) ([]*pb.Node, error) {
	if !pool.limit.Lock() {
		return nil, context.Canceled
	}
	defer pool.limit.Unlock()

	conn, err := pool.connect(ctx, ask)
	defer conn.release()
	if err != nil {
		return nil, err
	}

	resp, err := conn.client.Query(ctx, &pb.QueryRequest{
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

// Ping pings target.
func (pool *Pool) Ping(ctx context.Context, target pb.Node) (bool, error) {
	if !pool.limit.Lock() {
		return false, context.Canceled
	}
	defer pool.limit.Unlock()

	conn, err := pool.connect(ctx, target)
	defer conn.release()
	if err != nil {
		return false, err
	}

	_, err = conn.client.Ping(ctx, &pb.PingRequest{})

	// notify kademlia about success/failure

	return err == nil, err
}

// connect dials and adds target to cache.
// always call conn.release() after using the connection
func (pool *Pool) connect(ctx context.Context, target pb.Node) (*Conn, error) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	if pool.closed {
		return nil, context.Canceled
	}

	for i, conn := range pool.recent {
		// TODO: verify also other properties
		if conn.addr == target.GetAddress().Address {
			conn.acquireLocked()

			// this is a simplest way to implement recently used cache
			// avoiding random connections pushing important connections off
			k := i / 2
			pool.recent[k], pool.recent[i] = pool.recent[i], pool.recent[k]

			return conn, nil
		}
	}
	if len(pool.recent) >= pool.size {
		pool.disconnectOne()
	}

	conn, err := pool.dial(ctx, target)
	conn.acquireLocked()
	pool.recent = append(pool.recent, conn)
	return conn, err
}

// dial dials the specified node.
func (pool *Pool) dial(ctx context.Context, target pb.Node) (*Conn, error) {
	grpcconn, err := pool.transport.DialNode(ctx, &target)
	return &Conn{
		pool:     pool,
		mu:       sync.Mutex{},
		refcount: 0,
		addr:     target.GetAddress().Address,
		conn:     grpcconn,
		client:   pb.NewNodesClient(grpcconn),
	}, err
}

// acquireLocked increases refcount, requires holding pool.mu lock.
func (conn *Conn) acquireLocked() {
	conn.refcount++
}

// release releases the refcount of this connection.
// must always be called after acquireLocked
func (conn *Conn) release() {
	if conn == nil {
		return
	}
	conn.pool.mu.Lock()
	conn.refcount--
	conn.pool.mu.Unlock()
}

// disconnect disconnects this connection.
func (conn *Conn) disconnect() error {
	return conn.conn.Close()
}
