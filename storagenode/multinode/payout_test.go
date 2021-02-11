// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package multinode_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/multinodepb"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/storagenode/apikeys"
	"storj.io/storj/storagenode/multinode"
	"storj.io/storj/storagenode/payouts"
)

func TestEarnedPerSatellite(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		StorageNodeCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		log := zaptest.NewLogger(t)
		service := apikeys.NewService(planet.StorageNodes[0].DB.APIKeys())
		endpoint := multinode.NewPayoutEndpoint(log, service, planet.StorageNodes[0].DB.Payout())

		var amount int64 = 200

		err := planet.StorageNodes[0].DB.Payout().StorePayStub(ctx, payouts.PayStub{
			SatelliteID: testrand.NodeID(),
			CompAtRest:  amount,
		})
		require.NoError(t, err)

		key, err := service.Issue(ctx)
		require.NoError(t, err)

		response, err := endpoint.EarnedPerSatellite(ctx, &multinodepb.EarnedPerSatelliteRequest{
			Header: &multinodepb.RequestHeader{
				ApiKey: key.Secret[:],
			},
		})
		require.NoError(t, err)
		require.Equal(t, response.EarnedSatellite[0].Total, amount)
	})
}
