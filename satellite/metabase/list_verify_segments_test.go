// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"
	"time"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestListVerifySegments(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()
		now := time.Now()

		t.Run("Invalid limit", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.ListSegments{
				Opts: metabase.ListSegments{
					StreamID: obj.StreamID,
					Limit:    -1,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Invalid limit: -1",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("no segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.ListSegments{
				Opts: metabase.ListSegments{
					StreamID: uuidBefore(obj.StreamID),
					Limit:    1,
				},
				Result: metabase.ListSegmentsResult{},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			_ = metabasetest.CreateObject(ctx, t, db, obj, 10)

			expectedSegment := metabase.VerifySegment{
				StreamID: obj.StreamID,
				Position: metabase.SegmentPosition{
					Index: 0,
				},
				CreatedAt:   now,
				RootPieceID: storj.PieceID{1},
				AliasPieces: metabase.AliasPieces{{Number: 0, Alias: 1}},
				Redundancy:  metabasetest.DefaultRedundancy,
			}

			expectedSegments := make([]metabase.VerifySegment, 10)
			for i := range expectedSegments {
				expectedSegment.Position.Index = uint32(i)
				expectedSegments[i] = expectedSegment
			}

			metabasetest.ListVerifySegments{
				Opts: metabase.ListVerifySegments{
					Limit: 10,
				},
				Result: metabase.ListVerifySegmentsResult{
					Segments: expectedSegments,
				},
			}.Check(ctx, t, db)

			metabasetest.ListVerifySegments{
				Opts: metabase.ListVerifySegments{
					CursorStreamID: uuidBefore(obj.StreamID),
					CursorPosition: metabase.SegmentPosition{},
					Limit:          1,
				},
				Result: metabase.ListVerifySegmentsResult{
					Segments: expectedSegments[:1],
				},
			}.Check(ctx, t, db)

			metabasetest.ListVerifySegments{
				Opts: metabase.ListVerifySegments{
					Limit:          2,
					CursorStreamID: obj.StreamID,
					CursorPosition: metabase.SegmentPosition{
						Index: 1,
					},
				},
				Result: metabase.ListVerifySegmentsResult{
					Segments: expectedSegments[2:4],
				},
			}.Check(ctx, t, db)

			metabasetest.ListVerifySegments{
				Opts: metabase.ListVerifySegments{
					Limit:          2,
					CursorStreamID: obj.StreamID,
					CursorPosition: metabase.SegmentPosition{
						Index: 10,
					},
				},
				Result: metabase.ListVerifySegmentsResult{},
			}.Check(ctx, t, db)

			metabasetest.ListVerifySegments{
				Opts: metabase.ListVerifySegments{
					CursorStreamID: obj.StreamID,
					CursorPosition: metabase.SegmentPosition{
						Part:  1,
						Index: 10,
					},
					Limit: 2,
				},
				Result: metabase.ListVerifySegmentsResult{},
			}.Check(ctx, t, db)
		})
	})
}

func uuidBefore(v uuid.UUID) uuid.UUID {
	for i := len(v) - 1; i >= 0; i-- {
		v[i]--
		if v[i] != 0xFF { // we didn't wrap around
			break
		}
	}
	return v
}
