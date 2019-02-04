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

func TestQueryOneDay(t *testing.T) {
	tests := []struct {
		expected int
		rest     float64
		bw       []int64
	}{
		{
			expected: 0,
			rest:     float64(5000),
			bw:       []int64{1000, 2000, 3000, 4000},
		},
	}

	for _, tt := range tests {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		planet, err := testplanet.New(t, 1, 4, 0)
		assert.NoError(t, err)
		defer ctx.Check(planet.Shutdown)

		planet.Start(ctx)
		time.Sleep(2 * time.Second)

		nodeData, bwTotals := createData(planet, tt.rest, tt.bw)

		now := time.Now().UTC()
		before := now.Add(time.Hour * -24)

		err = planet.Satellites[0].DB.Accounting().SaveAtRestRaw(ctx, before, before, nodeData)
		assert.NoError(t, err)

		err = planet.Satellites[0].DB.Accounting().SaveBWRaw(ctx, before, before, bwTotals)
		assert.NoError(t, err)

		err = planet.Satellites[0].Accounting.Rollup.Query(ctx)
		assert.NoError(t, err)

		// rollup.Query cuts off the hr/min/sec before saving, we need to do the same when querying
		now = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		before = time.Date(before.Year(), before.Month(), before.Day(), 0, 0, 0, 0, before.Location())

		rows, err := planet.Satellites[0].DB.Accounting().QueryPaymentInfo(ctx, before, now)
		assert.NoError(t, err)
		assert.Equal(t, tt.expected, len(rows))
	}
}

func TestQueryTwoDays(t *testing.T) {
	tests := []struct {
		expected int
		rest     float64
		bw       []int64
	}{
		{
			expected: 4,
			rest:     float64(5000),
			bw:       []int64{1000, 2000, 3000, 4000},
		},
	}

	for _, tt := range tests {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		planet, err := testplanet.New(t, 1, 4, 0)
		assert.NoError(t, err)
		defer ctx.Check(planet.Shutdown)

		planet.Start(ctx)
		time.Sleep(2 * time.Second)

		nodeData, bwTotals := createData(planet, tt.rest, tt.bw)

		now := time.Now().UTC()
		before := now.Add(time.Hour * -24)

		// Save data for day 1
		err = planet.Satellites[0].DB.Accounting().SaveAtRestRaw(ctx, before, before, nodeData)
		assert.NoError(t, err)

		err = planet.Satellites[0].DB.Accounting().SaveBWRaw(ctx, before, before, bwTotals)
		assert.NoError(t, err)

		// Save data for day 2
		err = planet.Satellites[0].DB.Accounting().SaveAtRestRaw(ctx, now, now, nodeData)
		assert.NoError(t, err)

		err = planet.Satellites[0].DB.Accounting().SaveBWRaw(ctx, now, now, bwTotals)
		assert.NoError(t, err)

		err = planet.Satellites[0].Accounting.Rollup.Query(ctx)
		assert.NoError(t, err)

		// rollup.Query cuts off the hr/min/sec before saving, we need to do the same when querying
		now = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		before = time.Date(before.Year(), before.Month(), before.Day(), 0, 0, 0, 0, before.Location())

		rows, err := planet.Satellites[0].DB.Accounting().QueryPaymentInfo(ctx, before, now)
		assert.Equal(t, 4, len(rows))
		assert.NoError(t, err)

		// verify data is correct
		var nodeIDs []*storj.NodeID
		for _, n := range planet.StorageNodes {
			ptr := n.Identity.ID
			nodeIDs = append(nodeIDs, &ptr)
		}
		for _, r := range rows {
			assert.Equal(t, tt.bw[0], r.PutTotal)
			assert.Equal(t, tt.bw[1], r.GetTotal)
			assert.Equal(t, tt.bw[2], r.GetAuditTotal)
			assert.Equal(t, tt.bw[3], r.GetRepairTotal)
			assert.Equal(t, tt.rest, r.AtRestTotal)
			assert.True(t, contains(r.NodeID, nodeIDs))
		}
	}
}

// contains
func contains(entry storj.NodeID, list []*storj.NodeID) bool {
	for i, n := range list {
		if n == nil {
			continue
		}
		if entry == *n {
			list[i] = nil
			return true
		}
	}
	return false
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
