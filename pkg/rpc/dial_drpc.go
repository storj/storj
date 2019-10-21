// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build drpc

package rpc

import (
	"context"
	"crypto/tls"

	"storj.io/drpc/drpcconn"
)

const drpcHeader = "DRPC!!!1"

// dial performs the dialing to the drpc endpoint with tls.
func (d Dialer) dial(ctx context.Context, address string, tlsConfig *tls.Config) (_ *Conn, err error) {
	defer mon.Task()(&ctx)(&err)

	// open the tcp socket to the address
	rawConn, err := d.dialContext(ctx, address)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	// write the header bytes before the tls handshake
	if _, err := rawConn.Write([]byte(drpcHeader)); err != nil {
		_ = rawConn.Close()
		return nil, Error.Wrap(err)
	}

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

	return &Conn{
		raw:   drpcconn.New(conn),
		state: conn.ConnectionState(),
	}, nil
}

// dialInsecure performs dialing to the drpc endpoint with no tls.
func (d Dialer) dialInsecure(ctx context.Context, address string) (_ *Conn, err error) {
	defer mon.Task()(&ctx)(&err)

	// open the tcp socket to the address
	conn, err := d.dialContext(ctx, address)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	// write the header bytes before the tls handshake
	if _, err := conn.Write([]byte(drpcHeader)); err != nil {
		_ = conn.Close()
		return nil, Error.Wrap(err)
	}

	return &Conn{raw: drpcconn.New(conn)}, nil
}
