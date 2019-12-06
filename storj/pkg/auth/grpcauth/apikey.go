// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package grpcauth

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"storj.io/storj/pkg/auth"
)

// NewAPIKeyInterceptor creates instance of apikey interceptor
func NewAPIKeyInterceptor() grpc.UnaryServerInterceptor {
	return InterceptAPIKey
}

// InterceptAPIKey reads apikey from requests and puts the value into the context.
func InterceptAPIKey(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return handler(ctx, req)
	}

	apikeys, ok := md["apikey"]
	if !ok || len(apikeys) == 0 {
		return handler(ctx, req)
	}

	return handler(auth.WithAPIKey(ctx, []byte(apikeys[0])), req)
}

// DeprecatedAPIKeyCredentials implements grpc/credentials.PerRPCCredentials
// for authenticating with the grpc server. This does not work with drpc.
type DeprecatedAPIKeyCredentials struct {
	value string
}

// NewDeprecatedAPIKeyCredentials returns a new DeprecatedAPIKeyCredentials
func NewDeprecatedAPIKeyCredentials(apikey string) *DeprecatedAPIKeyCredentials {
	return &DeprecatedAPIKeyCredentials{apikey}
}

// GetRequestMetadata gets the current request metadata, refreshing tokens if required.
func (creds *DeprecatedAPIKeyCredentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"apikey": creds.value,
	}, nil
}

// RequireTransportSecurity indicates whether the credentials requires transport security.
func (creds *DeprecatedAPIKeyCredentials) RequireTransportSecurity() bool {
	return false // Deprecated anyway, but how was this the right choice?
}
