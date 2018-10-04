// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
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
			APIKey, ok := GetAPIKey(ctx)
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
