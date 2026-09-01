// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/currency"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/compensation"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestRecordPaystubs(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		comp := db.Compensation()

		period, err := compensation.PeriodFromString("2020-01")
		require.NoError(t, err)

		nodeID := testrand.NodeID()
		paystub := compensation.Paystub{
			Period:      period,
			NodeID:      compensation.NodeID(nodeID),
			Codes:       compensation.Codes{},
			Owed:        currency.NewMicroUnit(100),
			Held:        currency.NewMicroUnit(25),
			Disposed:    currency.NewMicroUnit(10),
			Paid:        currency.NewMicroUnit(110),
			Distributed: currency.Zero,
		}

		require.NoError(t, comp.RecordPaystubs(ctx, []compensation.Paystub{paystub}))

		totals, err := comp.QueryTotalAmounts(ctx, nodeID, nil)
		require.NoError(t, err)
		require.Equal(t, compensation.TotalAmounts{
			TotalHeld:        currency.NewMicroUnit(25),
			TotalDisposed:    currency.NewMicroUnit(10),
			TotalPaid:        currency.NewMicroUnit(110),
			TotalDistributed: currency.Zero,
		}, totals)

		// recording the same period again replaces the paystub instead of
		// adding its amounts a second time.
		require.NoError(t, comp.RecordPaystubs(ctx, []compensation.Paystub{paystub.LegalHold()}))

		totals, err = comp.QueryTotalAmounts(ctx, nodeID, nil)
		require.NoError(t, err)
		require.Equal(t, compensation.TotalAmounts{
			TotalHeld:        currency.NewMicroUnit(25),
			TotalDisposed:    currency.Zero,
			TotalPaid:        currency.Zero,
			TotalDistributed: currency.Zero,
		}, totals)
	})
}

func TestQueryPaystubConflicts(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		comp := db.Compensation()

		period, err := compensation.PeriodFromString("2020-01")
		require.NoError(t, err)
		otherPeriod, err := compensation.PeriodFromString("2020-02")
		require.NoError(t, err)

		paystub := func(period compensation.Period, nodeID storj.NodeID, distributed int64) compensation.Paystub {
			return compensation.Paystub{
				Period:      period,
				NodeID:      compensation.NodeID(nodeID),
				Codes:       compensation.Codes{},
				Owed:        currency.NewMicroUnit(100),
				Paid:        currency.NewMicroUnit(100),
				Distributed: currency.NewMicroUnit(distributed),
			}
		}

		distributedNode := testrand.NodeID()
		paidNode := testrand.NodeID()
		cleanNode := testrand.NodeID()
		otherPeriodNode := testrand.NodeID()
		unrecordedNode := testrand.NodeID()

		require.NoError(t, comp.RecordPaystubs(ctx, []compensation.Paystub{
			paystub(period, distributedNode, 100),
			paystub(period, paidNode, 0),
			paystub(period, cleanNode, 0),
			paystub(otherPeriod, otherPeriodNode, 0),
		}))

		receipt := "eth:0xdeadbeef"
		require.NoError(t, comp.RecordPayments(ctx, []compensation.Payment{{
			Period:  period,
			NodeID:  compensation.NodeID(paidNode),
			Amount:  currency.NewMicroUnit(100),
			Receipt: &receipt,
		}}))

		conflicts, err := comp.QueryPaystubConflicts(ctx, []compensation.Paystub{
			paystub(period, distributedNode, 0),
			paystub(period, paidNode, 0),
			paystub(period, cleanNode, 0),
			// recorded, but for a period the query is not asked about
			paystub(period, otherPeriodNode, 0),
			// not recorded at all
			paystub(period, unrecordedNode, 0),
		})
		require.NoError(t, err)

		byNode := make(map[storj.NodeID]compensation.PaystubConflict, len(conflicts))
		for _, conflict := range conflicts {
			byNode[conflict.NodeID] = conflict
		}
		require.Len(t, byNode, 3)

		require.Equal(t, compensation.PaystubConflict{
			Period:      period,
			NodeID:      distributedNode,
			Distributed: currency.NewMicroUnit(100),
		}, byNode[distributedNode])
		require.True(t, byNode[distributedNode].PaidOut())

		require.Equal(t, compensation.PaystubConflict{
			Period:      period,
			NodeID:      paidNode,
			Distributed: currency.Zero,
			Payments:    1,
		}, byNode[paidNode])
		require.True(t, byNode[paidNode].PaidOut())

		require.Equal(t, compensation.PaystubConflict{
			Period:      period,
			NodeID:      cleanNode,
			Distributed: currency.Zero,
		}, byNode[cleanNode])
		require.False(t, byNode[cleanNode].PaidOut())

		conflicts, err = comp.QueryPaystubConflicts(ctx, nil)
		require.NoError(t, err)
		require.Empty(t, conflicts)
	})
}
