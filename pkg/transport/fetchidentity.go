// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package transport

import (
	"context"
	"net"
	"sync"

	"github.com/zeebo/errs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls"
)

// handshakeCapture implements a credentials.TransportCredentials for capturing handshake information.
type handshakeCapture struct {
	credentials.TransportCredentials

	mu       sync.Mutex
	authInfo credentials.AuthInfo
}

// ClientHandshake does the authentication handshake specified by the corresponding
// authentication protocol on conn for clients. It returns the authenticated
// connection and the corresponding auth information about the connection.
func (capture *handshakeCapture) ClientHandshake(ctx context.Context, s string, conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	conn, auth, err := capture.TransportCredentials.ClientHandshake(ctx, s, conn)
	if err == nil {
		capture.mu.Lock()
		capture.authInfo = auth
		capture.mu.Unlock()
	}
	return conn, auth, err
}

// ServerHandshake does the authentication handshake for servers. It returns
// the authenticated connection and the corresponding auth information about
// the connection.
func (capture *handshakeCapture) ServerHandshake(conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	conn, auth, err := capture.TransportCredentials.ServerHandshake(conn)
	if err == nil {
		capture.mu.Lock()
		capture.authInfo = auth
		capture.mu.Unlock()
	}
	return conn, auth, err
}

// FetchPeerIdentity dials the node and fetches the identity
func (transport *Transport) FetchPeerIdentity(ctx context.Context, node *pb.Node, opts ...grpc.DialOption) (_ *identity.PeerIdentity, err error) {
	defer mon.Task()(&ctx, "node: "+node.Id.String()[0:8])(&err)

	if node.Address == nil || node.Address.Address == "" {
		return nil, Error.New("no address")
	}
	tlsConfig := transport.tlsOpts.ClientTLSConfig(node.Id)

	capture := &handshakeCapture{
		TransportCredentials: credentials.NewTLS(tlsConfig),
	}

	options := append([]grpc.DialOption{
		grpc.WithTransportCredentials(capture),
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

	conn, err := grpc.DialContext(timedCtx, node.GetAddress().Address, options...)
	if err != nil {
		if err == context.Canceled {
			return nil, err
		}
		transport.AlertFail(timedCtx, node, err)
		return nil, Error.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, conn.Close())
	}()
	transport.AlertSuccess(timedCtx, node)

	capture.mu.Lock()
	authinfo := capture.authInfo
	capture.mu.Unlock()

	switch info := authinfo.(type) {
	case credentials.TLSInfo:
		chain := info.State.PeerCertificates
		if len(chain)-1 < peertls.CAIndex {
			return nil, Error.New("invalid certificate chain")
		}

		pi, err := identity.PeerIdentityFromChain(chain)
		if err != nil {
			return nil, err
		}

		return pi, nil
	default:
		return nil, Error.New("unknown capture info %T", authinfo)
	}
}
