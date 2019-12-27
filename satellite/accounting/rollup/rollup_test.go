// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rollup_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/overlay"
)

type testData struct {
	nodeData map[storj.NodeID]float64
	bwTotals map[storj.NodeID][]int64
}

func TestRollupNoDeletes(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 10, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				// 0 so that we can disqualify a node immediately by triggering a failed audit
				config.Overlay.Node.AuditReputationLambda = 0
			},
		},
	},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			planet.Satellites[0].Accounting.Rollup.Loop.Pause()
			planet.Satellites[0].Accounting.Tally.Loop.Pause()

			dqedNodes, err := dqNodes(ctx, planet)
			require.NoError(t, err)
			require.NotEmpty(t, dqedNodes)

			days := 5
			testData := createData(planet, days)

			// Set timestamp back by the number of days we want to save
			timestamp := time.Now().UTC().AddDate(0, 0, -1*days)
			start := timestamp

			for i := 0; i < days; i++ {
				err := planet.Satellites[0].DB.StoragenodeAccounting().SaveTallies(ctx, timestamp, testData[i].nodeData)
				require.NoError(t, err)
				err = saveBW(ctx, planet, testData[i].bwTotals, timestamp)
				require.NoError(t, err)

				err = planet.Satellites[0].Accounting.Rollup.Rollup(ctx)
				require.NoError(t, err)

				// Advance time by 24 hours
				timestamp = timestamp.Add(time.Hour * 24)
				end := timestamp

				// rollup.RollupRaws cuts off the hr/min/sec before saving, we need to do the same when querying
				start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
				end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())

				rows, err := planet.Satellites[0].DB.StoragenodeAccounting().QueryPaymentInfo(ctx, start, end)
				require.NoError(t, err)
				if i == 0 { // we need at least two days for rollup to work
					assert.Equal(t, 0, len(rows))
					continue
				}
				// the number of rows should be number of nodes

				assert.Equal(t, len(planet.StorageNodes), len(rows))

				// verify data is correct
				for _, r := range rows {
					totals := expectedTotals(testData, r.NodeID, i)
					assert.Equal(t, int64(totals[0]), r.PutTotal)
					assert.Equal(t, int64(totals[1]), r.GetTotal)
					assert.Equal(t, int64(totals[2]), r.GetAuditTotal)
					assert.Equal(t, int64(totals[3]), r.GetRepairTotal)
					assert.Equal(t, totals[4], r.AtRestTotal)
					assert.NotEmpty(t, r.Wallet)
					if dqedNodes[r.NodeID] {
						assert.NotNil(t, r.Disqualified)
					} else {
						assert.Nil(t, r.Disqualified)
					}
				}
			}
			raw, err := planet.Satellites[0].DB.StoragenodeAccounting().GetTallies(ctx)
			require.NoError(t, err)
			assert.Equal(t, days*len(planet.StorageNodes), len(raw))
		})
}
func TestRollupDeletes(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 10, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Rollup.DeleteTallies = true
				// 0 so that we can disqualify a node immediately by triggering a failed audit
				config.Overlay.Node.AuditReputationLambda = 0
			},
		},
	},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			planet.Satellites[0].Accounting.Rollup.Loop.Pause()
			planet.Satellites[0].Accounting.Tally.Loop.Pause()

			dqedNodes, err := dqNodes(ctx, planet)
			require.NoError(t, err)
			require.NotEmpty(t, dqedNodes)

			days := 5
			testData := createData(planet, days)

			// Set timestamp back by the number of days we want to save
			timestamp := time.Now().UTC().AddDate(0, 0, -1*days)
			start := timestamp

			for i := 0; i < days; i++ {
				err := planet.Satellites[0].DB.StoragenodeAccounting().SaveTallies(ctx, timestamp, testData[i].nodeData)
				require.NoError(t, err)
				err = saveBW(ctx, planet, testData[i].bwTotals, timestamp)
				require.NoError(t, err)

				err = planet.Satellites[0].Accounting.Rollup.Rollup(ctx)
				require.NoError(t, err)

				// Assert that RollupStorage deleted all raws except for today's
				raw, err := planet.Satellites[0].DB.StoragenodeAccounting().GetTallies(ctx)
				require.NoError(t, err)
				for _, r := range raw {
					assert.Equal(t, r.IntervalEndTime.UTC().Truncate(time.Second), timestamp.Truncate(time.Second))
					assert.Equal(t, testData[i].nodeData[r.NodeID], r.DataTotal)

				}

				// Advance time by 24 hours
				timestamp = timestamp.Add(time.Hour * 24)
				end := timestamp

				// rollup.RollupRaws cuts off the hr/min/sec before saving, we need to do the same when querying
				start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
				end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())

				rows, err := planet.Satellites[0].DB.StoragenodeAccounting().QueryPaymentInfo(ctx, start, end)
				require.NoError(t, err)
				if i == 0 { // we need at least two days for rollup to work
					assert.Equal(t, 0, len(rows))
					continue
				}
				// the number of rows should be number of nodes

				assert.Equal(t, len(planet.StorageNodes), len(rows))

				// verify data is correct
				for _, r := range rows {
					totals := expectedTotals(testData, r.NodeID, i)
					assert.Equal(t, int64(totals[0]), r.PutTotal)
					assert.Equal(t, int64(totals[1]), r.GetTotal)
					assert.Equal(t, int64(totals[2]), r.GetAuditTotal)
					assert.Equal(t, int64(totals[3]), r.GetRepairTotal)
					assert.Equal(t, totals[4], r.AtRestTotal)
					assert.NotEmpty(t, r.Wallet)
					if dqedNodes[r.NodeID] {
						assert.NotNil(t, r.Disqualified)
					} else {
						assert.Nil(t, r.Disqualified)
					}
				}
			}
		})
}

// expectedTotals sums test data up to, but not including the current day's
func expectedTotals(data []testData, id storj.NodeID, currentDay int) []float64 {
	totals := make([]float64, 5)
	for i := 0; i < currentDay; i++ {
		totals[0] += float64(data[i].bwTotals[id][0])
		totals[1] += float64(data[i].bwTotals[id][1])
		totals[2] += float64(data[i].bwTotals[id][2])
		totals[3] += float64(data[i].bwTotals[id][3])
		totals[4] += data[i].nodeData[id]
	}
	return totals
}

func createData(planet *testplanet.Planet, days int) []testData {
	data := make([]testData, days)
	for i := 0; i < days; i++ {
		i := int64(i)
		data[i].nodeData = make(map[storj.NodeID]float64)
		data[i].bwTotals = make(map[storj.NodeID][]int64)
		for _, n := range planet.StorageNodes {
			id := n.Identity.ID
			data[i].nodeData[id] = float64(i * 5000)
			data[i].bwTotals[id] = []int64{i * 1000, i * 2000, i * 3000, i * 4000}
		}
	}
	return data
}

// dqNodes disqualifies half the nodes in the testplanet and returns a map of dqed nodes
func dqNodes(ctx *testcontext.Context, planet *testplanet.Planet) (map[storj.NodeID]bool, error) {
	dqed := make(map[storj.NodeID]bool)

	var updateRequests []*overlay.UpdateRequest
	for i, n := range planet.StorageNodes {
		if i%2 == 0 {
			continue
		}
		updateRequests = append(updateRequests, &overlay.UpdateRequest{
			NodeID:       n.ID(),
			IsUp:         true,
			AuditSuccess: false,
		})
	}

	_, err := planet.Satellites[0].Overlay.Service.BatchUpdateStats(ctx, updateRequests)
	if err != nil {
		return nil, err
	}
	for _, request := range updateRequests {
		dqed[request.NodeID] = true
	}
	return dqed, nil
}

func saveBW(ctx context.Context, planet *testplanet.Planet, bwTotals map[storj.NodeID][]int64, intervalStart time.Time) error {
	pieceActions := []pb.PieceAction{pb.PieceAction_PUT, pb.PieceAction_GET, pb.PieceAction_GET_AUDIT, pb.PieceAction_GET_REPAIR}
	for nodeID, actions := range bwTotals {
		for actionType, amount := range actions {
			err := planet.Satellites[0].DB.Orders().UpdateStoragenodeBandwidthSettle(ctx, nodeID, pieceActions[actionType], amount, intervalStart)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
