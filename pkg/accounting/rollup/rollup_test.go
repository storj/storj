// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rollup_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/storj"
)

type testFlags struct {
	days   int
	atRest float64
	bw     []int64
}

func TestOneDay(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 10, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		flags := &testFlags{
			days:   1,
			atRest: float64(5000),
			bw:     []int64{1000, 2000, 3000, 4000},
		}

		time.Sleep(time.Second * 2)

		saveAndQuery(t, ctx, flags, planet)
	})
}

func TestTwoDays(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 10, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		flags := &testFlags{
			days:   2,
			atRest: float64(5000),
			bw:     []int64{1000, 2000, 3000, 4000},
		}

		time.Sleep(time.Second * 2)

		saveAndQuery(t, ctx, flags, planet)
	})
}

func TestThreeDays(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 10, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		flags := &testFlags{
			days:   3,
			atRest: float64(5000),
			bw:     []int64{1000, 2000, 3000, 4000},
		}

		time.Sleep(time.Second * 2)

		saveAndQuery(t, ctx, flags, planet)
	})
}

func createData(planet *testplanet.Planet, atRest float64, bw []int64) (nodeData map[storj.NodeID]float64, bwTotals map[storj.NodeID][]int64) {
	nodeData = make(map[storj.NodeID]float64)
	bwTotals = make(map[storj.NodeID][]int64)
	for _, n := range planet.StorageNodes {
		id := n.Identity.ID
		nodeData[id] = atRest
		bwTotals[id] = bw
	}
	return nodeData, bwTotals
}

func saveAndQuery(t *testing.T, ctx *testcontext.Context, flags *testFlags, planet *testplanet.Planet) {
	nodeData, bwTotals := createData(planet, flags.atRest, flags.bw)

	// Set timestamp back by the number of days we want to save
	timestamp := time.Now().UTC().AddDate(0, 0, -1*flags.days)
	start := timestamp

	// Save data for n days
	for i := 0; i < flags.days; i++ {
		err := planet.Satellites[0].DB.Accounting().SaveAtRestRaw(ctx, timestamp, timestamp, nodeData)
		assert.NoError(t, err)

		err = planet.Satellites[0].DB.Accounting().SaveBWRaw(ctx, timestamp, timestamp, bwTotals)
		assert.NoError(t, err)

		// Advance time by 24 hours
		timestamp = timestamp.Add(time.Hour * 24)
	}

	end := timestamp

	err := planet.Satellites[0].Accounting.Rollup.Query(ctx)
	assert.NoError(t, err)

	// rollup.Query cuts off the hr/min/sec before saving, we need to do the same when querying
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())

	rows, err := planet.Satellites[0].DB.Accounting().QueryPaymentInfo(ctx, start, end)
	assert.NoError(t, err)
	if flags.days <= 1 {
		assert.Equal(t, 0, len(rows))
		return
	}
	// TODO: once we sum data totals by node ID across rollups, number of rows should be number of nodes
	assert.Equal(t, (flags.days-1)*len(planet.StorageNodes), len(rows))

	// verify data is correct
	for _, r := range rows {
		assert.Equal(t, flags.bw[0], r.PutTotal)
		assert.Equal(t, flags.bw[1], r.GetTotal)
		assert.Equal(t, flags.bw[2], r.GetAuditTotal)
		assert.Equal(t, flags.bw[3], r.GetRepairTotal)
		assert.Equal(t, flags.atRest, r.AtRestTotal)
		assert.NotNil(t, nodeData[r.NodeID])
	}
}
