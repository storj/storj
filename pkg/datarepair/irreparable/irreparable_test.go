// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package irreparable_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/datarepair/irreparable"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestIrreparable(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		irrdb := db.Irreparable()

		// Create and insert test segment infos into DB
		var segments []*irreparable.RemoteSegmentInfo
		for i := 0; i < 3; i++ {
			segments = append(segments, &irreparable.RemoteSegmentInfo{
				EncryptedSegmentPath:   []byte(strconv.Itoa(i)),
				EncryptedSegmentDetail: []byte(strconv.Itoa(i)),
				LostPiecesCount:        int64(i),
				RepairUnixSec:          time.Now().Unix(),
				RepairAttemptCount:     int64(10),
			})
			err := irrdb.IncrementRepairAttempts(ctx, segments[i])
			assert.NoError(t, err)
		}

		{ // GetLimited limit 1, offset 0
			segs, err := irrdb.GetLimited(ctx, 1, 0)
			assert.NoError(t, err)
			assert.Equal(t, 1, len(segs))
			assert.Equal(t, segments[0], segs[0])
		}

		{ // GetLimited limit 1, offset 1
			segs, err := irrdb.GetLimited(ctx, 1, 1)
			assert.NoError(t, err)
			assert.Equal(t, 1, len(segs))
			assert.Equal(t, segments[1], segs[0])
		}

		{ // GetLimited limit 2, offset 0
			segs, err := irrdb.GetLimited(ctx, 2, 0)
			assert.NoError(t, err)
			assert.Equal(t, 2, len(segs))
			assert.Equal(t, segments[0], segs[0])
			assert.Equal(t, segments[1], segs[1])
		}

		{ // GetLimited limit 2, offset 1
			segs, err := irrdb.GetLimited(ctx, 2, 1)
			assert.NoError(t, err)
			assert.Equal(t, 2, len(segs))
			assert.Equal(t, segments[1], segs[0])
			assert.Equal(t, segments[2], segs[1])
		}

		{ // GetLimited limit 3, offset 1
			segs, err := irrdb.GetLimited(ctx, 3, 1)
			assert.NoError(t, err)
			assert.Equal(t, 2, len(segs))
			assert.Equal(t, segments[1], segs[0])
			assert.Equal(t, segments[2], segs[1])
		}

		// When limit or offset is negative, postgres returns an error, but SQLite does not
		// { // Test GetLimited with negative limit
		// 	_, err := irrdb.GetLimited(ctx, -3, 0)
		// 	assert.Error(t, err)
		// }

		// { // Test GetLimited with negative offset
		// 	_, err := irrdb.GetLimited(ctx, 1, -3)
		// 	assert.Error(t, err)
		// }

		{ // Test repair count incrementation
			err := irrdb.IncrementRepairAttempts(ctx, segments[0])
			assert.NoError(t, err)
			segments[0].RepairAttemptCount++

			dbxInfo, err := irrdb.Get(ctx, segments[0].EncryptedSegmentPath)
			assert.NoError(t, err)
			assert.Equal(t, segments[0], dbxInfo)
		}

		{ //Delete existing entry
			err := irrdb.Delete(ctx, segments[0].EncryptedSegmentPath)
			assert.NoError(t, err)

			_, err = irrdb.Get(ctx, segments[0].EncryptedSegmentPath)
			assert.Error(t, err)
		}
	})
}
