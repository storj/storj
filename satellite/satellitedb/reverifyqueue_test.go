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

func randomLocator() *audit.PieceLocator {
	return &audit.PieceLocator{
		StreamID: testrand.UUID(),
		Position: metabase.SegmentPosition{
			Part:  uint32(testrand.Intn(1 << 10)),
			Index: uint32(testrand.Intn(1 << 20)),
		},
		NodeID:   testrand.NodeID(),
		PieceNum: testrand.Intn(1 << 10),
	}
}

func TestReverifyQueue(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		reverifyQueue := db.ReverifyQueue()

		locator1 := randomLocator()
		locator2 := randomLocator()

		err := reverifyQueue.Insert(ctx, locator1)
		require.NoError(t, err)

		// Postgres/Cockroach have only microsecond time resolution, so make
		// sure at least 1us has elapsed before inserting the second.
		sync2.Sleep(ctx, time.Microsecond)

		err = reverifyQueue.Insert(ctx, locator2)
		require.NoError(t, err)

		job1, err := reverifyQueue.GetNextJob(ctx)
		require.NoError(t, err)
		require.Equal(t, *locator1, job1.Locator)
		require.Equal(t, 1, job1.ReverifyCount)

		job2, err := reverifyQueue.GetNextJob(ctx)
		require.NoError(t, err)
		require.Equal(t, *locator2, job2.Locator)
		require.Equal(t, 1, job2.ReverifyCount)

		require.Truef(t, job1.InsertedAt.Before(job2.InsertedAt), "job1 [%s] should have an earlier insertion time than job2 [%s]", job1.InsertedAt, job2.InsertedAt)

		_, err = reverifyQueue.GetNextJob(ctx)
		require.Error(t, sql.ErrNoRows, err)

		// pretend that ReverifyRetryInterval has elapsed
		reverifyQueueTest := reverifyQueue.(interface {
			TestingFudgeUpdateTime(ctx context.Context, piece *audit.PieceLocator, updateTime time.Time) error
		})
		err = reverifyQueueTest.TestingFudgeUpdateTime(ctx, locator1, time.Now().Add(-satellitedb.ReverifyRetryInterval))
		require.NoError(t, err)

		// job 1 should be eligible for a new worker to take over now (whatever
		// worker acquired job 1 before is presumed to have died or timed out).
		job3, err := reverifyQueue.GetNextJob(ctx)
		require.NoError(t, err)
		require.Equal(t, *locator1, job3.Locator)
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

func TestReverifyQueueGetByNodeID(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		reverifyQueue := db.ReverifyQueue()

		locator1 := randomLocator()
		locator2 := randomLocator()
		locator3 := randomLocator()
		// two jobs on the same node
		locator3.NodeID = locator1.NodeID

		err := reverifyQueue.Insert(ctx, locator1)
		require.NoError(t, err)

		// Postgres/Cockroach have only microsecond time resolution, so make
		// sure at least 1us has elapsed between inserts.
		sync2.Sleep(ctx, time.Microsecond)

		err = reverifyQueue.Insert(ctx, locator2)
		require.NoError(t, err)

		sync2.Sleep(ctx, time.Microsecond)

		err = reverifyQueue.Insert(ctx, locator3)
		require.NoError(t, err)

		job1, err := reverifyQueue.GetByNodeID(ctx, locator1.NodeID)
		require.NoError(t, err)

		// we got either locator1 or locator3
		if job1.Locator.StreamID == locator1.StreamID {
			require.Equal(t, locator1.NodeID, job1.Locator.NodeID)
			require.Equal(t, locator1.PieceNum, job1.Locator.PieceNum)
			require.Equal(t, locator1.Position, job1.Locator.Position)
		} else {
			require.Equal(t, locator3.StreamID, job1.Locator.StreamID)
			require.Equal(t, locator3.NodeID, job1.Locator.NodeID)
			require.Equal(t, locator3.PieceNum, job1.Locator.PieceNum)
			require.Equal(t, locator3.Position, job1.Locator.Position)
		}

		job2, err := reverifyQueue.GetByNodeID(ctx, locator2.NodeID)
		require.NoError(t, err)
		require.Equal(t, locator2.StreamID, job2.Locator.StreamID)
		require.Equal(t, locator2.NodeID, job2.Locator.NodeID)
		require.Equal(t, locator2.PieceNum, job2.Locator.PieceNum)
		require.Equal(t, locator2.Position, job2.Locator.Position)

		// ask for a nonexistent node ID
		job3, err := reverifyQueue.GetByNodeID(ctx, testrand.NodeID())
		require.Error(t, err)
		require.Truef(t, audit.ErrContainedNotFound.Has(err), "expected ErrContainedNotFound error but got %+v", err)
		require.Nil(t, job3)
	})
}
