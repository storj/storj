// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package transport

import (
	"context"
	"crypto/tls"
	"net"
	"time"

	"github.com/zeebo/errs"
	"storj.io/storj/drpc"
	"storj.io/storj/drpc/drpcconn"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls"
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
	DialNode(ctx context.Context, node *pb.Node) (drpc.Conn, error)
	DialAddress(ctx context.Context, address string) (drpc.Conn, error)
	FetchPeerIdentity(ctx context.Context, node *pb.Node) (*identity.PeerIdentity, error)
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
		timeouts.Request = defaultTransportRequestTimeout
	}
	if timeouts.Dial == 0 {
		timeouts.Dial = defaultTransportDialTimeout
	}

	return &Transport{
		tlsOpts:   tlsOpts,
		timeouts:  timeouts,
		observers: obs,
	}
}

func drpcDial(ctx context.Context, network, address string, config *tls.Config) (*tls.Conn, error) {
	conn, err := new(net.Dialer).DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	if _, err := conn.Write([]byte("drpc!!!1")); err != nil {
		err = errs.Combine(conn.Close())
		return nil, err
	}
	tc := tls.Client(conn, config)
	if err := tc.Handshake(); err != nil {
		err = errs.Combine(conn.Close())
		return nil, err
	}
	return tc, nil
}

// DialNode returns a grpc connection with tls to a node.
//
// Use this method for communicating with nodes as it is more secure than
// DialAddress. The connection will be established successfully only if the
// target node has the private key for the requested node ID.
func (transport *Transport) DialNode(ctx context.Context, node *pb.Node) (c drpc.Conn, err error) {
	defer mon.Task()(&ctx, "node: "+node.Id.String()[0:8])(&err)

	if node.Address == nil || node.Address.Address == "" {
		return nil, Error.New("no address")
	}

	// TODO(jeff): lol what about all the options? I DON'T CARE! maybe i do.
	conn, err := drpcDial(ctx, "tcp", node.Address.Address, transport.tlsOpts.ClientTLSConfig(node.Id))
	if err != nil {
		transport.AlertFail(ctx, node, err)
		return nil, Error.Wrap(err)
	}
	transport.AlertSuccess(ctx, node)

	return drpcconn.New(conn), nil
}

// DialAddress returns a grpc connection with tls to an IP address.
//
// Do not use this method unless having a good reason. In most cases DialNode
// should be used for communicating with nodes as it is more secure than
// DialAddress.
func (transport *Transport) DialAddress(ctx context.Context, address string) (c drpc.Conn, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO(jeff): the following todo is definitely possible now
	// TODO: this should also call alertFail or alertSuccess with the node id. We should be able
	// to get gRPC to give us the node id after dialing?

	// TODO(jeff): lol what about all the options? I DON'T CARE! maybe i do.
	conn, err := drpcDial(ctx, "tcp", address, transport.tlsOpts.UnverifiedClientTLSConfig())
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return drpcconn.New(conn), nil
}

// FetchPeerIdentity dials the node and fetches the identity
func (transport *Transport) FetchPeerIdentity(ctx context.Context, node *pb.Node) (_ *identity.PeerIdentity, err error) {
	defer mon.Task()(&ctx, "node: "+node.Id.String()[0:8])(&err)

	if node.Address == nil || node.Address.Address == "" {
		return nil, Error.New("no address")
	}

	// TODO(jeff): lol what about all the options? I DON'T CARE! maybe i do.
	conn, err := drpcDial(ctx, "tcp", node.Address.Address, transport.tlsOpts.ClientTLSConfig(node.Id))
	if err != nil {
		transport.AlertFail(ctx, node, err)
		return nil, Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, conn.Close()) }()

	chain := conn.ConnectionState().PeerCertificates
	if len(chain)-1 < peertls.CAIndex {
		return nil, Error.New("invalid certificate chain")
	}

	pi, err := identity.PeerIdentityFromChain(chain)
	if err != nil {
		return nil, err
	}

	return pi, nil
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

// Timeouts returns the timeout values for dialing and requests.
func (transport *Transport) Timeouts() Timeouts {
	return transport.timeouts
}
