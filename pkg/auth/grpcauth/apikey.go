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

// APIKeyCredentials implements grpc/credentials.PerRPCCredentials
// for authenticating with the grpc server.
type APIKeyCredentials struct {
	value string
}

// NewAPIKeyCredentials returns a new APIKeyCredentials
func NewAPIKeyCredentials(apikey string) *APIKeyCredentials {
	return &APIKeyCredentials{apikey}
}

// GetRequestMetadata gets the current request metadata, refreshing tokens if required.
func (creds *APIKeyCredentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"apikey": creds.value,
	}, nil
}

// RequireTransportSecurity indicates whether the credentials requires transport security.
func (creds *APIKeyCredentials) RequireTransportSecurity() bool {
	return false
}
