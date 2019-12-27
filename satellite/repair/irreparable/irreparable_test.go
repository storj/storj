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
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestIrreparable(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		irrdb := db.Irreparable()

		// Create and insert test segment infos into DB
		var segments []*pb.IrreparableSegment
		for i := 0; i < 3; i++ {
			segments = append(segments, &pb.IrreparableSegment{
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

		{ //Delete existing entry
			err := irrdb.Delete(ctx, segments[0].Path)
			require.NoError(t, err)

			_, err = irrdb.Get(ctx, segments[0].Path)
			require.Error(t, err)
		}
	})
}
