// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rpc

import (
	"crypto/tls"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/rpc/rpcpool"
)

// RawConn is a type alias to a drpc client connection
type RawConn = rpcpool.Conn

// Conn is a wrapper around a drpc client connection.
type Conn struct {
	state tls.ConnectionState
	raw   *RawConn
}

// Close closes the connection.
func (c *Conn) Close() error { return c.raw.Close() }

// ConnectionState returns the tls connection state.
func (c *Conn) ConnectionState() tls.ConnectionState { return c.state }

// PeerIdentity returns the peer identity on the other end of the connection.
func (c *Conn) PeerIdentity() (*identity.PeerIdentity, error) {
	return identity.PeerIdentityFromChain(c.state.PeerCertificates)
}

// Raw returns the underlying connection.
func (c *Conn) Raw() *RawConn {
	return c.raw
}
