// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"context"
	"math"
	"testing"
	"time"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestDeletePart(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		now := time.Now()
		zombieDeadline := now.Add(24 * time.Hour)

		defaultDeletePieces := func(ctx context.Context, segment metabase.DeletedSegmentInfo) error {
			return nil
		}

		t.Run("StreamID missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.DeletePart{
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "StreamID missing",
			}.Check(ctx, t, db)

		})

		t.Run("DeletePieces missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.DeletePart{
				Opts: metabase.DeletePart{
					StreamID:     obj.StreamID,
					DeletePieces: defaultDeletePieces,
				},
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("empty metabase", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.DeletePart{
				Opts: metabase.DeletePart{
					StreamID:     obj.StreamID,
					DeletePieces: defaultDeletePieces,
				},
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("no segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.CreateObject(ctx, t, db, obj, 0)

			metabasetest.DeletePart{
				Opts: metabase.DeletePart{
					StreamID:     object.StreamID,
					DeletePieces: defaultDeletePieces,
				},
			}.Check(ctx, t, db)
			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(object),
				},
			}.Check(ctx, t, db)
		})

		t.Run("no StreamID", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.CreateObject(ctx, t, db, obj, 1)

			metabasetest.DeletePart{
				Opts: metabase.DeletePart{
					StreamID:     testrand.UUID(),
					DeletePieces: defaultDeletePieces,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(object),
				},
				Segments: []metabase.RawSegment{
					metabasetest.DefaultRawSegment(object.ObjectStream, metabase.SegmentPosition{
						Part: 0, Index: 0,
					}),
				},
			}.Check(ctx, t, db)
		})

		t.Run("success", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
				Version: obj.Version,
			}.Check(ctx, t, db)

			segments := []metabase.SegmentPosition{
				{Part: 0, Index: 0},
				{Part: 1, Index: 0},
				{Part: 1, Index: 10},
				{Part: 1, Index: math.MaxUint32},
				{Part: 2, Index: 0},
				{Part: 50, Index: 0},
			}
			for i, segmentPosition := range segments {
				metabasetest.BeginSegment{
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

				commitDefaultSegment(ctx, t, db, obj, segmentPosition)
			}

			// delete non-existing part
			metabasetest.DeletePart{
				Opts: metabase.DeletePart{
					StreamID:   obj.StreamID,
					PartNumber: 100,
				},
			}.Check(ctx, t, db)

			// delete only single part
			metabasetest.DeletePart{
				Opts: metabase.DeletePart{
					StreamID:   obj.StreamID,
					PartNumber: 1,
				},
				Result: []metabase.DeletedSegmentInfo{
					{
						RootPieceID: storj.PieceID{1},
						Pieces:      metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
					},
					{
						RootPieceID: storj.PieceID{1},
						Pieces:      metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
					},
					{
						RootPieceID: storj.PieceID{1},
						Pieces:      metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
					},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream:           obj,
						Encryption:             metabasetest.DefaultEncryption,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						ZombieDeletionDeadline: &zombieDeadline,
					},
				},
				Segments: []metabase.RawSegment{
					metabasetest.DefaultRawSegment(obj, metabase.SegmentPosition{
						Part: 0, Index: 0,
					}),
					metabasetest.DefaultRawSegment(obj, metabase.SegmentPosition{
						Part: 2, Index: 0,
					}),
					metabasetest.DefaultRawSegment(obj, metabase.SegmentPosition{
						Part: 50, Index: 0,
					}),
				},
			}.Check(ctx, t, db)
		})
	})
}

func commitDefaultSegment(ctx *testcontext.Context, t *testing.T, db *metabase.DB, obj metabase.ObjectStream, segmentPosition metabase.SegmentPosition) {
	metabasetest.CommitSegment{
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
			Redundancy:    metabasetest.DefaultRedundancy,
		},
	}.Check(ctx, t, db)
}
