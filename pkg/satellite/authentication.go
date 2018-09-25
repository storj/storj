// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package auth

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"storj.io/storj/pkg/pointerdb/auth"
	"storj.io/storj/pkg/provider"
)

type SatelliteAuthenticator struct {
	Identity *provider.FullIdentity
}

// Get
func (s *SatelliteAuthenticator) Auth(ctx context.Context, req interface{},
	info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{},
	err error) {

	if strings.HasPrefix(info.FullMethod, "/pointerdb") {
		md, _ := metadata.FromIncomingContext(ctx)

		APIKey, has := md["apikey"]
		if !has || !auth.ValidateAPIKey(strings.Join(APIKey, "")) {
			return nil, status.Errorf(codes.Unauthenticated, "Invalid API credential")
		}

		siganture, err := generateSignature(s.Identity)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Internal server error")
		}

		grpc.SetHeader(ctx, metadata.Pairs("signature", siganture))
	}

	return handler(ctx, req)
}

func generateSignature(identity *provider.FullIdentity) (string, error) {
	// TODO generate signature
	return identity.ID.String(), nil
}
