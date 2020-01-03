// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package queue_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/storj/satellite"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/storage"
)

func TestUntilEmpty(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		repairQueue := db.RepairQueue()

		// insert a bunch of segments
		pathsMap := make(map[string]int)
		for i := 0; i < 100; i++ {
			path := "/path/" + string(i)
			injuredSeg := &pb.InjuredSegment{Path: []byte(path)}
			err := repairQueue.Insert(ctx, injuredSeg)
			require.NoError(t, err)
			pathsMap[path] = 0
		}

		// select segments until no more are returned, and we should get each one exactly once
		for {
			injuredSeg, err := repairQueue.Select(ctx)
			if err != nil {
				require.True(t, storage.ErrEmptyQueue.Has(err))
				break
			}
			pathsMap[string(injuredSeg.Path)]++
		}

		for _, selectCount := range pathsMap {
			assert.Equal(t, selectCount, 1)
		}
	})
}

func TestOrder(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		repairQueue := db.RepairQueue()

		nullPath := []byte("/path/null")
		recentRepairPath := []byte("/path/recent")
		oldRepairPath := []byte("/path/old")
		olderRepairPath := []byte("/path/older")

		for _, path := range [][]byte{oldRepairPath, recentRepairPath, nullPath, olderRepairPath} {
			injuredSeg := &pb.InjuredSegment{Path: path}
			err := repairQueue.Insert(ctx, injuredSeg)
			require.NoError(t, err)
		}

		// TODO: remove dependency on *dbx.DB
		dbAccess := db.(interface{ TestDBAccess() *dbx.DB }).TestDBAccess()

		err := dbAccess.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
			updateList := []struct {
				path      []byte
				attempted time.Time
			}{
				{recentRepairPath, time.Now()},
				{oldRepairPath, time.Now().Add(-2 * time.Hour)},
				{olderRepairPath, time.Now().Add(-3 * time.Hour)},
			}
			for _, item := range updateList {
				res, err := tx.Tx.ExecContext(ctx, dbAccess.Rebind(`UPDATE injuredsegments SET attempted = ? AT TIME ZONE 'UTC' WHERE path = ?`), item.attempted, item.path)
				if err != nil {
					return err
				}
				count, err := res.RowsAffected()
				if err != nil {
					return err
				}
				require.EqualValues(t, 1, count)
			}
			return nil
		})
		require.NoError(t, err)

		// path with attempted = null should be selected first
		injuredSeg, err := repairQueue.Select(ctx)
		require.NoError(t, err)
		assert.Equal(t, string(nullPath), string(injuredSeg.Path))

		// path with attempted = 3 hours ago should be selected next
		injuredSeg, err = repairQueue.Select(ctx)
		require.NoError(t, err)
		assert.Equal(t, string(olderRepairPath), string(injuredSeg.Path))

		// path with attempted = 2 hours ago should be selected next
		injuredSeg, err = repairQueue.Select(ctx)
		require.NoError(t, err)
		assert.Equal(t, string(oldRepairPath), string(injuredSeg.Path))

		// queue should be considered "empty" now
		injuredSeg, err = repairQueue.Select(ctx)
		assert.True(t, storage.ErrEmptyQueue.Has(err))
		assert.Nil(t, injuredSeg)
	})
}

func TestCount(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		repairQueue := db.RepairQueue()

		// insert a bunch of segments
		pathsMap := make(map[string]int)
		numSegments := 100
		for i := 0; i < numSegments; i++ {
			path := "/path/" + string(i)
			injuredSeg := &pb.InjuredSegment{Path: []byte(path)}
			err := repairQueue.Insert(ctx, injuredSeg)
			require.NoError(t, err)
			pathsMap[path] = 0
		}

		count, err := repairQueue.Count(ctx)
		require.NoError(t, err)
		require.Equal(t, count, numSegments)
	})

}
