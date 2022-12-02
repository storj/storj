// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestVerifyQueueBasicUsage(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		verifyQueue := db.VerifyQueue()

		// generate random segment records
		segmentsToVerify := make([]audit.Segment, 6)
		for i := range segmentsToVerify {
			segmentsToVerify[i].StreamID = testrand.UUID()
			segmentsToVerify[i].Position = metabase.SegmentPositionFromEncoded(rand.Uint64())
			segmentsToVerify[i].EncryptedSize = rand.Int31()
		}
		expireTime := time.Now().Add(24 * time.Hour).Truncate(time.Microsecond)
		segmentsToVerify[1].ExpiresAt = &expireTime

		// add these segments to the queue, 3 at a time
		err := verifyQueue.Push(ctx, segmentsToVerify[0:3], 3)
		require.NoError(t, err)
		err = verifyQueue.Push(ctx, segmentsToVerify[3:6], 3)
		require.NoError(t, err)

		// sort both sets of 3. segments inserted in the same call to Push
		// can't be differentiated by insertion time, so they are ordered in the
		// queue by (stream_id, position). We will sort our list here so that it
		// matches the order we expect to receive them from the queue.
		sort.Sort(audit.ByStreamIDAndPosition(segmentsToVerify[0:3]))
		sort.Sort(audit.ByStreamIDAndPosition(segmentsToVerify[3:6]))

		// Pop all segments from the queue and check for a match with the input.
		for _, expected := range segmentsToVerify {
			popped, err := verifyQueue.Next(ctx)
			require.NoError(t, err)
			require.Equal(t, expected.StreamID, popped.StreamID)
			require.Equal(t, expected.Position, popped.Position)
			require.Equal(t, expected.ExpiresAt == nil, popped.ExpiresAt == nil)
			if expected.ExpiresAt != nil {
				require.Truef(t, expected.ExpiresAt.Equal(*popped.ExpiresAt), "expected %s but got %s", expected.ExpiresAt.Format(time.RFC3339), popped.ExpiresAt.Format(time.RFC3339))
			}
			require.Equal(t, expected.EncryptedSize, popped.EncryptedSize)
		}

		// Check that we got all segments.
		popped, err := verifyQueue.Next(ctx)
		require.Error(t, err)
		require.Truef(t, audit.ErrEmptyQueue.Has(err), "unexpected error %v", err)
		require.Equal(t, audit.Segment{}, popped)
	})
}

func TestVerifyQueueEmpty(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		verifyQueue := db.VerifyQueue()

		// insert empty list
		err := verifyQueue.Push(ctx, []audit.Segment{}, 1000)
		require.NoError(t, err)

		// read from empty queue
		popped, err := verifyQueue.Next(ctx)
		require.Error(t, err)
		require.Truef(t, audit.ErrEmptyQueue.Has(err), "unexpected error %v", err)
		require.Equal(t, audit.Segment{}, popped)
	})
}

func TestMultipleBatches(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		verifyQueue := db.VerifyQueue()

		// make a list of segments, and push them in batches
		const (
			numSegments = 20
			batchSize   = 5
		)
		segments := make([]audit.Segment, numSegments)
		for i := range segments {
			segments[i] = audit.Segment{
				StreamID:      testrand.UUID(),
				Position:      metabase.SegmentPositionFromEncoded(rand.Uint64()),
				EncryptedSize: rand.Int31(),
			}
		}

		err := verifyQueue.Push(ctx, segments, batchSize)
		require.NoError(t, err)

		// verify that all segments made it to the db
		sort.Sort(audit.ByStreamIDAndPosition(segments))

		for i := range segments {
			gotSegment, err := verifyQueue.Next(ctx)
			require.NoError(t, err)
			require.Equal(t, segments[i], gotSegment)
		}
	})
}
