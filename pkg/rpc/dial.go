// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rpc

import (
	"context"
	"net"
	"strings"
	"time"

	"go.uber.org/zap"

	"storj.io/drpc/drpcconn"
	"storj.io/drpc/drpcmanager"
	"storj.io/drpc/drpcstream"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/rpc/rpcpool"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/private/memory"
)

// NewDefaultManagerOptions returns the default options we use for drpc managers.
func NewDefaultManagerOptions() drpcmanager.Options {
	return drpcmanager.Options{
		WriterBufferSize: 1024,
		Stream: drpcstream.Options{
			SplitSize: (4096 * 2) - 256,
		},
	}
}

// Dialer holds configuration for dialing.
type Dialer struct {
	// TLSOptions controls the tls options for dialing. If it is nil, only
	// insecure connections can be made.
	TLSOptions *tlsopts.Options

	// DialTimeout causes all the tcp dials to error if they take longer
	// than it if it is non-zero.
	DialTimeout time.Duration

	// DialLatency sleeps this amount if it is non-zero before every dial.
	// The timeout runs while the sleep is happening.
	DialLatency time.Duration

	// TransferRate limits all read/write operations to go slower than
	// the size per second if it is non-zero.
	TransferRate memory.Size

	// PoolOptions controls options for the connection pool.
	PoolOptions rpcpool.Options

	// ConnectionOptions controls the options that we pass to drpc connections.
	ConnectionOptions drpcconn.Options
}

// NewDefaultDialer returns a Dialer with default timeouts set.
func NewDefaultDialer(tlsOptions *tlsopts.Options) Dialer {
	return Dialer{
		TLSOptions:  tlsOptions,
		DialTimeout: 20 * time.Second,
		PoolOptions: rpcpool.Options{
			Capacity:       5,
			IdleExpiration: 2 * time.Minute,
		},
		ConnectionOptions: drpcconn.Options{
			Manager: NewDefaultManagerOptions(),
		},
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
		// N.B. this error is not wrapped on purpose! grpc code cares about inspecting
		// it and it's not smart enough to attempt to do any unwrapping. :( Additionally
		// DialContext does not return an error that can be inspected easily to see if it
		// came from the context being canceled. Thus, we do this racy thing where if the
		// context is canceled at this point, we return it, rather than return the error
		// from dialing. It's a slight lie, but arguably still correct because the cancel
		// must be racing with the dial anyway.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			return nil, err
		}
	}

	return &timedConn{
		Conn: conn,
		rate: d.TransferRate,
	}, nil
}

// DialNode creates an rpc connection to the specified node.
func (d Dialer) DialNode(ctx context.Context, node *pb.Node) (_ *Conn, err error) {
	defer mon.Task()(&ctx, "node: "+node.Id.String()[0:8])(&err)

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

// DialAddressInsecureBestEffort is like DialAddressInsecure but tries to dial a node securely if
// it can.
//
// nodeURL is like a storj.NodeURL but (a) requires an address and (b) does not require a
// full node id and will work with just a node prefix. The format is either:
//  * node_host:node_port
//  * node_id_prefix@node_host:node_port
// Examples:
//  * 33.20.0.1:7777
//  * [2001:db8:1f70::999:de8:7648:6e8]:7777
//  * 12vha9oTFnerx@33.20.0.1:7777
//  * 12vha9oTFnerx@[2001:db8:1f70::999:de8:7648:6e8]:7777
//
// DialAddressInsecureBestEffort:
//  * will use a node id if provided in the nodeURL paramenter
//  * will otherwise look up the node address in a known map of node address to node ids and use
// 		the remembered node id.
//  * will otherwise dial insecurely
func (d Dialer) DialAddressInsecureBestEffort(ctx context.Context, nodeURL string) (_ *Conn, err error) {
	defer mon.Task()(&ctx)(&err)

	if d.TLSOptions == nil {
		return nil, Error.New("tls options not set when required for this dial")
	}

	var nodeIDPrefix, nodeAddress string
	parts := strings.Split(nodeURL, "@")
	switch len(parts) {
	default:
		return nil, Error.New("malformed node url: %q", nodeURL)
	case 1:
		nodeAddress = parts[0]
	case 2:
		nodeIDPrefix, nodeAddress = parts[0], parts[1]
	}

	if len(nodeIDPrefix) > 0 {
		return d.dial(ctx, nodeAddress, d.TLSOptions.ClientTLSConfigPrefix(nodeIDPrefix))
	}

	if nodeID, found := KnownNodeID(nodeAddress); found {
		return d.dial(ctx, nodeAddress, d.TLSOptions.ClientTLSConfig(nodeID))
	}

	zap.S().Warnf("unknown node id for address %q: please specify node id in form node_id@node_host:node_port for added security", nodeAddress)
	return d.dial(ctx, nodeAddress, d.TLSOptions.UnverifiedClientTLSConfig())
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

	return d.dialUnencrypted(ctx, address)
}
