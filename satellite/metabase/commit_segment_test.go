// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"fmt"
	"testing"
	"time"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestBeginSegment(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		for _, test := range metabasetest.InvalidObjectStreams(obj) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)
				metabasetest.BeginSegment{
					Opts: metabase.BeginSegment{
						ObjectStream: test.ObjectStream,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)
				metabasetest.Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("RootPieceID missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginSegment{
				Opts: metabase.BeginSegment{
					ObjectStream: obj,
					Pieces: []metabase.Piece{{
						Number:      1,
						StorageNode: testrand.NodeID(),
					}},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "RootPieceID missing",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Pieces missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginSegment{
				Opts: metabase.BeginSegment{
					ObjectStream: obj,
					RootPieceID:  storj.PieceID{1},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "pieces missing",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("StorageNode in pieces missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginSegment{
				Opts: metabase.BeginSegment{
					ObjectStream: obj,
					Pieces: []metabase.Piece{{
						Number:      1,
						StorageNode: storj.NodeID{},
					}},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "piece number 1 is missing storage node id",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Piece number 2 is duplicated", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginSegment{
				Opts: metabase.BeginSegment{
					ObjectStream: obj,
					Pieces: []metabase.Piece{
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
				ErrText:  "duplicated piece number 1",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Pieces should be ordered", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginSegment{
				Opts: metabase.BeginSegment{
					ObjectStream: obj,
					Pieces: []metabase.Piece{
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
				ErrText:  "pieces should be ordered",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("pending object missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginSegment{
				Opts: metabase.BeginSegment{
					ObjectStream: obj,
					RootPieceID:  storj.PieceID{1},
					Pieces: []metabase.Piece{{
						Number:      1,
						StorageNode: testrand.NodeID(),
					}},
				},
				ErrClass: &metabase.ErrPendingObjectMissing,
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("pending object missing when object committed", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
				},
			}.Check(ctx, t, db)

			metabasetest.BeginSegment{
				Opts: metabase.BeginSegment{
					ObjectStream: obj,
					RootPieceID:  storj.PieceID{1},
					Pieces: []metabase.Piece{{
						Number:      1,
						StorageNode: testrand.NodeID(),
					}},
				},
				ErrClass: &metabase.ErrPendingObjectMissing,
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("begin segment successfully", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.BeginSegment{
				Opts: metabase.BeginSegment{
					ObjectStream: obj,
					RootPieceID:  storj.PieceID{1},
					Pieces: []metabase.Piece{{
						Number:      1,
						StorageNode: testrand.NodeID(),
					}},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("multiple begin segment successfully", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			for i := 0; i < 5; i++ {
				metabasetest.BeginSegment{
					Opts: metabase.BeginSegment{
						ObjectStream: obj,
						RootPieceID:  storj.PieceID{1},
						Pieces: []metabase.Piece{{
							Number:      1,
							StorageNode: testrand.NodeID(),
						}},
					},
				}.Check(ctx, t, db)
			}

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})
	})
}

func TestCommitSegment(t *testing.T) {
	t.Parallel()
	for _, useMutations := range []bool{false, true} {
		t.Run(fmt.Sprintf("mutations=%v", useMutations), func(t *testing.T) {
			testCommitSegment(t, useMutations)
		})
	}
}

func testCommitSegment(t *testing.T, useMutations bool) {
	metabasetest.RunWithConfig(t, metabase.Config{
		ApplicationName: "metabase-tests",
	}, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		for _, test := range metabasetest.InvalidObjectStreams(obj) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)
				metabasetest.CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream:        test.ObjectStream,
						TestingUseMutations: useMutations,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)
				metabasetest.Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("invalid request", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Pieces: metabase.Pieces{{
						Number:      1,
						StorageNode: testrand.NodeID(),
					}},
					TestingUseMutations: useMutations,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "RootPieceID missing",
			}.Check(ctx, t, db)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream:        obj,
					TestingUseMutations: useMutations,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "pieces missing",
			}.Check(ctx, t, db)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Pieces: []metabase.Piece{{
						Number:      1,
						StorageNode: storj.NodeID{},
					}},
					TestingUseMutations: useMutations,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "piece number 1 is missing storage node id",
			}.Check(ctx, t, db)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Pieces: []metabase.Piece{
						{
							Number:      1,
							StorageNode: testrand.NodeID(),
						},
						{
							Number:      1,
							StorageNode: testrand.NodeID(),
						},
					},
					TestingUseMutations: useMutations,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "duplicated piece number 1",
			}.Check(ctx, t, db)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Pieces: []metabase.Piece{
						{
							Number:      2,
							StorageNode: testrand.NodeID(),
						},
						{
							Number:      1,
							StorageNode: testrand.NodeID(),
						},
					},
					TestingUseMutations: useMutations,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "pieces should be ordered",
			}.Check(ctx, t, db)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					RootPieceID:  testrand.PieceID(),
					Pieces: metabase.Pieces{{
						Number:      1,
						StorageNode: testrand.NodeID(),
					}},
					TestingUseMutations: useMutations,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedKey missing",
			}.Check(ctx, t, db)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					RootPieceID:  testrand.PieceID(),

					Pieces: metabase.Pieces{{
						Number:      1,
						StorageNode: testrand.NodeID(),
					}},

					EncryptedKey:        testrand.Bytes(32),
					TestingUseMutations: useMutations,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedKeyNonce missing",
			}.Check(ctx, t, db)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					RootPieceID:  testrand.PieceID(),

					Pieces: metabase.Pieces{{
						Number:      1,
						StorageNode: testrand.NodeID(),
					}},

					EncryptedKey:      testrand.Bytes(32),
					EncryptedKeyNonce: testrand.Bytes(32),

					EncryptedSize:       -1,
					TestingUseMutations: useMutations,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedSize negative or zero",
			}.Check(ctx, t, db)

			if metabase.ValidatePlainSize {
				metabasetest.CommitSegment{
					Opts: metabase.CommitSegment{
						ObjectStream: obj,
						RootPieceID:  testrand.PieceID(),

						Pieces: metabase.Pieces{{
							Number:      1,
							StorageNode: testrand.NodeID(),
						}},

						EncryptedKey:      testrand.Bytes(32),
						EncryptedKeyNonce: testrand.Bytes(32),

						EncryptedSize:       1024,
						PlainSize:           -1,
						TestingUseMutations: useMutations,
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "PlainSize negative or zero",
				}.Check(ctx, t, db)
			}

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					RootPieceID:  testrand.PieceID(),

					Pieces: metabase.Pieces{{
						Number:      1,
						StorageNode: testrand.NodeID(),
					}},

					EncryptedKey:      testrand.Bytes(32),
					EncryptedKeyNonce: testrand.Bytes(32),

					EncryptedSize:       1024,
					PlainSize:           512,
					PlainOffset:         -1,
					TestingUseMutations: useMutations,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "PlainOffset negative",
			}.Check(ctx, t, db)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					RootPieceID:  testrand.PieceID(),

					Pieces: metabase.Pieces{{
						Number:      1,
						StorageNode: testrand.NodeID(),
					}},

					EncryptedKey:      testrand.Bytes(32),
					EncryptedKeyNonce: testrand.Bytes(32),

					EncryptedSize:       1024,
					PlainSize:           512,
					PlainOffset:         0,
					TestingUseMutations: useMutations,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Redundancy zero",
			}.Check(ctx, t, db)

			redundancy := storj.RedundancyScheme{
				OptimalShares: 2,
			}

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Pieces: []metabase.Piece{
						{
							Number:      1,
							StorageNode: testrand.NodeID(),
						},
					},
					RootPieceID:       testrand.PieceID(),
					Redundancy:        redundancy,
					EncryptedKey:      testrand.Bytes(32),
					EncryptedKeyNonce: testrand.Bytes(32),

					EncryptedSize:       1024,
					PlainSize:           512,
					PlainOffset:         0,
					TestingUseMutations: useMutations,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "number of pieces is less than redundancy optimal shares value",
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("duplicate", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now1 := time.Now()
			object := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			rootPieceID := testrand.PieceID()
			pieces := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Bytes(32)

			metabasetest.BeginSegment{
				Opts: metabase.BeginSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Part: 0, Index: 0},
					RootPieceID:  rootPieceID,
					Pieces:       pieces,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Part: 0, Index: 0},
					RootPieceID:  rootPieceID,
					Pieces:       pieces,

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					EncryptedSize:       1024,
					PlainSize:           512,
					PlainOffset:         0,
					Redundancy:          metabasetest.DefaultRedundancy,
					TestingUseMutations: useMutations,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Part: 0, Index: 0},
					RootPieceID:  rootPieceID,
					Pieces:       pieces,

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					EncryptedSize:       1024,
					PlainSize:           512,
					PlainOffset:         0,
					Redundancy:          metabasetest.DefaultRedundancy,
					TestingUseMutations: useMutations,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: metabasetest.ObjectsToRaw(object),
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						Position:  metabase.SegmentPosition{Part: 0, Index: 0},
						CreatedAt: now1,

						RootPieceID:       rootPieceID,
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						EncryptedSize: 1024,
						PlainOffset:   0,
						PlainSize:     512,

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("overwrite", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now1 := time.Now()
			object := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			rootPieceID1 := testrand.PieceID()
			rootPieceID2 := testrand.PieceID()
			pieces1 := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			pieces2 := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Bytes(32)

			metabasetest.BeginSegment{
				Opts: metabase.BeginSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Part: 0, Index: 0},
					RootPieceID:  rootPieceID1,
					Pieces:       pieces1,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Part: 0, Index: 0},

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					PlainSize:   512,
					PlainOffset: 0,
					InlineData:  testrand.Bytes(512),
				},
			}.Check(ctx, t, db)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Part: 0, Index: 0},
					RootPieceID:  rootPieceID2,
					Pieces:       pieces2,

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					EncryptedSize:       1024,
					PlainSize:           512,
					PlainOffset:         0,
					Redundancy:          metabasetest.DefaultRedundancy,
					TestingUseMutations: useMutations,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: metabasetest.ObjectsToRaw(object),
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						Position:  metabase.SegmentPosition{Part: 0, Index: 0},
						CreatedAt: now1,

						RootPieceID:       rootPieceID2,
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						EncryptedSize: 1024,
						PlainOffset:   0,
						PlainSize:     512,

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces2,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("commit segment of missing object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			exptectedSegment := metabasetest.DefaultRawSegment(obj, metabase.SegmentPosition{})
			exptectedSegment.Pieces = metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					RootPieceID:  exptectedSegment.RootPieceID,
					Pieces:       exptectedSegment.Pieces,

					EncryptedKey:      exptectedSegment.EncryptedKey,
					EncryptedKeyNonce: exptectedSegment.EncryptedKeyNonce,
					EncryptedETag:     exptectedSegment.EncryptedETag,

					EncryptedSize:       exptectedSegment.EncryptedSize,
					PlainSize:           exptectedSegment.PlainSize,
					PlainOffset:         exptectedSegment.PlainOffset,
					Redundancy:          exptectedSegment.Redundancy,
					TestingUseMutations: useMutations,
				},
				ErrClass: &metabase.ErrPendingObjectMissing,
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					RootPieceID:  exptectedSegment.RootPieceID,
					Pieces:       exptectedSegment.Pieces,

					EncryptedKey:      exptectedSegment.EncryptedKey,
					EncryptedKeyNonce: exptectedSegment.EncryptedKeyNonce,
					EncryptedETag:     exptectedSegment.EncryptedETag,

					EncryptedSize:       exptectedSegment.EncryptedSize,
					PlainSize:           exptectedSegment.PlainSize,
					PlainOffset:         exptectedSegment.PlainOffset,
					Redundancy:          exptectedSegment.Redundancy,
					TestingUseMutations: useMutations,

					SkipPendingObject: true,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{Segments: []metabase.RawSegment{exptectedSegment}}.Check(ctx, t, db)
		})

		t.Run("commit segment of committed object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			rootPieceID := testrand.PieceID()
			pieces := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Bytes(32)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					RootPieceID:  rootPieceID,
					Pieces:       pieces,

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					EncryptedSize:       1024,
					PlainSize:           512,
					PlainOffset:         0,
					Redundancy:          metabasetest.DefaultRedundancy,
					TestingUseMutations: useMutations,
				},
				ErrClass: &metabase.ErrPendingObjectMissing,
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("commit segment of committed object with SkipPendingObject", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			exptectedSegment := metabasetest.DefaultRawSegment(obj, metabase.SegmentPosition{})

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
				},
			}.Check(ctx, t, db)

			// Should fail when trying to commit segment to committed object with SkipPendingObject
			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					RootPieceID:  exptectedSegment.RootPieceID,
					Pieces:       exptectedSegment.Pieces,

					EncryptedKey:      exptectedSegment.EncryptedKey,
					EncryptedKeyNonce: exptectedSegment.EncryptedKeyNonce,
					EncryptedETag:     exptectedSegment.EncryptedETag,

					EncryptedSize:       exptectedSegment.EncryptedSize,
					PlainSize:           exptectedSegment.PlainSize,
					PlainOffset:         exptectedSegment.PlainOffset,
					Redundancy:          exptectedSegment.Redundancy,
					TestingUseMutations: useMutations,

					SkipPendingObject: true,
				},
				ErrClass: &metabase.ErrPendingObjectMissing,
			}.Check(ctx, t, db)

			// Verify object is still committed without segments
			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("commit segment of object with expires at", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			rootPieceID := testrand.PieceID()
			pieces := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Bytes(32)

			expectedExpiresAt := time.Now().Add(33 * time.Hour)
			object := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
					ExpiresAt:    &expectedExpiresAt,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					ExpiresAt:    &expectedExpiresAt,
					RootPieceID:  rootPieceID,
					Pieces:       pieces,

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					EncryptedSize:       1024,
					PlainSize:           512,
					PlainOffset:         0,
					Redundancy:          metabasetest.DefaultRedundancy,
					TestingUseMutations: useMutations,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: metabasetest.ObjectsToRaw(object),
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						CreatedAt: object.CreatedAt,
						ExpiresAt: &expectedExpiresAt,

						RootPieceID:       rootPieceID,
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						EncryptedSize: 1024,
						PlainOffset:   0,
						PlainSize:     512,

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("commit segment of pending object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			rootPieceID := testrand.PieceID()
			pieces := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Bytes(32)
			encryptedETag := testrand.Bytes(32)

			object := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					RootPieceID:  rootPieceID,
					Pieces:       pieces,

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					EncryptedSize:       1024,
					PlainSize:           512,
					PlainOffset:         0,
					Redundancy:          metabasetest.DefaultRedundancy,
					EncryptedETag:       encryptedETag,
					TestingUseMutations: useMutations,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: metabasetest.ObjectsToRaw(object),
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						CreatedAt: object.CreatedAt,

						RootPieceID:       rootPieceID,
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						EncryptedSize: 1024,
						PlainOffset:   0,
						PlainSize:     512,
						EncryptedETag: encryptedETag,

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("update segment with SkipPendingObject", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			segment := metabasetest.DefaultRawSegment(obj, metabase.SegmentPosition{})

			// First commit a segment with SkipPendingObject
			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					RootPieceID:  segment.RootPieceID,
					Pieces:       segment.Pieces,

					EncryptedKey:      segment.EncryptedKey,
					EncryptedKeyNonce: segment.EncryptedKeyNonce,
					EncryptedETag:     segment.EncryptedETag,

					EncryptedSize:       segment.EncryptedSize,
					PlainSize:           segment.PlainSize,
					PlainOffset:         segment.PlainOffset,
					Redundancy:          segment.Redundancy,
					TestingUseMutations: useMutations,

					SkipPendingObject: true,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{Segments: []metabase.RawSegment{segment}}.Check(ctx, t, db)

			// Update the segment with new data
			newSegment := segment
			newSegment.RootPieceID = testrand.PieceID()
			newSegment.Pieces = metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			newSegment.EncryptedKey = testrand.Bytes(32)
			newSegment.EncryptedKeyNonce = testrand.Bytes(32)
			newSegment.EncryptedETag = testrand.Bytes(32)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					RootPieceID:  newSegment.RootPieceID,
					Pieces:       newSegment.Pieces,

					EncryptedKey:      newSegment.EncryptedKey,
					EncryptedKeyNonce: newSegment.EncryptedKeyNonce,
					EncryptedETag:     newSegment.EncryptedETag,

					EncryptedSize:       newSegment.EncryptedSize,
					PlainSize:           newSegment.PlainSize,
					PlainOffset:         newSegment.PlainOffset,
					Redundancy:          newSegment.Redundancy,
					TestingUseMutations: useMutations,

					SkipPendingObject: true,
				},
			}.Check(ctx, t, db)

			// Verify the segment was updated
			metabasetest.Verify{Segments: []metabase.RawSegment{newSegment}}.Check(ctx, t, db)
		})

		t.Run("commit segment with SkipPendingObject and ExpiresAt", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			segment := metabasetest.DefaultRawSegment(obj, metabase.SegmentPosition{})
			segment.Pieces = metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}

			expectedExpiresAt := time.Now().Add(33 * time.Hour)
			segment.ExpiresAt = &expectedExpiresAt

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					ExpiresAt:    &expectedExpiresAt,
					RootPieceID:  segment.RootPieceID,
					Pieces:       segment.Pieces,

					EncryptedKey:      segment.EncryptedKey,
					EncryptedKeyNonce: segment.EncryptedKeyNonce,
					EncryptedETag:     segment.EncryptedETag,

					EncryptedSize:       segment.EncryptedSize,
					PlainSize:           segment.PlainSize,
					PlainOffset:         segment.PlainOffset,
					Redundancy:          segment.Redundancy,
					TestingUseMutations: useMutations,

					SkipPendingObject: true,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Segments: []metabase.RawSegment{segment},
			}.Check(ctx, t, db)
		})
	})
}

func TestCommitInlineSegment(t *testing.T) {
	metabasetest.RunWithConfig(t, metabase.Config{
		ApplicationName: "metabase-tests",
	}, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()
		for _, test := range metabasetest.InvalidObjectStreams(obj) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)
				metabasetest.CommitInlineSegment{
					Opts: metabase.CommitInlineSegment{
						ObjectStream: test.ObjectStream,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)
				metabasetest.Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("invalid request", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedKey missing",
			}.Check(ctx, t, db)

			metabasetest.CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,
					InlineData:   []byte{1, 2, 3},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedKey missing",
			}.Check(ctx, t, db)

			metabasetest.CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,

					InlineData: []byte{1, 2, 3},

					EncryptedKey: testrand.Bytes(32),
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedKeyNonce missing",
			}.Check(ctx, t, db)

			if metabase.ValidatePlainSize {
				metabasetest.CommitInlineSegment{
					Opts: metabase.CommitInlineSegment{
						ObjectStream: obj,

						InlineData: []byte{1, 2, 3},

						EncryptedKey:      testrand.Bytes(32),
						EncryptedKeyNonce: testrand.Bytes(32),

						PlainSize: -1,
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  "PlainSize negative or zero",
				}.Check(ctx, t, db)
			}

			metabasetest.CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,

					InlineData: []byte{1, 2, 3},

					EncryptedKey:      testrand.Bytes(32),
					EncryptedKeyNonce: testrand.Bytes(32),

					PlainSize:   512,
					PlainOffset: -1,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "PlainOffset negative",
			}.Check(ctx, t, db)
		})

		t.Run("commit inline segment of missing object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Bytes(32)

			metabasetest.CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,
					InlineData:   []byte{1, 2, 3},

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					PlainSize:   512,
					PlainOffset: 0,

					SkipPendingObject: false,
				},
				ErrClass: &metabase.ErrPendingObjectMissing,
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)

			segment := metabasetest.DefaultRawSegment(obj, metabase.SegmentPosition{})
			segment.RootPieceID = storj.PieceID{}
			segment.InlineData = []byte{1, 2, 3}
			segment.EncryptedSize = int32(len(segment.InlineData))
			segment.Redundancy = storj.RedundancyScheme{}
			segment.Pieces = nil

			metabasetest.CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,

					InlineData: segment.InlineData,

					EncryptedKey:      segment.EncryptedKey,
					EncryptedKeyNonce: segment.EncryptedKeyNonce,
					EncryptedETag:     segment.EncryptedETag,

					PlainSize:   segment.PlainSize,
					PlainOffset: segment.PlainOffset,

					SkipPendingObject: true,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{Segments: []metabase.RawSegment{segment}}.Check(ctx, t, db)
		})

		t.Run("duplicate", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Bytes(32)

			metabasetest.CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Part: 0, Index: 0},
					InlineData:   []byte{1, 2, 3},

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					PlainSize:   512,
					PlainOffset: 0,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Part: 0, Index: 0},
					InlineData:   []byte{1, 2, 3},

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					PlainSize:   512,
					PlainOffset: 0,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: metabasetest.ObjectsToRaw(object),
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						Position:  metabase.SegmentPosition{Part: 0, Index: 0},
						CreatedAt: object.CreatedAt,

						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						PlainOffset: 0,
						PlainSize:   512,

						InlineData:    []byte{1, 2, 3},
						EncryptedSize: 3,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("overwrite", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Bytes(32)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Part: 0, Index: 0},
					RootPieceID:  testrand.PieceID(),
					Pieces:       metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}},

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					EncryptedSize: 1024,
					PlainSize:     512,
					PlainOffset:   999999,

					Redundancy: metabasetest.DefaultRedundancy,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Part: 0, Index: 0},
					InlineData:   []byte{4, 5, 6},

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					PlainSize:   512,
					PlainOffset: 0,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: metabasetest.ObjectsToRaw(object),
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						Position:  metabase.SegmentPosition{Part: 0, Index: 0},
						CreatedAt: object.CreatedAt,

						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						PlainOffset: 0,
						PlainSize:   512,

						InlineData:    []byte{4, 5, 6},
						EncryptedSize: 3,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("commit segment of committed object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Bytes(32)

			object := metabasetest.CreateObject(ctx, t, db, obj, 0)
			metabasetest.CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,
					InlineData:   []byte{1, 2, 3},

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					PlainSize:   512,
					PlainOffset: 0,
				},
				ErrClass: &metabase.ErrPendingObjectMissing,
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("commit empty segment of pending object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Bytes(32)
			encryptedETag := testrand.Bytes(32)

			object := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					PlainSize:     0,
					PlainOffset:   0,
					EncryptedETag: encryptedETag,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: metabasetest.ObjectsToRaw(object),
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						CreatedAt: object.CreatedAt,

						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						PlainOffset: 0,
						PlainSize:   0,

						EncryptedSize: 0,
						EncryptedETag: encryptedETag,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("commit segment of pending object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Bytes(32)
			encryptedETag := testrand.Bytes(32)

			object := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,
					InlineData:   []byte{1, 2, 3},

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					PlainSize:     512,
					PlainOffset:   0,
					EncryptedETag: encryptedETag,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: metabasetest.ObjectsToRaw(object),
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						CreatedAt: object.CreatedAt,

						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						PlainOffset: 0,
						PlainSize:   512,

						InlineData:    []byte{1, 2, 3},
						EncryptedSize: 3,
						EncryptedETag: encryptedETag,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("commit segment of object with expires at", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			encryptedKey := testrand.Bytes(32)
			encryptedKeyNonce := testrand.Bytes(32)
			encryptedETag := testrand.Bytes(32)

			expectedExpiresAt := time.Now().Add(33 * time.Hour)
			object := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
					ExpiresAt:    &expectedExpiresAt,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,
					ExpiresAt:    &expectedExpiresAt,
					InlineData:   []byte{1, 2, 3},

					EncryptedKey:      encryptedKey,
					EncryptedKeyNonce: encryptedKeyNonce,

					PlainSize:     512,
					PlainOffset:   0,
					EncryptedETag: encryptedETag,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: metabasetest.ObjectsToRaw(object),
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						CreatedAt: object.CreatedAt,
						ExpiresAt: &expectedExpiresAt,

						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,

						PlainOffset: 0,
						PlainSize:   512,

						InlineData:    []byte{1, 2, 3},
						EncryptedSize: 3,
						EncryptedETag: encryptedETag,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("commit inline segment of committed object with SkipPendingObject", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			segment := metabasetest.DefaultRawSegment(obj, metabase.SegmentPosition{})
			segment.InlineData = []byte{1, 2, 3}

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
				},
			}.Check(ctx, t, db)

			// Should fail when trying to commit inline segment to committed object with SkipPendingObject
			metabasetest.CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,

					InlineData: segment.InlineData,

					EncryptedKey:      segment.EncryptedKey,
					EncryptedKeyNonce: segment.EncryptedKeyNonce,
					EncryptedETag:     segment.EncryptedETag,

					PlainSize:   segment.PlainSize,
					PlainOffset: segment.PlainOffset,

					SkipPendingObject: true,
				},
				ErrClass: &metabase.ErrPendingObjectMissing,
			}.Check(ctx, t, db)

			// Verify object is still committed without segments
			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("update inline segment with SkipPendingObject", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			segment := metabasetest.DefaultRawInlineSegment(obj, metabase.SegmentPosition{})

			// First commit an inline segment with SkipPendingObject
			metabasetest.CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,

					InlineData: segment.InlineData,

					EncryptedKey:      segment.EncryptedKey,
					EncryptedKeyNonce: segment.EncryptedKeyNonce,
					EncryptedETag:     segment.EncryptedETag,

					PlainSize:   segment.PlainSize,
					PlainOffset: segment.PlainOffset,

					SkipPendingObject: true,
				},
			}.Check(ctx, t, db)

			// Update the inline segment with new data
			newSegment := segment
			newSegment.InlineData = []byte{4, 5, 6}
			newSegment.EncryptedKey = testrand.Bytes(32)
			newSegment.EncryptedKeyNonce = testrand.Bytes(32)
			newSegment.EncryptedETag = testrand.Bytes(32)

			metabasetest.CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,

					InlineData: newSegment.InlineData,

					EncryptedKey:      newSegment.EncryptedKey,
					EncryptedKeyNonce: newSegment.EncryptedKeyNonce,
					EncryptedETag:     newSegment.EncryptedETag,

					PlainSize:   newSegment.PlainSize,
					PlainOffset: newSegment.PlainOffset,

					SkipPendingObject: true,
				},
			}.Check(ctx, t, db)

			// Verify the inline segment was updated (not duplicated)
			metabasetest.Verify{
				Segments: []metabase.RawSegment{newSegment},
			}.Check(ctx, t, db)
		})

		t.Run("commit inline segment with SkipPendingObject and ExpiresAt", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			segment := metabasetest.DefaultRawSegment(obj, metabase.SegmentPosition{})
			segment.RootPieceID = storj.PieceID{}
			segment.InlineData = []byte{1, 2, 3}
			segment.EncryptedSize = int32(len(segment.InlineData))
			segment.Redundancy = storj.RedundancyScheme{}
			segment.Pieces = nil

			expectedExpiresAt := time.Now().Add(33 * time.Hour)
			segment.ExpiresAt = &expectedExpiresAt

			metabasetest.CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,
					ExpiresAt:    &expectedExpiresAt,

					InlineData: segment.InlineData,

					EncryptedKey:      segment.EncryptedKey,
					EncryptedKeyNonce: segment.EncryptedKeyNonce,
					EncryptedETag:     segment.EncryptedETag,

					PlainSize:   segment.PlainSize,
					PlainOffset: segment.PlainOffset,

					SkipPendingObject: true,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Segments: []metabase.RawSegment{segment},
			}.Check(ctx, t, db)
		})
	})
}
