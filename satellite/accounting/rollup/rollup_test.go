// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rollup_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting/rollup"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestRollupNoDeletes(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		// In testplanet the setting config.Rollup.DeleteTallies defaults to false.
		// That means if we do not delete any old tally data, then we expect that we
		// can tally/rollup data from anytime in the past.
		// To confirm, this test creates 5 days of tally and rollup data, then we check that all
		// the data is present in the accounting rollup table and in the storage node storage tally table.
		const (
			days            = 5
			atRestAmount    = 10
			getAmount       = 20
			putAmount       = 30
			getAuditAmount  = 40
			getRepairAmount = 50
			putRepairAmount = 60
		)

		var (
			ordersDB       = db.Orders()
			snAccountingDB = db.StoragenodeAccounting()
			storageNodes   = createNodes(ctx, t, db)
		)

		rollupService := rollup.New(testplanet.NewLogger(t), snAccountingDB, rollup.Config{Interval: 120 * time.Second}, time.Hour)

		// Set initialTime back by the number of days we want to save
		initialTime := time.Now().UTC().AddDate(0, 0, -days)
		currentTime := initialTime

		nodeData := make([]storj.NodeID, len(storageNodes))
		bwAmount := make([]float64, len(storageNodes))
		bwTotals := make(map[storj.NodeID][]int64)
		for i, storageNodeID := range storageNodes {
			nodeData[i] = storageNodeID
			bwAmount[i] = float64(atRestAmount)
			bwTotals[storageNodeID] = []int64{putAmount, getAmount, getAuditAmount, getRepairAmount, putRepairAmount}
		}

		// Create 5 days worth of tally and rollup data.
		// Add one additional day of data since the rollup service will truncate data from the most recent day.
		for i := 0; i < days+1; i++ {
			require.NoError(t, snAccountingDB.SaveTallies(ctx, currentTime, nodeData, bwAmount))
			require.NoError(t, saveBWPhase3(ctx, ordersDB, bwTotals, currentTime))

			require.NoError(t, rollupService.Rollup(ctx))

			currentTime = currentTime.Add(24 * time.Hour)
		}

		accountingCSVRows, err := snAccountingDB.QueryPaymentInfo(ctx, initialTime.Add(-24*time.Hour), currentTime.Add(24*time.Hour))
		require.NoError(t, err)
		assert.Equal(t, len(storageNodes), len(accountingCSVRows))

		// Confirm all the data saved over the 5 days is all summed in the accounting rollup table.
		for _, row := range accountingCSVRows {
			assert.Equal(t, int64(days*putAmount), row.PutTotal)
			assert.Equal(t, int64(days*getAmount), row.GetTotal)
			assert.Equal(t, int64(days*getAuditAmount), row.GetAuditTotal)
			assert.Equal(t, int64(days*getRepairAmount), row.GetRepairTotal)
			assert.Equal(t, float64(days*atRestAmount), row.AtRestTotal)
		}
		// Confirm there is a storage tally row for each time tally ran for each storage node.
		// We ran tally for one additional day, so expect 6 days of tallies.
		storagenodeTallies, err := snAccountingDB.GetTallies(ctx)
		require.NoError(t, err)
		assert.Equal(t, (days+1)*len(storageNodes), len(storagenodeTallies))
	})
}

func createNodes(ctx *testcontext.Context, t *testing.T, db satellite.DB) []storj.NodeID {
	storageNodes := []storj.NodeID{}
	for i := 0; i < 10; i++ {
		id := testrand.NodeID()
		storageNodes = append(storageNodes, id)
		err := db.OverlayCache().UpdateCheckIn(ctx, overlay.NodeCheckInInfo{
			NodeID: id,
			Address: &pb.NodeAddress{
				Address: "127.0.0.1:1234",
			},
			Version: &pb.NodeVersion{
				Version: "1.12.1",
			},
			Operator: &pb.NodeOperator{
				Wallet: "wallet",
			},
		}, time.Now(), overlay.NodeSelectionConfig{})
		require.NoError(t, err)
	}
	return storageNodes
}

func TestRollupDeletes(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		// In this test config.Rollup.DeleteTallies is set to true.
		// This means old tally data will be deleted when Rollup runs.
		// To confirm, this test creates 5 days of tally and rollup data, then we check
		// that the correct data is in the accounting rollup table and the storagenode storage tally table.
		const (
			days            = 5
			atRestAmount    = 10
			getAmount       = 20
			putAmount       = 30
			getAuditAmount  = 40
			getRepairAmount = 50
			putRepairAmount = 60
		)

		var (
			ordersDB       = db.Orders()
			snAccountingDB = db.StoragenodeAccounting()
			storageNodes   = createNodes(ctx, t, db)
		)

		rollupService := rollup.New(testplanet.NewLogger(t), snAccountingDB, rollup.Config{Interval: 120 * time.Second, DeleteTallies: true}, time.Hour)

		// Set timestamp back by the number of days we want to save
		now := time.Now().UTC()
		initialTime := now.AddDate(0, 0, -days)

		// TODO: this only runs the test for times that are farther back than
		// an hour from midnight UTC. something is wrong for the hour of
		// 11pm-midnight UTC.
		hour, _, _ := now.Clock()
		if hour == 23 {
			initialTime = initialTime.Add(-time.Hour)
		}

		currentTime := initialTime

		nodeData := make([]storj.NodeID, len(storageNodes))
		bwAmount := make([]float64, len(storageNodes))
		bwTotals := make(map[storj.NodeID][]int64)
		for i, storageNodeID := range storageNodes {
			nodeData[i] = storageNodeID
			bwAmount[i] = float64(atRestAmount)
			bwTotals[storageNodeID] = []int64{putAmount, getAmount, getAuditAmount, getRepairAmount, putRepairAmount}
		}

		// Create 5 days worth of tally and rollup data.
		// Add one additional day of data since the rollup service will truncate data from the most recent day.
		for i := 0; i < days+1; i++ {
			require.NoError(t, snAccountingDB.SaveTallies(ctx, currentTime, nodeData, bwAmount))
			require.NoError(t, saveBWPhase3(ctx, ordersDB, bwTotals, currentTime))

			// Since the config.Rollup.DeleteTallies is set to true, at the end of the Rollup(),
			// storagenode storage tallies that exist before the last rollup should be deleted.
			require.NoError(t, rollupService.Rollup(ctx))

			currentTime = currentTime.Add(24 * time.Hour)
		}

		accountingCSVRows, err := snAccountingDB.QueryPaymentInfo(ctx, initialTime.Add(-24*time.Hour), currentTime.Add(24*time.Hour))
		require.NoError(t, err)
		assert.Equal(t, len(storageNodes), len(accountingCSVRows))

		// Confirm all the data saved over the 5 days is all summed in the accounting rollup table.
		for _, row := range accountingCSVRows {
			assert.Equal(t, int64(days*putAmount), row.PutTotal)
			assert.Equal(t, int64(days*getAmount), row.GetTotal)
			assert.Equal(t, int64(days*getAuditAmount), row.GetAuditTotal)
			assert.Equal(t, int64(days*getRepairAmount), row.GetRepairTotal)
			assert.Equal(t, float64(days*atRestAmount), row.AtRestTotal)
		}
		// Confirm there are only storage tally rows for the last time tally ran for each storage node.
		storagenodeTallies, err := snAccountingDB.GetTallies(ctx)
		require.NoError(t, err)
		assert.Equal(t, len(storageNodes), len(storagenodeTallies))
	})
}

func TestRollupOldOrders(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 2, UplinkCount: 0,
	},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			// The purpose of this test is to ensure that running Rollup properly updates storagenode accounting data
			//   for a period of time which has already been accounted for in a previous call to Rollup.
			//   This is because orders can be added to the bandwidth settlement table in the past, so a previous rollup can become inaccurate.
			// Test overview:
			// We have 2 nodes (A, B).
			// We start at t=now is the initial time, which is right at the beginning of a day.
			// Phase 1:
			//   On node A, settle bandwidth {X} at t+2hr.
			//   Also settle bandwidth at t+26hr. This is necessary because rollup will truncate data from the most recent day, and we don't want to
			//     truncate the data from the day starting at t.
			//   Run rollup, expect data in storagenode accounting DB to match {X} for sn A, and have nothing for sn B.
			// Phase 2:
			//   On nodes A and B, settle bandwidth {Y} at t+1hr.
			//   Run rollup, expect data in storagenode accounting DB to match {X}+{Y} for sn A, and to match {Y} for sn B.

			var (
				satellitePeer  = planet.Satellites[0]
				ordersDB       = satellitePeer.DB.Orders()
				snAccountingDB = satellitePeer.DB.StoragenodeAccounting()
			)
			// Run rollup once to start so we add the correct accounting timestamps to the db
			satellitePeer.Accounting.Rollup.Loop.Pause()
			satellitePeer.Accounting.Tally.Loop.Pause()

			satellitePeer.Accounting.Rollup.Loop.TriggerWait()

			nodeA := planet.StorageNodes[0]
			nodeB := planet.StorageNodes[1]
			// initialTime must start at the beginning of a day so that we can be sure
			// that bandwidth data for both phases of the test is settled on the _same_ day.
			// Subtract 48 hours so that when rollup discards the latest day, the data we care about is not ignored.
			initialTime := time.Now().Truncate(24 * time.Hour)

			const (
				PutActionAmount1       = 100
				GetActionAmount1       = 200
				GetAuditActionAmount1  = 300
				GetRepairActionAmount1 = 400
				PutRepairActionAmount1 = 500
				AtRestAmount1          = 600

				PutActionAmount2       = 150
				GetActionAmount2       = 250
				GetAuditActionAmount2  = 350
				GetRepairActionAmount2 = 450
				PutRepairActionAmount2 = 550
				AtRestAmount2          = 650
			)

			// Phase 1
			storageNodesPhase1 := []storj.NodeID{nodeA.ID()}
			storageTotalsPhase1 := []float64{AtRestAmount1}
			require.NoError(t, snAccountingDB.SaveTallies(ctx, initialTime.Add(2*time.Hour), storageNodesPhase1, storageTotalsPhase1))
			// save tallies for the next day too, so that the period we are testing is not truncated by the rollup service.
			require.NoError(t, snAccountingDB.SaveTallies(ctx, initialTime.Add(26*time.Hour), storageNodesPhase1, storageTotalsPhase1))

			bwTotalsPhase1 := make(map[storj.NodeID][]int64)
			bwTotalsPhase1[nodeA.ID()] = []int64{PutActionAmount1, GetActionAmount1, GetAuditActionAmount1, GetRepairActionAmount1, PutRepairActionAmount1}
			require.NoError(t, saveBWPhase3(ctx, ordersDB, bwTotalsPhase1, initialTime.Add(2*time.Hour)))
			// save bandwidth for the next day too, so that the period we are testing is not truncated by the rollup service.
			require.NoError(t, saveBWPhase3(ctx, ordersDB, bwTotalsPhase1, initialTime.Add(26*time.Hour)))

			require.NoError(t, satellitePeer.Accounting.Rollup.Rollup(ctx))

			accountingCSVRows, err := snAccountingDB.QueryPaymentInfo(ctx, initialTime.Add(-24*time.Hour), initialTime.Add(24*time.Hour))
			require.NoError(t, err)

			// there should only be data for node A
			require.Len(t, accountingCSVRows, 1)
			accountingCSVRow := accountingCSVRows[0]
			require.Equal(t, nodeA.ID(), accountingCSVRow.NodeID)

			// verify data is correct
			require.EqualValues(t, PutActionAmount1, accountingCSVRow.PutTotal)
			require.EqualValues(t, GetActionAmount1, accountingCSVRow.GetTotal)
			require.EqualValues(t, GetAuditActionAmount1, accountingCSVRow.GetAuditTotal)
			require.EqualValues(t, GetRepairActionAmount1, accountingCSVRow.GetRepairTotal)
			require.EqualValues(t, PutRepairActionAmount1, accountingCSVRow.PutRepairTotal)
			require.EqualValues(t, AtRestAmount1, accountingCSVRow.AtRestTotal)

			// Phase 2
			storageNodesPhase2 := []storj.NodeID{nodeA.ID(), nodeB.ID()}
			storageTotalsPhase2 := []float64{AtRestAmount2, AtRestAmount2}
			require.NoError(t, snAccountingDB.SaveTallies(ctx, initialTime.Add(-2*time.Hour), storageNodesPhase2, storageTotalsPhase2))

			bwTotalsPhase2 := make(map[storj.NodeID][]int64)
			bwTotalsPhase2[nodeA.ID()] = []int64{PutActionAmount2, GetActionAmount2, GetAuditActionAmount2, GetRepairActionAmount2, PutRepairActionAmount2}
			bwTotalsPhase2[nodeB.ID()] = []int64{PutActionAmount2, GetActionAmount2, GetAuditActionAmount2, GetRepairActionAmount2, PutRepairActionAmount2}
			require.NoError(t, saveBWPhase3(ctx, ordersDB, bwTotalsPhase2, initialTime.Add(time.Hour)))

			require.NoError(t, satellitePeer.Accounting.Rollup.Rollup(ctx))

			accountingCSVRows, err = snAccountingDB.QueryPaymentInfo(ctx, initialTime.Add(-24*time.Hour), initialTime.Add(24*time.Hour))
			require.NoError(t, err)

			// there should be data for both nodes
			require.Len(t, accountingCSVRows, 2)
			rA := accountingCSVRows[0]
			rB := accountingCSVRows[1]
			if rA.NodeID != nodeA.ID() {
				rA = accountingCSVRows[1]
				rB = accountingCSVRows[0]
			}
			require.Equal(t, nodeA.ID(), rA.NodeID)
			require.Equal(t, nodeB.ID(), rB.NodeID)

			// verify data is correct
			require.EqualValues(t, PutActionAmount1+PutActionAmount2, rA.PutTotal)
			require.EqualValues(t, GetActionAmount1+GetActionAmount2, rA.GetTotal)
			require.EqualValues(t, GetAuditActionAmount1+GetAuditActionAmount2, rA.GetAuditTotal)
			require.EqualValues(t, GetRepairActionAmount1+GetRepairActionAmount2, rA.GetRepairTotal)
			require.EqualValues(t, PutRepairActionAmount1+PutRepairActionAmount2, rA.PutRepairTotal)
			require.EqualValues(t, AtRestAmount1+AtRestAmount2, rA.AtRestTotal)

			require.EqualValues(t, PutActionAmount2, rB.PutTotal)
			require.EqualValues(t, GetActionAmount2, rB.GetTotal)
			require.EqualValues(t, GetAuditActionAmount2, rB.GetAuditTotal)
			require.EqualValues(t, GetRepairActionAmount2, rB.GetRepairTotal)
			require.EqualValues(t, PutRepairActionAmount2, rB.PutRepairTotal)
			require.EqualValues(t, AtRestAmount2, rB.AtRestTotal)
		})
}

func saveBWPhase3(ctx context.Context, ordersDB orders.DB, bwTotals map[storj.NodeID][]int64, intervalStart time.Time) error {
	pieceActions := []pb.PieceAction{pb.PieceAction_PUT,
		pb.PieceAction_GET,
		pb.PieceAction_GET_AUDIT,
		pb.PieceAction_GET_REPAIR,
		pb.PieceAction_PUT_REPAIR,
	}
	for nodeID, actions := range bwTotals {
		var actionAmounts = map[int32]int64{}
		for actionType, amount := range actions {
			actionAmounts[int32(pieceActions[actionType])] = amount
		}

		_, _, err := ordersDB.UpdateStoragenodeBandwidthSettleWithWindow(ctx,
			nodeID,
			actionAmounts,
			intervalStart.Truncate(1*time.Hour),
		)
		if err != nil {
			return err
		}
	}
	return nil
}
