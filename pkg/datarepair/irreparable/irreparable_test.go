// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package irreparable_test

import (
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

		//testing variables
		segmentInfo := &irreparable.RemoteSegmentInfo{
			EncryptedSegmentPath:   []byte("IamSegmentkeyinfo"),
			EncryptedSegmentDetail: []byte("IamSegmentdetailinfo"),
			LostPiecesCount:        int64(10),
			RepairUnixSec:          time.Now().Unix(),
			RepairAttemptCount:     int64(10),
		}

		{ // New entry
			err := irrdb.IncrementRepairAttempts(ctx, segmentInfo)
			assert.NoError(t, err)
		}

		{ //Create the already existing entry
			err := irrdb.IncrementRepairAttempts(ctx, segmentInfo)
			assert.NoError(t, err)
			segmentInfo.RepairAttemptCount++

			dbxInfo, err := irrdb.Get(ctx, segmentInfo.EncryptedSegmentPath)
			assert.NoError(t, err)
			assert.Equal(t, segmentInfo, dbxInfo)
		}

		{ //Delete existing entry
			err := irrdb.Delete(ctx, segmentInfo.EncryptedSegmentPath)
			assert.NoError(t, err)

			_, err = irrdb.Get(ctx, segmentInfo.EncryptedSegmentPath)
			assert.Error(t, err)
		}
	})
}
