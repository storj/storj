// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metainfo/metabase"
	"storj.io/storj/storage"
)

func TestUpdateSegmentPieces(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := randObjectStream()

		now := time.Now()

		validPieces := []metabase.Piece{{
			Number:      1,
			StorageNode: testrand.NodeID(),
		}}

		t.Run("StreamID missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			UpdateSegmentPieces{
				Opts:     metabase.UpdateSegmentPieces{},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "StreamID missing",
			}.Check(ctx, t, db)
		})

		t.Run("OldPieces missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			UpdateSegmentPieces{
				Opts: metabase.UpdateSegmentPieces{
					StreamID: obj.StreamID,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "OldPieces: pieces missing",
			}.Check(ctx, t, db)
		})

		t.Run("OldPieces: piece number 1 is missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			UpdateSegmentPieces{
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
			Verify{}.Check(ctx, t, db)
		})

		t.Run("OldPieces: duplicated piece number 1", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			UpdateSegmentPieces{
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
			Verify{}.Check(ctx, t, db)
		})

		t.Run("OldPieces: pieces should be ordered", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			UpdateSegmentPieces{
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
			Verify{}.Check(ctx, t, db)
		})

		t.Run("NewRedundancy zero", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			UpdateSegmentPieces{
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
			defer DeleteAll{}.Check(ctx, t, db)

			UpdateSegmentPieces{
				Opts: metabase.UpdateSegmentPieces{
					StreamID: obj.StreamID,
					OldPieces: []metabase.Piece{{
						Number:      1,
						StorageNode: testrand.NodeID(),
					}},
					NewRedundancy: defaultTestRedundancy,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "number of new pieces is less than new redundancy repair shares value",
			}.Check(ctx, t, db)
		})

		t.Run("NewPieces: piece number 1 is missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			UpdateSegmentPieces{
				Opts: metabase.UpdateSegmentPieces{
					StreamID:      obj.StreamID,
					OldPieces:     validPieces,
					NewRedundancy: defaultTestRedundancy,
					NewPieces: []metabase.Piece{{
						Number:      1,
						StorageNode: storj.NodeID{},
					}},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "NewPieces: piece number 1 is missing storage node id",
			}.Check(ctx, t, db)
			Verify{}.Check(ctx, t, db)
		})

		t.Run("NewPieces: duplicated piece number 1", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			UpdateSegmentPieces{
				Opts: metabase.UpdateSegmentPieces{
					StreamID:      obj.StreamID,
					OldPieces:     validPieces,
					NewRedundancy: defaultTestRedundancy,
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
			Verify{}.Check(ctx, t, db)
		})

		t.Run("NewPieces: pieces should be ordered", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			UpdateSegmentPieces{
				Opts: metabase.UpdateSegmentPieces{
					StreamID:      obj.StreamID,
					OldPieces:     validPieces,
					NewRedundancy: defaultTestRedundancy,
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
			Verify{}.Check(ctx, t, db)
		})

		t.Run("segment not found", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			UpdateSegmentPieces{
				Opts: metabase.UpdateSegmentPieces{
					StreamID:      obj.StreamID,
					Position:      metabase.SegmentPosition{Index: 1},
					OldPieces:     validPieces,
					NewRedundancy: defaultTestRedundancy,
					NewPieces:     validPieces,
				},
				ErrClass: &metabase.ErrSegmentNotFound,
				ErrText:  "segment missing",
			}.Check(ctx, t, db)
		})

		t.Run("segment pieces column was changed", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			obj := createObject(ctx, t, db, obj, 1)

			newRedundancy := storj.RedundancyScheme{
				RequiredShares: 1,
				RepairShares:   1,
				OptimalShares:  1,
				TotalShares:    4,
			}

			UpdateSegmentPieces{
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
				ErrClass: &storage.ErrValueChanged,
				ErrText:  "segment remote_alias_pieces field was changed",
			}.Check(ctx, t, db)

			// verify that original pieces and redundancy did not change
			Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(obj),
				},
				Segments: []metabase.RawSegment{
					{
						StreamID:          obj.StreamID,
						RootPieceID:       storj.PieceID{1},
						CreatedAt:         &now,
						EncryptedKey:      []byte{3},
						EncryptedKeyNonce: []byte{4},
						EncryptedETag:     []byte{5},
						EncryptedSize:     1024,
						PlainOffset:       0,
						PlainSize:         512,

						Redundancy: defaultTestRedundancy,
						Pieces:     metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("update pieces", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			object := createObject(ctx, t, db, obj, 1)

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

			UpdateSegmentPieces{
				Opts: metabase.UpdateSegmentPieces{
					StreamID:      obj.StreamID,
					Position:      metabase.SegmentPosition{Index: 0},
					OldPieces:     segment.Pieces,
					NewRedundancy: defaultTestRedundancy,
					NewPieces:     expectedPieces,
				},
			}.Check(ctx, t, db)

			expectedSegment := segment
			expectedSegment.Pieces = expectedPieces
			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Committed,
						SegmentCount: 1,

						TotalPlainSize:     512,
						TotalEncryptedSize: 1024,
						FixedSegmentSize:   512,

						Encryption: defaultTestEncryption,
					},
				},
				Segments: []metabase.RawSegment{
					metabase.RawSegment(expectedSegment),
				},
			}.Check(ctx, t, db)
		})

		t.Run("update pieces and repair at", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			object := createObject(ctx, t, db, obj, 1)

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
			UpdateSegmentPieces{
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

			segment, err = db.GetSegmentByLocation(ctx, metabase.GetSegmentByLocation{
				SegmentLocation: metabase.SegmentLocation{
					ProjectID:  object.ProjectID,
					BucketName: object.BucketName,
					ObjectKey:  object.ObjectKey,
					Position:   metabase.SegmentPosition{Index: 0},
				},
			})
			require.NoError(t, err)
			diff := cmp.Diff(expectedSegment, segment, cmpopts.EquateApproxTime(5*time.Second))
			require.Zero(t, diff)

			segment, err = db.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
				StreamID: object.StreamID,
				Position: metabase.SegmentPosition{Index: 0},
			})
			require.NoError(t, err)
			diff = cmp.Diff(expectedSegment, segment, cmpopts.EquateApproxTime(5*time.Second))
			require.Zero(t, diff)

			segment, err = db.GetSegmentByOffset(ctx, metabase.GetSegmentByOffset{
				ObjectLocation: object.Location(),
				PlainOffset:    0,
			})
			require.NoError(t, err)
			diff = cmp.Diff(expectedSegment, segment, cmpopts.EquateApproxTime(5*time.Second))
			require.Zero(t, diff)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Committed,
						SegmentCount: 1,

						TotalPlainSize:     512,
						TotalEncryptedSize: 1024,
						FixedSegmentSize:   512,

						Encryption: defaultTestEncryption,
					},
				},
				Segments: []metabase.RawSegment{
					metabase.RawSegment(expectedSegment),
				},
			}.Check(ctx, t, db)
		})
	})
}
