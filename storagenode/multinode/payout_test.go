// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package multinode_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/multinodepb"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/apikeys"
	"storj.io/storj/storagenode/multinode"
	"storj.io/storj/storagenode/payouts"
	"storj.io/storj/storagenode/payouts/estimatedpayouts"
	"storj.io/storj/storagenode/pricing"
	"storj.io/storj/storagenode/reputation"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
	"storj.io/storj/storagenode/storageusage"
	"storj.io/storj/storagenode/trust"
)

var (
	actions = []pb.PieceAction{
		pb.PieceAction_INVALID,

		pb.PieceAction_PUT,
		pb.PieceAction_GET,
		pb.PieceAction_GET_AUDIT,
		pb.PieceAction_GET_REPAIR,
		pb.PieceAction_PUT_REPAIR,
		pb.PieceAction_DELETE,

		pb.PieceAction_PUT,
		pb.PieceAction_GET,
		pb.PieceAction_GET_AUDIT,
		pb.PieceAction_GET_REPAIR,
		pb.PieceAction_PUT_REPAIR,
		pb.PieceAction_DELETE,
	}
)

func TestEarnedPerSatellite(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		StorageNodeCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		log := zaptest.NewLogger(t)
		service := apikeys.NewService(planet.StorageNodes[0].DB.APIKeys())
		estimatedPayoutsService := estimatedpayouts.NewService(planet.StorageNodes[0].DB.Bandwidth(), planet.StorageNodes[0].DB.Reputation(), planet.StorageNodes[0].DB.StorageUsage(), planet.StorageNodes[0].DB.Pricing(), planet.StorageNodes[0].DB.Satellites(), &trust.Pool{})
		endpoint := multinode.NewPayoutEndpoint(log, service, estimatedPayoutsService, planet.StorageNodes[0].DB.Payout())

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

func TestStorageNodeApi(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		satelliteID := testrand.NodeID()
		bandwidthdb := db.Bandwidth()
		pricingdb := db.Pricing()
		storageusagedb := db.StorageUsage()
		reputationdb := db.Reputation()

		log := zaptest.NewLogger(t)
		service := apikeys.NewService(db.APIKeys())

		key, err := service.Issue(ctx)
		require.NoError(t, err)

		// Initialize a trust pool
		poolConfig := trust.Config{
			CachePath: ctx.File("trust-cache.json"),
		}
		poolConfig.Sources = append(poolConfig.Sources, &trust.StaticURLSource{URL: trust.SatelliteURL{ID: satelliteID}})

		trustPool, err := trust.NewPool(zaptest.NewLogger(t), trust.Dialer(rpc.Dialer{}), poolConfig)
		require.NoError(t, err)
		require.NoError(t, trustPool.Refresh(ctx))

		estimatedPayoutsService := estimatedpayouts.NewService(db.Bandwidth(), db.Reputation(), db.StorageUsage(), db.Pricing(), db.Satellites(), trustPool)
		endpoint := multinode.NewPayoutEndpoint(log, service, estimatedPayoutsService, db.Payout())

		now := time.Now().UTC().Add(-2 * time.Hour)

		for _, action := range actions {
			err := bandwidthdb.Add(ctx, satelliteID, action, 2300000000000, now)
			require.NoError(t, err)
		}
		var satellites []storj.NodeID

		satellites = append(satellites, satelliteID)
		stamps, _ := makeStorageUsageStamps(satellites)

		err = storageusagedb.Store(ctx, stamps)
		require.NoError(t, err)

		err = reputationdb.Store(ctx, reputation.Stats{
			SatelliteID: satelliteID,
			JoinedAt:    now.AddDate(0, -2, 0),
		})
		require.NoError(t, err)

		egressPrice, repairPrice, auditPrice, diskPrice := int64(2000), int64(1000), int64(1000), int64(150)

		err = pricingdb.Store(ctx, pricing.Pricing{
			SatelliteID:     satelliteID,
			EgressBandwidth: egressPrice,
			RepairBandwidth: repairPrice,
			AuditBandwidth:  auditPrice,
			DiskSpace:       diskPrice,
		})
		require.NoError(t, err)

		t.Run("test EstimatedPayoutTotal", func(t *testing.T) {
			estimation, err := estimatedPayoutsService.GetAllSatellitesEstimatedPayout(ctx, time.Now())
			require.NoError(t, err)

			resp, err := endpoint.EstimatedPayoutTotal(ctx, &multinodepb.EstimatedPayoutTotalRequest{Header: &multinodepb.RequestHeader{
				ApiKey: key.Secret[:],
			}})
			require.NoError(t, err)

			require.EqualValues(t, estimation.CurrentMonthExpectations, resp.EstimatedEarnings)
		})
	})
}

// makeStorageUsageStamps creates storage usage stamps and expected summaries for provided satellites.
// Creates one entry per day for 30 days with last date as beginning of provided endDate.
func makeStorageUsageStamps(satellites []storj.NodeID) ([]storageusage.Stamp, map[storj.NodeID]float64) {
	var stamps []storageusage.Stamp
	summary := make(map[storj.NodeID]float64)

	now := time.Now().UTC().Day()

	for _, satellite := range satellites {
		for i := 0; i < now; i++ {
			stamp := storageusage.Stamp{
				SatelliteID:   satellite,
				AtRestTotal:   31234567891234,
				IntervalStart: time.Now().UTC().Add(time.Hour * -24 * time.Duration(i)),
			}

			summary[satellite] += stamp.AtRestTotal
			stamps = append(stamps, stamp)
		}
	}

	return stamps, summary
}
