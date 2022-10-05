// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/sync2"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestReverifyQueue(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		reverifyQueue := db.ReverifyQueue()

		locator1 := audit.PieceLocator{
			StreamID: testrand.UUID(),
			Position: metabase.SegmentPosition{
				Part:  uint32(testrand.Intn(1 << 10)),
				Index: uint32(testrand.Intn(1 << 20)),
			},
			NodeID:   testrand.NodeID(),
			PieceNum: testrand.Intn(1 << 10),
		}
		locator2 := audit.PieceLocator{
			StreamID: testrand.UUID(),
			Position: metabase.SegmentPosition{
				Part:  uint32(testrand.Intn(1 << 10)),
				Index: uint32(testrand.Intn(1 << 20)),
			},
			NodeID:   testrand.NodeID(),
			PieceNum: testrand.Intn(1 << 10),
		}

		err := reverifyQueue.Insert(ctx, locator1)
		require.NoError(t, err)

		// Postgres/Cockroach have only microsecond time resolution, so make
		// sure at least 1us has elapsed before inserting the second.
		sync2.Sleep(ctx, time.Microsecond)

		err = reverifyQueue.Insert(ctx, locator2)
		require.NoError(t, err)

		job1, err := reverifyQueue.GetNextJob(ctx)
		require.NoError(t, err)
		require.Equal(t, locator1, job1.Locator)
		require.Equal(t, 1, job1.ReverifyCount)

		job2, err := reverifyQueue.GetNextJob(ctx)
		require.NoError(t, err)
		require.Equal(t, locator2, job2.Locator)
		require.Equal(t, 1, job2.ReverifyCount)

		require.Truef(t, job1.InsertedAt.Before(job2.InsertedAt), "job1 [%s] should have an earlier insertion time than job2 [%s]", job1.InsertedAt, job2.InsertedAt)

		_, err = reverifyQueue.GetNextJob(ctx)
		require.Error(t, sql.ErrNoRows, err)

		// pretend that ReverifyRetryInterval has elapsed
		reverifyQueueTest := reverifyQueue.(interface {
			TestingFudgeUpdateTime(ctx context.Context, piece audit.PieceLocator, updateTime time.Time) error
		})
		err = reverifyQueueTest.TestingFudgeUpdateTime(ctx, locator1, time.Now().Add(-satellitedb.ReverifyRetryInterval))
		require.NoError(t, err)

		// job 1 should be eligible for a new worker to take over now (whatever
		// worker acquired job 1 before is presumed to have died or timed out).
		job3, err := reverifyQueue.GetNextJob(ctx)
		require.NoError(t, err)
		require.Equal(t, locator1, job3.Locator)
		require.Equal(t, 2, job3.ReverifyCount)

		wasDeleted, err := reverifyQueue.Remove(ctx, locator1)
		require.NoError(t, err)
		require.True(t, wasDeleted)
		wasDeleted, err = reverifyQueue.Remove(ctx, locator2)
		require.NoError(t, err)
		require.True(t, wasDeleted)
		wasDeleted, err = reverifyQueue.Remove(ctx, locator1)
		require.NoError(t, err)
		require.False(t, wasDeleted)

		_, err = reverifyQueue.GetNextJob(ctx)
		require.Error(t, sql.ErrNoRows, err)
	})
}
