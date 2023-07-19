// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

var noServerSideCopyConfig = metabase.Config{
	ApplicationName:  "satellite-test",
	MinPartSize:      5 * memory.MiB,
	MaxNumberOfParts: 1000,
	ServerSideCopy:   false,
}

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
				ErrClass: &metabase.ErrObjectNotFound,
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
				ErrClass: &metabase.ErrObjectNotFound,
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
				ErrClass: &metabase.ErrObjectNotFound,
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
				ErrClass: &metabase.ErrObjectNotFound,
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
				Result: metabase.DeleteObjectResult{
					Objects: []metabase.Object{},
				},
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
				Result: metabase.DeleteObjectResult{
					Objects: []metabase.Object{},
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

			metabasetest.DeleteObjectExactVersion{
				Opts: metabase.DeleteObjectExactVersion{
					ObjectLocation: location,
					Version:        1,
				},
				Result: metabase.DeleteObjectResult{
					Objects: []metabase.Object{{
						ObjectStream: obj,
						CreatedAt:    now,
						Encryption:   metabasetest.DefaultEncryption,
						Status:       metabase.Pending,
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

			object, _ := metabasetest.CreateTestObject{
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

func TestDeleteObjectsAllVersions(t *testing.T) {
	metabasetest.RunWithConfig(t, noServerSideCopyConfig, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
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

		t.Run("Delete two objects with segments from same bucket", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj2 := metabasetest.RandObjectStream()
			obj2.ProjectID = obj.ProjectID
			obj2.BucketName = obj.BucketName

			object1 := metabasetest.CreateObject(ctx, t, db, obj, 1)
			object2 := metabasetest.CreateObject(ctx, t, db, obj2, 2)

			metabasetest.DeleteObjectsAllVersions{
				Opts: metabase.DeleteObjectsAllVersions{
					Locations: []metabase.ObjectLocation{location, obj2.Location()},
				},
				Result: metabase.DeleteObjectResult{
					Objects: []metabase.Object{object1, object2},
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

			metabasetest.DeleteObjectsAllVersions{
				Opts: metabase.DeleteObjectsAllVersions{
					Locations: []metabase.ObjectLocation{location, object2.Location()},
				},
				Result: metabase.DeleteObjectResult{
					Objects: []metabase.Object{object1, object2},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete multiple versions of the same object at once", func(t *testing.T) {
			t.Skip("skip for now as there is no easy way to have different versions of the same committed object")

			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			expected := metabase.DeleteObjectResult{}

			for i := 1; i <= 10; i++ {
				obj.StreamID = testrand.UUID()
				obj.Version = metabase.Version(i)
				expected.Objects = append(expected.Objects, metabasetest.CreateObject(ctx, t, db, obj, 1))
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

func TestDeleteCopyWithDuplicateMetadata(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		for _, numberOfSegments := range []int{0, 1, 3} {
			t.Run(fmt.Sprintf("%d segments", numberOfSegments), func(t *testing.T) {
				t.Run("delete copy", func(t *testing.T) {
					defer metabasetest.DeleteAll{}.Check(ctx, t, db)
					originalObjStream := metabasetest.RandObjectStream()

					originalObj, originalSegments := metabasetest.CreateTestObject{
						CommitObject: &metabase.CommitObject{
							ObjectStream:                  originalObjStream,
							EncryptedMetadata:             testrand.Bytes(64),
							EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
							EncryptedMetadataEncryptedKey: testrand.Bytes(265),
						},
					}.Run(ctx, t, db, originalObjStream, byte(numberOfSegments))

					copyObj, _, copySegments := metabasetest.CreateObjectCopy{
						OriginalObject: originalObj,
					}.Run(ctx, t, db, true)

					// check that copy went OK
					metabasetest.Verify{
						Objects: []metabase.RawObject{
							metabase.RawObject(originalObj),
							metabase.RawObject(copyObj),
						},
						Segments: append(metabasetest.SegmentsToRaw(originalSegments), copySegments...),
					}.Check(ctx, t, db)

					metabasetest.DeleteObjectExactVersion{
						Opts: metabase.DeleteObjectExactVersion{
							ObjectLocation: copyObj.Location(),
							Version:        copyObj.Version,
						},
						Result: metabase.DeleteObjectResult{
							Objects: []metabase.Object{copyObj},
						},
					}.Check(ctx, t, db)

					// Verify that we are back at the original single object
					metabasetest.Verify{
						Objects: []metabase.RawObject{
							metabase.RawObject(originalObj),
						},
						Segments: metabasetest.SegmentsToRaw(originalSegments),
					}.Check(ctx, t, db)
				})

				t.Run("delete one of two copies", func(t *testing.T) {
					defer metabasetest.DeleteAll{}.Check(ctx, t, db)
					originalObjectStream := metabasetest.RandObjectStream()

					originalObj, originalSegments := metabasetest.CreateTestObject{
						CommitObject: &metabase.CommitObject{
							ObjectStream:                  originalObjectStream,
							EncryptedMetadata:             testrand.Bytes(64),
							EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
							EncryptedMetadataEncryptedKey: testrand.Bytes(265),
						},
					}.Run(ctx, t, db, originalObjectStream, byte(numberOfSegments))

					copyObject1, _, _ := metabasetest.CreateObjectCopy{
						OriginalObject: originalObj,
					}.Run(ctx, t, db, true)
					copyObject2, _, copySegments2 := metabasetest.CreateObjectCopy{
						OriginalObject: originalObj,
					}.Run(ctx, t, db, true)

					metabasetest.DeleteObjectExactVersion{
						Opts: metabase.DeleteObjectExactVersion{
							ObjectLocation: copyObject1.Location(),
							Version:        copyObject1.Version,
						},
						Result: metabase.DeleteObjectResult{
							Objects: []metabase.Object{copyObject1},
						},
					}.Check(ctx, t, db)

					// Verify that only one of the copies is deleted
					metabasetest.Verify{
						Objects: []metabase.RawObject{
							metabase.RawObject(originalObj),
							metabase.RawObject(copyObject2),
						},
						Segments: append(metabasetest.SegmentsToRaw(originalSegments), copySegments2...),
					}.Check(ctx, t, db)
				})

				t.Run("delete original", func(t *testing.T) {
					defer metabasetest.DeleteAll{}.Check(ctx, t, db)
					originalObjectStream := metabasetest.RandObjectStream()

					originalObj, originalSegments := metabasetest.CreateTestObject{
						CommitObject: &metabase.CommitObject{
							ObjectStream:                  originalObjectStream,
							EncryptedMetadata:             testrand.Bytes(64),
							EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
							EncryptedMetadataEncryptedKey: testrand.Bytes(265),
						},
					}.Run(ctx, t, db, originalObjectStream, byte(numberOfSegments))

					copyObject, _, copySegments := metabasetest.CreateObjectCopy{
						OriginalObject: originalObj,
					}.Run(ctx, t, db, true)

					metabasetest.DeleteObjectExactVersion{
						Opts: metabase.DeleteObjectExactVersion{
							ObjectLocation: originalObj.Location(),
							Version:        originalObj.Version,
						},
						Result: metabase.DeleteObjectResult{
							Objects: []metabase.Object{originalObj},
						},
					}.Check(ctx, t, db)

					for i := range copySegments {
						copySegments[i].Pieces = originalSegments[i].Pieces
					}

					// verify that the copy is left
					metabasetest.Verify{
						Objects: []metabase.RawObject{
							metabase.RawObject(copyObject),
						},
						Segments: copySegments,
					}.Check(ctx, t, db)
				})

				t.Run("delete original and leave two copies", func(t *testing.T) {
					defer metabasetest.DeleteAll{}.Check(ctx, t, db)
					originalObjectStream := metabasetest.RandObjectStream()

					originalObj, originalSegments := metabasetest.CreateTestObject{
						CommitObject: &metabase.CommitObject{
							ObjectStream:                  originalObjectStream,
							EncryptedMetadata:             testrand.Bytes(64),
							EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
							EncryptedMetadataEncryptedKey: testrand.Bytes(265),
						},
					}.Run(ctx, t, db, originalObjectStream, byte(numberOfSegments))

					copyObject1, _, copySegments1 := metabasetest.CreateObjectCopy{
						OriginalObject: originalObj,
					}.Run(ctx, t, db, true)
					copyObject2, _, copySegments2 := metabasetest.CreateObjectCopy{
						OriginalObject: originalObj,
					}.Run(ctx, t, db, true)

					_, err := db.DeleteObjectExactVersion(ctx, metabase.DeleteObjectExactVersion{
						Version:        originalObj.Version,
						ObjectLocation: originalObj.Location(),
					})
					require.NoError(t, err)

					var expectedAncestorStreamID uuid.UUID

					if numberOfSegments > 0 {
						segments, err := db.TestingAllSegments(ctx)
						require.NoError(t, err)
						require.NotEmpty(t, segments)

						if segments[0].StreamID == copyObject1.StreamID {
							expectedAncestorStreamID = copyObject1.StreamID
						} else {
							expectedAncestorStreamID = copyObject2.StreamID
						}
					}

					// set pieces in expected ancestor for verifcation
					for _, segments := range [][]metabase.RawSegment{copySegments1, copySegments2} {
						for i := range segments {
							if segments[i].StreamID == expectedAncestorStreamID {
								segments[i].Pieces = originalSegments[i].Pieces
							}
						}
					}

					// verify that two functioning copies are left and the original object is gone
					metabasetest.Verify{
						Objects: []metabase.RawObject{
							metabase.RawObject(copyObject1),
							metabase.RawObject(copyObject2),
						},
						Segments: append(copySegments1, copySegments2...),
					}.Check(ctx, t, db)
				})
			})
		}
	})
}

func TestDeleteObjectLastCommitted(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		location := obj.Location()

		for _, test := range metabasetest.InvalidObjectLocations(obj.Location()) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				metabasetest.DeleteObjectLastCommitted{
					Opts: metabase.DeleteObjectLastCommitted{
						ObjectLocation: metabase.ObjectLocation{
							ProjectID:  test.ObjectLocation.ProjectID,
							BucketName: test.ObjectLocation.BucketName,
							ObjectKey:  test.ObjectLocation.ObjectKey,
						},
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)
				metabasetest.Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("Object missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: location,
				},
				Result: metabase.DeleteObjectResult{
					Objects: []metabase.Object{},
				},
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})
		t.Run("Delete object without segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			encryptedMetadata := testrand.Bytes(1024)
			encryptedMetadataNonce := testrand.Nonce()
			encryptedMetadataKey := testrand.Bytes(265)

			object, _ := metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:                  obj,
					EncryptedMetadataNonce:        encryptedMetadataNonce[:],
					EncryptedMetadata:             encryptedMetadata,
					EncryptedMetadataEncryptedKey: encryptedMetadataKey,
				},
			}.Run(ctx, t, db, obj, 0)

			metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: location,
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

			metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: location,
				},
				Result: metabase.DeleteObjectResult{
					Objects: []metabase.Object{object},
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
					ObjectStream:      obj,
					Position:          metabase.SegmentPosition{Part: 0, Index: 0},
					EncryptedKey:      testrand.Bytes(32),
					EncryptedKeyNonce: testrand.Bytes(32),
					InlineData:        testrand.Bytes(1024),
					PlainSize:         512,
					PlainOffset:       0,
				},
			}.Check(ctx, t, db)

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
				},
			}.Check(ctx, t, db)

			metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: location,
				},
				Result: metabase.DeleteObjectResult{
					Objects: []metabase.Object{object},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete last committed from several versions", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()
			newObj := metabasetest.RandObjectStream()

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  newObj.ProjectID,
						BucketName: newObj.BucketName,
						ObjectKey:  newObj.ObjectKey,
						Version:    newObj.Version,
						StreamID:   newObj.StreamID,
					},
					ZombieDeletionDeadline: &now,
				},
				Version: newObj.Version,
			}.Check(ctx, t, db)

			newObjDiffVersion := newObj
			newObjDiffVersion.Version = 4

			committedObject, _ := metabasetest.CreateTestObject{}.Run(ctx, t, db, newObjDiffVersion, 0)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream:           newObj,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						ZombieDeletionDeadline: &now,
					},
					{
						ObjectStream: newObjDiffVersion,
						CreatedAt:    now,
						Status:       metabase.Committed,
						Encryption:   metabasetest.DefaultEncryption,
					},
				},
			}.Check(ctx, t, db)

			metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: metabase.ObjectLocation{
						ProjectID:  newObj.ProjectID,
						BucketName: newObj.BucketName,
						ObjectKey:  newObj.ObjectKey,
					}},
				Result: metabase.DeleteObjectResult{Objects: []metabase.Object{committedObject}},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream:           newObj,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						ZombieDeletionDeadline: &now,
					},
				},
			}.Check(ctx, t, db)
		})
	})
}
