// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package referrals_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/referrals"
)

func TestServiceSuccess(t *testing.T) {
	tokenCount := 2
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			ReferralManagerServer: func(logger *zap.Logger) pb.ReferralManagerServer {
				endpoint := &endpointHappyPath{}
				endpoint.SetTokenCount(tokenCount)
				return endpoint
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		satellite := planet.Satellites[0]

		userID := testrand.UUID()
		tokens, err := satellite.API.Referrals.Service.GetTokens(ctx, &userID)
		require.NoError(t, err)
		require.Len(t, tokens, tokenCount)

		user := referrals.CreateUser{
			FullName:      "test",
			ShortName:     "test",
			Email:         "test@mail.test",
			Password:      "123a123",
			ReferralToken: testrand.UUID().String(),
		}

		createdUser, err := satellite.API.Referrals.Service.CreateUser(ctx, user)
		require.NoError(t, err)
		require.Equal(t, user.Email, createdUser.Email)
		require.Equal(t, user.FullName, createdUser.FullName)
		require.Equal(t, user.ShortName, createdUser.ShortName)
	})
}

func TestServiceRedeemFailure(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			ReferralManagerServer: func(logger *zap.Logger) pb.ReferralManagerServer {
				endpoint := &endpointFailedPath{}
				endpoint.SetTokenCount(2)
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
	testplanet.DefaultReferralManagerServer
}

type endpointFailedPath struct {
	testplanet.DefaultReferralManagerServer
}

func (endpoint *endpointFailedPath) RedeemToken(ctx context.Context, req *pb.RedeemTokenRequest) (*pb.RedeemTokenResponse, error) {
	return nil, rpcstatus.Error(rpcstatus.NotFound, "")
}
