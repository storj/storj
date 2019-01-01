// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"google.golang.org/grpc"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls"
)

// ServerOptions holds config, identity, and peer verification function data for use with a grpc server.
type ServerOptions struct {
	Config   ServerConfig
	Ident    *identity.FullIdentity
	RevDB    *peertls.RevocationDB
	PCVFuncs []peertls.PeerCertVerificationFunc
}

// NewServerOptions is a constructor for `serverOptions` given an identity and config
func NewServerOptions(i *identity.FullIdentity, c ServerConfig) (*ServerOptions, error) {
	serverOpts := &ServerOptions{
		Config: c,
		Ident:  i,
	}

	err := c.configure(serverOpts)
	if err != nil {
		return nil, err
	}

	return serverOpts, nil
}

func (so *ServerOptions) GRPCOpts() (grpc.ServerOption, error) {
	return so.Ident.ServerOption(so.PCVFuncs...)
}
