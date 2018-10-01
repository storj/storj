// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type mockGenerator struct {
	err error
}

func (g *mockGenerator) Generate(ctx context.Context) error {
	return g.err
}

type mockServerTransportStream struct {
	grpc.ServerTransportStream
}

func (s *mockServerTransportStream) SetHeader(md metadata.MD) error {
	return nil
}

func TestSatelliteAuthenticator(t *testing.T) {
	for _, tt := range []struct {
		APIKey string
		method string
		genErr error
		err    error
	}{
		// currently default apikey is empty
		{"", "/pointerdb", nil, nil},
		{"", "/pointerdb", errors.New("generate error"), status.Errorf(codes.Internal, "%v", errors.New("generate error"))},
		{"wrong key", "/pointerdb", nil, status.Errorf(codes.Unauthenticated, "Invalid API credential")},
		{"", "/otherservice", nil, nil},
	} {
		authenticator := NewSatelliteAuthenticator(&mockGenerator{err: tt.genErr})

		// mock for method handler
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return nil, nil
		}

		ctx := context.Background()
		ctx = grpc.NewContextWithServerTransportStream(ctx, &mockServerTransportStream{})
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("apikey", tt.APIKey))
		info := &grpc.UnaryServerInfo{FullMethod: tt.method}

		_, err := authenticator(ctx, nil, info, handler)

		assert.Equal(t, err, tt.err)
	}

}
