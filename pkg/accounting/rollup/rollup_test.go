// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rollup_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/storj"
)

func TestQueryOneDay(t *testing.T) {
	// TODO: use testplanet
	// change dbx accounting_raw created at to not be autoinsert
	// we'll then have to add a timestamp argument to saveAtRestRaw and SaveBWRaw

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 4, 0)
	assert.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)
	time.Sleep(2 * time.Second)

	fmt.Println("Node stats:")
	nodeData := make(map[storj.NodeID]float64)
	bwTotals := make(map[storj.NodeID][]int64)
	totals := []int64{1000, 2000, 3000, 4000, 5000}
	for _, n := range planet.StorageNodes {
		id := n.Identity.ID
		stats, err := planet.Satellites[0].DB.StatDB().Get(ctx, id)
		assert.NoError(t, err)
		fmt.Println(stats)
		nodeData[id] = float64(1000)
		bwTotals[id] = totals
	}

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

	rows, err := planet.Satellites[0].DB.Accounting().QueryPaymentInfo(ctx, before, now)
	fmt.Println("QueryPaymentInfo:", rows)
	assert.Equal(t, 0, len(rows))
	assert.NoError(t, err)
}

func TestQueryTwoDays(t *testing.T) {
	// TODO: use testplanet

	// ctx, _, db, nodeData, cleanup := createRollup(t)
	// defer cleanup()

	// now := time.Now().UTC()
	// then := now.Add(time.Hour * -24)

	// err := db.Accounting().SaveAtRestRaw(ctx, now, nodeData)
	// assert.NoError(t, err)

	// // db.db.Exec("UPDATE accounting_raws SET created_at= WHERE ")
	// // err = r.Query(ctx)
	// // assert.NoError(t, err)

	// _, err = db.Accounting().QueryPaymentInfo(ctx, then, now)
	// //assert.Equal(t, 10, len(rows))
	// assert.NoError(t, err)
}
