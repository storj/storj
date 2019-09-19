// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build !drpc

package rpc

import (
	"context"
	"crypto/tls"
	"net"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// dial performs the dialing to the grpc endpoint with tls.
func (d Dialer) dial(ctx context.Context, address string, tlsConfig *tls.Config) (_ *Conn, err error) {
	defer mon.Task()(&ctx)(&err)

	creds := &captureStateCreds{TransportCredentials: credentials.NewTLS(tlsConfig)}
	conn, err := grpc.DialContext(ctx, address,
		grpc.WithTransportCredentials(creds),
		grpc.WithBlock(),
		grpc.FailOnNonTempDialError(true),
		grpc.WithContextDialer(d.dialContext))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	state, ok := creds.Get()
	if !ok {
		_ = conn.Close()
		return nil, Error.New("unable to get tls connection state when dialing")
	}

	return &Conn{
		raw:   conn,
		state: state,
	}, nil
}

// dialInsecure performs dialing to the grpc endpoint with no tls.
func (d Dialer) dialInsecure(ctx context.Context, address string) (_ *Conn, err error) {
	defer mon.Task()(&ctx)(&err)

	conn, err := grpc.DialContext(ctx, address,
		grpc.WithBlock(),
		grpc.FailOnNonTempDialError(true),
		grpc.WithContextDialer(d.dialContext))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &Conn{raw: conn}, nil
}

// captureStateCreds captures the tls connection state from a client/server handshake.
type captureStateCreds struct {
	credentials.TransportCredentials
	once  sync.Once
	state tls.ConnectionState
	ok    bool
}

// Get returns the stored tls connection state.
func (c *captureStateCreds) Get() (state tls.ConnectionState, ok bool) {
	c.once.Do(func() {})
	return c.state, c.ok
}

// ClientHandshake dispatches to the underlying credentials and tries to store the
// connection state if possible.
func (c *captureStateCreds) ClientHandshake(ctx context.Context, authority string, rawConn net.Conn) (
	net.Conn, credentials.AuthInfo, error) {

	conn, auth, err := c.TransportCredentials.ClientHandshake(ctx, authority, rawConn)
	if tlsInfo, ok := auth.(credentials.TLSInfo); ok {
		c.once.Do(func() { c.state, c.ok = tlsInfo.State, true })
	}
	return conn, auth, err
}

// ServerHandshake dispatches to the underlying credentials and tries to store the
// connection state if possible.
func (c *captureStateCreds) ServerHandshake(rawConn net.Conn) (
	net.Conn, credentials.AuthInfo, error) {

	conn, auth, err := c.TransportCredentials.ServerHandshake(rawConn)
	if tlsInfo, ok := auth.(credentials.TLSInfo); ok {
		c.once.Do(func() { c.state, c.ok = tlsInfo.State, true })
	}
	return conn, auth, err
}
