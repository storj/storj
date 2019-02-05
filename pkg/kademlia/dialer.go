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
func (dialer *Dialer) Lookup(ctx context.Context, self pb.Node, ask pb.Node, find pb.Node) ([]*pb.Node, error) {
	if !dialer.limit.Lock() {
		return nil, context.Canceled
	}
	defer dialer.limit.Unlock()

	conn, err := dialer.dial(ctx, ask)
	if err != nil {
		return nil, err
	}

	resp, err := conn.client.Query(ctx, &pb.QueryRequest{
		Limit:    20, // TODO: should not be hardcoded, but instead kademlia k value, routing table depth, etc
		Sender:   &self,
		Target:   &find,
		Pingback: true, // should only be true during bucket refreshing
	})
	if err != nil {
		return nil, errs.Combine(err, conn.disconnect())
	}

	return resp.Response, conn.disconnect()
}

// Ping pings target.
func (dialer *Dialer) Ping(ctx context.Context, target pb.Node) (bool, error) {
	if !dialer.limit.Lock() {
		return false, context.Canceled
	}
	defer dialer.limit.Unlock()

	conn, err := dialer.dial(ctx, target)
	if err != nil {
		return false, err
	}

	_, err = conn.client.Ping(ctx, &pb.PingRequest{})

	return err == nil, errs.Combine(err, conn.disconnect())
}

// FetchPeerIdentity connects to a node and returns its peer identity
func (dialer *Dialer) FetchPeerIdentity(ctx context.Context, target pb.Node) (pID *identity.PeerIdentity, err error) {
	if !dialer.limit.Lock() {
		return nil, context.Canceled
	}
	defer dialer.limit.Unlock()

	conn, err := dialer.dial(ctx, target)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errs.Combine(err, conn.disconnect())
	}()

	p := &peer.Peer{}
	pCall := grpc.Peer(p)
	_, err = conn.client.Ping(ctx, &pb.PingRequest{}, pCall)
	return identity.PeerIdentityFromPeer(p)
}

// dial dials the specified node.
func (dialer *Dialer) dial(ctx context.Context, target pb.Node) (*Conn, error) {
	grpcconn, err := dialer.transport.DialNode(ctx, &target)
	return &Conn{
		conn:   grpcconn,
		client: pb.NewNodesClient(grpcconn),
	}, err
}

// disconnect disconnects this connection.
func (conn *Conn) disconnect() error {
	return conn.conn.Close()
}
