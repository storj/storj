// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package irreparable_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/metainfo/metabase"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestIrreparable(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		irrdb := db.Irreparable()

		// Create and insert test segment infos into DB
		var segments []*internalpb.IrreparableSegment
		for i := 0; i < 3; i++ {
			segments = append(segments, &internalpb.IrreparableSegment{
				Path: []byte(strconv.Itoa(i)),
				SegmentDetail: &pb.Pointer{
					CreationDate: time.Now(),
				},
				LostPieces:         int32(i),
				LastRepairAttempt:  time.Now().Unix(),
				RepairAttemptCount: int64(10),
			})

			err := irrdb.IncrementRepairAttempts(ctx, segments[i])
			require.NoError(t, err)
		}

		{ // GetLimited limit 1, starting from the beginning
			lastSeenSegmentPath := []byte{}
			segs, err := irrdb.GetLimited(ctx, 1, lastSeenSegmentPath)
			require.NoError(t, err)
			require.Equal(t, 1, len(segs))
			require.Empty(t, cmp.Diff(segments[0], segs[0], cmp.Comparer(pb.Equal)))
		}

		{ // GetLimited limit 1, starting after the first item
			lastSeenSegmentPath := []byte(strconv.Itoa(0))
			segs, err := irrdb.GetLimited(ctx, 1, lastSeenSegmentPath)
			require.NoError(t, err)
			require.Equal(t, 1, len(segs))
			require.Empty(t, cmp.Diff(segments[1], segs[0], cmp.Comparer(pb.Equal)))

		}

		{ // GetLimited limit 2, starting from the beginning
			lastSeenSegmentPath := []byte{}
			segs, err := irrdb.GetLimited(ctx, 2, lastSeenSegmentPath)
			require.NoError(t, err)
			require.Equal(t, 2, len(segs))
			require.Empty(t, cmp.Diff(segments[0], segs[0], cmp.Comparer(pb.Equal)))
			require.Empty(t, cmp.Diff(segments[1], segs[1], cmp.Comparer(pb.Equal)))
		}

		{ // GetLimited limit 2, starting after the first item
			lastSeenSegmentPath := []byte(strconv.Itoa(0))
			segs, err := irrdb.GetLimited(ctx, 2, lastSeenSegmentPath)
			require.NoError(t, err)
			require.Equal(t, 2, len(segs))
			require.Empty(t, cmp.Diff(segments[1], segs[0], cmp.Comparer(pb.Equal)))
			require.Empty(t, cmp.Diff(segments[2], segs[1], cmp.Comparer(pb.Equal)))

		}

		{ // GetLimited limit 3, starting after the first item
			lastSeenSegmentPath := []byte(strconv.Itoa(0))
			segs, err := irrdb.GetLimited(ctx, 3, lastSeenSegmentPath)
			require.NoError(t, err)
			require.Equal(t, 2, len(segs))
			require.Empty(t, cmp.Diff(segments[1], segs[0], cmp.Comparer(pb.Equal)))
			require.Empty(t, cmp.Diff(segments[2], segs[1], cmp.Comparer(pb.Equal)))
		}

		{ // Test repair count incrementation
			err := irrdb.IncrementRepairAttempts(ctx, segments[0])
			require.NoError(t, err)
			segments[0].RepairAttemptCount++

			dbxInfo, err := irrdb.Get(ctx, segments[0].Path)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(segments[0], dbxInfo, cmp.Comparer(pb.Equal)))
		}

		{ // Delete existing entry
			err := irrdb.Delete(ctx, segments[0].Path)
			require.NoError(t, err)

			_, err = irrdb.Get(ctx, segments[0].Path)
			require.Error(t, err)
		}
	})
}

func TestIrreparableProcess(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 3, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		checker := planet.Satellites[0].Repair.Checker
		checker.Loop.Stop()
		checker.IrreparableLoop.Stop()
		irreparabledb := planet.Satellites[0].DB.Irreparable()
		queue := planet.Satellites[0].DB.RepairQueue()

		seg := &internalpb.IrreparableSegment{
			Path: []byte{1},
			SegmentDetail: &pb.Pointer{
				Type:         pb.Pointer_REMOTE,
				CreationDate: time.Now(),
				Remote: &pb.RemoteSegment{
					Redundancy: &pb.RedundancyScheme{
						MinReq:           1,
						RepairThreshold:  2,
						SuccessThreshold: 3,
						Total:            4,
					},
					RemotePieces: []*pb.RemotePiece{
						{
							NodeId: planet.StorageNodes[0].ID(),
						},
						{
							NodeId: planet.StorageNodes[1].ID(),
						},
						{
							NodeId: planet.StorageNodes[2].ID(),
						},
					},
				},
			},
			LostPieces:         int32(4),
			LastRepairAttempt:  time.Now().Unix(),
			RepairAttemptCount: int64(10),
		}

		require.NoError(t, irreparabledb.IncrementRepairAttempts(ctx, seg))

		result, err := irreparabledb.Get(ctx, metabase.SegmentKey(seg.GetPath()))
		require.NoError(t, err)
		require.NotNil(t, result)

		// test healthy segment is removed from irreparable DB
		require.NoError(t, checker.IrreparableProcess(ctx))

		result, err = irreparabledb.Get(ctx, metabase.SegmentKey(seg.GetPath()))
		require.Error(t, err)
		require.Nil(t, result)

		// test unhealthy repairable segment is removed from irreparable DB and inserted into repair queue
		seg.SegmentDetail.Remote.RemotePieces[0] = &pb.RemotePiece{}
		seg.SegmentDetail.Remote.RemotePieces[1] = &pb.RemotePiece{}

		require.NoError(t, irreparabledb.IncrementRepairAttempts(ctx, seg))
		require.NoError(t, checker.IrreparableProcess(ctx))

		result, err = irreparabledb.Get(ctx, metabase.SegmentKey(seg.GetPath()))
		require.Error(t, err)
		require.Nil(t, result)

		injured, err := queue.Select(ctx)
		require.NoError(t, err)
		require.Equal(t, seg.GetPath(), injured.GetPath())

		n, err := queue.Clean(ctx, time.Now())
		require.NoError(t, err)
		require.EqualValues(t, 1, n)

		// test irreparable segment remains in irreparable DB and repair_attempt_count is incremented
		seg.SegmentDetail.Remote.RemotePieces[2] = &pb.RemotePiece{}

		require.NoError(t, irreparabledb.IncrementRepairAttempts(ctx, seg))
		require.NoError(t, checker.IrreparableProcess(ctx))

		result, err = irreparabledb.Get(ctx, metabase.SegmentKey(seg.GetPath()))
		require.NoError(t, err)
		require.Equal(t, seg.GetPath(), result.Path)
		require.Equal(t, seg.RepairAttemptCount+1, result.RepairAttemptCount)
	})
}
