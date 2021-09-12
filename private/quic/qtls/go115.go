// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

// +build go1.15

package qtls

import (
	"crypto/tls"

	quicgo "github.com/lucas-clemente/quic-go"
)

// ToTLSConnectionState converts a quic-go connection state to tls connection
// state.
func ToTLSConnectionState(state quicgo.ConnectionState) tls.ConnectionState {
	return state.TLS.ConnectionState
}
