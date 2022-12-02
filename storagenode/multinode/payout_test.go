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
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/apikeys"
	"storj.io/storj/storagenode/multinode"
	"storj.io/storj/storagenode/payouts"
	"storj.io/storj/storagenode/payouts/estimatedpayouts"
	"storj.io/storj/storagenode/pricing"
	"storj.io/storj/storagenode/reputation"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
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

func TestPayoutsEndpointSummary(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		log := zaptest.NewLogger(t)
		satelliteID := testrand.NodeID()
		apikeydb := db.APIKeys()
		payoutdb := db.Payout()
		service := apikeys.NewService(apikeydb)

		// Initialize a trust pool
		poolConfig := trust.Config{
			CachePath: ctx.File("trust-cache.json"),
		}
		poolConfig.Sources = append(poolConfig.Sources, &trust.StaticURLSource{URL: trust.SatelliteURL{ID: satelliteID}})

		trustPool, err := trust.NewPool(zaptest.NewLogger(t), trust.Dialer(rpc.Dialer{}), poolConfig, db.Satellites())
		require.NoError(t, err)
		require.NoError(t, trustPool.Refresh(ctx))

		payoutsService, err := payouts.NewService(log, db.Payout(), db.Reputation(), db.Satellites(), nil)
		require.NoError(t, err)
		estimatedPayoutsService := estimatedpayouts.NewService(db.Bandwidth(), db.Reputation(), db.StorageUsage(), db.Pricing(), db.Satellites(), trustPool)
		endpoint := multinode.NewPayoutEndpoint(log, service, db.Payout(), estimatedPayoutsService, payoutsService)

		id := testrand.NodeID()
		id2 := testrand.NodeID()

		var amount int64 = 200
		var amount2 int64 = 150

		err = payoutdb.StorePayStub(ctx, payouts.PayStub{
			SatelliteID: id,
			Held:        amount,
			Paid:        amount,
			CompAtRest:  amount,
			Period:      "2020-10",
		})
		require.NoError(t, err)

		err = payoutdb.StorePayStub(ctx, payouts.PayStub{
			SatelliteID: id2,
			Held:        amount2,
			Paid:        amount2,
			Period:      "2020-11",
		})
		require.NoError(t, err)

		key, err := service.Issue(ctx)
		require.NoError(t, err)

		response, err := endpoint.SummaryPeriod(ctx, &multinodepb.SummaryPeriodRequest{
			Header: &multinodepb.RequestHeader{
				ApiKey: key.Secret[:],
			}, Period: "2020-10",
		})
		require.NoError(t, err)
		require.Equal(t, response.PayoutInfo.Paid, amount)
		require.Equal(t, response.PayoutInfo.Held, amount)

		response2, err := endpoint.Summary(ctx, &multinodepb.SummaryRequest{
			Header: &multinodepb.RequestHeader{
				ApiKey: key.Secret[:],
			},
		})
		require.NoError(t, err)
		require.Equal(t, response2.PayoutInfo.Paid, amount+amount2)
		require.Equal(t, response2.PayoutInfo.Held, amount+amount2)

		response3, err := endpoint.SummarySatellitePeriod(ctx, &multinodepb.SummarySatellitePeriodRequest{
			Header: &multinodepb.RequestHeader{
				ApiKey: key.Secret[:],
			}, SatelliteId: id2, Period: "2020-11",
		})
		require.NoError(t, err)
		require.Equal(t, response3.PayoutInfo.Paid, amount2)
		require.Equal(t, response3.PayoutInfo.Held, amount2)

		response4, err := endpoint.SummarySatellite(ctx, &multinodepb.SummarySatelliteRequest{
			Header: &multinodepb.RequestHeader{
				ApiKey: key.Secret[:],
			}, SatelliteId: id,
		})
		require.NoError(t, err)
		require.Equal(t, response4.PayoutInfo.Paid, amount)
		require.Equal(t, response4.PayoutInfo.Held, amount)
	})
}

func TestPayoutsEndpointEstimations(t *testing.T) {
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

		trustPool, err := trust.NewPool(zaptest.NewLogger(t), trust.Dialer(rpc.Dialer{}), poolConfig, db.Satellites())
		require.NoError(t, err)
		require.NoError(t, trustPool.Refresh(ctx))

		payoutsService, err := payouts.NewService(log, db.Payout(), db.Reputation(), db.Satellites(), nil)
		require.NoError(t, err)
		estimatedPayoutsService := estimatedpayouts.NewService(db.Bandwidth(), db.Reputation(), db.StorageUsage(), db.Pricing(), db.Satellites(), trustPool)
		endpoint := multinode.NewPayoutEndpoint(log, service, db.Payout(), estimatedPayoutsService, payoutsService)

		now := time.Now().UTC().Add(-2 * time.Hour)

		for _, action := range actions {
			err := bandwidthdb.Add(ctx, satelliteID, action, 2300000000000, now)
			require.NoError(t, err)
		}
		var satellites []storj.NodeID

		satellites = append(satellites, satelliteID)
		stamps := storagenodedbtest.MakeStorageUsageStamps(satellites, 30, time.Now().UTC())

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

		t.Run("EstimatedPayoutTotal", func(t *testing.T) {
			estimation, err := estimatedPayoutsService.GetAllSatellitesEstimatedPayout(ctx, time.Now())
			require.NoError(t, err)

			resp, err := endpoint.EstimatedPayout(ctx, &multinodepb.EstimatedPayoutRequest{Header: &multinodepb.RequestHeader{
				ApiKey: key.Secret[:],
			}})
			require.NoError(t, err)

			require.EqualValues(t, estimation.CurrentMonthExpectations, resp.EstimatedEarnings)
		})
	})
}

func TestPayoutsUndistributedEndpoint(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		payoutdb := db.Payout()
		satelliteID := testrand.NodeID()

		log := zaptest.NewLogger(t)
		service := apikeys.NewService(db.APIKeys())

		key, err := service.Issue(ctx)
		require.NoError(t, err)

		// Initialize a trust pool
		poolConfig := trust.Config{
			CachePath: ctx.File("trust-cache.json"),
		}
		poolConfig.Sources = append(poolConfig.Sources, &trust.StaticURLSource{URL: trust.SatelliteURL{ID: satelliteID}})

		trustPool, err := trust.NewPool(zaptest.NewLogger(t), trust.Dialer(rpc.Dialer{}), poolConfig, db.Satellites())
		require.NoError(t, err)
		require.NoError(t, trustPool.Refresh(ctx))

		payoutsService, err := payouts.NewService(log, db.Payout(), db.Reputation(), db.Satellites(), nil)
		require.NoError(t, err)
		estimatedPayoutsService := estimatedpayouts.NewService(db.Bandwidth(), db.Reputation(), db.StorageUsage(), db.Pricing(), db.Satellites(), trustPool)
		endpoint := multinode.NewPayoutEndpoint(log, service, db.Payout(), estimatedPayoutsService, payoutsService)

		satelliteID1 := testrand.NodeID()
		satelliteID2 := testrand.NodeID()

		err = payoutdb.StorePayStub(ctx, payouts.PayStub{
			SatelliteID: satelliteID2,
			Period:      "2020-01",
			Distributed: 150,
			Paid:        250,
		})
		require.NoError(t, err)

		err = payoutdb.StorePayStub(ctx, payouts.PayStub{
			SatelliteID: satelliteID2,
			Period:      "2020-02",
			Distributed: 250,
			Paid:        350,
		})
		require.NoError(t, err)

		err = payoutdb.StorePayStub(ctx, payouts.PayStub{
			SatelliteID: satelliteID1,
			Period:      "2020-01",
			Distributed: 100,
			Paid:        300,
		})
		require.NoError(t, err)

		err = payoutdb.StorePayStub(ctx, payouts.PayStub{
			SatelliteID: satelliteID1,
			Period:      "2020-02",
			Distributed: 400,
			Paid:        500,
		})
		require.NoError(t, err)

		resp, err := endpoint.Undistributed(ctx, &multinodepb.UndistributedRequest{Header: &multinodepb.RequestHeader{
			ApiKey: key.Secret[:],
		}})
		require.NoError(t, err)

		require.EqualValues(t, 500, resp.Total)
	})
}
