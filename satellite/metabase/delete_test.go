// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"
	"time"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestDeletePendingObject(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()
		now := time.Now()
		zombieDeadline := now.Add(24 * time.Hour)

		for _, test := range metabasetest.InvalidObjectStreams(obj) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)
				metabasetest.DeletePendingObject{
					Opts: metabase.DeletePendingObject{
						ObjectStream: test.ObjectStream,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)
				metabasetest.Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("object missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.DeletePendingObject{
				Opts: metabase.DeletePendingObject{
					ObjectStream: obj,
				},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: no rows deleted",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("non existing object version", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			metabasetest.DeletePendingObject{
				Opts: metabase.DeletePendingObject{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  obj.ProjectID,
						BucketName: obj.BucketName,
						ObjectKey:  obj.ObjectKey,
						Version:    33,
						StreamID:   obj.StreamID,
					},
				},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: no rows deleted",
			}.Check(ctx, t, db)
			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Pending,

						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieDeadline,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("delete committed object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.CreateObject(ctx, t, db, obj, 0)

			metabasetest.DeletePendingObject{
				Opts: metabase.DeletePendingObject{
					ObjectStream: object.ObjectStream,
				},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: no rows deleted",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Committed,

						Encryption: metabasetest.DefaultEncryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("without segments with wrong StreamID", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			metabasetest.DeletePendingObject{
				Opts: metabase.DeletePendingObject{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  obj.ProjectID,
						BucketName: obj.BucketName,
						ObjectKey:  obj.ObjectKey,
						Version:    obj.Version,
						StreamID:   uuid.UUID{33},
					},
				},
				Result:   metabase.DeleteObjectResult{},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: no rows deleted",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Pending,

						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieDeadline,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("without segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			object := metabase.RawObject{
				ObjectStream: obj,
				CreatedAt:    now,
				Status:       metabase.Pending,
				Encryption:   metabasetest.DefaultEncryption,
			}
			metabasetest.DeletePendingObject{
				Opts: metabase.DeletePendingObject{
					ObjectStream: obj,
				},
				Result: metabase.DeleteObjectResult{
					Objects: []metabase.Object{metabase.Object(object)},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("with segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.CreatePendingObject(ctx, t, db, obj, 2)

			expectedSegmentInfo := metabase.DeletedSegmentInfo{
				RootPieceID: storj.PieceID{1},
				Pieces:      metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
			}

			metabasetest.DeletePendingObject{
				Opts: metabase.DeletePendingObject{
					ObjectStream: obj,
				},
				Result: metabase.DeleteObjectResult{
					Objects: []metabase.Object{
						{
							ObjectStream: obj,
							CreatedAt:    now,
							Status:       metabase.Pending,
							Encryption:   metabasetest.DefaultEncryption,
						},
					},
					Segments: []metabase.DeletedSegmentInfo{expectedSegmentInfo, expectedSegmentInfo},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("with inline segment", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
				Version: obj.Version,
			}.Check(ctx, t, db)

			metabasetest.CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Part: 0, Index: 0},

					EncryptedKey:      testrand.Bytes(32),
					EncryptedKeyNonce: testrand.Bytes(32),

					InlineData: testrand.Bytes(1024),

					PlainSize:   512,
					PlainOffset: 0,
				},
			}.Check(ctx, t, db)

			metabasetest.DeletePendingObject{
				Opts: metabase.DeletePendingObject{
					ObjectStream: obj,
				},
				Result: metabase.DeleteObjectResult{
					Objects: []metabase.Object{
						{
							ObjectStream: obj,
							CreatedAt:    now,
							Status:       metabase.Pending,
							Encryption:   metabasetest.DefaultEncryption,
						},
					},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})
	})
}

func TestDeleteObjectExactVersion(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		location := obj.Location()

		now := time.Now()
		zombieDeadline := now.Add(24 * time.Hour)

		for _, test := range metabasetest.InvalidObjectLocations(location) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)
				metabasetest.DeleteObjectExactVersion{
					Opts: metabase.DeleteObjectExactVersion{
						ObjectLocation: test.ObjectLocation,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)
				metabasetest.Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("Version invalid", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.DeleteObjectExactVersion{
				Opts: metabase.DeleteObjectExactVersion{
					ObjectLocation: location,
					Version:        0,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Version invalid: 0",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Object missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.DeleteObjectExactVersion{
				Opts: metabase.DeleteObjectExactVersion{
					ObjectLocation: location,
					Version:        1,
				},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: no rows deleted",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete non existing object version", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.DeleteObjectExactVersion{
				Opts: metabase.DeleteObjectExactVersion{
					ObjectLocation: location,
					Version:        33,
				},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: no rows deleted",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete partial object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			metabasetest.DeleteObjectExactVersion{
				Opts: metabase.DeleteObjectExactVersion{
					ObjectLocation: location,
					Version:        1,
				},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: no rows deleted",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Pending,

						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieDeadline,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("Delete object without segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			encryptedMetadata := testrand.Bytes(1024)
			encryptedMetadataNonce := testrand.Nonce()
			encryptedMetadataKey := testrand.Bytes(265)

			object := metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:                  obj,
					EncryptedMetadataNonce:        encryptedMetadataNonce[:],
					EncryptedMetadata:             encryptedMetadata,
					EncryptedMetadataEncryptedKey: encryptedMetadataKey,
				},
			}.Run(ctx, t, db, obj, 0)

			metabasetest.DeleteObjectExactVersion{
				Opts: metabase.DeleteObjectExactVersion{
					ObjectLocation: location,
					Version:        1,
				},
				Result: metabase.DeleteObjectResult{
					Objects: []metabase.Object{object},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete object with segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.CreateObject(ctx, t, db, obj, 2)

			expectedSegmentInfo := metabase.DeletedSegmentInfo{
				RootPieceID: storj.PieceID{1},
				Pieces:      metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
			}

			metabasetest.DeleteObjectExactVersion{
				Opts: metabase.DeleteObjectExactVersion{
					ObjectLocation: location,
					Version:        1,
				},
				Result: metabase.DeleteObjectResult{
					Objects:  []metabase.Object{object},
					Segments: []metabase.DeletedSegmentInfo{expectedSegmentInfo, expectedSegmentInfo},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete object with inline segment", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
				Version: obj.Version,
			}.Check(ctx, t, db)

			metabasetest.CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Part: 0, Index: 0},

					EncryptedKey:      testrand.Bytes(32),
					EncryptedKeyNonce: testrand.Bytes(32),

					InlineData: testrand.Bytes(1024),

					PlainSize:   512,
					PlainOffset: 0,
				},
			}.Check(ctx, t, db)

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
				},
			}.Check(ctx, t, db)

			metabasetest.DeleteObjectExactVersion{
				Opts: metabase.DeleteObjectExactVersion{
					ObjectLocation: location,
					Version:        1,
				},
				Result: metabase.DeleteObjectResult{
					Objects: []metabase.Object{object},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})
	})
}

func TestDeleteObjectAnyStatusAllVersions(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		location := obj.Location()

		now := time.Now()

		for _, test := range metabasetest.InvalidObjectLocations(location) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)
				metabasetest.DeleteObjectAnyStatusAllVersions{
					Opts:     metabase.DeleteObjectAnyStatusAllVersions{ObjectLocation: test.ObjectLocation},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)
				metabasetest.Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("Object missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.DeleteObjectAnyStatusAllVersions{
				Opts:     metabase.DeleteObjectAnyStatusAllVersions{ObjectLocation: obj.Location()},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: no rows deleted",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete non existing object version", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.DeleteObjectAnyStatusAllVersions{
				Opts:     metabase.DeleteObjectAnyStatusAllVersions{ObjectLocation: obj.Location()},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: no rows deleted",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete partial object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			metabasetest.DeleteObjectAnyStatusAllVersions{
				Opts: metabase.DeleteObjectAnyStatusAllVersions{ObjectLocation: obj.Location()},
				Result: metabase.DeleteObjectResult{
					Objects: []metabase.Object{{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Pending,

						Encryption: metabasetest.DefaultEncryption,
					}},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete object without segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			encryptedMetadata := testrand.Bytes(1024)
			encryptedMetadataNonce := testrand.Nonce()
			encryptedMetadataKey := testrand.Bytes(265)

			object := metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:                  obj,
					EncryptedMetadataNonce:        encryptedMetadataNonce[:],
					EncryptedMetadata:             encryptedMetadata,
					EncryptedMetadataEncryptedKey: encryptedMetadataKey,
				},
			}.Run(ctx, t, db, obj, 0)

			metabasetest.DeleteObjectAnyStatusAllVersions{
				Opts: metabase.DeleteObjectAnyStatusAllVersions{ObjectLocation: obj.Location()},
				Result: metabase.DeleteObjectResult{
					Objects: []metabase.Object{object},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete object with segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.CreateObject(ctx, t, db, obj, 2)

			expectedSegmentInfo := metabase.DeletedSegmentInfo{
				RootPieceID: storj.PieceID{1},
				Pieces:      metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
			}

			metabasetest.DeleteObjectAnyStatusAllVersions{
				Opts: metabase.DeleteObjectAnyStatusAllVersions{
					ObjectLocation: location,
				},
				Result: metabase.DeleteObjectResult{
					Objects:  []metabase.Object{object},
					Segments: []metabase.DeletedSegmentInfo{expectedSegmentInfo, expectedSegmentInfo},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete object with inline segment", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
				Version: obj.Version,
			}.Check(ctx, t, db)

			metabasetest.CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Part: 0, Index: 0},

					EncryptedKey:      testrand.Bytes(32),
					EncryptedKeyNonce: testrand.Bytes(32),

					InlineData: testrand.Bytes(1024),

					PlainSize:   512,
					PlainOffset: 0,
				},
			}.Check(ctx, t, db)

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
				},
			}.Check(ctx, t, db)

			metabasetest.DeleteObjectAnyStatusAllVersions{
				Opts: metabase.DeleteObjectAnyStatusAllVersions{ObjectLocation: obj.Location()},
				Result: metabase.DeleteObjectResult{
					Objects: []metabase.Object{object},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete multiple versions of the same object at once", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			expected := metabase.DeleteObjectResult{}

			obj := metabasetest.RandObjectStream()
			for i := 1; i <= 10; i++ {
				obj.StreamID = testrand.UUID()
				obj.Version = metabase.Version(i)
				expected.Objects = append(expected.Objects, metabasetest.CreateObject(ctx, t, db, obj, 1))
				expected.Segments = append(expected.Segments, metabase.DeletedSegmentInfo{
					RootPieceID: storj.PieceID{1},
					Pieces:      metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
				})
			}

			metabasetest.DeleteObjectAnyStatusAllVersions{
				Opts:   metabase.DeleteObjectAnyStatusAllVersions{ObjectLocation: obj.Location()},
				Result: expected,
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})
	})
}

func TestDeleteObjectsAllVersions(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		location := obj.Location()

		now := time.Now()
		zombieDeadline := now.Add(24 * time.Hour)

		for _, test := range metabasetest.InvalidObjectLocations(location) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)
				metabasetest.DeleteObjectsAllVersions{
					Opts: metabase.DeleteObjectsAllVersions{
						Locations: []metabase.ObjectLocation{test.ObjectLocation},
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)
				metabasetest.Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("Delete two objects from different projects", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj2 := metabasetest.RandObjectStream()

			metabasetest.DeleteObjectsAllVersions{
				Opts: metabase.DeleteObjectsAllVersions{
					Locations: []metabase.ObjectLocation{location, obj2.Location()},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "all objects must be in the same bucket",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete two objects from same project, but different buckets", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj2 := metabasetest.RandObjectStream()
			obj2.ProjectID = obj.ProjectID

			metabasetest.DeleteObjectsAllVersions{
				Opts: metabase.DeleteObjectsAllVersions{
					Locations: []metabase.ObjectLocation{location, obj2.Location()},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "all objects must be in the same bucket",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete empty list of objects", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.DeleteObjectsAllVersions{}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Object missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.DeleteObjectsAllVersions{
				Opts: metabase.DeleteObjectsAllVersions{
					Locations: []metabase.ObjectLocation{location},
				},
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete partial object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			metabasetest.DeleteObjectsAllVersions{
				Opts: metabase.DeleteObjectsAllVersions{
					Locations: []metabase.ObjectLocation{location},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Pending,

						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieDeadline,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("Delete object without segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.CreateObject(ctx, t, db, obj, 0)

			metabasetest.DeleteObjectsAllVersions{
				Opts: metabase.DeleteObjectsAllVersions{
					Locations: []metabase.ObjectLocation{location},
				},
				Result: metabase.DeleteObjectResult{
					Objects: []metabase.Object{object},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete object with segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.CreateObject(ctx, t, db, obj, 2)

			expectedSegmentInfo := metabase.DeletedSegmentInfo{
				RootPieceID: storj.PieceID{1},
				Pieces:      metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
			}

			metabasetest.DeleteObjectsAllVersions{
				Opts: metabase.DeleteObjectsAllVersions{
					Locations: []metabase.ObjectLocation{location},
				},
				Result: metabase.DeleteObjectResult{
					Objects:  []metabase.Object{object},
					Segments: []metabase.DeletedSegmentInfo{expectedSegmentInfo, expectedSegmentInfo},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete two objects with segments from same bucket", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj2 := metabasetest.RandObjectStream()
			obj2.ProjectID = obj.ProjectID
			obj2.BucketName = obj.BucketName

			object1 := metabasetest.CreateObject(ctx, t, db, obj, 1)
			object2 := metabasetest.CreateObject(ctx, t, db, obj2, 2)

			expectedSegmentInfo := metabase.DeletedSegmentInfo{
				RootPieceID: storj.PieceID{1},
				Pieces:      metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
			}

			metabasetest.DeleteObjectsAllVersions{
				Opts: metabase.DeleteObjectsAllVersions{
					Locations: []metabase.ObjectLocation{location, obj2.Location()},
				},
				Result: metabase.DeleteObjectResult{
					Objects:  []metabase.Object{object1, object2},
					Segments: []metabase.DeletedSegmentInfo{expectedSegmentInfo, expectedSegmentInfo, expectedSegmentInfo},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete object with inline segment", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
				Version: obj.Version,
			}.Check(ctx, t, db)

			metabasetest.CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Part: 0, Index: 0},

					EncryptedKey:      testrand.Bytes(32),
					EncryptedKeyNonce: testrand.Bytes(32),

					InlineData: testrand.Bytes(1024),

					PlainSize:   512,
					PlainOffset: 0,
				},
			}.Check(ctx, t, db)

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
				},
			}.Check(ctx, t, db)

			metabasetest.DeleteObjectsAllVersions{
				Opts: metabase.DeleteObjectsAllVersions{
					Locations: []metabase.ObjectLocation{location},
				},
				Result: metabase.DeleteObjectResult{
					Objects: []metabase.Object{object},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete object with inline segment and object with remote segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
				Version: obj.Version,
			}.Check(ctx, t, db)

			metabasetest.CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: obj,
					Position:     metabase.SegmentPosition{Part: 0, Index: 0},

					EncryptedKey:      testrand.Bytes(32),
					EncryptedKeyNonce: testrand.Bytes(32),

					InlineData: testrand.Bytes(1024),

					PlainSize:   512,
					PlainOffset: 0,
				},
			}.Check(ctx, t, db)

			object1 := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
				},
			}.Check(ctx, t, db)

			obj2 := metabasetest.RandObjectStream()
			obj2.ProjectID = obj.ProjectID
			obj2.BucketName = obj.BucketName

			object2 := metabasetest.CreateObject(ctx, t, db, obj2, 2)

			expectedSegmentInfo := metabase.DeletedSegmentInfo{
				RootPieceID: storj.PieceID{1},
				Pieces:      metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
			}

			metabasetest.DeleteObjectsAllVersions{
				Opts: metabase.DeleteObjectsAllVersions{
					Locations: []metabase.ObjectLocation{location, object2.Location()},
				},
				Result: metabase.DeleteObjectResult{
					Objects:  []metabase.Object{object1, object2},
					Segments: []metabase.DeletedSegmentInfo{expectedSegmentInfo, expectedSegmentInfo},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete multiple versions of the same object at once", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			expected := metabase.DeleteObjectResult{}

			for i := 1; i <= 10; i++ {
				obj.StreamID = testrand.UUID()
				obj.Version = metabase.Version(i)
				expected.Objects = append(expected.Objects, metabasetest.CreateObject(ctx, t, db, obj, 1))
				expected.Segments = append(expected.Segments, metabase.DeletedSegmentInfo{
					RootPieceID: storj.PieceID{1},
					Pieces:      metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
				})
			}

			metabasetest.DeleteObjectsAllVersions{
				Opts: metabase.DeleteObjectsAllVersions{
					Locations: []metabase.ObjectLocation{location},
				},
				Result: expected,
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})
	})
}
