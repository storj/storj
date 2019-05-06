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
}

// Transport interface structure
type Transport struct {
	tlsOpts        *tlsopts.Options
	observers      []Observer
	requestTimeout time.Duration
}

// NewClient returns a transport client with a default timeout for requests
func NewClient(tlsOpts *tlsopts.Options, obs ...Observer) Client {
	return NewClientWithTimeout(tlsOpts, defaultRequestTimeout, obs...)
}

// NewClientWithTimeout returns a transport client with a specified timeout for requests
func NewClientWithTimeout(tlsOpts *tlsopts.Options, requestTimeout time.Duration, obs ...Observer) Client {
	return &Transport{
		tlsOpts:        tlsOpts,
		requestTimeout: requestTimeout,
		observers:      obs,
	}
}

// DialNode returns a grpc connection with tls to a node.
//
// Use this method for communicating with nodes as it is more secure than
// DialAddress. The connection will be established successfully only if the
// target node has the private key for the requested node ID.
func (transport *Transport) DialNode(ctx context.Context, node *pb.Node, opts ...grpc.DialOption) (conn *grpc.ClientConn, err error) {
	defer mon.Task()(&ctx)(&err)

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
		grpc.WithUnaryInterceptor(InvokeTimeout{transport.requestTimeout}.Intercept),
		grpc.WithStreamInterceptor(InvokeStreamTimeout{transport.requestTimeout}.Intercept),
	}, opts...)

	timedCtx, cancel := context.WithTimeout(ctx, defaultDialTimeout)
	defer cancel()

	conn, err = grpc.DialContext(timedCtx, node.GetAddress().Address, options...)
	if err != nil {
		if err == context.Canceled {
			return nil, err
		}
		alertFail(timedCtx, transport.observers, node, err)
		return nil, Error.Wrap(err)
	}

	ipAddr, err := GetIP(conn.Target())
	if err != nil {
		return nil, err
	}
	node.LastIp = ipAddr

	alertSuccess(timedCtx, transport.observers, node)

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
		grpc.WithUnaryInterceptor(InvokeTimeout{transport.requestTimeout}.Intercept),
		grpc.WithStreamInterceptor(InvokeStreamTimeout{transport.requestTimeout}.Intercept),
	}, opts...)

	timedCtx, cancel := context.WithTimeout(ctx, defaultDialTimeout)
	defer cancel()

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
	tr := &Transport{tlsOpts: transport.tlsOpts, requestTimeout: transport.requestTimeout}
	tr.observers = append(tr.observers, transport.observers...)
	tr.observers = append(tr.observers, obs...)
	return tr
}

// GetIP is a helper function to get resolved IP address from node address
func GetIP(target string) (string, error) {
	host, _, err := net.SplitHostPort(target)
	if err != nil {
		return "", err
	}
	ipAddr, err := net.ResolveIPAddr("ip", host)
	if err != nil {
		return "", err
	}
	return ipAddr.String(), nil
}

func alertFail(ctx context.Context, obs []Observer, node *pb.Node, err error) {
	for _, o := range obs {
		o.ConnFailure(ctx, node, err)
	}
}

func alertSuccess(ctx context.Context, obs []Observer, node *pb.Node) {
	for _, o := range obs {
		o.ConnSuccess(ctx, node)
	}
}
