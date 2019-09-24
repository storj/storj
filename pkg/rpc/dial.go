// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rpc

import (
	"context"
	"net"
	"time"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/storj"
)

// Dialer holds configuration for dialing.
type Dialer struct {
	// TLSOptions controls the tls options for dialing. If it is nil, only
	// insecure connections can be made.
	TLSOptions *tlsopts.Options

	// RequestTimeout causes any read/write operations on the raw socket
	// to error if they take longer than it if it is non-zero.
	RequestTimeout time.Duration

	// DialTimeout causes all the tcp dials to error if they take longer
	// than it if it is non-zero.
	DialTimeout time.Duration

	// DialLatency sleeps this amount if it is non-zero before every dial.
	// The timeout runs while the sleep is happening.
	DialLatency time.Duration

	// TransferRate limits all read/write operations to go slower than
	// the size per second if it is non-zero.
	TransferRate memory.Size
}

// NewDefaultDialer returns a Dialer with default timeouts set.
func NewDefaultDialer(tlsOptions *tlsopts.Options) Dialer {
	return Dialer{
		TLSOptions:     tlsOptions,
		RequestTimeout: 10 * time.Minute,
		DialTimeout:    20 * time.Second,
	}
}

// dialContext does a raw tcp dial to the address and wraps the connection with the
// provided timeout.
func (d Dialer) dialContext(ctx context.Context, address string) (net.Conn, error) {
	if d.DialTimeout > 0 {
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, d.DialTimeout)
		defer cancel()
	}

	if d.DialLatency > 0 {
		timer := time.NewTimer(d.DialLatency)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return nil, Error.Wrap(ctx.Err())
		}
	}

	conn, err := new(net.Dialer).DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &timedConn{
		Conn:    conn,
		timeout: d.RequestTimeout,
		rate:    d.TransferRate,
	}, nil
}

// DialNode creates an rpc connection to the specified node.
func (d Dialer) DialNode(ctx context.Context, node *pb.Node) (_ *Conn, err error) {
	defer mon.Task()(&ctx)(&err)

	if d.TLSOptions == nil {
		return nil, Error.New("tls options not set when required for this dial")
	}

	return d.dial(ctx, node.GetAddress().GetAddress(), d.TLSOptions.ClientTLSConfig(node.Id))
}

// DialAddressID dials to the specified address and asserts it has the given node id.
func (d Dialer) DialAddressID(ctx context.Context, address string, id storj.NodeID) (_ *Conn, err error) {
	defer mon.Task()(&ctx)(&err)

	if d.TLSOptions == nil {
		return nil, Error.New("tls options not set when required for this dial")
	}

	return d.dial(ctx, address, d.TLSOptions.ClientTLSConfig(id))
}

// DialAddressInsecure dials to the specified address and does not check the node id.
func (d Dialer) DialAddressInsecure(ctx context.Context, address string) (_ *Conn, err error) {
	defer mon.Task()(&ctx)(&err)

	if d.TLSOptions == nil {
		return nil, Error.New("tls options not set when required for this dial")
	}

	return d.dial(ctx, address, d.TLSOptions.UnverifiedClientTLSConfig())
}

// DialAddressUnencrypted dials to the specified address without tls.
func (d Dialer) DialAddressUnencrypted(ctx context.Context, address string) (_ *Conn, err error) {
	defer mon.Task()(&ctx)(&err)

	return d.dialInsecure(ctx, address)
}
