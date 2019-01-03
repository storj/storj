// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/auth/grpcauth"
	"storj.io/storj/pkg/grpcutils"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls"
)

// SetupRPCs is a common place where both the public and private gRPC servers
// are set up.
func SetupRPCs(l *zap.Logger, ident *identity.FullIdentity, pcvs []peertls.PeerCertVerificationFunc) (public, private *grpc.Server, err error) {

	grpcOpts, err := ident.ServerOption(pcvs...)
	if err != nil {
		return nil, nil, err
	}

	serverOpts := make([]grpc.ServerOption, 0, 3)
	serverOpts = append(serverOpts, grpcOpts)
	serverOpts = append(serverOpts,
		grpcutils.ServerInterceptors(
			grpcauth.NewAPIKeyInterceptor(),
			defaultLogger(l))...)
	publicSrv := grpc.NewServer(serverOpts...)

	privateSrv := grpc.NewServer(grpcutils.ServerInterceptors(defaultLogger(l))...)

	return publicSrv, privateSrv, nil
}
