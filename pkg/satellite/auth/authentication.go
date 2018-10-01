// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package auth

import (
	"context"
	"crypto/ecdsa"
	"encoding/base64"
	"strings"

	"github.com/gogo/protobuf/proto"
	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/gtank/cryptopasta"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb/auth"
	"storj.io/storj/pkg/provider"
)

// ResponseGenerator interface for generating signature
type ResponseGenerator interface {
	Generate(ctx context.Context) error
}

// NewSatelliteAuthenticator creates instance of satellite authenticator
func NewSatelliteAuthenticator(generator ResponseGenerator) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{},
		info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{},
		err error) {

		// TODO(michal) we can extend auth to overlay requests also
		if strings.HasPrefix(info.FullMethod, "/pointerdb") {
			APIKey := metautils.ExtractIncoming(ctx).Get("apikey")
			if !auth.ValidateAPIKey(APIKey) {
				return nil, status.Errorf(codes.Unauthenticated, "Invalid API credential")
			}

			err := generator.Generate(ctx)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "%v", err)
			}
		}

		return handler(ctx, req)
	}
}

type defaultResponseGenerator struct {
	identity *provider.FullIdentity
}

// NewResponseGenerator creates default response generator based on identity
func NewResponseGenerator(identity *provider.FullIdentity) ResponseGenerator {
	return &defaultResponseGenerator{identity: identity}
}

func (s *defaultResponseGenerator) Generate(ctx context.Context) error {
	pbd := &pb.PayerBandwidthAllocation_Data{Payer: s.identity.ID.Bytes(), MaxSize: -1, ExpirationUnixSec: -1}
	serializedPbd, err := proto.Marshal(pbd)
	if err != nil {
		return err
	}

	signature, err := cryptopasta.Sign(serializedPbd, s.identity.Key.(*ecdsa.PrivateKey))
	if err != nil {
		return err
	}

	pba := &pb.PayerBandwidthAllocation{Data: serializedPbd, Signature: signature}
	serializedPba, err := proto.Marshal(pba)
	if err != nil {
		return err
	}

	encoding := base64.StdEncoding
	pbaHeader := encoding.EncodeToString(serializedPba)
	err = grpc.SetHeader(ctx, metadata.Pairs("pba", pbaHeader))
	if err != nil {
		return err
	}

	return nil
}
