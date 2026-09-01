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

		paystubFor := func(nodeID storj.NodeID) compensation.Paystub {
			return compensation.Paystub{
				Period:      period,
				NodeID:      compensation.NodeID(nodeID),
				Codes:       compensation.Codes{compensation.Offline},
				UsageAtRest: 1.5,
				UsageGet:    2,
				CompAtRest:  currency.NewMicroUnit(3),
				Owed:        currency.NewMicroUnit(100),
				Held:        currency.NewMicroUnit(25),
				Disposed:    currency.NewMicroUnit(10),
				Paid:        currency.NewMicroUnit(110),
				Distributed: currency.Zero,
			}
		}

		t.Run("record and replace", func(t *testing.T) {
			nodeID := testrand.NodeID()
			paystub := paystubFor(nodeID)

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

		t.Run("more rows than fit in one batch", func(t *testing.T) {
			// one more than the batch size, so the batching loop has to issue a
			// second statement and the tail is not silently dropped.
			nodeIDs := make([]storj.NodeID, 1001)
			paystubs := make([]compensation.Paystub, len(nodeIDs))
			for i := range nodeIDs {
				nodeIDs[i] = testrand.NodeID()
				paystubs[i] = paystubFor(nodeIDs[i])
			}

			require.NoError(t, comp.RecordPaystubs(ctx, paystubs))

			all, err := comp.QueryAllTotalAmounts(ctx, nil)
			require.NoError(t, err)
			for _, nodeID := range nodeIDs {
				require.Equal(t, compensation.TotalAmounts{
					TotalHeld:        currency.NewMicroUnit(25),
					TotalDisposed:    currency.NewMicroUnit(10),
					TotalPaid:        currency.NewMicroUnit(110),
					TotalDistributed: currency.Zero,
				}, all[nodeID], "node %q", nodeID)
			}
		})

		t.Run("empty", func(t *testing.T) {
			require.NoError(t, comp.RecordPaystubs(ctx, nil))
		})

		t.Run("duplicate node in a period is rejected", func(t *testing.T) {
			nodeID := testrand.NodeID()
			paystub := paystubFor(nodeID)

			err := comp.RecordPaystubs(ctx, []compensation.Paystub{paystub, paystub})
			require.ErrorContains(t, err, "duplicate paystub")

			// nothing was written
			totals, err := comp.QueryTotalAmounts(ctx, nodeID, nil)
			require.NoError(t, err)
			require.Equal(t, compensation.TotalAmounts{}, totals)
		})

		t.Run("same node in different periods", func(t *testing.T) {
			otherPeriod, err := compensation.PeriodFromString("2020-02")
			require.NoError(t, err)

			nodeID := testrand.NodeID()
			first := paystubFor(nodeID)
			second := paystubFor(nodeID)
			second.Period = otherPeriod

			require.NoError(t, comp.RecordPaystubs(ctx, []compensation.Paystub{first, second}))

			totals, err := comp.QueryTotalAmounts(ctx, nodeID, nil)
			require.NoError(t, err)
			require.Equal(t, currency.NewMicroUnit(220), totals.TotalPaid)

			// the genesis filter still sees only the later period
			totals, err = comp.QueryTotalAmounts(ctx, nodeID, &otherPeriod)
			require.NoError(t, err)
			require.Equal(t, currency.NewMicroUnit(110), totals.TotalPaid)
		})
	})
}

func TestRecordPaymentsWithDistribution(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		comp := db.Compensation()

		period, err := compensation.PeriodFromString("2020-01")
		require.NoError(t, err)

		paystubFor := func(nodeID storj.NodeID) compensation.Paystub {
			return compensation.Paystub{
				Period:      period,
				NodeID:      compensation.NodeID(nodeID),
				Owed:        currency.NewMicroUnit(100),
				Held:        currency.NewMicroUnit(25),
				Disposed:    currency.NewMicroUnit(10),
				Paid:        currency.NewMicroUnit(110),
				Distributed: currency.Zero,
			}
		}

		paymentFor := func(nodeID storj.NodeID, amount int64, receipt string) compensation.Payment {
			return compensation.Payment{
				Period:  period,
				NodeID:  compensation.NodeID(nodeID),
				Amount:  currency.NewMicroUnit(amount),
				Receipt: &receipt,
			}
		}

		distributedOf := func(t *testing.T, nodeID storj.NodeID) currency.MicroUnit {
			totals, err := comp.QueryTotalAmounts(ctx, nodeID, nil)
			require.NoError(t, err)
			return totals.TotalDistributed
		}

		t.Run("records the payment and distributes it on the paystub", func(t *testing.T) {
			nodeID := testrand.NodeID()
			require.NoError(t, comp.RecordPaystubs(ctx, []compensation.Paystub{paystubFor(nodeID)}))

			payment := paymentFor(nodeID, 90, "eth:0xdeadbeef")
			require.NoError(t, comp.RecordPaymentsWithDistribution(ctx, []compensation.Payment{payment}))

			recorded, err := db.SNOPayouts().GetAllPayments(ctx, nodeID)
			require.NoError(t, err)
			require.Len(t, recorded, 1)
			require.Equal(t, int64(90), recorded[0].Amount)
			require.Equal(t, "eth:0xdeadbeef", recorded[0].Receipt)
			require.Equal(t, period.String(), recorded[0].Period)

			require.Equal(t, currency.NewMicroUnit(90), distributedOf(t, nodeID))
		})

		t.Run("recording the same payments again changes nothing", func(t *testing.T) {
			nodeID := testrand.NodeID()
			require.NoError(t, comp.RecordPaystubs(ctx, []compensation.Paystub{paystubFor(nodeID)}))

			payments := []compensation.Payment{paymentFor(nodeID, 90, "eth:0xdeadbeef")}
			require.NoError(t, comp.RecordPaymentsWithDistribution(ctx, payments))
			require.NoError(t, comp.RecordPaymentsWithDistribution(ctx, payments))

			recorded, err := db.SNOPayouts().GetAllPayments(ctx, nodeID)
			require.NoError(t, err)
			require.Len(t, recorded, 1)

			// the distributed amount is the total of the payments recorded for
			// the period, so a re-run must not double it.
			require.Equal(t, currency.NewMicroUnit(90), distributedOf(t, nodeID))
		})

		t.Run("several payments in a period add up", func(t *testing.T) {
			nodeID := testrand.NodeID()
			require.NoError(t, comp.RecordPaystubs(ctx, []compensation.Paystub{paystubFor(nodeID)}))

			require.NoError(t, comp.RecordPaymentsWithDistribution(ctx, []compensation.Payment{
				paymentFor(nodeID, 50, "eth:0xaaaa"),
				paymentFor(nodeID, 40, "eth:0xbbbb"),
			}))
			require.Equal(t, currency.NewMicroUnit(90), distributedOf(t, nodeID))

			// a top-up recorded in a later run adds to the distributed amount
			// instead of replacing it.
			require.NoError(t, comp.RecordPaymentsWithDistribution(ctx, []compensation.Payment{
				paymentFor(nodeID, 20, "eth:0xcccc"),
			}))
			require.Equal(t, currency.NewMicroUnit(110), distributedOf(t, nodeID))

			recorded, err := db.SNOPayouts().GetAllPayments(ctx, nodeID)
			require.NoError(t, err)
			require.Len(t, recorded, 3)
		})

		t.Run("missing paystub is an error and nothing is recorded", func(t *testing.T) {
			withPaystub := testrand.NodeID()
			withoutPaystub := testrand.NodeID()
			require.NoError(t, comp.RecordPaystubs(ctx, []compensation.Paystub{paystubFor(withPaystub)}))

			err := comp.RecordPaymentsWithDistribution(ctx, []compensation.Payment{
				paymentFor(withPaystub, 90, "eth:0xaaaa"),
				paymentFor(withoutPaystub, 80, "eth:0xbbbb"),
			})
			require.ErrorContains(t, err, "no paystub")
			require.ErrorContains(t, err, withoutPaystub.String())
			require.ErrorContains(t, err, period.String())

			// the whole call is rolled back, including the payment of the node
			// that does have a paystub.
			recorded, err := db.SNOPayouts().GetAllPayments(ctx, withPaystub)
			require.NoError(t, err)
			require.Empty(t, recorded)
			require.Equal(t, currency.Zero, distributedOf(t, withPaystub))
		})

		t.Run("paystub of another period does not count", func(t *testing.T) {
			otherPeriod, err := compensation.PeriodFromString("2020-02")
			require.NoError(t, err)

			nodeID := testrand.NodeID()
			require.NoError(t, comp.RecordPaystubs(ctx, []compensation.Paystub{paystubFor(nodeID)}))

			payment := paymentFor(nodeID, 90, "eth:0xdeadbeef")
			payment.Period = otherPeriod

			err = comp.RecordPaymentsWithDistribution(ctx, []compensation.Payment{payment})
			require.ErrorContains(t, err, "no paystub")
			require.ErrorContains(t, err, otherPeriod.String())
		})

		t.Run("duplicate payment in the input is rejected", func(t *testing.T) {
			nodeID := testrand.NodeID()
			require.NoError(t, comp.RecordPaystubs(ctx, []compensation.Paystub{paystubFor(nodeID)}))

			payment := paymentFor(nodeID, 90, "eth:0xdeadbeef")
			err := comp.RecordPaymentsWithDistribution(ctx, []compensation.Payment{payment, payment})
			require.ErrorContains(t, err, "duplicate payment")

			recorded, err := db.SNOPayouts().GetAllPayments(ctx, nodeID)
			require.NoError(t, err)
			require.Empty(t, recorded)
		})

		t.Run("payment without a receipt or notes", func(t *testing.T) {
			nodeID := testrand.NodeID()
			require.NoError(t, comp.RecordPaystubs(ctx, []compensation.Paystub{paystubFor(nodeID)}))

			payment := compensation.Payment{
				Period: period,
				NodeID: compensation.NodeID(nodeID),
				Amount: currency.NewMicroUnit(90),
			}
			require.NoError(t, comp.RecordPaymentsWithDistribution(ctx, []compensation.Payment{payment}))
			// the dedup has to match the NULL receipt and notes of the recorded
			// row, otherwise a re-run inserts a second payment.
			require.NoError(t, comp.RecordPaymentsWithDistribution(ctx, []compensation.Payment{payment}))

			recorded, err := db.SNOPayouts().GetAllPayments(ctx, nodeID)
			require.NoError(t, err)
			require.Len(t, recorded, 1)
			require.Equal(t, "", recorded[0].Receipt)
			require.Equal(t, currency.NewMicroUnit(90), distributedOf(t, nodeID))
		})

		t.Run("more rows than fit in one batch", func(t *testing.T) {
			// one more than the batch size, so the batching loop has to issue a
			// second statement and the tail is not silently dropped.
			nodeIDs := make([]storj.NodeID, 1001)
			paystubs := make([]compensation.Paystub, len(nodeIDs))
			payments := make([]compensation.Payment, len(nodeIDs))
			for i := range nodeIDs {
				nodeIDs[i] = testrand.NodeID()
				paystubs[i] = paystubFor(nodeIDs[i])
				payments[i] = paymentFor(nodeIDs[i], 90, "eth:0xdeadbeef")
			}
			require.NoError(t, comp.RecordPaystubs(ctx, paystubs))

			require.NoError(t, comp.RecordPaymentsWithDistribution(ctx, payments))

			all, err := comp.QueryAllTotalAmounts(ctx, nil)
			require.NoError(t, err)
			for _, nodeID := range nodeIDs {
				require.Equal(t, currency.NewMicroUnit(90), all[nodeID].TotalDistributed, "node %q", nodeID)
			}
		})

		t.Run("empty", func(t *testing.T) {
			require.NoError(t, comp.RecordPaymentsWithDistribution(ctx, nil))
		})
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
