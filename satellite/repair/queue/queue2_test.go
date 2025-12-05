// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package queue_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/satellite/repair/repairqueuetest"
	"storj.io/storj/shared/dbutil"
)

func TestUntilEmpty(t *testing.T) {
	repairqueuetest.Run(t, func(ctx *testcontext.Context, t *testing.T, repairQueue queue.RepairQueue) {
		// insert a bunch of segments
		idsMap := make(map[uuid.UUID]int)
		for i := 0; i < 20; i++ {
			injuredSeg := &queue.InjuredSegment{
				StreamID: testrand.UUID(),
			}
			alreadyInserted, err := repairQueue.Insert(ctx, injuredSeg)
			require.NoError(t, err)
			require.False(t, alreadyInserted)
			idsMap[injuredSeg.StreamID] = 0
		}

		// select segments until no more are returned, and we should get each one exactly once
		for {
			injuredSegs, err := repairQueue.Select(ctx, 1, nil, nil)
			if err != nil {
				require.True(t, queue.ErrEmpty.Has(err))
				break
			}
			err = repairQueue.Release(ctx, injuredSegs[0], true)
			require.NoError(t, err)
			idsMap[injuredSegs[0].StreamID]++
		}

		for _, selectCount := range idsMap {
			assert.Equal(t, selectCount, 1)
		}
	})
}

func TestOrder(t *testing.T) {
	repairqueuetest.Run(t, func(ctx *testcontext.Context, t *testing.T, repairQueue queue.RepairQueue) {
		nullID := testrand.UUID()
		recentID := testrand.UUID()
		oldID := testrand.UUID()
		olderID := testrand.UUID()

		for _, streamID := range []uuid.UUID{oldID, recentID, nullID, olderID} {
			injuredSeg := &queue.InjuredSegment{
				StreamID:      streamID,
				SegmentHealth: 10,
			}
			alreadyInserted, err := repairQueue.Insert(ctx, injuredSeg)
			require.NoError(t, err)
			require.False(t, alreadyInserted)
		}

		updateList := []struct {
			streamID    uuid.UUID
			attemptedAt time.Time
		}{
			{recentID, time.Now()},
			{oldID, time.Now().Add(-7 * time.Hour)},
			{olderID, time.Now().Add(-8 * time.Hour)},
		}
		for _, item := range updateList {
			rowsAffected, err := repairQueue.TestingSetAttemptedTime(ctx,
				0, item.streamID, metabase.SegmentPosition{}, item.attemptedAt)
			require.NoError(t, err)
			require.EqualValues(t, 1, rowsAffected)
		}

		// segment with attempted = null should be selected first
		injuredSegs, err := repairQueue.Select(ctx, 1, nil, nil)
		require.NoError(t, err)
		err = repairQueue.Release(ctx, injuredSegs[0], true)
		require.NoError(t, err)
		assert.Equal(t, nullID, injuredSegs[0].StreamID)

		// segment with attempted = 8 hours ago should be selected next
		injuredSegs, err = repairQueue.Select(ctx, 1, nil, nil)
		require.NoError(t, err)
		err = repairQueue.Release(ctx, injuredSegs[0], true)
		require.NoError(t, err)
		assert.Equal(t, olderID, injuredSegs[0].StreamID)

		// segment with attempted = 7 hours ago should be selected next
		injuredSegs, err = repairQueue.Select(ctx, 1, nil, nil)
		require.NoError(t, err)
		err = repairQueue.Release(ctx, injuredSegs[0], true)
		require.NoError(t, err)
		assert.Equal(t, oldID, injuredSegs[0].StreamID)

		// segment should be considered "empty" now
		injuredSegs, err = repairQueue.Select(ctx, 1, nil, nil)
		assert.True(t, queue.ErrEmpty.Has(err))
		assert.Nil(t, injuredSegs)
	})
}

// TestOrderHealthyPieces ensures that we select in the correct order, accounting for segment health as well as last attempted repair time. We only test on Postgres since Cockraoch doesn't order by segment health due to performance.
func TestOrderHealthyPieces(t *testing.T) {
	type hasImplementation interface {
		Implementation() dbutil.Implementation
	}
	repairqueuetest.Run(t, func(ctx *testcontext.Context, t *testing.T, rq queue.RepairQueue) {
		if dbi, ok := rq.(hasImplementation); ok {
			if dbi.Implementation() == dbutil.Cockroach {
				t.Skip("Cockroach does not order by segment health")
			}
		}
		testorderHealthyPieces(ctx, t, rq)
	})
}

func testorderHealthyPieces(ctx *testcontext.Context, t *testing.T, repairQueue queue.RepairQueue) {
	// we insert (path, segmentHealth, lastAttempted) as follows:
	// ("a", 6, now-8h)
	// ("b", 7, now)
	// ("c", 8, null)
	// ("d", 9, null)
	// ("e", 9, now-7h)
	// ("f", 9, now-8h)
	// ("g", 10, null)
	// ("h", 10, now-8h)

	// insert the 8 segments according to the plan above
	injuredSegList := []struct {
		streamID      uuid.UUID
		segmentHealth float64
		attempted     time.Time
	}{
		{uuid.UUID{'a'}, 6, time.Now().Add(-8 * time.Hour)},
		{uuid.UUID{'b'}, 7, time.Now()},
		{uuid.UUID{'c'}, 8, time.Time{}},
		{uuid.UUID{'d'}, 9, time.Time{}},
		{uuid.UUID{'e'}, 9, time.Now().Add(-7 * time.Hour)},
		{uuid.UUID{'f'}, 9, time.Now().Add(-8 * time.Hour)},
		{uuid.UUID{'g'}, 10, time.Time{}},
		{uuid.UUID{'h'}, 10, time.Now().Add(-8 * time.Hour)},
	}
	// shuffle list since select order should not depend on insert order
	rand.Shuffle(len(injuredSegList), func(i, j int) {
		injuredSegList[i], injuredSegList[j] = injuredSegList[j], injuredSegList[i]
	})
	for _, item := range injuredSegList {
		// first, insert the injured segment
		injuredSeg := &queue.InjuredSegment{
			StreamID:      item.streamID,
			SegmentHealth: item.segmentHealth,
		}
		alreadyInserted, err := repairQueue.Insert(ctx, injuredSeg)
		require.NoError(t, err)
		require.False(t, alreadyInserted)

		// next, if applicable, update the "attempted at" timestamp
		if !item.attempted.IsZero() {
			rowsAffected, err := repairQueue.TestingSetAttemptedTime(ctx, 0, item.streamID, metabase.SegmentPosition{}, item.attempted)
			require.NoError(t, err)
			require.EqualValues(t, 1, rowsAffected)
		}
	}

	// we expect segment health to be prioritized first
	// if segment health is equal, we expect the least recently attempted, with nulls first, to be prioritized first
	// (excluding segments that have been attempted in the past six hours)
	// we do not expect to see segments that have been attempted in the past hour
	// therefore, the order of selection should be:
	// "a", "c", "d", "f", "e", "g", "h"
	// "b" will not be selected because it was attempted recently

	for _, nextID := range []uuid.UUID{
		{'a'},
		{'c'},
		{'d'},
		{'f'},
		{'e'},
		{'g'},
		{'h'},
	} {
		injuredSegs, err := repairQueue.Select(ctx, 1, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, nextID, injuredSegs[0].StreamID)
		err = repairQueue.Release(ctx, injuredSegs[0], true)
		require.NoError(t, err)
	}

	// queue should be considered "empty" now
	injuredSeg, err := repairQueue.Select(ctx, 1, nil, nil)
	assert.True(t, queue.ErrEmpty.Has(err))
	assert.Nil(t, injuredSeg)
}

// TestOrderOverwrite ensures that re-inserting the same segment with a lower health, will properly adjust its prioritizationTestOrderOverwrite ensures that re-inserting the same segment with a lower health, will properly adjust its prioritization.
func TestOrderOverwrite(t *testing.T) {
	repairqueuetest.Run(t, func(ctx *testcontext.Context, t *testing.T, repairQueue queue.RepairQueue) {
		// insert segment A with segment health 10
		// insert segment B with segment health 9
		// re-insert segment A with segment segment health 8
		// when we select, expect segment A first since after the re-insert, it is the least durable segment.

		segmentA := uuid.UUID{1}
		segmentB := uuid.UUID{2}
		// insert the 8 segments according to the plan above
		injuredSegList := []struct {
			streamID      uuid.UUID
			segmentHealth float64
		}{
			{segmentA, 10},
			{segmentB, 9},
			{segmentA, 8},
		}
		for i, item := range injuredSegList {
			injuredSeg := &queue.InjuredSegment{
				StreamID:      item.streamID,
				SegmentHealth: item.segmentHealth,
			}
			alreadyInserted, err := repairQueue.Insert(ctx, injuredSeg)
			require.NoError(t, err)
			if i == 2 {
				require.True(t, alreadyInserted)
			} else {
				require.False(t, alreadyInserted)
			}
		}

		for _, nextStreamID := range []uuid.UUID{
			segmentA,
			segmentB,
		} {
			injuredSegs, err := repairQueue.Select(ctx, 1, nil, nil)
			require.NoError(t, err)
			assert.Equal(t, nextStreamID, injuredSegs[0].StreamID)
			err = repairQueue.Release(ctx, injuredSegs[0], true)
			require.NoError(t, err)
		}

		// queue should be considered "empty" now
		injuredSegs, err := repairQueue.Select(ctx, 1, nil, nil)
		assert.True(t, queue.ErrEmpty.Has(err))
		assert.Empty(t, injuredSegs)
	})
}

func TestCount(t *testing.T) {
	repairqueuetest.Run(t, func(ctx *testcontext.Context, t *testing.T, repairQueue queue.RepairQueue) {
		// insert a bunch of segments
		numSegments := 20
		for i := 0; i < numSegments; i++ {
			injuredSeg := &queue.InjuredSegment{
				StreamID: testrand.UUID(),
			}
			alreadyInserted, err := repairQueue.Insert(ctx, injuredSeg)
			require.NoError(t, err)
			require.False(t, alreadyInserted)
		}

		count, err := repairQueue.Count(ctx)
		require.NoError(t, err)
		require.Equal(t, count, numSegments)
	})

}
