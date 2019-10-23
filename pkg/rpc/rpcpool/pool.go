// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rpcpool

import (
	"context"
	"sync"

	"github.com/zeebo/errs"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/drpc"
	"storj.io/drpc/drpcconn"
)

var mon = monkit.Package()

// Error is the class of errors returned by this package.
var Error = errs.Class("rpcpool")

// Dialer is the type of function to create a new connection.
type Dialer = func(context.Context) (drpc.Transport, error)

// Conn implements drpc.Conn but keeps a pool of connections open.
type Conn struct {
	mu   sync.Mutex
	pool chan *drpcconn.Conn
	done chan struct{}
	dial Dialer
}

var _ drpc.Conn = (*Conn)(nil)

// New returns a new Conn that will keep cap connections open using the provided
// dialer when it needs new ones.
func New(cap int, dial Dialer) *Conn {
	return &Conn{
		pool: make(chan *drpcconn.Conn, cap),
		done: make(chan struct{}),
		dial: dial,
	}
}

// Close closes all of the pool's connections and ensures no new ones will be made.
func (c *Conn) Close() (err error) {
	var pool chan *drpcconn.Conn

	// only one call will ever see a non-nil pool variable. additionally, anyone
	// holding the mutex will either see a nil c.pool or a non-closed c.pool.
	c.mu.Lock()
	pool, c.pool = c.pool, nil
	c.mu.Unlock()

	if pool != nil {
		close(pool)
		for conn := range pool {
			err = errs.Combine(err, conn.Close())
		}
		close(c.done)
	}

	<-c.done
	return err
}

// newConn creates a new connection using the dialer.
func (c *Conn) newConn(ctx context.Context) (_ *drpcconn.Conn, err error) {
	defer mon.Task()(&ctx)(&err)

	tr, err := c.dial(ctx)
	if err != nil {
		return nil, err
	}
	return drpcconn.New(tr), nil
}

// getConn attempts to get a pooled connection or dials a new one if necessary.
func (c *Conn) getConn(ctx context.Context) (_ *drpcconn.Conn, err error) {
	defer mon.Task()(&ctx)(&err)

	c.mu.Lock()
	pool := c.pool
	c.mu.Unlock()

	for {
		select {
		case conn, ok := <-pool:
			if !ok {
				return nil, Error.New("connection pool closed")
			}

			// if the connection died in the pool, try again
			if conn.Closed() {
				continue
			}

			return conn, nil
		default:
			return c.newConn(ctx)
		}
	}
}

// Put places the connection back into the pool if there's room. It
// closes the connection if there is no room or the pool is closed. If the
// connection is closed, it does not attempt to place it into the pool.
func (c *Conn) Put(conn *drpcconn.Conn) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// if the connection is closed already, don't replace it.
	if conn.Closed() {
		return nil
	}

	select {
	case c.pool <- conn:
		return nil
	default:
		return conn.Close()
	}
}

// Transport returns nil because there is no well defined transport to use.
func (c *Conn) Transport() drpc.Transport { return nil }

// Invoke implements drpc.Conn's Invoke method using a pooled connection.
func (c *Conn) Invoke(ctx context.Context, rpc string, in drpc.Message, out drpc.Message) (err error) {
	defer mon.Task()(&ctx)(&err)

	conn, err := c.getConn(ctx)
	if err != nil {
		return err
	}
	err = conn.Invoke(ctx, rpc, in, out)
	return errs.Combine(err, c.Put(conn))
}

// NewStream implements drpc.Conn's NewStream method using a pooled connection. It
// waits for the stream to be finished before replacing the connection into the pool.
func (c *Conn) NewStream(ctx context.Context, rpc string) (_ drpc.Stream, err error) {
	defer mon.Task()(&ctx)(&err)

	conn, err := c.getConn(ctx)
	if err != nil {
		return nil, err
	}

	stream, err := conn.NewStream(ctx, rpc)
	if err != nil {
		return nil, err
	}

	// the stream's done channel is closed when we're sure no reads/writes are
	// coming in for that stream anymore. it has been fully terminated.
	go func() {
		<-stream.Context().Done()
		_ = c.Put(conn)
	}()

	return stream, nil
}
