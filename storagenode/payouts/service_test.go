// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package payouts_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/payouts"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestServiceHeldAmountHistory(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		log := zaptest.NewLogger(t)
		payoutsDB := db.Payout()
		satellitesDB := db.Satellites()

		satelliteID1 := testrand.NodeID()
		satelliteID2 := testrand.NodeID()
		satelliteID3 := testrand.NodeID()

		err := satellitesDB.SetAddress(ctx, satelliteID1, "foo.test:7777")
		require.NoError(t, err)
		err = satellitesDB.SetAddress(ctx, satelliteID2, "bar.test:7777")
		require.NoError(t, err)
		err = satellitesDB.SetAddress(ctx, satelliteID3, "baz.test:7777")
		require.NoError(t, err)

		// add paystubs
		paystubs := []payouts.PayStub{
			{
				SatelliteID: satelliteID1,
				Period:      "2021-01",
				Held:        10,
			},
			{
				SatelliteID: satelliteID1,
				Period:      "2021-02",
				Held:        10,
			},
			{
				SatelliteID: satelliteID2,
				Period:      "2021-01",
				Held:        0,
			},
			{
				SatelliteID: satelliteID2,
				Period:      "2021-02",
				Held:        0,
			},
		}
		for _, paystub := range paystubs {
			err = payoutsDB.StorePayStub(ctx, paystub)
			require.NoError(t, err)
		}

		expected := []payouts.HeldAmountHistory{
			{
				SatelliteID: satelliteID1,
				HeldAmounts: []payouts.HeldForPeriod{
					{
						Period: "2021-01",
						Amount: 10,
					},
					{
						Period: "2021-02",
						Amount: 10,
					},
				},
			},
			{
				SatelliteID: satelliteID2,
				HeldAmounts: []payouts.HeldForPeriod{
					{
						Period: "2021-01",
						Amount: 0,
					},
					{
						Period: "2021-02",
						Amount: 0,
					},
				},
			},
			{
				SatelliteID: satelliteID3,
			},
		}

		service, err := payouts.NewService(log, payoutsDB, db.Reputation(), db.Satellites())
		require.NoError(t, err)

		history, err := service.HeldAmountHistory(ctx)
		require.NoError(t, err)
		require.ElementsMatch(t, expected, history)
	})
}
