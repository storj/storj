// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"
	"time"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metainfo/metabase"
)

func TestListSegments(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := randObjectStream()
		now := time.Now()

		t.Run("StreamID missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			ListSegments{
				Opts:     metabase.ListSegments{},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "StreamID missing",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Invalid limit", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			ListSegments{
				Opts: metabase.ListSegments{
					StreamID: obj.StreamID,
					Limit:    -1,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Invalid limit: -1",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("List no segments", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			ListSegments{
				Opts: metabase.ListSegments{
					StreamID: obj.StreamID,
					Limit:    1,
				},
				Result: metabase.ListSegmentsResult{},
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("List segments", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			expectedObject := createObject(ctx, t, db, obj, 10)

			expectedSegment := metabase.Segment{
				StreamID: obj.StreamID,
				Position: metabase.SegmentPosition{
					Index: 0,
				},
				CreatedAt:         &now,
				RootPieceID:       storj.PieceID{1},
				EncryptedKey:      []byte{3},
				EncryptedKeyNonce: []byte{4},
				EncryptedETag:     []byte{5},
				EncryptedSize:     1024,
				PlainSize:         512,
				Pieces:            metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
				Redundancy:        defaultTestRedundancy,
			}

			expectedRawSegments := make([]metabase.RawSegment, 10)
			expectedSegments := make([]metabase.Segment, 10)
			for i := range expectedSegments {
				expectedSegment.Position.Index = uint32(i)
				expectedSegments[i] = expectedSegment
				expectedRawSegments[i] = metabase.RawSegment(expectedSegment)
			}

			ListSegments{
				Opts: metabase.ListSegments{
					StreamID: obj.StreamID,
					Limit:    10,
				},
				Result: metabase.ListSegmentsResult{
					Segments: expectedSegments,
				},
			}.Check(ctx, t, db)

			ListSegments{
				Opts: metabase.ListSegments{
					StreamID: obj.StreamID,
					Limit:    1,
				},
				Result: metabase.ListSegmentsResult{
					Segments: expectedSegments[:1],
					More:     true,
				},
			}.Check(ctx, t, db)

			ListSegments{
				Opts: metabase.ListSegments{
					StreamID: obj.StreamID,
					Limit:    2,
					Cursor: metabase.SegmentPosition{
						Index: 1,
					},
				},
				Result: metabase.ListSegmentsResult{
					Segments: expectedSegments[2:4],
					More:     true,
				},
			}.Check(ctx, t, db)

			ListSegments{
				Opts: metabase.ListSegments{
					StreamID: obj.StreamID,
					Limit:    2,
					Cursor: metabase.SegmentPosition{
						Index: 10,
					},
				},
				Result: metabase.ListSegmentsResult{
					More: false,
				},
			}.Check(ctx, t, db)

			ListSegments{
				Opts: metabase.ListSegments{
					StreamID: obj.StreamID,
					Limit:    2,
					Cursor: metabase.SegmentPosition{
						Part:  1,
						Index: 10,
					},
				},
				Result: metabase.ListSegmentsResult{
					More: false,
				},
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(expectedObject),
				},
				Segments: expectedRawSegments,
			}.Check(ctx, t, db)
		})

		t.Run("List segments from unordered parts", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			var testCases = []struct {
				segments []metabase.SegmentPosition
			}{
				{[]metabase.SegmentPosition{
					{Part: 3, Index: 0},
					{Part: 0, Index: 0},
					{Part: 1, Index: 0},
					{Part: 2, Index: 0},
				}},
				{[]metabase.SegmentPosition{
					{Part: 3, Index: 0},
					{Part: 2, Index: 0},
					{Part: 1, Index: 0},
					{Part: 0, Index: 0},
				}},
				{[]metabase.SegmentPosition{
					{Part: 0, Index: 0},
					{Part: 2, Index: 0},
					{Part: 1, Index: 0},
					{Part: 3, Index: 0},
				}},
			}

			expectedSegment := metabase.Segment{
				StreamID:          obj.StreamID,
				CreatedAt:         &now,
				RootPieceID:       storj.PieceID{1},
				EncryptedKey:      []byte{3},
				EncryptedKeyNonce: []byte{4},
				EncryptedETag:     []byte{5},
				EncryptedSize:     1024,
				PlainSize:         512,
				Pieces:            metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
				Redundancy:        defaultTestRedundancy,
			}

			for _, tc := range testCases {
				obj := randObjectStream()

				BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: obj,
						Encryption:   defaultTestEncryption,
					},
					Version: obj.Version,
				}.Check(ctx, t, db)

				for i, segmentPosition := range tc.segments {
					BeginSegment{
						Opts: metabase.BeginSegment{
							ObjectStream: obj,
							Position:     segmentPosition,
							RootPieceID:  storj.PieceID{byte(i + 1)},
							Pieces: []metabase.Piece{{
								Number:      1,
								StorageNode: testrand.NodeID(),
							}},
						},
					}.Check(ctx, t, db)

					CommitSegment{
						Opts: metabase.CommitSegment{
							ObjectStream: obj,
							Position:     segmentPosition,
							RootPieceID:  storj.PieceID{1},
							Pieces:       metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},

							EncryptedKey:      []byte{3},
							EncryptedKeyNonce: []byte{4},
							EncryptedETag:     []byte{5},

							EncryptedSize: 1024,
							PlainSize:     512,
							PlainOffset:   0,
							Redundancy:    defaultTestRedundancy,
						},
					}.Check(ctx, t, db)
				}

				CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream: obj,
					},
				}.Check(ctx, t, db)

				expectedSegments := make([]metabase.Segment, 4)
				for i := range expectedSegments {
					expectedSegments[i] = expectedSegment
					expectedSegments[i].StreamID = obj.StreamID
					expectedSegments[i].Position.Part = uint32(i)
				}

				ListSegments{
					Opts: metabase.ListSegments{
						StreamID: obj.StreamID,
						Limit:    0,
					},
					Result: metabase.ListSegmentsResult{
						Segments: expectedSegments,
					},
				}.Check(ctx, t, db)
			}
		})
	})
}

func TestListStreamPositions(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := randObjectStream()
		now := time.Now()

		t.Run("StreamID missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			ListStreamPositions{
				Opts:     metabase.ListStreamPositions{},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "StreamID missing",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Invalid limit", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			ListStreamPositions{
				Opts: metabase.ListStreamPositions{
					StreamID: obj.StreamID,
					Limit:    -1,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Invalid limit: -1",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("List no segments", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			ListStreamPositions{
				Opts: metabase.ListStreamPositions{
					StreamID: obj.StreamID,
					Limit:    1,
				},
				Result: metabase.ListStreamPositionsResult{},
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("List segments", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			expectedObject := createObject(ctx, t, db, obj, 10)

			expectedSegment := metabase.Segment{
				StreamID: obj.StreamID,
				Position: metabase.SegmentPosition{
					Index: 0,
				},
				CreatedAt:         &now,
				RootPieceID:       storj.PieceID{1},
				EncryptedKey:      []byte{3},
				EncryptedKeyNonce: []byte{4},
				EncryptedETag:     []byte{5},
				EncryptedSize:     1024,
				PlainSize:         512,
				Pieces:            metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
				Redundancy:        defaultTestRedundancy,
			}

			expectedRawSegments := make([]metabase.RawSegment, 10)
			expectedSegments := make([]metabase.SegmentPositionInfo, 10)
			for i := range expectedSegments {
				expectedSegment.Position.Index = uint32(i)
				expectedRawSegments[i] = metabase.RawSegment(expectedSegment)
				expectedSegments[i] = metabase.SegmentPositionInfo{
					Position:          expectedSegment.Position,
					PlainSize:         expectedSegment.PlainSize,
					CreatedAt:         &now,
					EncryptedKey:      expectedSegment.EncryptedKey,
					EncryptedKeyNonce: expectedSegment.EncryptedKeyNonce,
					EncryptedETag:     expectedSegment.EncryptedETag,
				}
			}

			ListStreamPositions{
				Opts: metabase.ListStreamPositions{
					StreamID: obj.StreamID,
					Limit:    10,
				},
				Result: metabase.ListStreamPositionsResult{
					Segments: expectedSegments,
				},
			}.Check(ctx, t, db)

			ListStreamPositions{
				Opts: metabase.ListStreamPositions{
					StreamID: obj.StreamID,
					Limit:    1,
				},
				Result: metabase.ListStreamPositionsResult{
					Segments: expectedSegments[:1],
					More:     true,
				},
			}.Check(ctx, t, db)

			ListStreamPositions{
				Opts: metabase.ListStreamPositions{
					StreamID: obj.StreamID,
					Limit:    2,
					Cursor: metabase.SegmentPosition{
						Index: 1,
					},
				},
				Result: metabase.ListStreamPositionsResult{
					Segments: expectedSegments[2:4],
					More:     true,
				},
			}.Check(ctx, t, db)

			ListStreamPositions{
				Opts: metabase.ListStreamPositions{
					StreamID: obj.StreamID,
					Limit:    2,
					Cursor: metabase.SegmentPosition{
						Index: 10,
					},
				},
				Result: metabase.ListStreamPositionsResult{
					More: false,
				},
			}.Check(ctx, t, db)

			ListStreamPositions{
				Opts: metabase.ListStreamPositions{
					StreamID: obj.StreamID,
					Limit:    2,
					Cursor: metabase.SegmentPosition{
						Part:  1,
						Index: 10,
					},
				},
				Result: metabase.ListStreamPositionsResult{
					More: false,
				},
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(expectedObject),
				},
				Segments: expectedRawSegments,
			}.Check(ctx, t, db)
		})

		t.Run("List segments from unordered parts", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			var testCases = []struct {
				segments []metabase.SegmentPosition
			}{
				{[]metabase.SegmentPosition{
					{Part: 3, Index: 0},
					{Part: 0, Index: 0},
					{Part: 1, Index: 0},
					{Part: 2, Index: 0},
				}},
				{[]metabase.SegmentPosition{
					{Part: 3, Index: 0},
					{Part: 2, Index: 0},
					{Part: 1, Index: 0},
					{Part: 0, Index: 0},
				}},
				{[]metabase.SegmentPosition{
					{Part: 0, Index: 0},
					{Part: 2, Index: 0},
					{Part: 1, Index: 0},
					{Part: 3, Index: 0},
				}},
			}

			expectedSegment := metabase.Segment{
				StreamID:          obj.StreamID,
				RootPieceID:       storj.PieceID{1},
				EncryptedKey:      []byte{3},
				EncryptedKeyNonce: []byte{4},
				EncryptedETag:     []byte{5},
				EncryptedSize:     1024,
				PlainSize:         512,
				Pieces:            metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
				Redundancy:        defaultTestRedundancy,
			}

			for _, tc := range testCases {
				obj := randObjectStream()

				BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: obj,
						Encryption:   defaultTestEncryption,
					},
					Version: obj.Version,
				}.Check(ctx, t, db)

				for i, segmentPosition := range tc.segments {
					BeginSegment{
						Opts: metabase.BeginSegment{
							ObjectStream: obj,
							Position:     segmentPosition,
							RootPieceID:  storj.PieceID{byte(i + 1)},
							Pieces: []metabase.Piece{{
								Number:      1,
								StorageNode: testrand.NodeID(),
							}},
						},
					}.Check(ctx, t, db)

					CommitSegment{
						Opts: metabase.CommitSegment{
							ObjectStream: obj,
							Position:     segmentPosition,
							RootPieceID:  storj.PieceID{1},
							Pieces:       metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},

							EncryptedKey:      []byte{3},
							EncryptedKeyNonce: []byte{4},
							EncryptedETag:     []byte{5},

							EncryptedSize: 1024,
							PlainSize:     512,
							PlainOffset:   0,
							Redundancy:    defaultTestRedundancy,
						},
					}.Check(ctx, t, db)
				}

				CommitObject{
					Opts: metabase.CommitObject{
						ObjectStream: obj,
					},
				}.Check(ctx, t, db)

				expectedSegments := make([]metabase.SegmentPositionInfo, 4)
				for i := range expectedSegments {
					pos := expectedSegment.Position
					pos.Part = uint32(i)
					expectedSegments[i] = metabase.SegmentPositionInfo{
						Position:          pos,
						PlainSize:         expectedSegment.PlainSize,
						CreatedAt:         &now,
						EncryptedKey:      expectedSegment.EncryptedKey,
						EncryptedKeyNonce: expectedSegment.EncryptedKeyNonce,
						EncryptedETag:     expectedSegment.EncryptedETag,
					}
				}

				ListStreamPositions{
					Opts: metabase.ListStreamPositions{
						StreamID: obj.StreamID,
						Limit:    0,
					},
					Result: metabase.ListStreamPositionsResult{
						Segments: expectedSegments,
					},
				}.Check(ctx, t, db)
			}
		})
	})
}
