// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademliaclient

import (
	"context"
	"crypto/tls"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/drpc"
	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
)

var mon = monkit.Package()

// Dialer sends requests to kademlia endpoints on storage nodes
type Dialer struct {
	log       *zap.Logger
	transport transport.Client
	limit     sync2.Semaphore
}

// Conn represents a connection
type Conn struct {
	raw    *tls.Conn
	conn   drpc.Conn
	client pb.DRPCNodesClient
}

// NewDialer creates a new kademlia dialer.
func NewDialer(log *zap.Logger, transport transport.Client) *Dialer {
	dialer := &Dialer{
		log:       log,
		transport: transport,
	}
	dialer.limit.Init(32) // TODO: limit should not be hardcoded
	return dialer
}

// Close closes the pool resources and prevents new connections to be made.
func (dialer *Dialer) Close() error {
	dialer.limit.Close()
	return nil
}

// Lookup queries ask about find, and also sends information about self.
// If self is nil, pingback will be false.
func (dialer *Dialer) Lookup(ctx context.Context, self *pb.Node, ask pb.Node, find storj.NodeID, limit int) (_ []*pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)
	if !dialer.limit.Lock() {
		return nil, context.Canceled
	}
	defer dialer.limit.Unlock()

	req := pb.QueryRequest{
		Limit:  int64(limit),
		Target: &pb.Node{Id: find}, // TODO: should not be a Node protobuf!
	}
	if self != nil {
		req.Pingback = true
		req.Sender = self
	}

	conn, err := dialer.dialNode(ctx, ask)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, conn.Close()) }()

	resp, err := conn.client.Query(ctx, &req)
	if err != nil {
		return nil, err
	}

	return resp.Response, nil
}

// PingNode pings target.
func (dialer *Dialer) PingNode(ctx context.Context, target pb.Node) (_ bool, err error) {
	defer mon.Task()(&ctx)(&err)
	if !dialer.limit.Lock() {
		return false, context.Canceled
	}
	defer dialer.limit.Unlock()

	conn, err := dialer.dialNode(ctx, target)
	if err != nil {
		return false, err
	}

	_, err = conn.client.Ping(ctx, &pb.PingRequest{})
	return err == nil, errs.Combine(err, conn.Close())
}

// FetchPeerIdentity connects to a node and returns its peer identity
func (dialer *Dialer) FetchPeerIdentity(ctx context.Context, target pb.Node) (_ *identity.PeerIdentity, err error) {
	defer mon.Task()(&ctx)(&err)
	if !dialer.limit.Lock() {
		return nil, context.Canceled
	}
	defer dialer.limit.Unlock()

	conn, err := dialer.dialNode(ctx, target)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, conn.Close()) }()

	return identity.PeerIdentityFromChain(conn.raw.ConnectionState().PeerCertificates)
}

// FetchPeerIdentityUnverified connects to an address and returns its peer identity (no node ID verification).
func (dialer *Dialer) FetchPeerIdentityUnverified(ctx context.Context, address string) (_ *identity.PeerIdentity, err error) {
	defer mon.Task()(&ctx)(&err)
	if !dialer.limit.Lock() {
		return nil, context.Canceled
	}
	defer dialer.limit.Unlock()

	conn, err := dialer.dialAddress(ctx, address)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, conn.Close()) }()

	return identity.PeerIdentityFromChain(conn.raw.ConnectionState().PeerCertificates)
}

// FetchInfo connects to a node and returns its node info.
func (dialer *Dialer) FetchInfo(ctx context.Context, target pb.Node) (_ *pb.InfoResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	if !dialer.limit.Lock() {
		return nil, context.Canceled
	}
	defer dialer.limit.Unlock()

	conn, err := dialer.dialNode(ctx, target)
	if err != nil {
		return nil, err
	}

	resp, err := conn.client.RequestInfo(ctx, &pb.InfoRequest{})
	return resp, errs.Combine(err, conn.Close())
}

// AlertSuccess alerts the transport observers of a successful connection
func (dialer *Dialer) AlertSuccess(ctx context.Context, node *pb.Node) {
	dialer.transport.AlertSuccess(ctx, node)
}

// dialNode dials the specified node.
func (dialer *Dialer) dialNode(ctx context.Context, target pb.Node) (_ *Conn, err error) {
	defer mon.Task()(&ctx)(&err)

	conn, err := dialer.transport.DialNode(ctx, &target)
	if err != nil {
		return nil, err
	}
	return &Conn{
		// TODO(jeff): this doesn't always work and is fragile
		raw:    conn.Transport().(*tls.Conn),
		conn:   conn,
		client: pb.NewDRPCNodesClient(conn),
	}, nil
}

// dialAddress dials the specified node by address (no node ID verification)
func (dialer *Dialer) dialAddress(ctx context.Context, address string) (_ *Conn, err error) {
	defer mon.Task()(&ctx)(&err)

	conn, err := dialer.transport.DialAddress(ctx, address)
	if err != nil {
		return nil, err
	}
	return &Conn{
		// TODO(jeff): this doesn't always work and is fragile
		raw:    conn.Transport().(*tls.Conn),
		conn:   conn,
		client: pb.NewDRPCNodesClient(conn),
	}, nil
}

// Close disconnects this connection.
func (conn *Conn) Close() error {
	return conn.conn.Close()
}
