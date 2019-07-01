// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
)

// Dialer is a kademlia dialer
type Dialer struct {
	log       *zap.Logger
	transport transport.Client
	limit     sync2.Semaphore
}

// Conn represents a kademlia connection
type Conn struct {
	conn   *grpc.ClientConn
	client pb.NodesClient
}

// NewDialer creates a dialer for kademlia.
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
	defer func() {
		err = errs.Combine(err, conn.disconnect())
	}()

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

	return err == nil, errs.Combine(err, conn.disconnect())
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
	defer func() {
		err = errs.Combine(err, conn.disconnect())
	}()

	p := &peer.Peer{}
	_, err = conn.client.Ping(ctx, &pb.PingRequest{}, grpc.Peer(p))
	ident, errFromPeer := identity.PeerIdentityFromPeer(p)
	return ident, errs.Combine(err, errFromPeer)
}

// FetchPeerIdentityUnverified connects to an address and returns its peer identity (no node ID verification).
func (dialer *Dialer) FetchPeerIdentityUnverified(ctx context.Context, address string, opts ...grpc.CallOption) (_ *identity.PeerIdentity, err error) {
	defer mon.Task()(&ctx)(&err)
	if !dialer.limit.Lock() {
		return nil, context.Canceled
	}
	defer dialer.limit.Unlock()

	conn, err := dialer.dialAddress(ctx, address)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errs.Combine(err, conn.disconnect())
	}()

	p := &peer.Peer{}
	_, err = conn.client.Ping(ctx, &pb.PingRequest{}, grpc.Peer(p))
	ident, errFromPeer := identity.PeerIdentityFromPeer(p)
	return ident, errs.Combine(err, errFromPeer)
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

	return resp, errs.Combine(err, conn.disconnect())
}

// dialNode dials the specified node.
func (dialer *Dialer) dialNode(ctx context.Context, target pb.Node) (_ *Conn, err error) {
	defer mon.Task()(&ctx)(&err)
	grpcconn, err := dialer.transport.DialNode(ctx, &target)
	return &Conn{
		conn:   grpcconn,
		client: pb.NewNodesClient(grpcconn),
	}, err
}

// dialAddress dials the specified node by address (no node ID verification)
func (dialer *Dialer) dialAddress(ctx context.Context, address string) (_ *Conn, err error) {
	defer mon.Task()(&ctx)(&err)
	grpcconn, err := dialer.transport.DialAddress(ctx, address)
	return &Conn{
		conn:   grpcconn,
		client: pb.NewNodesClient(grpcconn),
	}, err
}

// disconnect disconnects this connection.
func (conn *Conn) disconnect() error {
	return conn.conn.Close()
}
