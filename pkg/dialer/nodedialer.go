// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package dialer

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/transport"
)

type Config struct {
	Limit int `help:"Semaphore size" Default:"32"`
}

var mon = monkit.Package()

// NodeDialer assists dialer to nodes
type NodeDialer struct {
	log       *zap.Logger
	transport transport.Client
	limit     sync2.Semaphore
}

// Conn represents a node connection
type Conn struct {
	conn   *grpc.ClientConn
	client pb.NodesClient
}

// NewNodeDialer instantiates a new node dialer struct
func NewNodeDialer(log *zap.Logger, config Config, transport transport.Client) *NodeDialer {
	dialer := &NodeDialer{
		log:       log,
		transport: transport,
	}
	dialer.limit.Init(config.Limit)
	return dialer
}

// Close closes the pool resources and prevents new connections to be made.
func (dialer *NodeDialer) Close() error {
	dialer.limit.Close()
	return nil
}

// PingNode pings the target node
func (dialer *NodeDialer) PingNode(ctx context.Context, target pb.Node) (_ bool, err error) {
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
func (dialer *NodeDialer) FetchPeerIdentity(ctx context.Context, target pb.Node) (_ *identity.PeerIdentity, err error) {
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
func (dialer *NodeDialer) FetchPeerIdentityUnverified(ctx context.Context, address string, opts ...grpc.CallOption) (_ *identity.PeerIdentity, err error) {
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
func (dialer *NodeDialer) FetchInfo(ctx context.Context, target pb.Node) (_ *pb.InfoResponse, err error) {
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

// AlertSuccess alerts the transport observers of a successful connection
func (dialer *NodeDialer) AlertSuccess(ctx context.Context, node *pb.Node) {
	dialer.transport.AlertSuccess(ctx, node)
}

// dialNode dials the specified node.
func (dialer *NodeDialer) dialNode(ctx context.Context, target pb.Node) (_ *Conn, err error) {
	defer mon.Task()(&ctx)(&err)
	grpcconn, err := dialer.transport.DialNode(ctx, &target)
	return &Conn{
		conn:   grpcconn,
		client: pb.NewNodesClient(grpcconn),
	}, err
}

// dialAddress dials the specified node by address (no node ID verification)
func (dialer *NodeDialer) dialAddress(ctx context.Context, address string) (_ *Conn, err error) {
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
