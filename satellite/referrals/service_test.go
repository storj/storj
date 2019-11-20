// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package referrals_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/server"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/private/errs2"
	"storj.io/storj/private/testcontext"
	"storj.io/storj/private/testidentity"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/private/testrand"
	"storj.io/storj/satellite"
)

func TestConcurrentConnections(t *testing.T) {
	// set up a mock referral manager server
	ctx := testcontext.New(t)
	endpoint := &Endpoint{}
	server, group, id := mockReferralManager(ctx, endpoint)
	require.NotNil(t, server)
	require.NotNil(t, group)
	defer func() {
		ctx.Cleanup()
		_ = group.Wait()
		server.Close()
	}()

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(logger *zap.Logger, index int, config *satellite.Config) {
				addr := id.ID.String() + "@127.0.0.1:7777"
				config.Referrals.ReferralManagerURL, _ = storj.ParseNodeURL(addr)
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		satellite := planet.Satellites[0]

		userID := testrand.UUID()
		_, err := satellite.API.Referrals.Service.GetTokens(ctx, &userID)
		require.NoError(t, err)
	})
}

func mockReferralManager(ctx context.Context, endpoint pb.DRPCReferralManagerServer) (*server.Server, *errgroup.Group, *identity.FullIdentity) {
	type config struct {
		Address        string
		PrivateAddress string
	}
	version := storj.LatestIDVersion()
	identity, err := testidentity.NewPregeneratedSignedIdentities(version).NewIdentity()
	if err != nil {
		return nil, nil, nil
	}

	serverConfig := config{
		Address:        "127.0.0.1:7777",
		PrivateAddress: "127.0.0.1:7778",
	}

	tlsOptions, err := tlsopts.NewOptions(identity, tlsopts.Config{}, nil)
	if err != nil {
		return nil, nil, nil
	}

	referralManager, err := server.New(nil, tlsOptions, serverConfig.Address, serverConfig.PrivateAddress, nil)
	if err != nil {
		return nil, nil, nil
	}

	pb.DRPCRegisterReferralManager(referralManager.DRPC(), endpoint)

	group, ctx := errgroup.WithContext(ctx)

	group.Go(func() error {
		return errs2.IgnoreCanceled(referralManager.Run(ctx))
	})

	return referralManager, group, identity
}

type Endpoint struct{}

func (endpoint *Endpoint) GetTokens(ctx context.Context, req *pb.GetTokensRequest) (*pb.GetTokensResponse, error) {
	fmt.Println("getTokenEndpint")
	return nil, nil
}

func (endpoint *Endpoint) RedeemToken(ctx context.Context, req *pb.RedeemTokenRequest) (*pb.RedeemTokenResponse, error) {
	fmt.Println("redeemh")
	return nil, nil
}
