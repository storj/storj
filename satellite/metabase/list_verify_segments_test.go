// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"context"
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
				ErrText:  "invalid limit: -1",
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

		t.Run("streamID list", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			expectedVerifySegments := []metabase.VerifySegment{}

			nbBuckets := 3
			bucketList := metabase.ListVerifyBucketList{}
			for i := 0; i < nbBuckets; i++ {
				projectID := testrand.UUID()
				bucketName := metabase.BucketName(testrand.BucketName())
				bucketList.Add(projectID, bucketName)

				obj := metabasetest.RandObjectStream()
				obj.ProjectID = projectID
				obj.BucketName = bucketName
				obj.StreamID[0] = byte(i) // make StreamIDs ordered
				_ = metabasetest.CreateObject(ctx, t, db, obj, 1)
				expectedVerifySegments = append(expectedVerifySegments, defaultVerifySegment(obj.StreamID, 0))
				// create a un-related object
				_ = metabasetest.CreateObject(ctx, t, db, metabasetest.RandObjectStream(), 2)

			}

			allStreamIDs := []uuid.UUID{}

			for _, bucket := range bucketList.Buckets {
				opts := metabase.ListBucketStreamIDs{
					Bucket: bucket,
					Limit:  10,
				}
				err := db.ListBucketStreamIDs(ctx, opts, func(ctx context.Context, streamIDs []uuid.UUID) error {
					allStreamIDs = append(allStreamIDs, streamIDs...)
					return nil
				})
				require.NoError(t, err)
			}
			require.Len(t, allStreamIDs, nbBuckets)

			metabasetest.ListVerifySegments{
				Opts: metabase.ListVerifySegments{
					StreamIDs: allStreamIDs,
					Limit:     5,
				},
				Result: metabase.ListVerifySegmentsResult{
					Segments: expectedVerifySegments,
				},
			}.Check(ctx, t, db)
		})

		t.Run("creation time filtering", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()

			expectedVerifySegments := []metabase.VerifySegment{}

			for i := 0; i < 5; i++ {
				obj = metabasetest.RandObjectStream()
				obj.StreamID[0] = byte(i) // make StreamIDs ordered
				_ = metabasetest.CreateObject(ctx, t, db, obj, 1)

				expectedVerifySegments = append(expectedVerifySegments, defaultVerifySegment(obj.StreamID, 0))
			}

			date := func(t time.Time) *time.Time {
				return &t
			}

			metabasetest.ListVerifySegments{
				Opts: metabase.ListVerifySegments{
					CreatedAfter: date(now.Add(-time.Hour)),
					Limit:        10,
				},
				Result: metabase.ListVerifySegmentsResult{Segments: expectedVerifySegments},
			}.Check(ctx, t, db)

			metabasetest.ListVerifySegments{
				Opts: metabase.ListVerifySegments{
					CreatedAfter: date(now.Add(time.Hour)),
					Limit:        10,
				},
				Result: metabase.ListVerifySegmentsResult{},
			}.Check(ctx, t, db)

			metabasetest.ListVerifySegments{
				Opts: metabase.ListVerifySegments{
					CreatedBefore: date(now.Add(time.Hour)),
					Limit:         10,
				},
				Result: metabase.ListVerifySegmentsResult{Segments: expectedVerifySegments},
			}.Check(ctx, t, db)

			metabasetest.ListVerifySegments{
				Opts: metabase.ListVerifySegments{
					CreatedBefore: date(now.Add(-time.Hour)),
					Limit:         10,
				},
				Result: metabase.ListVerifySegmentsResult{},
			}.Check(ctx, t, db)

			metabasetest.ListVerifySegments{
				Opts: metabase.ListVerifySegments{
					CreatedAfter:  date(now.Add(-time.Hour)),
					CreatedBefore: date(now.Add(time.Hour)),
					Limit:         10,
				},
				Result: metabase.ListVerifySegmentsResult{Segments: expectedVerifySegments},
			}.Check(ctx, t, db)

			metabasetest.ListVerifySegments{
				Opts: metabase.ListVerifySegments{
					CreatedAfter:  date(now.Add(time.Hour)),
					CreatedBefore: date(now.Add(2 * time.Hour)),
					Limit:         10,
				},
				Result: metabase.ListVerifySegmentsResult{},
			}.Check(ctx, t, db)
		})
	})
}

func TestListBucketStreamIDs(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		t.Run("many objects segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			nbBuckets := 3
			bucketList := metabase.ListVerifyBucketList{}
			expectedStreamIDs := []uuid.UUID{}
			obj := metabasetest.RandObjectStream()
			for i := 0; i < nbBuckets; i++ {
				projectID := testrand.UUID()
				projectID[0] = byte(i) // make projectID ordered
				bucketName := metabase.BucketName(testrand.BucketName())
				bucketList.Add(projectID, bucketName)

				obj.ProjectID = projectID
				obj.BucketName = bucketName
				obj.StreamID[0] = byte(i) // make StreamIDs ordered
				object := metabasetest.CreateObject(ctx, t, db, obj, 3)
				expectedStreamIDs = append(expectedStreamIDs, object.StreamID)
				// create a un-related object
				_ = metabasetest.CreateObject(ctx, t, db, metabasetest.RandObjectStream(), 2)

			}

			allStreamIDs := []uuid.UUID{}
			for _, bucket := range bucketList.Buckets {
				opts := metabase.ListBucketStreamIDs{
					Bucket: bucket,
					Limit:  10,
				}
				err := db.ListBucketStreamIDs(ctx, opts, func(ctx context.Context, streamIDs []uuid.UUID) error {
					allStreamIDs = append(allStreamIDs, streamIDs...)
					return nil
				})
				require.NoError(t, err)
			}
			require.Equal(t, expectedStreamIDs, allStreamIDs)
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
