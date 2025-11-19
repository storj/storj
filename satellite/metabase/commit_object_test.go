// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"
	"time"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestCommitObjectWithSegments(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		t.Run("invalid order", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			pos00 := metabase.SegmentPosition{Part: 0, Index: 0}
			pos01 := metabase.SegmentPosition{Part: 0, Index: 1}
			pos10 := metabase.SegmentPosition{Part: 1, Index: 0}

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:     obj,
					SpecificSegments: true,
					OnlySegments: []metabase.SegmentPosition{
						pos01,
						pos00,
					},
				},
				ErrClass: &metabase.Error,
				ErrText:  "segments not in ascending order, got {0 1} before {0 0}",
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:     obj,
					SpecificSegments: true,
					OnlySegments: []metabase.SegmentPosition{
						pos10,
						pos00,
					},
				},
				ErrClass: &metabase.Error,
				ErrText:  "segments not in ascending order, got {1 0} before {0 0}",
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:     obj,
					SpecificSegments: true,
					OnlySegments: []metabase.SegmentPosition{
						pos00,
						pos00,
					},
				},
				ErrClass: &metabase.Error,
				ErrText:  "segments not in ascending order, got {0 0} before {0 0}",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("segments missing in database", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			pos00 := metabase.SegmentPosition{Part: 0, Index: 0}
			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:     obj,
					SpecificSegments: true,
					OnlySegments: []metabase.SegmentPosition{
						pos00,
					},
				},
				ErrClass: &metabase.Error,
				ErrText:  "segments and database does not match: {0 0}: segment not committed",
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("delete segments that are not in proofs", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)
			now := time.Now()

			pos00 := metabase.SegmentPosition{Part: 0, Index: 0}
			rootPieceID00 := testrand.PieceID()
			pieces00 := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey00 := testrand.Bytes(32)
			encryptedKeyNonce00 := testrand.Bytes(32)

			pos01 := metabase.SegmentPosition{Part: 0, Index: 1}
			rootPieceID01 := testrand.PieceID()
			pieces01 := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey01 := testrand.Bytes(32)
			encryptedKeyNonce01 := testrand.Bytes(32)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,

					Position:    pos00,
					RootPieceID: rootPieceID00,
					Pieces:      pieces00,

					EncryptedKey:      encryptedKey00,
					EncryptedKeyNonce: encryptedKeyNonce00,

					EncryptedSize: 1024,
					PlainSize:     512,
					PlainOffset:   0,
					Redundancy:    metabasetest.DefaultRedundancy,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,

					Position:    pos01,
					RootPieceID: rootPieceID01,
					Pieces:      pieces01,

					EncryptedKey:      encryptedKey01,
					EncryptedKeyNonce: encryptedKeyNonce01,

					EncryptedSize: 1024,
					PlainSize:     512,
					PlainOffset:   0,
					Redundancy:    metabasetest.DefaultRedundancy,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:     obj,
					SpecificSegments: true,
					OnlySegments: []metabase.SegmentPosition{
						pos01,
					},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.CommittedUnversioned,
						SegmentCount: 1,

						TotalPlainSize:     512,
						TotalEncryptedSize: 1024,
						FixedSegmentSize:   -1,

						Encryption: metabasetest.DefaultEncryption,
					},
				},
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						Position:  pos01,
						CreatedAt: now,

						RootPieceID:       rootPieceID01,
						EncryptedKey:      encryptedKey01,
						EncryptedKeyNonce: encryptedKeyNonce01,

						EncryptedSize: 1024,
						PlainOffset:   0,
						PlainSize:     512,

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces01,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("delete inline segments that are not in proofs", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)
			now := time.Now()

			pos00 := metabase.SegmentPosition{Part: 0, Index: 0}
			data00 := testrand.Bytes(32)
			encryptedKey00 := testrand.Bytes(32)
			encryptedKeyNonce00 := testrand.Bytes(32)

			pos01 := metabase.SegmentPosition{Part: 0, Index: 1}
			data01 := testrand.Bytes(1024)
			encryptedKey01 := testrand.Bytes(32)
			encryptedKeyNonce01 := testrand.Bytes(32)

			metabasetest.CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,
					Position:     pos00,

					EncryptedKey:      encryptedKey00,
					EncryptedKeyNonce: encryptedKeyNonce00,

					PlainSize:   512,
					PlainOffset: 0,

					InlineData: data00,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,
					Position:     pos01,

					EncryptedKey:      encryptedKey01,
					EncryptedKeyNonce: encryptedKeyNonce01,

					PlainSize:   512,
					PlainOffset: 0,

					InlineData: data01,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:     obj,
					SpecificSegments: true,
					OnlySegments: []metabase.SegmentPosition{
						pos01,
					},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.CommittedUnversioned,
						SegmentCount: 1,

						TotalPlainSize:     512,
						TotalEncryptedSize: 1024,
						FixedSegmentSize:   -1,

						Encryption: metabasetest.DefaultEncryption,
					},
				},
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						Position:  pos01,
						CreatedAt: now,

						EncryptedKey:      encryptedKey01,
						EncryptedKeyNonce: encryptedKeyNonce01,

						EncryptedSize: 1024,
						PlainOffset:   0,
						PlainSize:     512,

						InlineData: data01,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("updated plain offset", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)
			now := time.Now()

			pos00 := metabase.SegmentPosition{Part: 0, Index: 0}
			rootPieceID00 := testrand.PieceID()
			pieces00 := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey00 := testrand.Bytes(32)
			encryptedKeyNonce00 := testrand.Bytes(32)

			pos01 := metabase.SegmentPosition{Part: 0, Index: 1}
			rootPieceID01 := testrand.PieceID()
			pieces01 := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey01 := testrand.Bytes(32)
			encryptedKeyNonce01 := testrand.Bytes(32)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,

					Position:    pos00,
					RootPieceID: rootPieceID00,
					Pieces:      pieces00,

					EncryptedKey:      encryptedKey00,
					EncryptedKeyNonce: encryptedKeyNonce00,

					EncryptedSize: 1024,
					PlainSize:     512,
					PlainOffset:   0,
					Redundancy:    metabasetest.DefaultRedundancy,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,

					Position:    pos01,
					RootPieceID: rootPieceID01,
					Pieces:      pieces01,

					EncryptedKey:      encryptedKey01,
					EncryptedKeyNonce: encryptedKeyNonce01,

					EncryptedSize: 1024,
					PlainSize:     512,
					PlainOffset:   0,
					Redundancy:    metabasetest.DefaultRedundancy,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:     obj,
					SpecificSegments: true,
					OnlySegments: []metabase.SegmentPosition{
						pos00,
						pos01,
					},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.CommittedUnversioned,
						SegmentCount: 2,

						TotalPlainSize:     1024,
						TotalEncryptedSize: 2048,
						FixedSegmentSize:   512,

						Encryption: metabasetest.DefaultEncryption,
					},
				},
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						Position:  pos00,
						CreatedAt: now,

						RootPieceID:       rootPieceID00,
						EncryptedKey:      encryptedKey00,
						EncryptedKeyNonce: encryptedKeyNonce00,

						EncryptedSize: 1024,
						PlainOffset:   0,
						PlainSize:     512,

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces00,
					},
					{
						StreamID:  obj.StreamID,
						Position:  pos01,
						CreatedAt: now,

						RootPieceID:       rootPieceID01,
						EncryptedKey:      encryptedKey01,
						EncryptedKeyNonce: encryptedKeyNonce01,

						EncryptedSize: 1024,
						PlainOffset:   512,
						PlainSize:     512,

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces01,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("fixed segment size", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)
			now := time.Now()

			pos00 := metabase.SegmentPosition{Part: 0, Index: 0}
			rootPieceID00 := testrand.PieceID()
			pieces00 := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey00 := testrand.Bytes(32)
			encryptedKeyNonce00 := testrand.Bytes(32)

			pos10 := metabase.SegmentPosition{Part: 1, Index: 0}
			rootPieceID10 := testrand.PieceID()
			pieces10 := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey10 := testrand.Bytes(32)
			encryptedKeyNonce10 := testrand.Bytes(32)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,

					Position:    pos00,
					RootPieceID: rootPieceID00,
					Pieces:      pieces00,

					EncryptedKey:      encryptedKey00,
					EncryptedKeyNonce: encryptedKeyNonce00,

					EncryptedSize: 1024,
					PlainSize:     512,
					PlainOffset:   0,
					Redundancy:    metabasetest.DefaultRedundancy,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,

					Position:    pos10,
					RootPieceID: rootPieceID10,
					Pieces:      pieces10,

					EncryptedKey:      encryptedKey10,
					EncryptedKeyNonce: encryptedKeyNonce10,

					EncryptedSize: 1024,
					PlainSize:     512,
					PlainOffset:   0,
					Redundancy:    metabasetest.DefaultRedundancy,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:     obj,
					SpecificSegments: true,
					OnlySegments: []metabase.SegmentPosition{
						pos00,
						pos10,
					},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.CommittedUnversioned,
						SegmentCount: 2,

						TotalPlainSize:     1024,
						TotalEncryptedSize: 2048,
						FixedSegmentSize:   -1,

						Encryption: metabasetest.DefaultEncryption,
					},
				},
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						Position:  pos00,
						CreatedAt: now,

						RootPieceID:       rootPieceID00,
						EncryptedKey:      encryptedKey00,
						EncryptedKeyNonce: encryptedKeyNonce00,

						EncryptedSize: 1024,
						PlainOffset:   0,
						PlainSize:     512,

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces00,
					},
					{
						StreamID:  obj.StreamID,
						Position:  pos10,
						CreatedAt: now,

						RootPieceID:       rootPieceID10,
						EncryptedKey:      encryptedKey10,
						EncryptedKeyNonce: encryptedKeyNonce10,

						EncryptedSize: 1024,
						PlainOffset:   512,
						PlainSize:     512,

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces10,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("skipped fixed segment size", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)
			now := time.Now()

			pos00 := metabase.SegmentPosition{Part: 0, Index: 0}
			rootPieceID00 := testrand.PieceID()
			pieces00 := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey00 := testrand.Bytes(32)
			encryptedKeyNonce00 := testrand.Bytes(32)

			pos02 := metabase.SegmentPosition{Part: 0, Index: 2}
			rootPieceID02 := testrand.PieceID()
			pieces02 := metabase.Pieces{{Number: 0, StorageNode: testrand.NodeID()}}
			encryptedKey02 := testrand.Bytes(32)
			encryptedKeyNonce02 := testrand.Bytes(32)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,

					Position:    pos00,
					RootPieceID: rootPieceID00,
					Pieces:      pieces00,

					EncryptedKey:      encryptedKey00,
					EncryptedKeyNonce: encryptedKeyNonce00,

					EncryptedSize: 1024,
					PlainSize:     512,
					PlainOffset:   0,
					Redundancy:    metabasetest.DefaultRedundancy,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj,

					Position:    pos02,
					RootPieceID: rootPieceID02,
					Pieces:      pieces02,

					EncryptedKey:      encryptedKey02,
					EncryptedKeyNonce: encryptedKeyNonce02,

					EncryptedSize: 1024,
					PlainSize:     512,
					PlainOffset:   0,
					Redundancy:    metabasetest.DefaultRedundancy,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream:     obj,
					SpecificSegments: true,
					OnlySegments: []metabase.SegmentPosition{
						pos00,
						pos02,
					},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.CommittedUnversioned,
						SegmentCount: 2,

						TotalPlainSize:     1024,
						TotalEncryptedSize: 2048,
						FixedSegmentSize:   -1,

						Encryption: metabasetest.DefaultEncryption,
					},
				},
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						Position:  pos00,
						CreatedAt: now,

						RootPieceID:       rootPieceID00,
						EncryptedKey:      encryptedKey00,
						EncryptedKeyNonce: encryptedKeyNonce00,

						EncryptedSize: 1024,
						PlainOffset:   0,
						PlainSize:     512,

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces00,
					},
					{
						StreamID:  obj.StreamID,
						Position:  pos02,
						CreatedAt: now,

						RootPieceID:       rootPieceID02,
						EncryptedKey:      encryptedKey02,
						EncryptedKeyNonce: encryptedKeyNonce02,

						EncryptedSize: 1024,
						PlainOffset:   512,
						PlainSize:     512,

						Redundancy: metabasetest.DefaultRedundancy,

						Pieces: pieces02,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("no segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)
			now := time.Now()

			metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.CommittedUnversioned,

						Encryption: metabasetest.DefaultEncryption,
					},
				},
			}.Check(ctx, t, db)
		})
	})
}
