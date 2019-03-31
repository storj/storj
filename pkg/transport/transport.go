// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package transport

import (
	"context"
	"net"
	"runtime/debug"
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
	WithObservers(obs ...Observer) *Transport
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
	if node != nil {
		node.Type.DPanicOnInvalid("transport dial node")
	}
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
		grpc.WithContextDialer(dialLeakCheck),
		grpc.FailOnNonTempDialError(true),
		grpc.WithUnaryInterceptor(InvokeTimeout{transport.requestTimeout}.Intercept),
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
		grpc.WithContextDialer(dialLeakCheck),
		grpc.FailOnNonTempDialError(true),
		grpc.WithUnaryInterceptor(InvokeTimeout{transport.requestTimeout}.Intercept),
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
func (transport *Transport) WithObservers(obs ...Observer) *Transport {
	tr := &Transport{tlsOpts: transport.tlsOpts, requestTimeout: transport.requestTimeout}
	tr.observers = append(tr.observers, transport.observers...)
	tr.observers = append(tr.observers, obs...)
	return tr
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

type leakCheck struct {
	net.Conn
	cancel func()
}

func dialLeakCheck(ctx context.Context, address string) (net.Conn, error) {
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return conn, err
	}
	return newLeakCheck(conn, time.Minute), err
}

func newLeakCheck(conn net.Conn, timeout time.Duration) net.Conn {
	leak := &leakCheck{Conn: conn}

	ctx, cancel := context.WithCancel(context.Background())
	leak.cancel = cancel

	stack := string(debug.Stack())

	go func() {
		defer leak.cancel()
		timer := time.NewTimer(timeout)
		defer timer.Stop()

		select {
		case <-ctx.Done():
		case <-timer.C:
			panic(stack)
		}
	}()

	return leak
}

func (leak *leakCheck) Close() error {
	leak.cancel()
	return leak.Conn.Close()
}
