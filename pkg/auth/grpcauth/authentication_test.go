// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package grpcauth

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"storj.io/storj/pkg/auth"
)

func TestAPIKeyInterceptor(t *testing.T) {
	for _, tt := range []struct {
		APIKey string
		err    error
	}{
		{"", status.Errorf(codes.Unauthenticated, "Invalid API credential")},
		{"good key", nil},
		{"wrong key", status.Errorf(codes.Unauthenticated, "Invalid API credential")},
	} {
		interceptor := NewAPIKeyInterceptor()

		// mock for method handler
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			APIKey, ok := auth.GetAPIKey(ctx)
			if !ok || string(APIKey) != "good key" {
				return nil, status.Errorf(codes.Unauthenticated, "Invalid API credential")
			}
			return nil, nil
		}

		ctx := context.Background()

		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("apikey", tt.APIKey))
		info := &grpc.UnaryServerInfo{}

		_, err := interceptor(ctx, nil, info, handler)

		assert.Equal(t, err, tt.err)
	}
}

func TestAPIKeyInjector(t *testing.T) {
	for _, tt := range []struct {
		APIKey string
		err    error
	}{
		{"abc123", nil},
		{"", nil},
	} {
		injector := NewAPIKeyInjector(tt.APIKey)

		// mock for method invoker
		var outputCtx context.Context
		invoker := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			outputCtx = ctx
			return nil
		}

		ctx := context.Background()
		err := injector(ctx, "/test.method", nil, nil, nil, invoker)

		assert.Equal(t, err, tt.err)

		md, ok := metadata.FromOutgoingContext(outputCtx)
		assert.Equal(t, true, ok)
		assert.Equal(t, tt.APIKey, strings.Join(md["apikey"], ""))
	}
}
