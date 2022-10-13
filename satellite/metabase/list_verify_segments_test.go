// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestListVerifySegments(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		t.Run("Invalid limit", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.ListVerifySegments{
				Opts: metabase.ListVerifySegments{
					Limit: -1,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Invalid limit: -1",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("no segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.ListVerifySegments{
				Opts: metabase.ListVerifySegments{
					Limit: 1,
				},
				Result: metabase.ListVerifySegmentsResult{},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("aost", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.ListVerifySegments{
				Opts: metabase.ListVerifySegments{
					Limit:              1,
					AsOfSystemTime:     time.Now(),
					AsOfSystemInterval: time.Nanosecond,
				},
				Result: metabase.ListVerifySegmentsResult{},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("single object segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			_ = metabasetest.CreateObject(ctx, t, db, obj, 10)

			expectedSegments := make([]metabase.VerifySegment, 10)
			for i := range expectedSegments {
				expectedSegments[i] = defaultVerifySegment(obj.StreamID, uint32(i))
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

		t.Run("many objects segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			expectedVerifySegments := []metabase.VerifySegment{}

			for i := 0; i < 5; i++ {
				obj = metabasetest.RandObjectStream()
				obj.StreamID[0] = byte(i) // make StreamIDs ordered
				_ = metabasetest.CreateObject(ctx, t, db, obj, 1)

				expectedVerifySegments = append(expectedVerifySegments, defaultVerifySegment(obj.StreamID, 0))
			}

			metabasetest.ListVerifySegments{
				Opts: metabase.ListVerifySegments{
					Limit: 5,
				},
				Result: metabase.ListVerifySegmentsResult{
					Segments: expectedVerifySegments,
				},
			}.Check(ctx, t, db)

			metabasetest.ListVerifySegments{
				Opts: metabase.ListVerifySegments{
					Limit: 2,
				},
				Result: metabase.ListVerifySegmentsResult{
					Segments: expectedVerifySegments[:2],
				},
			}.Check(ctx, t, db)

			metabasetest.ListVerifySegments{
				Opts: metabase.ListVerifySegments{
					CursorStreamID: uuidBefore(expectedVerifySegments[2].StreamID),
					Limit:          2,
				},
				Result: metabase.ListVerifySegmentsResult{
					Segments: expectedVerifySegments[2:4],
				},
			}.Check(ctx, t, db)

			metabasetest.ListVerifySegments{
				Opts: metabase.ListVerifySegments{
					CursorStreamID: expectedVerifySegments[4].StreamID,
					Limit:          1,
				},
				Result: metabase.ListVerifySegmentsResult{},
			}.Check(ctx, t, db)
		})

		t.Run("mixed with inline segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			expectedVerifySegments := []metabase.VerifySegment{}

			obj.StreamID = uuid.UUID{0}
			for i := 0; i < 5; i++ {
				// object with inline segment
				obj.ObjectKey = metabasetest.RandObjectKey()
				obj.StreamID[obj.StreamID.Size()-1]++
				createInlineSegment := func(object metabase.Object, index int) metabase.Segment {
					err := db.CommitInlineSegment(ctx, metabase.CommitInlineSegment{
						ObjectStream: obj,
						Position: metabase.SegmentPosition{
							Index: uint32(index),
						},
						EncryptedKey:      testrand.Bytes(32),
						EncryptedKeyNonce: testrand.Bytes(32),
					})
					require.NoError(t, err)
					return metabase.Segment{}
				}
				metabasetest.CreateTestObject{
					CreateSegment: createInlineSegment,
				}.Run(ctx, t, db, obj, 1)

				// object with remote segment
				obj.ObjectKey = metabasetest.RandObjectKey()
				obj.StreamID[obj.StreamID.Size()-1]++
				metabasetest.CreateObject(ctx, t, db, obj, 1)

				expectedVerifySegments = append(expectedVerifySegments, defaultVerifySegment(obj.StreamID, 0))
			}

			metabasetest.ListVerifySegments{
				Opts: metabase.ListVerifySegments{
					Limit: 5,
				},
				Result: metabase.ListVerifySegmentsResult{
					Segments: expectedVerifySegments,
				},
			}.Check(ctx, t, db)

			metabasetest.ListVerifySegments{
				Opts: metabase.ListVerifySegments{
					Limit: 2,
				},
				Result: metabase.ListVerifySegmentsResult{
					Segments: expectedVerifySegments[:2],
				},
			}.Check(ctx, t, db)

			metabasetest.ListVerifySegments{
				Opts: metabase.ListVerifySegments{
					CursorStreamID: uuidBefore(expectedVerifySegments[2].StreamID),
					Limit:          2,
				},
				Result: metabase.ListVerifySegmentsResult{
					Segments: expectedVerifySegments[2:4],
				},
			}.Check(ctx, t, db)

			metabasetest.ListVerifySegments{
				Opts: metabase.ListVerifySegments{
					CursorStreamID: expectedVerifySegments[4].StreamID,
					Limit:          1,
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

func defaultVerifySegment(streamID uuid.UUID, index uint32) metabase.VerifySegment {
	return metabase.VerifySegment{
		StreamID: streamID,
		Position: metabase.SegmentPosition{
			Index: index,
		},
		CreatedAt:   time.Now(),
		RootPieceID: storj.PieceID{1},
		AliasPieces: metabase.AliasPieces{{Number: 0, Alias: 1}},
		Redundancy:  metabasetest.DefaultRedundancy,
	}
}
