// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestDeleteZombieObjects(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj1 := metabasetest.RandObjectStream()
		obj2 := metabasetest.RandObjectStream()
		obj3 := metabasetest.RandObjectStream()

		t.Run("none", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.DeleteZombieObjects{
				Opts: metabase.DeleteZombieObjects{
					DeadlineBefore: time.Now(),
				},
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("partial objects", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()
			pastTime := now.Add(-1 * time.Hour)
			futureTime := now.Add(1 * time.Hour)

			// zombie object with default deadline
			pending1 := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj1,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			// zombie object with deadline time in the past
			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream:           obj2,
					ZombieDeletionDeadline: &pastTime,
					Encryption:             metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			// pending object with expiration time in the future
			pending3 := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream:           obj3,
					ZombieDeletionDeadline: &futureTime,
					Encryption:             metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.DeleteZombieObjects{
				Opts: metabase.DeleteZombieObjects{
					DeadlineBefore:   now,
					InactiveDeadline: now,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{ // the object with zombie deadline time in the past is gone
				Objects: []metabase.RawObject{
					metabase.RawObject(pending1),
					metabase.RawObject(pending3),
				},
			}.Check(ctx, t, db)
		})

		t.Run("partial object with segment", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream:           obj1,
					Encryption:             metabasetest.DefaultEncryption,
					ZombieDeletionDeadline: &now,
				},
			}.Check(ctx, t, db)
			metabasetest.BeginSegment{
				Opts: metabase.BeginSegment{
					ObjectStream: obj1,
					RootPieceID:  storj.PieceID{1},
					Pieces: []metabase.Piece{{
						Number:      1,
						StorageNode: testrand.NodeID(),
					}},
				},
			}.Check(ctx, t, db)
			metabasetest.CommitSegment{
				Opts: metabase.CommitSegment{
					ObjectStream: obj1,
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

			// object will be checked if is inactive but inactive time is in future
			metabasetest.DeleteZombieObjects{
				Opts: metabase.DeleteZombieObjects{
					DeadlineBefore:   now.Add(1 * time.Hour),
					InactiveDeadline: now.Add(-1 * time.Hour),
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj1,
						CreatedAt:    now,
						Status:       metabase.Pending,

						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &now,
					},
				},
				Segments: []metabase.RawSegment{
					{
						StreamID:    obj1.StreamID,
						RootPieceID: storj.PieceID{1},
						Pieces:      metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
						CreatedAt:   now,

						EncryptedKey:      []byte{3},
						EncryptedKeyNonce: []byte{4},
						EncryptedETag:     []byte{5},

						EncryptedSize: 1024,
						PlainSize:     512,
						PlainOffset:   0,
						Redundancy:    metabasetest.DefaultRedundancy,
					},
				},
			}.Check(ctx, t, db)

			// object will be checked if is inactive and will be deleted with segment
			metabasetest.DeleteZombieObjects{
				Opts: metabase.DeleteZombieObjects{
					DeadlineBefore:     now.Add(1 * time.Hour),
					InactiveDeadline:   now.Add(2 * time.Hour),
					AsOfSystemInterval: -1 * time.Microsecond,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("batch size", func(t *testing.T) {
			for i := 0; i < 33; i++ {
				obj := metabasetest.RandObjectStream()

				metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: obj,
						Encryption:   metabasetest.DefaultEncryption,
						// use default 24h zombie deletion deadline
					},
				}.Check(ctx, t, db)

				for i := byte(0); i < 3; i++ {
					metabasetest.BeginSegment{
						Opts: metabase.BeginSegment{
							ObjectStream: obj,
							Position:     metabase.SegmentPosition{Part: 0, Index: uint32(i)},
							RootPieceID:  storj.PieceID{i + 1},
							Pieces: []metabase.Piece{{
								Number:      1,
								StorageNode: testrand.NodeID(),
							}},
						},
					}.Check(ctx, t, db)

					metabasetest.CommitSegment{
						Opts: metabase.CommitSegment{
							ObjectStream: obj,
							Position:     metabase.SegmentPosition{Part: 0, Index: uint32(i)},
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
			}

			metabasetest.DeleteZombieObjects{
				Opts: metabase.DeleteZombieObjects{
					DeadlineBefore:   time.Now().Add(25 * time.Hour),
					InactiveDeadline: time.Now().Add(48 * time.Hour),
					BatchSize:        4,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("committed objects", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()
			pastTime := now.Add(-1 * time.Hour)
			futureTime := now.Add(1 * time.Hour)

			object1, _ := metabasetest.CreateTestObject{}.Run(ctx, t, db, obj1, 1)

			object2 := object1
			object2.ObjectStream = obj2
			metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream:           object2.ObjectStream,
					ZombieDeletionDeadline: &pastTime,
					Encryption:             metabasetest.DefaultEncryption,
				},
			}.Run(ctx, t, db, object2.ObjectStream, 1)

			object3, _ := metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream:           obj3,
					ZombieDeletionDeadline: &futureTime,
					Encryption:             metabasetest.DefaultEncryption,
				},
			}.Run(ctx, t, db, obj3, 1)

			obj3.Version = object3.Version + 1
			object4 := metabasetest.CreateObjectVersioned(ctx, t, db, obj3, 0)

			deletionResult := metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: obj3.Location(),
					Versioned:      true,
				},
				Result: metabase.DeleteObjectResult{
					Markers: []metabase.Object{
						{
							ObjectStream: metabase.ObjectStream{
								ProjectID:  obj3.ProjectID,
								BucketName: obj3.BucketName,
								ObjectKey:  obj3.ObjectKey,
								Version:    object4.Version + 1,
							},
							Status:    metabase.DeleteMarkerVersioned,
							CreatedAt: time.Now(),
						},
					},
				},
			}.Check(ctx, t, db)

			expectedObj1Segment := metabase.Segment{
				StreamID:          obj1.StreamID,
				RootPieceID:       storj.PieceID{1},
				CreatedAt:         now,
				EncryptedKey:      []byte{3},
				EncryptedKeyNonce: []byte{4},
				EncryptedETag:     []byte{5},
				EncryptedSize:     1060,
				PlainSize:         512,
				Pieces:            metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
				Redundancy:        metabasetest.DefaultRedundancy,
			}

			expectedObj2Segment := expectedObj1Segment
			expectedObj2Segment.StreamID = object2.StreamID
			expectedObj3Segment := expectedObj1Segment
			expectedObj3Segment.StreamID = object3.StreamID

			metabasetest.DeleteZombieObjects{
				Opts: metabase.DeleteZombieObjects{
					DeadlineBefore:   now,
					InactiveDeadline: now.Add(1 * time.Hour),
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{ // all committed objects should NOT be deleted
				Objects: []metabase.RawObject{
					metabase.RawObject(object1),
					metabase.RawObject(object2),
					metabase.RawObject(object3),
					metabase.RawObject(object4),
					metabase.RawObject(deletionResult.Markers[0]),
				},
				Segments: []metabase.RawSegment{
					metabase.RawSegment(expectedObj1Segment),
					metabase.RawSegment(expectedObj2Segment),
					metabase.RawSegment(expectedObj3Segment),
				},
			}.Check(ctx, t, db)
		})

		// pending objects migrated to metabase doesn't have zombie_deletion_deadline
		// column set correctly but we need to delete them too
		t.Run("migrated objects", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			require.NoError(t, db.TestingBatchInsertObjects(ctx, []metabase.RawObject{
				{
					ObjectStream:           obj1,
					Status:                 metabase.Pending,
					ZombieDeletionDeadline: nil,
				},
			}))

			objects, err := db.TestingAllObjects(ctx)
			require.NoError(t, err)
			require.Nil(t, objects[0].ZombieDeletionDeadline)

			metabasetest.DeleteZombieObjects{
				Opts: metabase.DeleteZombieObjects{
					DeadlineBefore:   time.Now(),
					InactiveDeadline: time.Now().Add(1 * time.Hour),
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})
	})
}
