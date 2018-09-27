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

// SignatureGenerator interface for generating signature
type SignatureGenerator interface {
	Generate() (string, error)
}

// NewSatelliteAuthenticator creates instance of satellite authenticator
func NewSatelliteAuthenticator(generator SignatureGenerator) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{},
		info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{},
		err error) {

		// TODO(michal) we can extend auth to overlay requests also
		if strings.HasPrefix(info.FullMethod, "/pointerdb") {
			APIKey := metautils.ExtractIncoming(ctx).Get("apikey")
			if !auth.ValidateAPIKey(APIKey) {
				return nil, status.Errorf(codes.Unauthenticated, "Invalid API credential")
			}

			siganture, err := generator.Generate()
			if err != nil {
				return nil, status.Errorf(codes.Internal, "%v", err)
			}

			err = grpc.SetHeader(ctx, metadata.Pairs("signature", siganture))
			if err != nil {
				return nil, status.Errorf(codes.Internal, "%v", err)
			}
		}

		return handler(ctx, req)
	}
}

type defaultSignatureGenerator struct {
	identity *provider.FullIdentity
}

// NewSignatureGenerator creates default signature generator based on identity
func NewSignatureGenerator(identity *provider.FullIdentity) SignatureGenerator {
	return &defaultSignatureGenerator{identity: identity}
}

func (s *defaultSignatureGenerator) Generate() (string, error) {
	signature, err := cryptopasta.Sign(s.identity.ID.Bytes(), s.identity.Key.(*ecdsa.PrivateKey))
	if err != nil {
		return "", err
	}
	encoding := base64.StdEncoding
	return encoding.EncodeToString(signature), nil
}
