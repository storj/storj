// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package transport

import (
	"context"
	"time"

	"google.golang.org/grpc"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
)

// SlowTransport is a slow version of transport
type SlowTransport struct {
	client      Client
	dialLatency time.Duration
}

// NewClientWithLatency makes a slower transport client for testing purposes
func NewClientWithLatency(client Client, dialLatency time.Duration) Client {
	return &SlowTransport{
		client:      client,
		dialLatency: dialLatency,
	}
}

// DialNode dials a node with latency
func (slowTransport *SlowTransport) DialNode(ctx context.Context, node *pb.Node, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(slowTransport.dialLatency):
		break
	}

	return slowTransport.client.DialNode(ctx, node, opts...)
}

// DialAddress dials an address with latency
func (slowTransport *SlowTransport) DialAddress(ctx context.Context, address string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(slowTransport.dialLatency):
		break
	}

	return slowTransport.client.DialAddress(ctx, address, opts...)
}

// Identity for SlowTransport
func (slowTransport *SlowTransport) Identity() *identity.FullIdentity {
	return slowTransport.client.Identity()
}

// WithObservers calls WithObservers for SlowTransport
func (slowTransport *SlowTransport) WithObservers(obs ...Observer) *Transport {
	return slowTransport.client.WithObservers(obs...)
}
