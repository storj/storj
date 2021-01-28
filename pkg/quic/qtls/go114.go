// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

// +build !go1.15

package qtls

import (
	"crypto/tls"

	quicgo "github.com/lucas-clemente/quic-go"
)

// ToTLSConnectionState converts a quic-go connection state to tls connection
// state.
func ToTLSConnectionState(state quicgo.ConnectionState) tls.ConnectionState {
	return tls.ConnectionState{
		Version:                     state.TLS.Version,
		HandshakeComplete:           state.TLS.HandshakeComplete,
		DidResume:                   state.TLS.DidResume,
		CipherSuite:                 state.TLS.CipherSuite,
		NegotiatedProtocol:          state.TLS.NegotiatedProtocol,
		ServerName:                  state.TLS.ServerName,
		PeerCertificates:            state.TLS.PeerCertificates,
		VerifiedChains:              state.TLS.VerifiedChains,
		SignedCertificateTimestamps: state.TLS.SignedCertificateTimestamps,
		OCSPResponse:                state.TLS.OCSPResponse,
	}
}
