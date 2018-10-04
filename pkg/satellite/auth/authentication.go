// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package auth

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// APIKeyInterceptor creates instance of apikey interceptor
func NewAPIKeyInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{},
		info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{},
		err error) {

		md, ok := metadata.FromIncomingContext(ctx)
		APIKey := strings.Join(md["apikey"], "")
		if !ok || APIKey == "" {
			return handler(ctx, req)
		}
		return handler(WithAPIKey(ctx, []byte(APIKey)), req)
	}
}
