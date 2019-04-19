// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rollup_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

type testData struct {
	nodeData map[storj.NodeID]float64
	bwTotals map[storj.NodeID][]int64
}

func TestRollup(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 10, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		days := 5
		testData := createData(planet, days)

		// Set timestamp back by the number of days we want to save
		timestamp := time.Now().UTC().AddDate(0, 0, -1*days)
		start := timestamp

		for i := 0; i < days; i++ {
			err := planet.Satellites[0].DB.Accounting().SaveAtRestRaw(ctx, timestamp, timestamp, testData[i].nodeData)
			require.NoError(t, err)
			err = saveBW(ctx, planet, testData[i].bwTotals, timestamp)
			require.NoError(t, err)

			err = planet.Satellites[0].Accounting.Rollup.Rollup(ctx)
			require.NoError(t, err)

			// Assert that RollupStorage deleted all raws except for today's
			raw, err := planet.Satellites[0].DB.Accounting().GetRaw(ctx)
			require.NoError(t, err)
			for _, r := range raw {
				assert.Equal(t, r.IntervalEndTime.UTC().Truncate(time.Second), timestamp.Truncate(time.Second))
				if r.DataType == accounting.AtRest {
					assert.Equal(t, testData[i].nodeData[r.NodeID], r.DataTotal)
				} else {
					assert.Equal(t, testData[i].bwTotals[r.NodeID][r.DataType], int64(r.DataTotal))
				}
			}

			// Advance time by 24 hours
			timestamp = timestamp.Add(time.Hour * 24)
			end := timestamp

			// rollup.RollupRaws cuts off the hr/min/sec before saving, we need to do the same when querying
			start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
			end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())

			rows, err := planet.Satellites[0].DB.Accounting().QueryPaymentInfo(ctx, start, end)
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
