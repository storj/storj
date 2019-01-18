// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/transport"
)

// Dialer is a kademlia dialer
type Dialer struct {
	log       *zap.Logger
	transport transport.Client
	limit     sync2.Semaphore
}

// Conn represents a kademlia conneciton
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
	dialer.limit.Init(32)
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
	defer conn.disconnect()

	resp, err := conn.client.Query(ctx, &pb.QueryRequest{
		Limit:    20,
		Sender:   &self,
		Target:   &find,
		Pingback: true,
	})
	if err != nil {
		return nil, err
	}

	return resp.Response, nil
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
	defer conn.disconnect()

	_, err = conn.client.Ping(ctx, &pb.PingRequest{})

	return err == nil, err
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
