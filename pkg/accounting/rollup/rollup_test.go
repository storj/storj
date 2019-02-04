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
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 4, 0)
	assert.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)
	time.Sleep(2 * time.Second)

	nodeData, bwTotals := createData(planet)

	now := time.Now().UTC()
	before := now.Add(time.Hour * -48)

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
	assert.Equal(t, 0, len(rows))
}

func TestQueryTwoDays(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 4, 0)
	assert.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)
	time.Sleep(2 * time.Second)

	nodeData, bwTotals := createData(planet)

	now := time.Now().UTC()
	before := now.Add(time.Hour * -48)

	err = planet.Satellites[0].DB.Accounting().SaveAtRestRaw(ctx, before, before, nodeData)
	assert.NoError(t, err)

	err = planet.Satellites[0].DB.Accounting().SaveBWRaw(ctx, before, before, bwTotals)
	assert.NoError(t, err)

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
}

func createData(planet *testplanet.Planet) (nodeData map[storj.NodeID]float64, bwTotals map[storj.NodeID][]int64) {
	nodeData = make(map[storj.NodeID]float64)
	bwTotals = make(map[storj.NodeID][]int64)
	totals := []int64{1000, 2000, 3000, 4000, 5000}
	for _, n := range planet.StorageNodes {
		id := n.Identity.ID
		nodeData[id] = float64(6000)
		bwTotals[id] = totals
	}
	return nodeData, bwTotals
}
