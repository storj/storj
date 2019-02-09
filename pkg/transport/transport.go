// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package transport

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"google.golang.org/grpc"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

var (
	mon = monkit.Package()
	//Error is the errs class of standard Transport Client errors
	Error = errs.Class("transport error")
	// default time to wait for a connection to be established
	timeout = 20 * time.Second
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
	identity  *identity.FullIdentity
	observers []Observer
}

// NewClient returns a newly instantiated Transport Client
func NewClient(identity *identity.FullIdentity, obs ...Observer) Client {
	return &Transport{
		identity:  identity,
		observers: obs,
	}
}

// DialNode returns a grpc connection with tls to a node
func (transport *Transport) DialNode(ctx context.Context, node *pb.Node, opts ...grpc.DialOption) (conn *grpc.ClientConn, err error) {
	defer mon.Task()(&ctx)(&err)
	if node != nil {
		node.Type.DPanicOnInvalid("transport dial node")
	}
	if node.Address == nil || node.Address.Address == "" {
		return nil, Error.New("no address")
	}

	// add ID of node we are wanting to connect to
	dialOpt, err := transport.identity.DialOption(node.Id)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	options := append([]grpc.DialOption{dialOpt, grpc.WithBlock(), grpc.FailOnNonTempDialError(true)}, opts...)

	ctx, cf := context.WithTimeout(ctx, timeout)
	defer cf()

	conn, err = grpc.DialContext(ctx, node.GetAddress().Address, options...)
	if err != nil {
		if err == context.Canceled {
			return nil, err
		}
		alertFail(ctx, transport.observers, node, err)
		return nil, Error.Wrap(err)
	}

	alertSuccess(ctx, transport.observers, node)

	return conn, nil
}

// DialAddress returns a grpc connection with tls to an IP address
func (transport *Transport) DialAddress(ctx context.Context, address string, opts ...grpc.DialOption) (conn *grpc.ClientConn, err error) {
	defer mon.Task()(&ctx)(&err)

	dialOpt, err := transport.identity.DialOption(storj.NodeID{})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	options := append([]grpc.DialOption{dialOpt, grpc.WithBlock(), grpc.FailOnNonTempDialError(true)}, opts...)
	conn, err = grpc.DialContext(ctx, address, options...)
	if err == context.Canceled {
		return nil, err
	}
	return conn, Error.Wrap(err)
}

// Identity is a getter for the transport's identity
func (transport *Transport) Identity() *identity.FullIdentity {
	return transport.identity
}

// WithObservers returns a new transport including the listed observers.
func (transport *Transport) WithObservers(obs ...Observer) *Transport {
	tr := &Transport{identity: transport.identity}
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
