// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package transport

import (
	"context"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/benchmark/latency"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
)

// SlowTransport is a slow version of transport
type SlowTransport struct {
	client  Client
	network latency.Network
}

// NewClientWithLatency makes a slower transport client for testing purposes
func NewClientWithLatency(client Client, network latency.Network) Client {
	return &SlowTransport{
		client:  client,
		network: network,
	}
}

// DialNode dials a node with latency
func (slowTransport *SlowTransport) DialNode(ctx context.Context, node *pb.Node, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	dialerOpt := grpc.WithDialer(func(address string, dur time.Duration) (net.Conn, error) {
		netdialer := &net.Dialer{}
		conn, err := netdialer.DialContext(ctx, "tcp", address)
		if err != nil {
			return nil, err
		}
		return slowTransport.network.Conn(conn)
	})

	return slowTransport.client.DialNode(ctx, node, append(opts, dialerOpt)...)
}

// DialAddress dials an address with latency
func (slowTransport *SlowTransport) DialAddress(ctx context.Context, address string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	dialerOpt := grpc.WithDialer(func(address string, dur time.Duration) (net.Conn, error) {
		netdialer := &net.Dialer{}
		conn, err := netdialer.DialContext(ctx, "tcp", address)
		if err != nil {
			return nil, err
		}
		return slowTransport.network.Conn(conn)
	})

	return slowTransport.client.DialAddress(ctx, address, append(opts, dialerOpt)...)
}

// Identity for SlowTransport
func (slowTransport *SlowTransport) Identity() *identity.FullIdentity {
	return slowTransport.client.Identity()
}

// WithObservers calls WithObservers for SlowTransport
func (slowTransport *SlowTransport) WithObservers(obs ...Observer) *Transport {
	return slowTransport.client.WithObservers(obs...)
}
