// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package queue_test

import (
	"context"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/storage"
)

func TestUntilEmpty(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		repairQueue := db.RepairQueue()

		// insert a bunch of segments
		pathsMap := make(map[string]int)
		for i := 0; i < 20; i++ {
			path := "/path/" + strconv.Itoa(i)
			injuredSeg := &internalpb.InjuredSegment{Path: []byte(path)}
			alreadyInserted, err := repairQueue.Insert(ctx, injuredSeg, 10)
			require.NoError(t, err)
			require.False(t, alreadyInserted)
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
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		repairQueue := db.RepairQueue()

		nullPath := []byte("/path/null")
		recentRepairPath := []byte("/path/recent")
		oldRepairPath := []byte("/path/old")
		olderRepairPath := []byte("/path/older")

		for _, path := range [][]byte{oldRepairPath, recentRepairPath, nullPath, olderRepairPath} {
			injuredSeg := &internalpb.InjuredSegment{Path: path}
			alreadyInserted, err := repairQueue.Insert(ctx, injuredSeg, 10)
			require.NoError(t, err)
			require.False(t, alreadyInserted)
		}

		// TODO: remove dependency on *dbx.DB
		dbAccess := db.(interface{ TestDBAccess() *dbx.DB }).TestDBAccess()

		err := dbAccess.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
			updateList := []struct {
				path      []byte
				attempted time.Time
			}{
				{recentRepairPath, time.Now()},
				{oldRepairPath, time.Now().Add(-7 * time.Hour)},
				{olderRepairPath, time.Now().Add(-8 * time.Hour)},
			}
			for _, item := range updateList {
				res, err := tx.Tx.ExecContext(ctx, dbAccess.Rebind(`UPDATE injuredsegments SET attempted = ? WHERE path = ?`), item.attempted, item.path)
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

		// path with attempted = 8 hours ago should be selected next
		injuredSeg, err = repairQueue.Select(ctx)
		require.NoError(t, err)
		assert.Equal(t, string(olderRepairPath), string(injuredSeg.Path))

		// path with attempted = 7 hours ago should be selected next
		injuredSeg, err = repairQueue.Select(ctx)
		require.NoError(t, err)
		assert.Equal(t, string(oldRepairPath), string(injuredSeg.Path))

		// queue should be considered "empty" now
		injuredSeg, err = repairQueue.Select(ctx)
		assert.True(t, storage.ErrEmptyQueue.Has(err))
		assert.Nil(t, injuredSeg)
	})
}

// TestOrderHealthyPieces ensures that we select in the correct order, accounting for segment health as well as last attempted repair time.
func TestOrderHealthyPieces(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		repairQueue := db.RepairQueue()

		// we insert (path, health, lastAttempted) as follows:
		// ("path/a", 6, now-8h)
		// ("path/b", 7, now)
		// ("path/c", 8, null)
		// ("path/d", 9, null)
		// ("path/e", 9, now-7h)
		// ("path/f", 9, now-8h)
		// ("path/g", 10, null)
		// ("path/h", 10, now-8h)

		// TODO: remove dependency on *dbx.DB
		dbAccess := db.(interface{ TestDBAccess() *dbx.DB }).TestDBAccess()

		// insert the 8 segments according to the plan above
		injuredSegList := []struct {
			path      []byte
			health    int
			attempted time.Time
		}{
			{[]byte("path/a"), 6, time.Now().Add(-8 * time.Hour)},
			{[]byte("path/b"), 7, time.Now()},
			{[]byte("path/c"), 8, time.Time{}},
			{[]byte("path/d"), 9, time.Time{}},
			{[]byte("path/e"), 9, time.Now().Add(-7 * time.Hour)},
			{[]byte("path/f"), 9, time.Now().Add(-8 * time.Hour)},
			{[]byte("path/g"), 10, time.Time{}},
			{[]byte("path/h"), 10, time.Now().Add(-8 * time.Hour)},
		}
		// shuffle list since select order should not depend on insert order
		rand.Seed(time.Now().UnixNano())
		rand.Shuffle(len(injuredSegList), func(i, j int) {
			injuredSegList[i], injuredSegList[j] = injuredSegList[j], injuredSegList[i]
		})
		for _, item := range injuredSegList {
			// first, insert the injured segment
			injuredSeg := &internalpb.InjuredSegment{Path: item.path}
			alreadyInserted, err := repairQueue.Insert(ctx, injuredSeg, item.health)
			require.NoError(t, err)
			require.False(t, alreadyInserted)

			// next, if applicable, update the "attempted at" timestamp
			if !item.attempted.IsZero() {
				res, err := dbAccess.ExecContext(ctx, dbAccess.Rebind(`UPDATE injuredsegments SET attempted = ? WHERE path = ?`), item.attempted, item.path)
				require.NoError(t, err)
				count, err := res.RowsAffected()
				require.NoError(t, err)
				require.EqualValues(t, 1, count)
			}
		}

		// we expect segment health to be prioritized first
		// if segment health is equal, we expect the least recently attempted, with nulls first, to be prioritized first
		// (excluding segments that have been attempted in the past six hours)
		// we do not expect to see segments that have been attempted in the past hour
		// therefore, the order of selection should be:
		// "path/a", "path/c", "path/d", "path/f", "path/e", "path/g", "path/h"
		// "path/b" will not be selected because it was attempted recently

		for _, nextPath := range []string{
			"path/a",
			"path/c",
			"path/d",
			"path/f",
			"path/e",
			"path/g",
			"path/h",
		} {
			injuredSeg, err := repairQueue.Select(ctx)
			require.NoError(t, err)
			assert.Equal(t, nextPath, string(injuredSeg.Path))
		}

		// queue should be considered "empty" now
		injuredSeg, err := repairQueue.Select(ctx)
		assert.True(t, storage.ErrEmptyQueue.Has(err))
		assert.Nil(t, injuredSeg)
	})
}

// TestOrderOverwrite ensures that re-inserting the same segment with a lower health, will properly adjust its prioritizationTestOrderOverwrite ensures that re-inserting the same segment with a lower health, will properly adjust its prioritization.
func TestOrderOverwrite(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		repairQueue := db.RepairQueue()

		// insert "path/a" with segment health 10
		// insert "path/b" with segment health 9
		// re-insert "path/a" with segment health 8
		// when we select, expect "path/a" first since after the re-insert, it is the least durable segment.

		// insert the 8 segments according to the plan above
		injuredSegList := []struct {
			path   []byte
			health int
		}{
			{[]byte("path/a"), 10},
			{[]byte("path/b"), 9},
			{[]byte("path/a"), 8},
		}
		for i, item := range injuredSegList {
			injuredSeg := &internalpb.InjuredSegment{Path: item.path}
			alreadyInserted, err := repairQueue.Insert(ctx, injuredSeg, item.health)
			require.NoError(t, err)
			if i == 2 {
				require.True(t, alreadyInserted)
			} else {
				require.False(t, alreadyInserted)
			}
		}

		for _, nextPath := range []string{
			"path/a",
			"path/b",
		} {
			injuredSeg, err := repairQueue.Select(ctx)
			require.NoError(t, err)
			assert.Equal(t, nextPath, string(injuredSeg.Path))
		}

		// queue should be considered "empty" now
		injuredSeg, err := repairQueue.Select(ctx)
		assert.True(t, storage.ErrEmptyQueue.Has(err))
		assert.Nil(t, injuredSeg)
	})
}

func TestCount(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		repairQueue := db.RepairQueue()

		// insert a bunch of segments
		pathsMap := make(map[string]int)
		numSegments := 20
		for i := 0; i < numSegments; i++ {
			path := "/path/" + strconv.Itoa(i)
			injuredSeg := &internalpb.InjuredSegment{Path: []byte(path)}
			alreadyInserted, err := repairQueue.Insert(ctx, injuredSeg, 10)
			require.NoError(t, err)
			require.False(t, alreadyInserted)
			pathsMap[path] = 0
		}

		count, err := repairQueue.Count(ctx)
		require.NoError(t, err)
		require.Equal(t, count, numSegments)
	})

}
