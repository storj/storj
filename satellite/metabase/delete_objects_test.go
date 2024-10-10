// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestDeleteExpiredObjects(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj1 := metabasetest.RandObjectStream()
		obj2 := metabasetest.RandObjectStream()
		obj3 := metabasetest.RandObjectStream()

		t.Run("none", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.DeleteExpiredObjects{
				Opts: metabase.DeleteExpiredObjects{
					ExpiredBefore: time.Now(),
				},
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("partial objects", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()
			zombieDeadline := now.Add(24 * time.Hour)
			pastTime := now.Add(-1 * time.Hour)
			futureTime := now.Add(1 * time.Hour)

			// pending object without expiration time
			pending1 := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj1,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			// pending object with expiration time in the past
			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj2,
					ExpiresAt:    &pastTime,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			// pending object with expiration time in the future
			pending3 := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj3,
					ExpiresAt:    &futureTime,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.DeleteExpiredObjects{
				Opts: metabase.DeleteExpiredObjects{
					ExpiredBefore: time.Now(),
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{ // the object with expiration time in the past is gone
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj1,
						CreatedAt:    pending1.CreatedAt,
						Status:       metabase.Pending,

						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieDeadline,
					},
					{
						ObjectStream: obj3,
						CreatedAt:    pending3.CreatedAt,
						ExpiresAt:    &futureTime,
						Status:       metabase.Pending,

						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieDeadline,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("batch size", func(t *testing.T) {
			expiresAt := time.Now().Add(-30 * 24 * time.Hour)
			for i := 0; i < 32; i++ {
				_ = metabasetest.CreateExpiredObject(ctx, t, db, metabasetest.RandObjectStream(), 3, expiresAt)
			}
			metabasetest.DeleteExpiredObjects{
				Opts: metabase.DeleteExpiredObjects{
					ExpiredBefore: time.Now().Add(time.Hour),
					BatchSize:     4,
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
			metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: obj2,
					ExpiresAt:    &pastTime,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Run(ctx, t, db, obj2, 1)
			object3, _ := metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: obj3,
					ExpiresAt:    &futureTime,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Run(ctx, t, db, obj3, 1)

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

			expectedObj3Segment := expectedObj1Segment
			expectedObj3Segment.StreamID = obj3.StreamID
			expectedObj3Segment.ExpiresAt = &futureTime

			metabasetest.DeleteExpiredObjects{
				Opts: metabase.DeleteExpiredObjects{
					ExpiredBefore: time.Now(),
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{ // the object with expiration time in the past is gone
				Objects: []metabase.RawObject{
					metabase.RawObject(object1),
					metabase.RawObject(object3),
				},
				Segments: []metabase.RawSegment{
					metabase.RawSegment(expectedObj1Segment),
					metabase.RawSegment(expectedObj3Segment),
				},
			}.Check(ctx, t, db)
		})

		t.Run("concurrent deletes", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()
			pastTime := now.Add(-1 * time.Hour)

			for _, batchSize := range []int{0, 1, 2, 3, 8, 100} {
				for i := 0; i < 13; i++ {
					_ = metabasetest.CreateExpiredObject(ctx, t, db, metabasetest.RandObjectStream(), 3, pastTime)
				}

				metabasetest.DeleteExpiredObjects{
					Opts: metabase.DeleteExpiredObjects{
						ExpiredBefore:     time.Now(),
						DeleteConcurrency: batchSize,
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{}.Check(ctx, t, db)
			}
		})
	})
}

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
			zombieDeadline := now.Add(24 * time.Hour)
			pastTime := now.Add(-1 * time.Hour)
			futureTime := now.Add(1 * time.Hour)

			// zombie object with default deadline
			metabasetest.BeginObjectExactVersion{
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
			metabasetest.BeginObjectExactVersion{
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
					{
						ObjectStream: obj1,
						CreatedAt:    now,
						Status:       metabase.Pending,

						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieDeadline,
					},
					{
						ObjectStream: obj3,
						CreatedAt:    now,
						Status:       metabase.Pending,

						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &futureTime,
					},
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

func TestDeleteObjects(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		projectID := testrand.UUID()
		bucketName := metabase.BucketName(testrand.BucketName())

		createObject := func(t *testing.T, objStream metabase.ObjectStream) (metabase.Object, []metabase.Segment) {
			return metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream: objStream,
				},
			}.Run(ctx, t, db, objStream, 2)
		}

		randVersion := func() metabase.Version {
			return metabase.Version(1 + (testrand.Int63n(math.MaxInt64) - 1))
		}

		randObjectStream := func() metabase.ObjectStream {
			return metabase.ObjectStream{
				ProjectID:  projectID,
				BucketName: bucketName,
				ObjectKey:  metabase.ObjectKey(testrand.Path()),
				Version:    randVersion(),
				StreamID:   testrand.UUID(),
			}
		}

		t.Run("Basic", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj1, _ := createObject(t, randObjectStream())
			obj2, _ := createObject(t, randObjectStream())

			// These objects are added to ensure that we don't accidentally
			// delete objects residing in different projects or buckets.
			differentBucketObj, differentBucketSegs := createObject(t, metabase.ObjectStream{
				ProjectID:  obj1.ProjectID,
				BucketName: metabase.BucketName(testrand.BucketName()),
				ObjectKey:  obj1.ObjectKey,
				Version:    obj1.Version,
				StreamID:   testrand.UUID(),
			})

			differentProjectObj, differentProjectSegs := createObject(t, metabase.ObjectStream{
				ProjectID:  testrand.UUID(),
				BucketName: obj1.BucketName,
				ObjectKey:  obj1.ObjectKey,
				Version:    obj1.Version,
				StreamID:   testrand.UUID(),
			})

			metabasetest.DeleteObjects{
				Opts: metabase.DeleteObjects{
					ProjectID:  projectID,
					BucketName: bucketName,
					Items: []metabase.DeleteObjectsItem{
						{
							ObjectKey: obj1.ObjectKey,
							Version:   obj1.Version,
						}, {
							ObjectKey: obj2.ObjectKey,
							Version:   metabase.DeleteObjectsLastCommittedVersion,
						},
					},
				},
				Result: metabase.DeleteObjectsResult{
					Items: []metabase.DeleteObjectsResultItem{
						{
							ObjectKey: obj1.ObjectKey,
							Version:   obj1.Version,
							Status:    metabase.DeleteStatusOK,
						}, {
							ObjectKey: obj2.ObjectKey,
							Version:   metabase.DeleteObjectsLastCommittedVersion,
							Status:    metabase.DeleteStatusOK,
						},
					},
					DeletedSegmentCount: int64(obj1.SegmentCount + obj2.SegmentCount),
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(differentBucketObj),
					metabase.RawObject(differentProjectObj),
				},
				Segments: metabasetest.SegmentsToRaw(concat(differentBucketSegs, differentProjectSegs)),
			}.Check(ctx, t, db)
		})

		t.Run("Not found", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			key1, key2 := metabase.ObjectKey(testrand.Path()), metabase.ObjectKey(testrand.Path())
			version1 := randVersion()
			metabasetest.DeleteObjects{
				Opts: metabase.DeleteObjects{
					ProjectID:  projectID,
					BucketName: bucketName,
					Items: []metabase.DeleteObjectsItem{
						{
							ObjectKey: key1,
							Version:   version1,
						}, {
							ObjectKey: key2,
							Version:   metabase.DeleteObjectsLastCommittedVersion,
						},
					},
				},
				Result: metabase.DeleteObjectsResult{
					Items: []metabase.DeleteObjectsResultItem{
						{
							ObjectKey: key1,
							Version:   version1,
							Status:    metabase.DeleteStatusNotFound,
						}, {
							ObjectKey: key2,
							Version:   metabase.DeleteObjectsLastCommittedVersion,
							Status:    metabase.DeleteStatusNotFound,
						},
					},
					DeletedSegmentCount: 0,
				},
			}.Check(ctx, t, db)
		})

		t.Run("Pending object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: randObjectStream(),
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			segments := metabasetest.CreateSegments(ctx, t, db, obj.ObjectStream, nil, 2)

			metabasetest.DeleteObjects{
				Opts: metabase.DeleteObjects{
					ProjectID:  projectID,
					BucketName: bucketName,
					Items: []metabase.DeleteObjectsItem{{
						ObjectKey: obj.ObjectKey,
						Version:   metabase.DeleteObjectsLastCommittedVersion,
					}},
				},
				Result: metabase.DeleteObjectsResult{
					Items: []metabase.DeleteObjectsResultItem{{
						ObjectKey: obj.ObjectKey,
						Version:   metabase.DeleteObjectsLastCommittedVersion,
						Status:    metabase.DeleteStatusNotFound,
					}},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects:  []metabase.RawObject{metabase.RawObject(obj)},
				Segments: metabasetest.SegmentsToRaw(segments),
			}.Check(ctx, t, db)

			metabasetest.DeleteObjects{
				Opts: metabase.DeleteObjects{
					ProjectID:  projectID,
					BucketName: bucketName,
					Items: []metabase.DeleteObjectsItem{{
						ObjectKey: obj.ObjectKey,
						Version:   obj.Version,
					}},
				},
				Result: metabase.DeleteObjectsResult{
					Items: []metabase.DeleteObjectsResultItem{{
						ObjectKey: obj.ObjectKey,
						Version:   obj.Version,
						Status:    metabase.DeleteStatusOK,
					}},
					DeletedSegmentCount: int64(len(segments)),
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Duplicate deletion", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj, _ := createObject(t, randObjectStream())
			reqItem := metabase.DeleteObjectsItem{
				ObjectKey: obj.ObjectKey,
				Version:   obj.Version,
			}

			metabasetest.DeleteObjects{
				Opts: metabase.DeleteObjects{
					ProjectID:  projectID,
					BucketName: bucketName,
					Items:      []metabase.DeleteObjectsItem{reqItem, reqItem},
				},
				Result: metabase.DeleteObjectsResult{
					Items: []metabase.DeleteObjectsResultItem{{
						ObjectKey: obj.ObjectKey,
						Version:   obj.Version,
						Status:    metabase.DeleteStatusOK,
					}},
					DeletedSegmentCount: int64(obj.SegmentCount),
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		// This tests the case where an object's last committed version is specified
		// in the deletion request both indirectly and explicitly.
		t.Run("Duplicate deletion (indirect)", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj, _ := createObject(t, randObjectStream())

			metabasetest.DeleteObjects{
				Opts: metabase.DeleteObjects{
					ProjectID:  projectID,
					BucketName: bucketName,
					Items: []metabase.DeleteObjectsItem{
						{
							ObjectKey: obj.ObjectKey,
							Version:   obj.Version,
						}, {
							ObjectKey: obj.ObjectKey,
							Version:   metabase.DeleteObjectsLastCommittedVersion,
						},
					},
				},
				Result: metabase.DeleteObjectsResult{
					Items: []metabase.DeleteObjectsResultItem{
						{
							ObjectKey: obj.ObjectKey,
							Version:   obj.Version,
							Status:    metabase.DeleteStatusOK,
						}, {
							ObjectKey: obj.ObjectKey,
							Version:   metabase.DeleteObjectsLastCommittedVersion,
							Status:    metabase.DeleteStatusOK,
						},
					},
					DeletedSegmentCount: int64(obj.SegmentCount),
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Invalid options", func(t *testing.T) {
			validItem := metabase.DeleteObjectsItem{
				ObjectKey: metabase.ObjectKey(testrand.Path()),
				Version:   randVersion(),
			}

			for _, tt := range []struct {
				name   string
				opts   metabase.DeleteObjects
				errMsg string
			}{
				{
					name: "Project ID missing",
					opts: metabase.DeleteObjects{
						BucketName: bucketName,
						Items:      []metabase.DeleteObjectsItem{validItem},
					},
					errMsg: "ProjectID missing",
				}, {
					name: "Bucket name missing",
					opts: metabase.DeleteObjects{
						ProjectID: projectID,
						Items:     []metabase.DeleteObjectsItem{validItem},
					},
					errMsg: "BucketName missing",
				}, {
					name: "Items missing",
					opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
					},
					errMsg: "Items missing",
				}, {
					name: "Too many items",
					opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Items:      make([]metabase.DeleteObjectsItem, 1001),
					},
					errMsg: "Items is too long; expected <= 1000, but got 1001",
				}, {
					name: "Missing object key",
					opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Items: []metabase.DeleteObjectsItem{{
							Version: validItem.Version,
						}},
					},
					errMsg: "Items[0].ObjectKey missing",
				}, {
					name: "Invalid version",
					opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Items: []metabase.DeleteObjectsItem{{
							ObjectKey: validItem.ObjectKey,
							Version:   -1,
						}},
					},
					errMsg: "Items[0].Version invalid: -1",
				},
			} {
				t.Run(tt.name, func(t *testing.T) {
					defer metabasetest.DeleteAll{}.Check(ctx, t, db)

					metabasetest.DeleteObjects{
						Opts:     tt.opts,
						ErrClass: &metabase.ErrInvalidRequest,
						ErrText:  tt.errMsg,
					}.Check(ctx, t, db)

					metabasetest.Verify{}.Check(ctx, t, db)
				})
			}
		})
	})
}
