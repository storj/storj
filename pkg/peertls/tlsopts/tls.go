// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package tlsopts

import (
	"crypto/tls"
	"crypto/x509"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/storj"
)

// ServerOption returns a grpc `ServerOption` for incoming connections
// to the node with this full identity.
func (opts *Options) ServerOption() grpc.ServerOption {
	tlsConfig := opts.ServerTLSConfig()
	return grpc.Creds(credentials.NewTLS(tlsConfig))
}

// DialOption returns a grpc `DialOption` for making outgoing connections
// to the node with this peer identity.
func (opts *Options) DialOption(id storj.NodeID) (grpc.DialOption, error) {
	if id.IsZero() {
		return nil, Error.New("no ID specified for DialOption")
	}
	tlsConfig := opts.ClientTLSConfig(id)
	return grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)), nil
}

// DialUnverifiedIDOption returns a grpc `DialUnverifiedIDOption`
func (opts *Options) DialUnverifiedIDOption() grpc.DialOption {
	tlsConfig := opts.tlsConfig(false)
	return grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig))
}

// ServerTLSConfig returns a TSLConfig for use as a server in handshaking with a peer.
func (opts *Options) ServerTLSConfig() *tls.Config {
	return opts.tlsConfig(true)
}

// ClientTLSConfig returns a TSLConfig for use as a client in handshaking with a peer.
func (opts *Options) ClientTLSConfig(id storj.NodeID) *tls.Config {
	return opts.tlsConfig(false, verifyIdentity(id))
}

// UnverifiedClientTLSConfig returns a TLSConfig for use as a client in handshaking with
// an unknown peer.
func (opts *Options) UnverifiedClientTLSConfig() *tls.Config {
	return opts.tlsConfig(false)
}

func (opts *Options) tlsConfig(isServer bool, verificationFuncs ...peertls.PeerCertVerificationFunc) *tls.Config {
	verificationFuncs = append(
		[]peertls.PeerCertVerificationFunc{
			peertls.VerifyPeerCertChains,
		},
		verificationFuncs...,
	)

	switch isServer {
	case true:
		verificationFuncs = append(
			verificationFuncs,
			opts.VerificationFuncs.server...,
		)
	case false:
		verificationFuncs = append(
			verificationFuncs,
			opts.VerificationFuncs.client...,
		)
	}

	config := &tls.Config{
		Certificates:       []tls.Certificate{*opts.Cert},
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
		VerifyPeerCertificate: peertls.VerifyPeerFunc(
			verificationFuncs...,
		),
	}

	if isServer {
		config.ClientAuth = tls.RequireAnyClientCert
	}

	return config
}

func verifyIdentity(id storj.NodeID) peertls.PeerCertVerificationFunc {
	return func(_ [][]byte, parsedChains [][]*x509.Certificate) (err error) {
		defer mon.TaskNamed("verifyIdentity")(nil)(&err)
		peer, err := identity.PeerIdentityFromChain(parsedChains[0])
		if err != nil {
			return err
		}

		if peer.ID.String() != id.String() {
			return Error.New("peer ID did not match requested ID")
		}

		return nil
	}
}
