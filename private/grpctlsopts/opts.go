// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package grpctlsopts

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"storj.io/common/peertls/tlsopts"
	"storj.io/common/storj"
)

// ServerOption returns a grpc `ServerOption` for incoming connections
// to the node with this full identity.
func ServerOption(opts *tlsopts.Options) grpc.ServerOption {
	tlsConfig := opts.ServerTLSConfig()
	return grpc.Creds(credentials.NewTLS(tlsConfig))
}

// DialOption returns a grpc `DialOption` for making outgoing connections
// to the node with this peer identity.
func DialOption(opts *tlsopts.Options, id storj.NodeID) (grpc.DialOption, error) {
	if id.IsZero() {
		return nil, tlsopts.Error.New("no ID specified for DialOption")
	}
	tlsConfig := opts.ClientTLSConfig(id)
	return grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)), nil
}

// DialUnverifiedIDOption returns a grpc `DialUnverifiedIDOption`
func DialUnverifiedIDOption(opts *tlsopts.Options) grpc.DialOption {
	tlsConfig := opts.UnverifiedClientTLSConfig()
	return grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig))
}
