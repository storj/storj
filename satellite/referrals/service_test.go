// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package referrals_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/revocation"
	"storj.io/storj/pkg/server"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/private/testcontext"
	"storj.io/storj/private/testidentity"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/private/testrand"
	"storj.io/storj/satellite"
)

func TestService(t *testing.T) {
	endpoint := &Endpoint{}
	version := storj.LatestIDVersion()
	identity, err := testidentity.NewPregeneratedSignedIdentities(version).NewIdentity()
	require.NoError(t, err)
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(logger *zap.Logger, index int, config *satellite.Config) {
				addr := identity.ID.String() + "@127.0.0.1:7777"
				config.Referrals.ReferralManagerURL, _ = storj.ParseNodeURL(addr)
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		satellite := planet.Satellites[0]

		// set up mock referral manager server
		wl, err := planet.WriteWhitelist(storj.LatestIDVersion())
		require.NoError(t, err)
		tlscfg := tlsopts.Config{
			RevocationDBURL:     "bolt://" + filepath.Join(ctx.Dir("fakestoragenode"), "revocation.db"),
			UsePeerCAWhitelist:  true,
			PeerCAWhitelistPath: wl,
			PeerIDVersions:      "*",
			Extensions: extensions.Config{
				Revocation:          false,
				WhitelistSignedLeaf: false,
			},
		}

		revocationDB, err := revocation.NewDBFromCfg(tlscfg)
		require.NoError(t, err)

		tlsOptions, err := tlsopts.NewOptions(identity, tlscfg, revocationDB)
		require.NoError(t, err)

		server, err := server.New(satellite.Log.Named("mock-server"), tlsOptions, "127.0.0.1:7777", "127.0.0.1:7778", nil)
		require.NoError(t, err)
		pb.DRPCRegisterReferralManager(server.DRPC(), endpoint)
		go func() {
			// TODO: get goroutine under control
			err := server.Run(ctx)
			require.NoError(t, err)

			err = revocationDB.Close()
			require.NoError(t, err)
		}()
		defer func() {
			err = server.Close()
			require.NoError(t, err)
		}()

		userID := testrand.UUID()
		_, err = satellite.API.Referrals.Service.GetTokens(ctx, &userID)
		require.NoError(t, err)
	})
}

type Endpoint struct{}

func (endpoint *Endpoint) GetTokens(ctx context.Context, req *pb.GetTokensRequest) (*pb.GetTokensResponse, error) {
	fmt.Println("getTokenEndpint")
	return &pb.GetTokensResponse{}, nil
}

func (endpoint *Endpoint) RedeemToken(ctx context.Context, req *pb.RedeemTokenRequest) (*pb.RedeemTokenResponse, error) {
	fmt.Println("redeemh")
	return nil, nil
}
