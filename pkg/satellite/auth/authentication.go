// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package auth

import (
	"context"
	"crypto/ecdsa"
	"encoding/base64"
	"strings"

	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/gtank/cryptopasta"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"storj.io/storj/pkg/pointerdb/auth"
	"storj.io/storj/pkg/provider"
)

type satelliteAuthenticator struct {
	provider *provider.Provider
}

func NewSatelliteAuthenticator() *satelliteAuthenticator {
	return &satelliteAuthenticator{}
}

func (s *satelliteAuthenticator) Get(provider *provider.Provider) grpc.UnaryServerInterceptor {
	s.provider = provider
	return s.auth
}

func (s *satelliteAuthenticator) auth(ctx context.Context, req interface{},
	info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{},
	err error) {

	// TODO(michal) we can extend auth to overlay requests also
	if strings.HasPrefix(info.FullMethod, "/pointerdb") {
		APIKey := metautils.ExtractIncoming(ctx).Get("apikey")
		if !auth.ValidateAPIKey(APIKey) {
			return nil, status.Errorf(codes.Unauthenticated, "Invalid API credential")
		}

		siganture, err := generateSignature(s.provider.Identity())
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Internal server error")
		}

		grpc.SetHeader(ctx, metadata.Pairs("signature", siganture))
	}

	return handler(ctx, req)
}

func generateSignature(identity *provider.FullIdentity) (string, error) {
	signature, err := cryptopasta.Sign(identity.ID.Bytes(), identity.Key.(*ecdsa.PrivateKey))
	if err != nil {
		return "", err
	}
	encoding := base64.StdEncoding
	return encoding.EncodeToString(signature), nil
}
