// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestUpdateSegmentPieces(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		now := time.Now()

		validPieces := []metabase.Piece{{
			Number:      1,
			StorageNode: testrand.NodeID(),
		}}

		t.Run("StreamID missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.UpdateSegmentPieces{
				Opts:     metabase.UpdateSegmentPieces{},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "StreamID missing",
			}.Check(ctx, t, db)
		})

		t.Run("OldPieces missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.UpdateSegmentPieces{
				Opts: metabase.UpdateSegmentPieces{
					StreamID: obj.StreamID,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "OldPieces: pieces missing",
			}.Check(ctx, t, db)
		})

		t.Run("OldPieces: piece number 1 is missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.UpdateSegmentPieces{
				Opts: metabase.UpdateSegmentPieces{
					StreamID: obj.StreamID,
					OldPieces: []metabase.Piece{{
						Number:      1,
						StorageNode: storj.NodeID{},
					}},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "OldPieces: piece number 1 is missing storage node id",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("OldPieces: duplicated piece number 1", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.UpdateSegmentPieces{
				Opts: metabase.UpdateSegmentPieces{
					StreamID: obj.StreamID,
					OldPieces: []metabase.Piece{
						{
							Number:      1,
							StorageNode: testrand.NodeID(),
						},
						{
							Number:      1,
							StorageNode: testrand.NodeID(),
						},
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "OldPieces: duplicated piece number 1",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("OldPieces: pieces should be ordered", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.UpdateSegmentPieces{
				Opts: metabase.UpdateSegmentPieces{
					StreamID: obj.StreamID,
					OldPieces: []metabase.Piece{
						{
							Number:      2,
							StorageNode: testrand.NodeID(),
						},
						{
							Number:      1,
							StorageNode: testrand.NodeID(),
						},
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "OldPieces: pieces should be ordered",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("NewRedundancy zero", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.UpdateSegmentPieces{
				Opts: metabase.UpdateSegmentPieces{
					StreamID: obj.StreamID,
					OldPieces: []metabase.Piece{{
						Number:      1,
						StorageNode: testrand.NodeID(),
					}},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "NewRedundancy zero",
			}.Check(ctx, t, db)
		})

		t.Run("NewPieces vs NewRedundancy", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.UpdateSegmentPieces{
				Opts: metabase.UpdateSegmentPieces{
					StreamID: obj.StreamID,
					OldPieces: []metabase.Piece{{
						Number:      1,
						StorageNode: testrand.NodeID(),
					}},
					NewRedundancy: metabasetest.DefaultRedundancy,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "number of new pieces is less than new redundancy repair shares value",
			}.Check(ctx, t, db)
		})

		t.Run("NewPieces: piece number 1 is missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.UpdateSegmentPieces{
				Opts: metabase.UpdateSegmentPieces{
					StreamID:      obj.StreamID,
					OldPieces:     validPieces,
					NewRedundancy: metabasetest.DefaultRedundancy,
					NewPieces: []metabase.Piece{{
						Number:      1,
						StorageNode: storj.NodeID{},
					}},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "NewPieces: piece number 1 is missing storage node id",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("NewPieces: duplicated piece number 1", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.UpdateSegmentPieces{
				Opts: metabase.UpdateSegmentPieces{
					StreamID:      obj.StreamID,
					OldPieces:     validPieces,
					NewRedundancy: metabasetest.DefaultRedundancy,
					NewPieces: []metabase.Piece{
						{
							Number:      1,
							StorageNode: testrand.NodeID(),
						},
						{
							Number:      1,
							StorageNode: testrand.NodeID(),
						},
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "NewPieces: duplicated piece number 1",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("NewPieces: pieces should be ordered", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.UpdateSegmentPieces{
				Opts: metabase.UpdateSegmentPieces{
					StreamID:      obj.StreamID,
					OldPieces:     validPieces,
					NewRedundancy: metabasetest.DefaultRedundancy,
					NewPieces: []metabase.Piece{
						{
							Number:      2,
							StorageNode: testrand.NodeID(),
						},
						{
							Number:      1,
							StorageNode: testrand.NodeID(),
						},
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "NewPieces: pieces should be ordered",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("segment not found", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.UpdateSegmentPieces{
				Opts: metabase.UpdateSegmentPieces{
					StreamID:      obj.StreamID,
					Position:      metabase.SegmentPosition{Index: 1},
					OldPieces:     validPieces,
					NewRedundancy: metabasetest.DefaultRedundancy,
					NewPieces:     validPieces,
				},
				ErrClass: &metabase.ErrSegmentNotFound,
				ErrText:  "segment missing",
			}.Check(ctx, t, db)
		})

		t.Run("segment pieces column was changed", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.CreateObject(ctx, t, db, obj, 1)

			newRedundancy := storj.RedundancyScheme{
				RequiredShares: 1,
				RepairShares:   1,
				OptimalShares:  1,
				TotalShares:    4,
			}

			metabasetest.UpdateSegmentPieces{
				Opts: metabase.UpdateSegmentPieces{
					StreamID:      obj.StreamID,
					Position:      metabase.SegmentPosition{Index: 0},
					OldPieces:     validPieces,
					NewRedundancy: newRedundancy,
					NewPieces: metabase.Pieces{
						metabase.Piece{
							Number:      1,
							StorageNode: testrand.NodeID(),
						},
					},
				},
				ErrClass: &metabase.ErrValueChanged,
				ErrText:  "segment remote_alias_pieces field was changed",
			}.Check(ctx, t, db)

			// verify that original pieces and redundancy did not change
			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(obj),
				},
				Segments: []metabase.RawSegment{
					{
						StreamID:          obj.StreamID,
						RootPieceID:       storj.PieceID{1},
						CreatedAt:         now,
						EncryptedKey:      []byte{3},
						EncryptedKeyNonce: []byte{4},
						EncryptedETag:     []byte{5},
						EncryptedSize:     1024,
						PlainOffset:       0,
						PlainSize:         512,

						Redundancy: metabasetest.DefaultRedundancy,
						Pieces:     metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("update pieces", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.CreateObject(ctx, t, db, obj, 1)

			segment, err := db.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
				StreamID: object.StreamID,
				Position: metabase.SegmentPosition{Index: 0},
			})
			require.NoError(t, err)

			expectedPieces := metabase.Pieces{
				metabase.Piece{
					Number:      1,
					StorageNode: testrand.NodeID(),
				},
				metabase.Piece{
					Number:      2,
					StorageNode: testrand.NodeID(),
				},
			}

			metabasetest.UpdateSegmentPieces{
				Opts: metabase.UpdateSegmentPieces{
					StreamID:      obj.StreamID,
					Position:      metabase.SegmentPosition{Index: 0},
					OldPieces:     segment.Pieces,
					NewRedundancy: metabasetest.DefaultRedundancy,
					NewPieces:     expectedPieces,
				},
			}.Check(ctx, t, db)

			expectedSegment := segment
			expectedSegment.Pieces = expectedPieces
			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Committed,
						SegmentCount: 1,

						TotalPlainSize:     512,
						TotalEncryptedSize: 1024,
						FixedSegmentSize:   512,

						Encryption: metabasetest.DefaultEncryption,
					},
				},
				Segments: []metabase.RawSegment{
					metabase.RawSegment(expectedSegment),
				},
			}.Check(ctx, t, db)
		})

		t.Run("update pieces and repair at", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.CreateObject(ctx, t, db, obj, 1)

			segment, err := db.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
				StreamID: object.StreamID,
				Position: metabase.SegmentPosition{Index: 0},
			})
			require.NoError(t, err)

			expectedPieces := metabase.Pieces{
				metabase.Piece{
					Number:      1,
					StorageNode: testrand.NodeID(),
				},
				metabase.Piece{
					Number:      2,
					StorageNode: testrand.NodeID(),
				},
			}

			repairedAt := now.Add(time.Hour)
			metabasetest.UpdateSegmentPieces{
				Opts: metabase.UpdateSegmentPieces{
					StreamID:      obj.StreamID,
					Position:      metabase.SegmentPosition{Index: 0},
					OldPieces:     segment.Pieces,
					NewRedundancy: segment.Redundancy,
					NewPieces:     expectedPieces,
					NewRepairedAt: repairedAt,
				},
			}.Check(ctx, t, db)

			expectedSegment := segment
			expectedSegment.Pieces = expectedPieces
			expectedSegment.RepairedAt = &repairedAt

			segments, err := db.TestingAllSegments(ctx)
			require.NoError(t, err)
			require.Len(t, segments, 1)

			segment = segments[0]

			require.NoError(t, err)
			diff := cmp.Diff(expectedSegment, segment, metabasetest.DefaultTimeDiff())
			require.Zero(t, diff)

			segment, err = db.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
				StreamID: object.StreamID,
				Position: metabase.SegmentPosition{Index: 0},
			})
			require.NoError(t, err)
			diff = cmp.Diff(expectedSegment, segment, metabasetest.DefaultTimeDiff())
			require.Zero(t, diff)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Committed,
						SegmentCount: 1,

						TotalPlainSize:     512,
						TotalEncryptedSize: 1024,
						FixedSegmentSize:   512,

						Encryption: metabasetest.DefaultEncryption,
					},
				},
				Segments: []metabase.RawSegment{
					metabase.RawSegment(expectedSegment),
				},
			}.Check(ctx, t, db)
		})
	})
}
