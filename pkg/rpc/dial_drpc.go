// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build !grpc

package rpc

import (
	"context"
	"crypto/tls"
	"net"
	"sync"

	"storj.io/drpc"
	"storj.io/drpc/drpcconn"
	"storj.io/storj/pkg/rpc/rpcpool"
)

// drpcHeader is the first bytes we send on a connection so that the remote
// knows to expect drpc on the wire instead of grpc.
const drpcHeader = "DRPC!!!1"

// dial performs the dialing to the drpc endpoint with tls.
func (d Dialer) dial(ctx context.Context, address string, tlsConfig *tls.Config) (_ *Conn, err error) {
	defer mon.Task()(&ctx)(&err)

	pool := rpcpool.New(d.PoolOptions, func(ctx context.Context) (drpc.Transport, error) {
		return d.dialTransport(ctx, address, tlsConfig)
	})

	conn, err := d.dialTransport(ctx, address, tlsConfig)
	if err != nil {
		return nil, err
	}
	state := conn.ConnectionState()

	if err := pool.Put(drpcconn.New(conn)); err != nil {
		return nil, err
	}

	return &Conn{
		state: state,
		raw:   pool,
	}, nil
}

// dialTransport performs dialing to the drpc endpoint with tls.
func (d Dialer) dialTransport(ctx context.Context, address string, tlsConfig *tls.Config) (_ *tlsConnWrapper, err error) {
	defer mon.Task()(&ctx)(&err)

	// open the tcp socket to the address
	rawConn, err := d.dialContext(ctx, address)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	rawConn = newDrpcHeaderConn(rawConn)

	// perform the handshake racing with the context closing. we use a buffer
	// of size 1 so that the handshake can proceed even if no one is reading.
	errCh := make(chan error, 1)
	conn := tls.Client(rawConn, tlsConfig)
	go func() { errCh <- conn.Handshake() }()

	// see which wins and close the raw conn if there was any error. we can't
	// close the tls connection concurrently with handshakes or it sometimes
	// will panic. cool, huh?
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case err = <-errCh:
	}
	if err != nil {
		_ = rawConn.Close()
		return nil, Error.Wrap(err)
	}

	return &tlsConnWrapper{
		Conn:       conn,
		underlying: rawConn,
	}, nil
}

// dialUnencrypted performs dialing to the drpc endpoint with no tls.
func (d Dialer) dialUnencrypted(ctx context.Context, address string) (_ *Conn, err error) {
	defer mon.Task()(&ctx)(&err)

	return &Conn{
		raw: rpcpool.New(d.PoolOptions, func(ctx context.Context) (drpc.Transport, error) {
			return d.dialTransportUnencrypted(ctx, address)
		}),
	}, nil
}

// dialTransportUnencrypted performs dialing to the drpc endpoint with no tls.
func (d Dialer) dialTransportUnencrypted(ctx context.Context, address string) (_ net.Conn, err error) {
	defer mon.Task()(&ctx)(&err)

	// open the tcp socket to the address
	conn, err := d.dialContext(ctx, address)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return newDrpcHeaderConn(conn), nil
}

// tlsConnWrapper is a wrapper around a *tls.Conn that calls Close on the
// underlying connection when closed rather than trying to send a
// notification to the other side which may block forever.
type tlsConnWrapper struct {
	*tls.Conn
	underlying net.Conn
}

// Close closes the underlying connection
func (t *tlsConnWrapper) Close() error { return t.underlying.Close() }

// drpcHeaderConn fulfills the net.Conn interface. On the first call to Write
// it will write the drpcHeader.
type drpcHeaderConn struct {
	net.Conn
	once sync.Once
}

// newDrpcHeaderConn returns a new *drpcHeaderConn
func newDrpcHeaderConn(conn net.Conn) *drpcHeaderConn {
	return &drpcHeaderConn{
		Conn: conn,
	}
}

// Write will write buf to the underlying conn. If this is the first time Write
// is called it will prepend the drpcHeader to the beginning of the write.
func (d *drpcHeaderConn) Write(buf []byte) (n int, err error) {
	var didOnce bool
	d.once.Do(func() {
		didOnce = true
		header := []byte(drpcHeader)
		n, err = d.Conn.Write(append(header, buf...))
	})
	if didOnce {
		n -= len(drpcHeader)
		if n < 0 {
			n = 0
		}
		return n, err
	}
	return d.Conn.Write(buf)
}
