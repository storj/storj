// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

// +build go1.13 go1.14

package qtls

import (
	"crypto/tls"

	quicgo "github.com/lucas-clemente/quic-go"
)

// ToTLSConnectionState converts a quic-go connection state to tls connection
// state.
func ToTLSConnectionState(state quicgo.ConnectionState) tls.ConnectionState {
	return tls.ConnectionState{
		Version:                     state.Version,
		HandshakeComplete:           state.HandshakeComplete,
		DidResume:                   state.DidResume,
		CipherSuite:                 state.CipherSuite,
		NegotiatedProtocol:          state.NegotiatedProtocol,
		ServerName:                  state.ServerName,
		PeerCertificates:            state.PeerCertificates,
		VerifiedChains:              state.VerifiedChains,
		SignedCertificateTimestamps: state.SignedCertificateTimestamps,
		OCSPResponse:                state.OCSPResponse,
	}
}
