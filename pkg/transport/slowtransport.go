// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package transport

import (
	"context"
	"net"
	"time"

	"google.golang.org/grpc"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
)

// SimulatedNetwork allows creating connections that try to simulated realistic network conditions.
type SimulatedNetwork struct {
	DialLatency    time.Duration
	BytesPerSecond memory.Size
}

// NewClient wraps an exiting client with the simulated network params.
func (network *SimulatedNetwork) NewClient(client Client) Client {
	return &slowTransport{
		client:  client,
		network: network,
	}
}

// slowTransport is a slow version of transport
type slowTransport struct {
	client  Client
	network *SimulatedNetwork
}

// DialNode dials a node with latency
func (client *slowTransport) DialNode(ctx context.Context, node *pb.Node, opts ...grpc.DialOption) (_ *grpc.ClientConn, err error) {
	defer mon.Task()(&ctx)(&err)
	return client.client.DialNode(ctx, node, append(client.network.DialOptions(), opts...)...)
}

// DialAddress dials an address with latency
func (client *slowTransport) DialAddress(ctx context.Context, address string, opts ...grpc.DialOption) (_ *grpc.ClientConn, err error) {
	defer mon.Task()(&ctx)(&err)
	return client.client.DialAddress(ctx, address, append(client.network.DialOptions(), opts...)...)
}

// Identity for slowTransport
func (client *slowTransport) Identity() *identity.FullIdentity {
	return client.client.Identity()
}

// WithObservers calls WithObservers for slowTransport
func (client *slowTransport) WithObservers(obs ...Observer) Client {
	return &slowTransport{client.client.WithObservers(obs...), client.network}
}

// AlertSuccess implements the transport.Client interface
func (client *slowTransport) AlertSuccess(ctx context.Context, node *pb.Node) {
	defer mon.Task()(&ctx)(nil)
	client.client.AlertSuccess(ctx, node)
}

// AlertFail implements the transport.Client interface
func (client *slowTransport) AlertFail(ctx context.Context, node *pb.Node, err error) {
	defer mon.Task()(&ctx)(nil)
	client.client.AlertFail(ctx, node, err)
}

// DialOptions returns options such that it will use simulated network parameters
func (network *SimulatedNetwork) DialOptions() []grpc.DialOption {
	return []grpc.DialOption{grpc.WithContextDialer(network.GRPCDialContext)}
}

// GRPCDialContext implements DialContext that is suitable for `grpc.WithContextDialer`
func (network *SimulatedNetwork) GRPCDialContext(ctx context.Context, address string) (_ net.Conn, err error) {
	defer mon.Task()(&ctx)(&err)
	timer := time.NewTimer(network.DialLatency)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
	}

	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if err != nil {
		return conn, err
	}

	if network.BytesPerSecond == 0 {
		return conn, err
	}

	return &simulatedConn{network, conn}, nil
}

// simulatedConn implements slow reading and writing to the connection
//
// This does not handle read deadline and write deadline properly.
type simulatedConn struct {
	network *SimulatedNetwork
	net.Conn
}

// delay sleeps specified amount of time
func (conn *simulatedConn) delay(actualWait time.Duration, bytes int) {
	expectedWait := time.Duration(bytes * int(time.Second) / conn.network.BytesPerSecond.Int())
	if actualWait < expectedWait {
		time.Sleep(expectedWait - actualWait)
	}
}

// Read reads data from the connection.
func (conn *simulatedConn) Read(b []byte) (n int, err error) {
	start := time.Now()
	n, err = conn.Conn.Read(b)
	if err == context.Canceled {
		return n, err
	}
	conn.delay(time.Since(start), n)
	return n, err
}

// Write writes data to the connection.
func (conn *simulatedConn) Write(b []byte) (n int, err error) {
	start := time.Now()
	n, err = conn.Conn.Write(b)
	if err == context.Canceled {
		return n, err
	}
	conn.delay(time.Since(start), n)
	return n, err
}
