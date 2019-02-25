// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package tlsopts

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/storj"
)

// TransportCredentials wraps an embedded credentials.TransportCredentials.
// It decorates the `ClientHandshake` method.
type TransportCredentials struct {
	credentials.TransportCredentials
}

// ServerOption returns a grpc `ServerOption` for incoming connections
// to the node with this full identity.
func (opts *Options) ServerOption() grpc.ServerOption {
	pcvFuncs := append(
		[]peertls.PeerCertVerificationFunc{
			peertls.VerifyPeerCertChains,
		},
		opts.PCVFuncs...,
	)
	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{*opts.Cert},
		InsecureSkipVerify: true,
		ClientAuth:         tls.RequireAnyClientCert,
		VerifyPeerCertificate: peertls.VerifyPeerFunc(
			pcvFuncs...,
		),
	}

	return grpc.Creds(credentials.NewTLS(tlsConfig))
}

// DialOption returns a grpc `DialOption` for making outgoing connections
// to the node with this peer identity.
// id is an optional id of the node we are dialing.
func (opts *Options) DialOption(ctx context.Context, id storj.NodeID) grpc.DialOption {
	return grpc.WithTransportCredentials(opts.TransportCredentials(id))
}

// TransportCredentials returns a grpc `credentials.TransportCredentials`
// implementation for use within peertls.
func (opts *Options) TransportCredentials(id storj.NodeID) credentials.TransportCredentials {
	return &TransportCredentials{
		TransportCredentials: credentials.NewTLS(opts.TLSConfig(id)),
	}
}

// TLSConfig returns a TSLConfig for use in handshaking with a peer.
func (opts *Options) TLSConfig(id storj.NodeID) *tls.Config {
	pcvFuncs := append(
		[]peertls.PeerCertVerificationFunc{
			peertls.VerifyPeerCertChains,
			verifyIdentity(id),
		},
		opts.PCVFuncs...,
	)
	return &tls.Config{
		Certificates:       []tls.Certificate{*opts.Cert},
		InsecureSkipVerify: true,
		VerifyPeerCertificate: peertls.VerifyPeerFunc(
			pcvFuncs...,
		),
	}
}

// ClientHandshake returns a non-temporary error if an error is returned from
// the underlying `ClientHandshake` call.
func (tc *TransportCredentials) ClientHandshake(ctx context.Context, authority string, conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	tlsConn, authInfo, err := tc.TransportCredentials.ClientHandshake(ctx, authority, conn)
	if err != nil {
		return tlsConn, authInfo, peertls.NewNonTemporaryError(err)
	}
	return tlsConn, authInfo, err
}

func verifyIdentity(id storj.NodeID) peertls.PeerCertVerificationFunc {
	return func(_ [][]byte, parsedChains [][]*x509.Certificate) (err error) {
		defer mon.TaskNamed("verifyIdentity")(nil)(&err)
		if id == (storj.NodeID{}) {
			return nil
		}

		peer, err := identity.PeerIdentityFromCerts(parsedChains[0][0], parsedChains[0][1], parsedChains[0][2:])
		if err != nil {
			return err
		}

		if peer.ID.String() != id.String() {
			return Error.New("peer ID did not match requested ID")
		}

		return nil
	}
}
