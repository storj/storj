// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademliaclient

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/rpc"
	"storj.io/storj/pkg/storj"
)

var mon = monkit.Package()

// Conn represents a connection
type Conn struct {
	conn   *rpc.Conn
	client rpc.NodesClient
}

// Close closes this connection.
func (conn *Conn) Close() error {
	return conn.conn.Close()
}

// Dialer sends requests to kademlia endpoints on storage nodes
type Dialer struct {
	log    *zap.Logger
	dialer rpc.Dialer
	obs    Observer
	limit  sync2.Semaphore
}

// Observer implements the ConnSuccess and ConnFailure methods
// for Discovery and other services to use
type Observer interface {
	ConnSuccess(ctx context.Context, node *pb.Node)
	ConnFailure(ctx context.Context, node *pb.Node, err error)
}

// NewDialer creates a new kademlia dialer.
func NewDialer(log *zap.Logger, dialer rpc.Dialer, obs Observer) *Dialer {
	d := &Dialer{
		log:    log,
		dialer: dialer,
		obs:    obs,
	}
	d.limit.Init(32) // TODO: limit should not be hardcoded
	return d
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
	defer func() { err = errs.Combine(err, conn.Close()) }()

	_, err = conn.client.Ping(ctx, &pb.PingRequest{})
	return err == nil, err
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

	return conn.conn.PeerIdentity()
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

	return conn.conn.PeerIdentity()
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
	defer func() { err = errs.Combine(err, conn.Close()) }()

	resp, err := conn.client.RequestInfo(ctx, &pb.InfoRequest{})
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// dialNode dials the specified node.
func (dialer *Dialer) dialNode(ctx context.Context, target pb.Node) (_ *Conn, err error) {
	defer mon.Task()(&ctx)(&err)

	conn, err := dialer.dialer.DialNode(ctx, &target)
	if err != nil {
		if dialer.obs != nil {
			dialer.obs.ConnFailure(ctx, &target, err)
		}
		return nil, err
	}
	if dialer.obs != nil {
		dialer.obs.ConnSuccess(ctx, &target)
	}

	return &Conn{
		conn:   conn,
		client: conn.NodesClient(),
	}, nil
}

// dialAddress dials the specified node by address (no node ID verification)
func (dialer *Dialer) dialAddress(ctx context.Context, address string) (_ *Conn, err error) {
	defer mon.Task()(&ctx)(&err)

	conn, err := dialer.dialer.DialAddressInsecure(ctx, address)
	if err != nil {
		// TODO: can't get an id here because we failed to dial
		return nil, err
	}
	if ident, err := conn.PeerIdentity(); err == nil && dialer.obs != nil {
		dialer.obs.ConnSuccess(ctx, &pb.Node{
			Id: ident.ID,
			Address: &pb.NodeAddress{
				Transport: pb.NodeTransport_TCP_TLS_GRPC,
				Address:   address,
			},
		})
	}

	return &Conn{
		conn:   conn,
		client: conn.NodesClient(),
	}, nil
}
