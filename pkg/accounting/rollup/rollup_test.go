// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rollup_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/accounting/rollup"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb"
)

func TestQueryOneDay(t *testing.T) {
	// TODO: use testplanet

	ctx, r, db, nodeData, cleanup := createRollup(t)
	defer cleanup()

	now := time.Now().UTC()
	later := now.Add(time.Hour * 24)

	err := db.Accounting().SaveAtRestRaw(ctx, now, true, nodeData)
	assert.NoError(t, err)

	// test should return error because we delete latest day's rollup
	err = r.Query(ctx)
	assert.NoError(t, err)

	rows, err := db.Accounting().QueryPaymentInfo(ctx, now, later)
	assert.Equal(t, 0, len(rows))
	assert.NoError(t, err)
}

func TestQueryTwoDays(t *testing.T) {
	// TODO: use testplanet
	ctx, _, db, nodeData, cleanup := createRollup(t)
	defer cleanup()

	now := time.Now().UTC()
	later := now.Add(time.Hour * 48)

	db.Accounting().SetTimeHook(func() time.Time { return now })
	err := db.Accounting().SaveAtRestRaw(ctx, now, true, nodeData)
	assert.NoError(t, err)

	db.Accounting().SetTimeHook(func() time.Time { return later })
	err = db.Accounting().SaveAtRestRaw(ctx, later, false, nodeData)
	assert.NoError(t, err)

	allRaw, err := db.Accounting().GetRawSince(ctx, now)
	assert.NoError(t, err)
	assert.Equal(t, 20, len(allRaw))
	assert.Equal(t, now, allRaw[5].CreatedAt)
	assert.Equal(t, later, allRaw[15].CreatedAt)

	_, err = db.Accounting().QueryPaymentInfo(ctx, now, later)
	//todo: fix QueryPaymentInfo to be awesome
	//rows, err := db.Accounting().QueryPaymentInfo(ctx, now, later)
	//assert.Equal(t, 10, len(rows))
	assert.NoError(t, err)
}

func createRollup(t *testing.T) (*testcontext.Context, *rollup.Rollup, satellite.DB, map[storj.NodeID]float64, func()) {
	ctx := testcontext.New(t)
	db, err := satellitedb.NewInMemory()
	assert.NoError(t, err)

	assert.NoError(t, db.CreateTables())
	cleanup := func() {
		defer ctx.Cleanup()
		defer ctx.Check(db.Close)
	}
	statdb := db.StatDB()
	// generate nodeData
	nodeData := make(map[storj.NodeID]float64)
	for i := 1; i <= 10; i++ {
		id := teststorj.NodeIDFromString(string(i))
		nodeData[id] = float64(i * 100)
		_, err := statdb.Create(ctx, id, nil)
		assert.NoError(t, err)
		_, err = statdb.Get(ctx, id)
		assert.NoError(t, err)
	}

	return ctx, rollup.New(zap.NewNop(), db.Accounting(), time.Second), db, nodeData, cleanup
}
