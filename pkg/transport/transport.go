// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package transport

import (
	"context"
	"net"
	"time"

	"google.golang.org/grpc"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls/tlsopts"
)

// Observer implements the ConnSuccess and ConnFailure methods
// for Discovery and other services to use
type Observer interface {
	ConnSuccess(ctx context.Context, node *pb.Node)
	ConnFailure(ctx context.Context, node *pb.Node, err error)
}

// Client defines the interface to an transport client.
type Client interface {
	DialNode(ctx context.Context, node *pb.Node, opts ...grpc.DialOption) (*grpc.ClientConn, error)
	DialAddress(ctx context.Context, address string, opts ...grpc.DialOption) (*grpc.ClientConn, error)
	Identity() *identity.FullIdentity
	WithObservers(obs ...Observer) Client
	AlertSuccess(ctx context.Context, node *pb.Node)
	AlertFail(ctx context.Context, node *pb.Node, err error)
}

// Timeouts contains all of the timeouts configurable for a transport
type Timeouts struct {
	Request time.Duration
	Dial    time.Duration
}

// Transport interface structure
type Transport struct {
	tlsOpts   *tlsopts.Options
	observers []Observer
	timeouts  Timeouts
}

// NewClient returns a transport client with a default timeout for requests
func NewClient(tlsOpts *tlsopts.Options, obs ...Observer) Client {
	return NewClientWithTimeouts(tlsOpts, Timeouts{}, obs...)
}

// NewClientWithTimeouts returns a transport client with a specified timeout for requests
func NewClientWithTimeouts(tlsOpts *tlsopts.Options, timeouts Timeouts, obs ...Observer) Client {
	if timeouts.Request == 0 {
		timeouts.Request = defaultRequestTimeout
	}
	if timeouts.Dial == 0 {
		timeouts.Dial = defaultDialTimeout
	}

	return &Transport{
		tlsOpts:   tlsOpts,
		timeouts:  timeouts,
		observers: obs,
	}
}

// DialNode returns a grpc connection with tls to a node.
//
// Use this method for communicating with nodes as it is more secure than
// DialAddress. The connection will be established successfully only if the
// target node has the private key for the requested node ID.
func (transport *Transport) DialNode(ctx context.Context, node *pb.Node, opts ...grpc.DialOption) (conn *grpc.ClientConn, err error) {
	defer mon.Task()(&ctx, "node: "+node.Id.String()[0:8])(&err)

	if node.Address == nil || node.Address.Address == "" {
		return nil, Error.New("no address")
	}
	dialOption, err := transport.tlsOpts.DialOption(node.Id)
	if err != nil {
		return nil, err
	}

	options := append([]grpc.DialOption{
		dialOption,
		grpc.WithBlock(),
		grpc.FailOnNonTempDialError(true),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
			if err != nil {
				return nil, err
			}
			return &timeoutConn{conn: conn, timeout: transport.timeouts.Request}, nil
		}),
	}, opts...)

	timedCtx, cancel := context.WithTimeout(ctx, transport.timeouts.Dial)
	defer cancel()

	conn, err = grpc.DialContext(timedCtx, node.GetAddress().Address, options...)
	if err != nil {
		if err == context.Canceled {
			return nil, err
		}
		transport.AlertFail(timedCtx, node, err)
		return nil, Error.Wrap(err)
	}

	transport.AlertSuccess(timedCtx, node)

	return conn, nil
}

// DialAddress returns a grpc connection with tls to an IP address.
//
// Do not use this method unless having a good reason. In most cases DialNode
// should be used for communicating with nodes as it is more secure than
// DialAddress.
func (transport *Transport) DialAddress(ctx context.Context, address string, opts ...grpc.DialOption) (conn *grpc.ClientConn, err error) {
	defer mon.Task()(&ctx)(&err)

	options := append([]grpc.DialOption{
		transport.tlsOpts.DialUnverifiedIDOption(),
		grpc.WithBlock(),
		grpc.FailOnNonTempDialError(true),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
			if err != nil {
				return nil, err
			}
			return &timeoutConn{conn: conn, timeout: transport.timeouts.Request}, nil
		}),
	}, opts...)

	timedCtx, cancel := context.WithTimeout(ctx, transport.timeouts.Dial)
	defer cancel()

	// TODO: this should also call alertFail or alertSuccess with the node id. We should be able
	// to get gRPC to give us the node id after dialing?
	conn, err = grpc.DialContext(timedCtx, address, options...)
	if err == context.Canceled {
		return nil, err
	}
	return conn, Error.Wrap(err)
}

// Identity is a getter for the transport's identity
func (transport *Transport) Identity() *identity.FullIdentity {
	return transport.tlsOpts.Ident
}

// WithObservers returns a new transport including the listed observers.
func (transport *Transport) WithObservers(obs ...Observer) Client {
	tr := &Transport{tlsOpts: transport.tlsOpts, timeouts: transport.timeouts}
	tr.observers = append(tr.observers, transport.observers...)
	tr.observers = append(tr.observers, obs...)
	return tr
}

// AlertFail alerts any subscribed observers of the failure 'err' for 'node'
func (transport *Transport) AlertFail(ctx context.Context, node *pb.Node, err error) {
	defer mon.Task()(&ctx)(nil)
	for _, o := range transport.observers {
		o.ConnFailure(ctx, node, err)
	}
}

// AlertSuccess alerts any subscribed observers of success for 'node'
func (transport *Transport) AlertSuccess(ctx context.Context, node *pb.Node) {
	defer mon.Task()(&ctx)(nil)
	for _, o := range transport.observers {
		o.ConnSuccess(ctx, node)
	}
}
