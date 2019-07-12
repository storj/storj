// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/satellite"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/storage"
)

func TestRepairQueue(t *testing.T) {
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

func TestRepairQueueOrder(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		repairQueue := db.RepairQueue()

		nullPath := "/path/null"
		recentRepairPath := "/path/recent"
		oldRepairPath := "/path/old"
		olderRepairPath := "/path/older"

		for _, path := range []string{oldRepairPath, recentRepairPath, nullPath, olderRepairPath} {
			injuredSeg := &pb.InjuredSegment{Path: []byte(path)}
			err := repairQueue.Insert(ctx, injuredSeg)
			require.NoError(t, err)
		}

		dbAccess := db.(interface{ TestDBAccess() *dbx.DB }).TestDBAccess()
		// set recentRepairPath attempted to now, oldRepairPath attempted to 2 hours ago, olderRepairPath to 3 hours ago
		_, err := dbAccess.ExecContext(ctx, dbAccess.Rebind(`UPDATE injuredsegments SET attempted = datetime(?) WHERE path = ?`), time.Now(), recentRepairPath)
		require.NoError(t, err)
		_, err = dbAccess.ExecContext(ctx, dbAccess.Rebind(`UPDATE injuredsegments SET attempted = datetime(?) WHERE path = ?`), time.Now().Add(-2*time.Hour), oldRepairPath)
		require.NoError(t, err)
		_, err = dbAccess.ExecContext(ctx, dbAccess.Rebind(`UPDATE injuredsegments SET attempted = datetime(?) WHERE path = ?`), time.Now().Add(-3*time.Hour), olderRepairPath)
		require.NoError(t, err)

		// path with attempted = null should be selected first
		injuredSeg, err := repairQueue.Select(ctx)
		require.NoError(t, err)
		require.Equal(t, nullPath, string(injuredSeg.Path))

		// path with attempted = 3 hours ago should be selected next
		injuredSeg, err = repairQueue.Select(ctx)
		require.NoError(t, err)
		require.Equal(t, olderRepairPath, string(injuredSeg.Path))

		// path with attempted = 2 hours ago should be selected next
		injuredSeg, err = repairQueue.Select(ctx)
		require.NoError(t, err)
		require.Equal(t, oldRepairPath, string(injuredSeg.Path))

		// queue should be considered "empty" now
		injuredSeg, err = repairQueue.Select(ctx)
		require.True(t, storage.ErrEmptyQueue.Has(err))
		require.Nil(t, injuredSeg)
	})
}
