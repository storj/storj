// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package referrals_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/rpc/rpcstatus"
	"storj.io/storj/private/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/private/testrand"
	"storj.io/storj/satellite/referrals"
)

func TestServiceSuccess(t *testing.T) {
	endpoint := &endpointHappyPath{
		TokenCount: 2,
	}
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			ReferralManagerServer: func(logger *zap.Logger) pb.ReferralManagerServer {

				return endpoint
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		satellite := planet.Satellites[0]

		userID := testrand.UUID()
		tokens, err := satellite.API.Referrals.Service.GetTokens(ctx, &userID)
		require.NoError(t, err)
		require.Len(t, tokens, endpoint.TokenCount)

		user := referrals.CreateUser{
			FullName:      "test",
			ShortName:     "test",
			Email:         "test@mail.test",
			Password:      "123a123",
			ReferralToken: testrand.UUID().String(),
		}
		_, err = satellite.API.Referrals.Service.CreateUser(ctx, user)
		require.NoError(t, err)
	})
}

func TestServiceRedeemFailure(t *testing.T) {
	endpoint := &endpointFailedPath{
		TokenCount: 2,
	}
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			ReferralManagerServer: func(logger *zap.Logger) pb.ReferralManagerServer {

				return endpoint
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		satellite := planet.Satellites[0]

		user := referrals.CreateUser{
			FullName:      "test",
			ShortName:     "test",
			Email:         "test@mail.test",
			Password:      "123a123",
			ReferralToken: testrand.UUID().String(),
		}
		_, err := satellite.API.Referrals.Service.CreateUser(ctx, user)
		require.Error(t, err)
	})
}

type endpointHappyPath struct {
	TokenCount int
}

func (endpoint *endpointHappyPath) GetTokens(ctx context.Context, req *pb.GetTokensRequest) (*pb.GetTokensResponse, error) {
	tokens := make([][]byte, endpoint.TokenCount)
	for i := 0; i < len(tokens); i++ {
		token := testrand.UUID()
		tokens[i] = token[:]
	}
	return &pb.GetTokensResponse{
		Token: tokens,
	}, nil
}

func (endpoint *endpointHappyPath) RedeemToken(ctx context.Context, req *pb.RedeemTokenRequest) (*pb.RedeemTokenResponse, error) {
	return &pb.RedeemTokenResponse{}, nil
}

type endpointFailedPath struct {
	TokenCount int
}

func (endpoint *endpointFailedPath) GetTokens(ctx context.Context, req *pb.GetTokensRequest) (*pb.GetTokensResponse, error) {
	tokens := make([][]byte, endpoint.TokenCount)
	for i := 0; i < len(tokens); i++ {
		token := testrand.UUID()
		tokens[i] = token[:]
	}
	return &pb.GetTokensResponse{
		Token: tokens,
	}, nil
}

func (endpoint *endpointFailedPath) RedeemToken(ctx context.Context, req *pb.RedeemTokenRequest) (*pb.RedeemTokenResponse, error) {
	return nil, rpcstatus.Error(rpcstatus.NotFound, "")
}
