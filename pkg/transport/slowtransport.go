// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package transport

import (
	"context"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/benchmark/latency"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
)

// SlowTransport is a slow version of transport
type SlowTransport struct {
	Client  Client
	Network latency.Network
}

// NewClientWithLatency makes a slower transport client for testing purposes
func NewClientWithLatency(client Client, network latency.Network) Client {
	return &SlowTransport{
		Client:  client,
		Network: network,
	}
}

// DialNode dials a node with latency
func (slowTransport *SlowTransport) DialNode(ctx context.Context, node *pb.Node, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	dialerOpt := grpc.WithContextDialer(func(ctx context.Context, address string) (net.Conn, error) {
		netdialer := &net.Dialer{}
		dctx := slowTransport.Network.ContextDialer(netdialer.DialContext)

		conn, err := dctx(ctx, "tcp", address)
		if err != nil {
			return nil, err
		}
		return slowTransport.Network.Conn(conn)
	})

	return slowTransport.Client.DialNode(ctx, node, append(opts, dialerOpt)...)
}

// DialAddress dials an address with latency
func (slowTransport *SlowTransport) DialAddress(ctx context.Context, address string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	dialerOpt := grpc.WithContextDialer(func(ctx context.Context, address string) (net.Conn, error) {
		netdialer := &net.Dialer{}

		dctx := slowTransport.Network.ContextDialer(netdialer.DialContext)

		conn, err := dctx(ctx, "tcp", address)
		if err != nil {
			return nil, err
		}
		return slowTransport.Network.Conn(conn)
	})

	return slowTransport.Client.DialAddress(ctx, address, append(opts, dialerOpt)...)
}

// Identity for SlowTransport
func (slowTransport *SlowTransport) Identity() *identity.FullIdentity {
	return slowTransport.Client.Identity()
}

// WithObservers calls WithObservers for SlowTransport
func (slowTransport *SlowTransport) WithObservers(obs ...Observer) *Transport {
	return slowTransport.Client.WithObservers(obs...)
}
