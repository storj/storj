// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestBeginCopyObject(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		for _, test := range metabasetest.InvalidObjectLocations(obj.Location()) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)
				metabasetest.BeginCopyObject{
					Opts: metabase.BeginCopyObject{
						ObjectLocation: test.ObjectLocation,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)

				metabasetest.Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("invalid segment limit", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.CreateObject(ctx, t, db, metabasetest.RandObjectStream(), 3)

			metabasetest.BeginCopyObject{
				Opts: metabase.BeginCopyObject{
					ObjectLocation: object.Location(),
					SegmentLimit:   0,
				},
				ErrText: "metabase: invalid request: Segment limit invalid: 0",
			}.Check(ctx, t, db)
		})

		t.Run("begin copy object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			expectedRawObjects := []metabase.RawObject{}
			expectedRawSegments := []metabase.RawSegment{}

			for _, expectedVersion := range []metabase.Version{1, 2, 3, 11} {
				obj.ObjectKey = metabasetest.RandObjectKey()
				obj.StreamID = testrand.UUID()
				obj.Version = expectedVersion
				expectedObject, expectedSegments := metabasetest.CreateTestObject{
					CommitObject: &metabase.CommitObject{
						ObjectStream: obj,
					},
				}.Run(ctx, t, db, obj, 10)

				expectedRawObjects = append(expectedRawObjects, metabase.RawObject(expectedObject))

				var encKeyAndNonces []metabase.EncryptedKeyAndNonce
				for _, expectedSegment := range expectedSegments {
					encKeyAndNonces = append(encKeyAndNonces, metabase.EncryptedKeyAndNonce{
						EncryptedKeyNonce: expectedSegment.EncryptedKeyNonce,
						EncryptedKey:      expectedSegment.EncryptedKey,
						Position:          expectedSegment.Position,
					})

					expectedRawSegments = append(expectedRawSegments, metabase.RawSegment(expectedSegment))
				}

				metabasetest.BeginCopyObject{
					Opts: metabase.BeginCopyObject{
						ObjectLocation: obj.Location(),
						SegmentLimit:   10,
					},
					Result: metabase.BeginCopyObjectResult{
						StreamID:             expectedObject.StreamID,
						Version:              expectedVersion,
						EncryptedKeysNonces:  encKeyAndNonces,
						EncryptionParameters: expectedObject.Encryption,
					},
				}.Check(ctx, t, db)
			}

			metabasetest.Verify{
				Objects:  expectedRawObjects,
				Segments: expectedRawSegments,
			}.Check(ctx, t, db)
		})

		t.Run("begin copy object multiple versions", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := obj
			obj.Version = 1
			obj1 := metabasetest.CreateObject(ctx, t, db, obj, 0)
			obj.Version = 2
			obj2 := metabasetest.CreateObjectVersioned(ctx, t, db, obj, 0)
			obj.Version = 3
			obj3 := metabasetest.CreateObjectVersioned(ctx, t, db, obj, 0)

			for _, object := range []metabase.Object{obj1, obj2, obj3} {
				metabasetest.BeginCopyObject{
					Opts: metabase.BeginCopyObject{
						ObjectLocation: object.Location(),
						Version:        object.Version,
						SegmentLimit:   10,
					},
					Result: metabase.BeginCopyObjectResult{
						StreamID:             object.StreamID,
						Version:              object.Version,
						EncryptionParameters: object.Encryption,
					},
				}.Check(ctx, t, db)
			}
		})

		t.Run("begin copy object with delete marker as source", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.CreateObjectVersioned(ctx, t, db, metabasetest.RandObjectStream(), 0)

			result, err := db.DeleteObjectLastCommitted(ctx, metabase.DeleteObjectLastCommitted{
				ObjectLocation: object.Location(),
				Versioned:      true,
			})
			require.NoError(t, err)

			metabasetest.BeginCopyObject{
				Opts: metabase.BeginCopyObject{
					ObjectLocation: object.Location(),
					Version:        result.Markers[0].Version,
					SegmentLimit:   10,
				},
				ErrClass: &metabase.ErrObjectNotFound,
			}.Check(ctx, t, db)
		})

		t.Run("segment limit", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.CreateObject(ctx, t, db, metabasetest.RandObjectStream(), 3)

			metabasetest.BeginCopyObject{
				Opts: metabase.BeginCopyObject{
					ObjectLocation: object.Location(),
					SegmentLimit:   2,
				},
				ErrText: "metabase: invalid request: object has too many segments (3). Limit is 2.",
			}.Check(ctx, t, db)
		})
	})
}

func TestFinishCopyObject(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()
		newBucketName := metabase.BucketName("New bucket name")

		newStreamID := testrand.UUID()
		for _, test := range metabasetest.InvalidObjectStreams(obj) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)
				metabasetest.FinishCopyObject{
					Opts: metabase.FinishCopyObject{
						NewBucket:    newBucketName,
						ObjectStream: test.ObjectStream,
						NewStreamID:  newStreamID,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)

				metabasetest.Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("invalid NewBucket", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.FinishCopyObject{
				Opts: metabase.FinishCopyObject{
					ObjectStream:          obj,
					NewEncryptedObjectKey: metabasetest.RandObjectKey(),
					NewEncryptedUserData:  metabasetest.RandEncryptedUserData(),
					NewStreamID:           newStreamID,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "NewBucket is missing",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("invalid NewStreamID", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.FinishCopyObject{
				Opts: metabase.FinishCopyObject{
					ObjectStream:          obj,
					NewBucket:             newBucketName,
					NewEncryptedObjectKey: metabasetest.RandObjectKey(),
					NewEncryptedUserData:  metabasetest.RandEncryptedUserData(),
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "NewStreamID is missing",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})
		t.Run("copy to the same StreamID", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.FinishCopyObject{
				Opts: metabase.FinishCopyObject{
					ObjectStream: obj,
					NewBucket:    newBucketName,
					NewStreamID:  obj.StreamID,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "StreamIDs are identical",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("invalid NewEncryptedObjectKey", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.FinishCopyObject{
				Opts: metabase.FinishCopyObject{
					NewBucket:    newBucketName,
					ObjectStream: obj,
					NewStreamID:  newStreamID,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "NewEncryptedObjectKey is missing",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("invalid EncryptedMetadataNonce", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.FinishCopyObject{
				Opts: metabase.FinishCopyObject{
					NewBucket:             newBucketName,
					ObjectStream:          obj,
					NewEncryptedObjectKey: metabasetest.RandObjectKey(),
					NewStreamID:           newStreamID,
					NewEncryptedUserData: metabase.EncryptedUserData{
						EncryptedMetadataEncryptedKey: []byte{0},
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedMetadataNonce is missing",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("invalid EncryptedMetadataEncryptedKey", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.FinishCopyObject{
				Opts: metabase.FinishCopyObject{
					NewBucket:             newBucketName,
					ObjectStream:          obj,
					NewEncryptedObjectKey: metabasetest.RandObjectKey(),
					NewEncryptedUserData: metabase.EncryptedUserData{
						EncryptedMetadataNonce: testrand.Nonce().Bytes(),
					},
					NewStreamID: newStreamID,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedMetadataEncryptedKey is missing",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("empty EncryptedMetadataEncryptedKey and EncryptedMetadataNonce", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.FinishCopyObject{
				Opts: metabase.FinishCopyObject{
					NewBucket:             newBucketName,
					ObjectStream:          obj,
					NewEncryptedObjectKey: metabasetest.RandObjectKey(),
					NewStreamID:           newStreamID,
				},
				// validation pass without EncryptedMetadataEncryptedKey and EncryptedMetadataNonce
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "source object not found",
			}.Check(ctx, t, db)
		})

		t.Run("empty EncryptedMetadata with OverrideMetadata=true", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.FinishCopyObject{
				Opts: metabase.FinishCopyObject{
					NewBucket:             newBucketName,
					ObjectStream:          obj,
					NewEncryptedObjectKey: metabasetest.RandObjectKey(),

					OverrideMetadata: true,
					NewEncryptedUserData: metabase.EncryptedUserData{
						EncryptedMetadataEncryptedKey: []byte{1},
						EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
					},
					NewStreamID: newStreamID,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must be empty when EncryptedMetadata or EncryptedETag are empty",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("empty NewEncryptedMetadataEncryptedKey and NewEncryptedMetadataNonce with OverrideMetadata=true", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.FinishCopyObject{
				Opts: metabase.FinishCopyObject{
					NewBucket:             newBucketName,
					ObjectStream:          obj,
					NewEncryptedObjectKey: metabasetest.RandObjectKey(),
					NewStreamID:           newStreamID,

					OverrideMetadata: true,
					NewEncryptedUserData: metabase.EncryptedUserData{
						EncryptedMetadata: testrand.BytesInt(256),
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must be set when EncryptedMetadata or EncryptedETag are set",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("object does not exist", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			newObj := metabasetest.RandObjectStream()

			newEncryptedMetadataKeyNonce := testrand.Nonce().Bytes()
			newEncryptedMetadataKey := testrand.Bytes(32)
			newEncryptedKeysNonces := make([]metabase.EncryptedKeyAndNonce, 10)

			metabasetest.FinishCopyObject{
				Opts: metabase.FinishCopyObject{
					NewBucket:             newBucketName,
					NewStreamID:           newStreamID,
					ObjectStream:          newObj,
					NewSegmentKeys:        newEncryptedKeysNonces,
					NewEncryptedObjectKey: metabasetest.RandObjectKey(),
					NewEncryptedUserData: metabase.EncryptedUserData{
						EncryptedMetadataNonce:        newEncryptedMetadataKeyNonce,
						EncryptedMetadataEncryptedKey: newEncryptedMetadataKey,
					},
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "source object not found",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		// Assert that an error occurs when a new object has been put at the source key
		// between BeginCopyObject and FinishCopyObject. (stream_id of source key changed)
		t.Run("source object changed", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			newObj, _ := metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:              obj,
					OverrideEncryptedMetadata: true,
					EncryptedUserData:         metabasetest.RandEncryptedUserDataWithoutETag(),
				},
			}.Run(ctx, t, db, obj, 2)

			metabasetest.FinishCopyObject{
				Opts: metabase.FinishCopyObject{
					NewStreamID: testrand.UUID(),
					NewBucket:   newBucketName,
					ObjectStream: metabase.ObjectStream{
						ProjectID:  newObj.ProjectID,
						BucketName: newObj.BucketName,
						ObjectKey:  newObj.ObjectKey,
						Version:    newObj.Version,
						StreamID:   testrand.UUID(),
					},
					NewSegmentKeys: []metabase.EncryptedKeyAndNonce{
						metabasetest.RandEncryptedKeyAndNonce(0),
						metabasetest.RandEncryptedKeyAndNonce(1),
					},
					NewEncryptedObjectKey: metabasetest.RandObjectKey(),
					NewEncryptedUserData: metabase.EncryptedUserData{
						EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
						EncryptedMetadataEncryptedKey: testrand.Bytes(32),
					},
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "object was changed during copy",
			}.Check(ctx, t, db)
		})

		t.Run("not enough segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			numberOfSegments := 10

			newObj, _ := metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:      obj,
					EncryptedUserData: metabasetest.RandEncryptedUserDataWithoutETag(),
				},
			}.Run(ctx, t, db, obj, byte(numberOfSegments))

			newEncryptedMetadataKeyNonce := testrand.Nonce()
			newEncryptedMetadataKey := testrand.Bytes(32)
			newEncryptedKeysNonces := make([]metabase.EncryptedKeyAndNonce, newObj.SegmentCount-1)
			expectedSegments := make([]metabase.RawSegment, newObj.SegmentCount)

			for i := 0; i < int(newObj.SegmentCount-1); i++ {
				newEncryptedKeysNonces[i] = metabase.EncryptedKeyAndNonce{
					Position:          metabase.SegmentPosition{Index: uint32(i)},
					EncryptedKeyNonce: testrand.Nonce().Bytes(),
					EncryptedKey:      testrand.Bytes(32),
				}

				expectedSegments[i] = metabasetest.DefaultRawSegment(newObj.ObjectStream, metabase.SegmentPosition{Index: uint32(i)})
				expectedSegments[i].EncryptedKeyNonce = newEncryptedKeysNonces[i].EncryptedKeyNonce
				expectedSegments[i].EncryptedKey = newEncryptedKeysNonces[i].EncryptedKey
				expectedSegments[i].PlainOffset = int64(int32(i) * expectedSegments[i].PlainSize)
				expectedSegments[i].EncryptedSize = int32(0)
			}

			metabasetest.FinishCopyObject{
				Opts: metabase.FinishCopyObject{
					NewBucket:             newBucketName,
					ObjectStream:          obj,
					NewStreamID:           newStreamID,
					NewSegmentKeys:        newEncryptedKeysNonces,
					NewEncryptedObjectKey: metabasetest.RandObjectKey(),
					NewEncryptedUserData: metabase.EncryptedUserData{
						EncryptedMetadataNonce:        newEncryptedMetadataKeyNonce.Bytes(),
						EncryptedMetadataEncryptedKey: newEncryptedMetadataKey,
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "wrong number of segments keys received (received 9, need 10)",
			}.Check(ctx, t, db)
		})

		t.Run("wrong segment indexes", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			numberOfSegments := 10

			newObj, _ := metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:      obj,
					EncryptedUserData: metabasetest.RandEncryptedUserDataWithoutETag(),
				},
			}.Run(ctx, t, db, obj, byte(numberOfSegments))

			newEncryptedMetadataKeyNonce := testrand.Nonce()
			newEncryptedMetadataKey := testrand.Bytes(32)
			newEncryptedKeysNonces := make([]metabase.EncryptedKeyAndNonce, newObj.SegmentCount)
			expectedEncryptedSize := 1060
			expectedSegments := make([]metabase.RawSegment, newObj.SegmentCount)

			for i := 0; i < int(newObj.SegmentCount); i++ {
				newEncryptedKeysNonces[i] = metabase.EncryptedKeyAndNonce{
					Position:          metabase.SegmentPosition{Index: uint32(i + 5)},
					EncryptedKeyNonce: testrand.Nonce().Bytes(),
					EncryptedKey:      testrand.Bytes(32),
				}

				expectedSegments[i] = metabasetest.DefaultRawSegment(newObj.ObjectStream, metabase.SegmentPosition{Index: uint32(i)})
				expectedSegments[i].EncryptedKeyNonce = newEncryptedKeysNonces[i].EncryptedKeyNonce
				expectedSegments[i].EncryptedKey = newEncryptedKeysNonces[i].EncryptedKey
				expectedSegments[i].PlainOffset = int64(int32(i) * expectedSegments[i].PlainSize)
				expectedSegments[i].EncryptedSize = int32(expectedEncryptedSize)
			}

			metabasetest.FinishCopyObject{
				Opts: metabase.FinishCopyObject{
					NewStreamID:           newStreamID,
					NewBucket:             newBucketName,
					ObjectStream:          obj,
					NewSegmentKeys:        newEncryptedKeysNonces,
					NewEncryptedObjectKey: metabasetest.RandObjectKey(),
					NewEncryptedUserData: metabase.EncryptedUserData{
						EncryptedMetadataNonce:        newEncryptedMetadataKeyNonce.Bytes(),
						EncryptedMetadataEncryptedKey: newEncryptedMetadataKey,
					},
				},
				ErrClass: &metabase.Error,
				ErrText:  "missing new segment keys for segment 0",
			}.Check(ctx, t, db)
		})

		t.Run("returned object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			expectedRawObjects := []metabase.RawObject{}

			for _, expectedVersion := range []metabase.Version{1} {
				objStream := metabasetest.RandObjectStream()
				objStream.Version = expectedVersion

				copyStream := metabasetest.RandObjectStream()
				copyStream.ProjectID = objStream.ProjectID
				copyStream.BucketName = objStream.BucketName

				originalObj, _ := metabasetest.CreateTestObject{
					CommitObject: &metabase.CommitObject{
						ObjectStream:      objStream,
						EncryptedUserData: metabasetest.RandEncryptedUserDataWithoutETag(),
					},
				}.Run(ctx, t, db, objStream, 0)

				metadataNonce := testrand.Nonce()
				expectedCopyObject := originalObj
				expectedCopyObject.ObjectKey = copyStream.ObjectKey
				expectedCopyObject.StreamID = copyStream.StreamID
				expectedCopyObject.Version = 0
				expectedCopyObject.EncryptedMetadataEncryptedKey = testrand.Bytes(32)
				expectedCopyObject.EncryptedMetadataNonce = metadataNonce.Bytes()

				objectCopy := metabasetest.FinishCopyObject{
					Opts: metabase.FinishCopyObject{
						ObjectStream:          objStream,
						NewBucket:             copyStream.BucketName,
						NewStreamID:           copyStream.StreamID,
						NewEncryptedObjectKey: copyStream.ObjectKey,
						NewEncryptedUserData: metabase.EncryptedUserData{
							EncryptedMetadataEncryptedKey: expectedCopyObject.EncryptedMetadataEncryptedKey,
							EncryptedMetadataNonce:        metadataNonce.Bytes(),
						},
					},
					Result: expectedCopyObject,
				}.Check(ctx, t, db)

				require.NotEqual(t, originalObj.CreatedAt, objectCopy.CreatedAt)
				expectedCopyObject.Version = objectCopy.Version

				expectedRawObjects = append(expectedRawObjects, metabase.RawObject(originalObj))
				expectedRawObjects = append(expectedRawObjects, metabase.RawObject(expectedCopyObject))
			}

			metabasetest.Verify{
				Objects: expectedRawObjects,
			}.Check(ctx, t, db)
		})

		t.Run("finish copy object with existing metadata", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			numberOfSegments := 10
			copyStream := metabasetest.RandObjectStream()

			originalObj, _ := metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:      obj,
					EncryptedUserData: metabasetest.RandEncryptedUserDataWithoutETag(),
				},
			}.Run(ctx, t, db, obj, byte(numberOfSegments))

			copyObj, expectedOriginalSegments, expectedCopySegments := metabasetest.CreateObjectCopy{
				OriginalObject:   originalObj,
				CopyObjectStream: &copyStream,
			}.Run(ctx, t, db)

			var expectedRawSegments []metabase.RawSegment
			expectedRawSegments = append(expectedRawSegments, expectedOriginalSegments...)
			expectedRawSegments = append(expectedRawSegments, expectedCopySegments...)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(originalObj),
					metabase.RawObject(copyObj),
				},
				Segments: expectedRawSegments,
			}.Check(ctx, t, db)

			// TODO find better names
			copyOfCopyStream := metabasetest.RandObjectStream()
			copyOfCopyObj, _, expectedCopyOfCopySegments := metabasetest.CreateObjectCopy{
				OriginalObject:   copyObj,
				CopyObjectStream: &copyOfCopyStream,
			}.Run(ctx, t, db)

			expectedRawSegments = append(expectedRawSegments, expectedCopyOfCopySegments...)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(originalObj),
					metabase.RawObject(copyObj),
					metabase.RawObject(copyOfCopyObj),
				},
				Segments: expectedRawSegments,
			}.Check(ctx, t, db)
		})

		t.Run("finish copy object with new metadata", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			copyStream := metabasetest.RandObjectStream()
			copyStreamNoOverride := metabasetest.RandObjectStream()

			originalData := metabasetest.RandEncryptedUserDataWithoutETag()

			originalObj, _ := metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:              obj,
					OverrideEncryptedMetadata: true,
					EncryptedUserData:         originalData,
				},
			}.Run(ctx, t, db, obj, 0)

			newData := metabasetest.RandEncryptedUserDataWithoutETag()

			// do a copy without OverrideMetadata field set to true,
			// metadata shouldn't be updated even if NewEncryptedMetadata
			// field is set
			copyObjNoOverride, _, _ := metabasetest.CreateObjectCopy{
				OriginalObject:   originalObj,
				CopyObjectStream: &copyStreamNoOverride,
				FinishObject: &metabase.FinishCopyObject{
					ObjectStream: originalObj.ObjectStream,

					NewBucket:   copyStreamNoOverride.BucketName,
					NewStreamID: copyStreamNoOverride.StreamID,

					NewEncryptedObjectKey: copyStreamNoOverride.ObjectKey,

					OverrideMetadata:     false,
					NewEncryptedUserData: newData,
				},
			}.Run(ctx, t, db)

			// Only EncryptedMetadataEncryptedKey and EncryptedMetadataNonce should change when
			// OverrideMetadata = false.
			expectedData := originalData
			expectedData.EncryptedMetadataEncryptedKey = newData.EncryptedMetadataEncryptedKey
			expectedData.EncryptedMetadataNonce = newData.EncryptedMetadataNonce
			require.Equal(t, expectedData, copyObjNoOverride.EncryptedUserData)

			// do a copy WITH OverrideMetadata field set to true,
			// metadata should be updated to NewEncryptedMetadata
			copyObj, _, _ := metabasetest.CreateObjectCopy{
				OriginalObject:   originalObj,
				CopyObjectStream: &copyStream,
				FinishObject: &metabase.FinishCopyObject{
					ObjectStream: originalObj.ObjectStream,

					NewBucket:   copyStream.BucketName,
					NewStreamID: copyStream.StreamID,

					NewEncryptedObjectKey: copyStream.ObjectKey,

					OverrideMetadata:     true,
					NewEncryptedUserData: newData,
				},
			}.Run(ctx, t, db)

			require.Equal(t, newData, copyObj.EncryptedUserData)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(originalObj),
					metabase.RawObject(copyObj),
					metabase.RawObject(copyObjNoOverride),
				},
			}.Check(ctx, t, db)
		})

		t.Run("finish copy object to already existing destination", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			// Test:
			// - 3 objects: objA, objB, objC
			// - copy objB to objA - creating objBprime
			// - check that segments of original objA have been deleted
			// - check that we now have three objects: objBprime, objB, objC
			// - copy objC to objB creating objCprime
			// - check that we now have three objects: objBprime, objCprime, objC
			// - check that objBprime has become an original object, now that its ancestor
			// objB has been overwritten

			// object that already exists
			objStreamA := metabasetest.RandObjectStream()
			objStreamB := metabasetest.RandObjectStream()
			objStreamC := metabasetest.RandObjectStream()

			// set same projectID for all
			objStreamB.ProjectID = objStreamA.ProjectID
			objStreamC.ProjectID = objStreamA.ProjectID

			objA, _ := metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:      objStreamA,
					EncryptedUserData: metabasetest.RandEncryptedUserDataWithoutETag(),
				},
			}.Run(ctx, t, db, objStreamA, 4)

			objB, _ := metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:      objStreamB,
					EncryptedUserData: metabasetest.RandEncryptedUserDataWithoutETag(),
				},
			}.Run(ctx, t, db, objStreamB, 3)

			objC, segmentsOfC := metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:      objStreamC,
					EncryptedUserData: metabasetest.RandEncryptedUserDataWithoutETag(),
				},
			}.Run(ctx, t, db, objStreamC, 1)

			// B' is a copy of B to A
			objStreamBprime := objStreamA
			objStreamBprime.StreamID = testrand.UUID()
			objBprime, expectedSegmentsOfB, expectedSegmentsOfBprime := metabasetest.CreateObjectCopy{
				OriginalObject:   objB,
				CopyObjectStream: &objStreamBprime,
			}.Run(ctx, t, db)

			// check that we indeed overwrote object A
			require.Equal(t, objA.BucketName, objBprime.BucketName)
			require.Equal(t, objA.ProjectID, objBprime.ProjectID)
			require.Equal(t, objA.ObjectKey, objBprime.ObjectKey)

			require.NotEqual(t, objA.StreamID, objBprime.StreamID)

			var expectedRawSegments []metabase.RawSegment
			expectedRawSegments = append(expectedRawSegments, expectedSegmentsOfBprime...)
			expectedRawSegments = append(expectedRawSegments, expectedSegmentsOfB...)
			expectedRawSegments = append(expectedRawSegments, metabasetest.SegmentsToRaw(segmentsOfC)...)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(objBprime),
					metabase.RawObject(objB),
					metabase.RawObject(objC),
				},
				Segments: expectedRawSegments,
			}.Check(ctx, t, db)

			// C' is a copy of C to B
			objStreamCprime := objStreamB
			objStreamCprime.StreamID = testrand.UUID()
			objCprime, _, expectedSegmentsOfCprime := metabasetest.CreateObjectCopy{
				OriginalObject:   objC,
				CopyObjectStream: &objStreamCprime,
			}.Run(ctx, t, db)

			require.Equal(t, objStreamB.BucketName, objCprime.BucketName)
			require.Equal(t, objStreamB.ProjectID, objCprime.ProjectID)
			require.Equal(t, objStreamB.ObjectKey, objCprime.ObjectKey)
			require.NotEqual(t, objB.StreamID, objCprime)

			// B' should become the original of B and now hold pieces.
			for i := range expectedSegmentsOfBprime {
				expectedSegmentsOfBprime[i].EncryptedETag = nil
				expectedSegmentsOfBprime[i].Pieces = expectedSegmentsOfB[i].Pieces
			}

			var expectedSegments []metabase.RawSegment
			expectedSegments = append(expectedSegments, expectedSegmentsOfBprime...)
			expectedSegments = append(expectedSegments, expectedSegmentsOfCprime...)
			expectedSegments = append(expectedSegments, metabasetest.SegmentsToRaw(segmentsOfC)...)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(objBprime),
					metabase.RawObject(objCprime),
					metabase.RawObject(objC),
				},
				Segments: expectedSegments,
			}.Check(ctx, t, db)
		})

		// checks that a copy can be copied to it's ancestor location
		t.Run("Copy child to ancestor", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			originalObjStream := metabasetest.RandObjectStream()
			copyObjStream := metabasetest.RandObjectStream()
			// Copy back to original object key.
			// StreamID is independent of key.
			copyBackObjStream := originalObjStream
			copyBackObjStream.StreamID = testrand.UUID()

			originalObj, originalSegments := metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:      originalObjStream,
					EncryptedUserData: metabasetest.RandEncryptedUserDataWithoutETag(),
				},
			}.Run(ctx, t, db, originalObjStream, 4)

			copyObj, _, copySegments := metabasetest.CreateObjectCopy{
				OriginalObject:   originalObj,
				CopyObjectStream: &copyObjStream,
			}.Run(ctx, t, db)

			// Copy the copy back to the source location
			opts := metabase.FinishCopyObject{
				// source
				ObjectStream: copyObj.ObjectStream,
				// destination
				NewBucket:             originalObj.BucketName,
				NewEncryptedObjectKey: originalObj.ObjectKey,
				NewStreamID:           copyBackObjStream.StreamID,
				OverrideMetadata:      false,
				NewSegmentKeys: []metabase.EncryptedKeyAndNonce{
					metabasetest.RandEncryptedKeyAndNonce(0),
					metabasetest.RandEncryptedKeyAndNonce(1),
					metabasetest.RandEncryptedKeyAndNonce(2),
					metabasetest.RandEncryptedKeyAndNonce(3),
				},
			}
			copyObjResult, _, _ := metabasetest.CreateObjectCopy{
				OriginalObject:   copyObj,
				CopyObjectStream: &copyBackObjStream,
				FinishObject:     &opts,
			}.Run(ctx, t, db)
			require.Greater(t, copyObjResult.Version, originalObj.Version)

			// expected object at the location which was previously the original object
			copyBackObj := originalObj
			copyBackObj.Version = copyObjResult.Version

			copyBackObj.StreamID = opts.NewStreamID

			for i := 0; i < 4; i++ {
				copySegments[i].Pieces = originalSegments[i].Pieces
				copySegments[i].InlineData = originalSegments[i].InlineData
				copySegments[i].EncryptedETag = nil // TODO: ETag seems lost after copy

				originalSegments[i].StreamID = opts.NewStreamID
				originalSegments[i].InlineData = nil
				originalSegments[i].EncryptedKey = opts.NewSegmentKeys[i].EncryptedKey
				originalSegments[i].EncryptedKeyNonce = opts.NewSegmentKeys[i].EncryptedKeyNonce
				originalSegments[i].EncryptedETag = nil // TODO: ETag seems lost after copy
			}

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(copyObj),
					metabase.RawObject(copyBackObj),
				},
				Segments: append(metabasetest.SegmentsToRaw(originalSegments), copySegments...),
			}.Check(ctx, t, db)
		})

		// checks that a copy ancestor can be copied to itself
		t.Run("Copy ancestor to itself", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			originalObjStream := metabasetest.RandObjectStream()
			copyObjStream := metabasetest.RandObjectStream()
			// Copy back to same object key.
			// StreamID is independent of key.
			copyBackObjStream := originalObjStream
			copyBackObjStream.StreamID = testrand.UUID()

			originalObj, _ := metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:      originalObjStream,
					EncryptedUserData: metabasetest.RandEncryptedUserDataWithoutETag(),
				},
			}.Run(ctx, t, db, originalObjStream, 4)

			copyObj, originalSegments, copySegments := metabasetest.CreateObjectCopy{
				OriginalObject:   originalObj,
				CopyObjectStream: &copyObjStream,
			}.Run(ctx, t, db)

			opts := metabase.FinishCopyObject{
				// source
				ObjectStream: copyObj.ObjectStream,
				// destination
				NewBucket:             originalObj.BucketName,
				NewEncryptedObjectKey: originalObj.ObjectKey,
				NewStreamID:           copyBackObjStream.StreamID,
				OverrideMetadata:      false,
				NewSegmentKeys: []metabase.EncryptedKeyAndNonce{
					metabasetest.RandEncryptedKeyAndNonce(0),
					metabasetest.RandEncryptedKeyAndNonce(1),
					metabasetest.RandEncryptedKeyAndNonce(2),
					metabasetest.RandEncryptedKeyAndNonce(3),
				},
			}
			// Copy the copy back to the source location
			copyObjResult, _, _ := metabasetest.CreateObjectCopy{
				OriginalObject:   originalObj,
				CopyObjectStream: &copyBackObjStream,
				FinishObject:     &opts,
			}.Run(ctx, t, db)
			require.Greater(t, copyObjResult.Version, originalObj.Version)

			copyBackObj := originalObj
			copyBackObj.Version = copyObjResult.Version
			copyBackObj.StreamID = copyBackObjStream.StreamID

			for i := 0; i < 4; i++ {
				copySegments[i].Pieces = originalSegments[i].Pieces
				copySegments[i].InlineData = originalSegments[i].InlineData
				copySegments[i].EncryptedETag = nil // TODO: ETag seems lost after copy

				originalSegments[i].StreamID = opts.NewStreamID
				originalSegments[i].InlineData = nil
				originalSegments[i].EncryptedKey = opts.NewSegmentKeys[i].EncryptedKey
				originalSegments[i].EncryptedKeyNonce = opts.NewSegmentKeys[i].EncryptedKeyNonce
				originalSegments[i].EncryptedETag = nil // TODO: ETag seems lost after copy
			}

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(copyObj),
					metabase.RawObject(copyBackObj),
				},
				Segments: append(originalSegments, copySegments...),
			}.Check(ctx, t, db)
		})

		t.Run("copied segments has same expires_at as original", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			expiresAt := time.Now().Add(2 * time.Hour)

			objStream := metabasetest.RandObjectStream()
			copyStream := metabasetest.RandObjectStream()
			copyStream.ProjectID = objStream.ProjectID
			copyStream.BucketName = objStream.BucketName

			originalObj := metabasetest.CreateExpiredObject(ctx, t, db, objStream, 10, expiresAt)

			metadataNonce := testrand.Nonce()
			expectedCopyObject := originalObj
			expectedCopyObject.Version = 0 // ignore version check
			expectedCopyObject.ObjectKey = copyStream.ObjectKey
			expectedCopyObject.StreamID = copyStream.StreamID
			expectedCopyObject.EncryptedMetadataEncryptedKey = testrand.Bytes(32)
			expectedCopyObject.EncryptedMetadataNonce = metadataNonce.Bytes()

			newEncryptedKeysNonces := make([]metabase.EncryptedKeyAndNonce, originalObj.SegmentCount)
			expectedSegments := make([]metabase.RawSegment, originalObj.SegmentCount)

			for i := 0; i < int(originalObj.SegmentCount); i++ {
				newEncryptedKeysNonces[i] = metabase.EncryptedKeyAndNonce{
					Position:          metabase.SegmentPosition{Index: uint32(i)},
					EncryptedKeyNonce: testrand.Nonce().Bytes(),
					EncryptedKey:      testrand.Bytes(32),
				}

				expectedSegments[i] = metabasetest.DefaultRawSegment(originalObj.ObjectStream, metabase.SegmentPosition{Index: uint32(i)})
				expectedSegments[i].EncryptedKeyNonce = newEncryptedKeysNonces[i].EncryptedKeyNonce
				expectedSegments[i].EncryptedKey = newEncryptedKeysNonces[i].EncryptedKey
				expectedSegments[i].PlainOffset = int64(int32(i) * expectedSegments[i].PlainSize)
				expectedSegments[i].EncryptedSize = int32(0)
			}

			copyObj := metabasetest.FinishCopyObject{
				Opts: metabase.FinishCopyObject{
					ObjectStream:          objStream,
					NewBucket:             copyStream.BucketName,
					NewStreamID:           copyStream.StreamID,
					NewEncryptedObjectKey: copyStream.ObjectKey,
					NewEncryptedUserData: metabase.EncryptedUserData{
						EncryptedMetadataEncryptedKey: expectedCopyObject.EncryptedMetadataEncryptedKey,
						EncryptedMetadataNonce:        metadataNonce.Bytes(),
					},
					NewSegmentKeys: newEncryptedKeysNonces,
				},
				Result: expectedCopyObject,
			}.Check(ctx, t, db)

			require.NotZero(t, copyObj.Version)
			expectedCopyObject.Version = copyObj.Version

			var listSegments []metabase.Segment

			copiedSegments, err := db.ListSegments(ctx, metabase.ListSegments{
				StreamID: copyObj.StreamID,
			})
			require.NoError(t, err)

			originalSegments, err := db.ListSegments(ctx, metabase.ListSegments{
				StreamID: originalObj.StreamID,
			})
			require.NoError(t, err)

			listSegments = append(listSegments, originalSegments.Segments...)
			listSegments = append(listSegments, copiedSegments.Segments...)

			for _, v := range listSegments {
				require.Equal(t, expiresAt.Unix(), v.ExpiresAt.Unix())
			}

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(originalObj),
					metabase.RawObject(copyObj),
				},
				Segments: metabasetest.SegmentsToRaw(listSegments),
			}.Check(ctx, t, db)
		})

		t.Run("finish copy object to same destination", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()
			numberOfSegments := 10
			originalObj, _ := metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:      obj,
					EncryptedUserData: metabasetest.RandEncryptedUserDataWithoutETag(),
				},
			}.Run(ctx, t, db, obj, byte(numberOfSegments))

			obj.StreamID = testrand.UUID()
			expectedCopy, _, expectedCopySegments := metabasetest.CreateObjectCopy{
				OriginalObject:   originalObj,
				CopyObjectStream: &obj,
			}.Run(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(expectedCopy),
				},
				Segments: expectedCopySegments,
			}.Check(ctx, t, db)
		})

		t.Run("finish copy object versioned to same destination", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			// both should be preserved

			obj := metabasetest.RandObjectStream()
			numberOfSegments := 10
			originalObj, _ := metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:      obj,
					EncryptedUserData: metabasetest.RandEncryptedUserDataWithoutETag(),
				},
			}.Run(ctx, t, db, obj, byte(numberOfSegments))

			obj.StreamID = testrand.UUID()
			expectedCopy, expectedOriginalSegments, expectedCopySegments := metabasetest.CreateObjectCopy{
				OriginalObject:   originalObj,
				CopyObjectStream: &obj,

				NewVersioned: true,
			}.Run(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(originalObj),
					metabase.RawObject(expectedCopy),
				},
				Segments: append(expectedCopySegments, expectedOriginalSegments...),
			}.Check(ctx, t, db)
		})

		t.Run("finish copy object to existing pending destination", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			sourceObjStream := metabasetest.RandObjectStream()
			destinationObjStream := metabasetest.RandObjectStream()
			destinationObjStream.ProjectID = sourceObjStream.ProjectID
			// testcases:
			// - versions of pending objects
			// - version of committed object
			// - expected copy version

			testCases := []struct {
				Bucket                       metabase.BucketName
				Key                          metabase.ObjectKey
				NewBucket                    metabase.BucketName
				NewKey                       metabase.ObjectKey
				sourcePendingVersions        []metabase.Version
				sourceCommitVersion          metabase.Version
				sourceCommittedVersion       metabase.Version
				destinationPendingVersions   []metabase.Version
				destinationCommitVersion     metabase.Version
				destionationCommittedVersion metabase.Version
			}{
				// the same bucket
				0: {
					"testbucket", "object", "testbucket", "new-object",
					[]metabase.Version{},
					2, 2,
					[]metabase.Version{},
					1, 1,
				},
				1: {
					"testbucket", "object", "testbucket", "new-object",
					[]metabase.Version{},
					1, 1,
					[]metabase.Version{1},
					2, 2,
				},
				2: {
					"testbucket", "object", "testbucket", "new-object",
					[]metabase.Version{},
					1, 1,
					[]metabase.Version{1, 3},
					2, 4,
				},
				3: {
					"testbucket", "object", "testbucket", "new-object",
					[]metabase.Version{1, 5},
					2, 6,
					[]metabase.Version{1, 3},
					2, 4,
				},
				4: {
					"testbucket", "object", "newbucket", "object",
					[]metabase.Version{2, 3},
					1, 4,
					[]metabase.Version{1, 5},
					2, 6,
				},
			}

			for i, tc := range testCases {
				t.Run(strconv.Itoa(i), func(t *testing.T) {
					defer metabasetest.DeleteAll{}.Check(ctx, t, db)
					sourceObjStream.BucketName = tc.Bucket
					sourceObjStream.ObjectKey = tc.Key
					destinationObjStream.BucketName = tc.NewBucket
					destinationObjStream.ObjectKey = tc.NewKey

					var rawObjects []metabase.RawObject
					for _, version := range tc.sourcePendingVersions {
						sourceObjStream.Version = version
						sourceObjStream.StreamID = testrand.UUID()
						object := metabasetest.CreatePendingObject(ctx, t, db, sourceObjStream, 0)

						rawObjects = append(rawObjects, metabase.RawObject(object))
					}
					sourceObjStream.Version = tc.sourceCommitVersion
					sourceObjStream.StreamID = testrand.UUID()
					sourceObj, _ := metabasetest.CreateTestObject{
						BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
							ObjectStream: sourceObjStream,
							Encryption:   metabasetest.DefaultEncryption,
						},
						CommitObject: &metabase.CommitObject{
							ObjectStream:              sourceObjStream,
							OverrideEncryptedMetadata: true,
							EncryptedUserData:         metabasetest.RandEncryptedUserDataWithoutETag(),
						},
						ExpectVersion: 0,
					}.Run(ctx, t, db, sourceObjStream, 0)

					rawObjects = append(rawObjects, metabase.RawObject(sourceObj))

					for _, version := range tc.destinationPendingVersions {
						destinationObjStream.Version = version
						destinationObjStream.StreamID = testrand.UUID()
						object := metabasetest.CreatePendingObject(ctx, t, db, destinationObjStream, 0)

						rawObjects = append(rawObjects, metabase.RawObject(object))
					}

					if tc.destinationCommitVersion != 0 {
						destinationObjStream.StreamID = testrand.UUID()
						destinationObjStream.Version = tc.destinationCommitVersion
						_, _ = metabasetest.CreateTestObject{
							BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
								ObjectStream: destinationObjStream,
								Encryption:   metabasetest.DefaultEncryption,
							},
							CommitObject: &metabase.CommitObject{
								ObjectStream:              destinationObjStream,
								OverrideEncryptedMetadata: true,
								EncryptedUserData:         metabasetest.RandEncryptedUserDataWithoutETag(),
							},
							ExpectVersion: 0,
						}.Run(ctx, t, db, destinationObjStream, 0)
					}

					copyObj, expectedOriginalSegments, _ := metabasetest.CreateObjectCopy{
						OriginalObject:   sourceObj,
						CopyObjectStream: &destinationObjStream,
					}.Run(ctx, t, db)

					require.NotZero(t, copyObj.Version)

					rawObjects = append(rawObjects, metabase.RawObject(copyObj))

					metabasetest.Verify{
						Objects:  rawObjects,
						Segments: expectedOriginalSegments,
					}.Check(ctx, t, db)
				})
			}
		})

		t.Run("existing object is overwritten", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			initialStream := metabasetest.RandObjectStream()
			initialObject := metabasetest.CreateObject(ctx, t, db, initialStream, 0)

			conflictObjStream := metabasetest.RandObjectStream()
			conflictObjStream.ProjectID = initialStream.ProjectID
			metabasetest.CreateObject(ctx, t, db, conflictObjStream, 0)

			newNonce := testrand.Nonce()
			newMetadataKey := testrand.Bytes(265)
			newUUID := testrand.UUID()

			now := time.Now()

			copiedObject := metabase.Object{
				ObjectStream: metabase.ObjectStream{
					ProjectID:  conflictObjStream.ProjectID,
					BucketName: conflictObjStream.BucketName,
					ObjectKey:  conflictObjStream.ObjectKey,
					StreamID:   newUUID,
					Version:    0,
				},
				CreatedAt:  now,
				Status:     metabase.CommittedUnversioned,
				Encryption: initialObject.Encryption,
				EncryptedUserData: metabase.EncryptedUserData{
					EncryptedMetadataNonce:        newNonce.Bytes(),
					EncryptedMetadataEncryptedKey: newMetadataKey,
				},
			}

			copyObjResult := metabasetest.FinishCopyObject{
				Opts: metabase.FinishCopyObject{
					NewBucket:             conflictObjStream.BucketName,
					NewStreamID:           newUUID,
					ObjectStream:          initialStream,
					NewEncryptedObjectKey: conflictObjStream.ObjectKey,
					NewEncryptedUserData: metabase.EncryptedUserData{
						EncryptedMetadataNonce:        newNonce.Bytes(),
						EncryptedMetadataEncryptedKey: newMetadataKey,
					},
				},
				Result: copiedObject,
			}.Check(ctx, t, db)

			require.NotZero(t, copyObjResult.Version)
			copiedObject.Version = copyObjResult.Version

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(initialObject),
					metabase.RawObject(copiedObject),
				},
			}.Check(ctx, t, db)
		})

		t.Run("existing object is not overwritten, permission denied", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			initialStream := metabasetest.RandObjectStream()
			initialObject := metabasetest.CreateObject(ctx, t, db, initialStream, 0)

			conflictObjStream := metabasetest.RandObjectStream()
			conflictObjStream.ProjectID = initialStream.ProjectID
			conflictObject := metabasetest.CreateObject(ctx, t, db, conflictObjStream, 0)

			newNonce := testrand.Nonce()
			newMetadataKey := testrand.Bytes(265)
			newUUID := testrand.UUID()

			metabasetest.FinishCopyObject{
				Opts: metabase.FinishCopyObject{
					NewBucket:    conflictObjStream.BucketName,
					ObjectStream: initialStream,
					NewStreamID:  newUUID,

					NewEncryptedObjectKey: conflictObjStream.ObjectKey,
					NewEncryptedUserData: metabase.EncryptedUserData{
						EncryptedMetadataNonce:        newNonce.Bytes(),
						EncryptedMetadataEncryptedKey: newMetadataKey,
					},

					NewDisallowDelete: true,
				},
				ErrClass: &metabase.ErrPermissionDenied,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(conflictObject),
					metabase.RawObject(initialObject),
				},
			}.Check(ctx, t, db)
		})

		t.Run("versioned targets unversioned and versioned", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()
			obj.Version = 12000
			unversionedObject := metabasetest.CreateObject(ctx, t, db, obj, 0)
			obj.Version = 13000
			versionedObject := metabasetest.CreateObjectVersioned(ctx, t, db, obj, 0)

			sourceStream := metabasetest.RandObjectStream()
			sourceStream.ProjectID = obj.ProjectID
			sourceObject := metabasetest.CreateObject(ctx, t, db, sourceStream, 0)

			newStreamID := testrand.UUID()

			copiedObject := sourceObject
			copiedObject.ObjectStream.ProjectID = obj.ProjectID
			copiedObject.ObjectStream.BucketName = obj.BucketName
			copiedObject.ObjectStream.ObjectKey = obj.ObjectKey
			copiedObject.ObjectStream.Version = 0
			copiedObject.ObjectStream.StreamID = newStreamID
			copiedObject.Status = metabase.CommittedVersioned

			// versioned copy should leave everything else as is
			copyObjResult := metabasetest.FinishCopyObject{
				Opts: metabase.FinishCopyObject{
					ObjectStream:          sourceStream,
					NewBucket:             obj.BucketName,
					NewStreamID:           newStreamID,
					NewEncryptedObjectKey: obj.ObjectKey,

					NewVersioned: true,
				},
				Result: copiedObject,
			}.Check(ctx, t, db)
			require.Greater(t, copyObjResult.Version, obj.Version)
			copiedObject.Version = copyObjResult.Version

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(unversionedObject),
					metabase.RawObject(versionedObject),
					metabase.RawObject(sourceObject),
					metabase.RawObject(copiedObject),
				},
			}.Check(ctx, t, db)
		})

		t.Run("unversioned targets unversioned and versioned", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()
			obj.Version = 12000
			metabasetest.CreateObject(ctx, t, db, obj, 0)
			obj.Version = 13000
			versionedObject := metabasetest.CreateObjectVersioned(ctx, t, db, obj, 0)

			sourceStream := metabasetest.RandObjectStream()
			sourceStream.ProjectID = obj.ProjectID
			sourceObject := metabasetest.CreateObject(ctx, t, db, sourceStream, 0)

			newStreamID := testrand.UUID()

			copiedObject := sourceObject
			copiedObject.ObjectStream.ProjectID = obj.ProjectID
			copiedObject.ObjectStream.BucketName = obj.BucketName
			copiedObject.ObjectStream.ObjectKey = obj.ObjectKey
			copiedObject.ObjectStream.Version = 0
			copiedObject.ObjectStream.StreamID = newStreamID
			copiedObject.Status = metabase.CommittedUnversioned

			// unversioned copy should only delete the unversioned object
			copyObjectResult := metabasetest.FinishCopyObject{
				Opts: metabase.FinishCopyObject{
					ObjectStream:          sourceStream,
					NewBucket:             obj.BucketName,
					NewStreamID:           newStreamID,
					NewEncryptedObjectKey: obj.ObjectKey,
					NewVersioned:          false,
				},
				Result: copiedObject,
			}.Check(ctx, t, db)

			require.Greater(t, copyObjectResult.Version, obj.Version)
			copiedObject.ObjectStream.Version = copyObjectResult.Version

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(versionedObject),
					metabase.RawObject(sourceObject),
					metabase.RawObject(copiedObject),
				},
			}.Check(ctx, t, db)
		})

		t.Run("unversioned delete marker targets unversioned and versioned", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()
			obj.Version = 12000
			unversionedObject := metabasetest.CreateObject(ctx, t, db, obj, 0)
			obj.Version = 13000
			versionedObject := metabasetest.CreateObjectVersioned(ctx, t, db, obj, 0)

			sourceStream := metabasetest.RandObjectStream()
			sourceStream.ProjectID = obj.ProjectID
			sourceObject := metabasetest.CreateObject(ctx, t, db, sourceStream, 0)

			deletionResult := metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: sourceObject.Location(),
					Suspended:      true,
				},
				Result: metabase.DeleteObjectResult{
					Removed: []metabase.Object{sourceObject},
					Markers: []metabase.Object{
						{
							ObjectStream: metabase.ObjectStream{
								ProjectID:  sourceObject.ProjectID,
								BucketName: sourceObject.BucketName,
								ObjectKey:  sourceObject.ObjectKey,
								StreamID:   sourceStream.StreamID,
							},
							Status:    metabase.DeleteMarkerUnversioned,
							CreatedAt: time.Now(),
						},
					},
				},
			}.Check(ctx, t, db)

			// copy of delete marker should fail
			metabasetest.FinishCopyObject{
				Opts: metabase.FinishCopyObject{
					ObjectStream:          deletionResult.Markers[0].ObjectStream,
					NewBucket:             obj.BucketName,
					NewStreamID:           testrand.UUID(),
					NewEncryptedObjectKey: obj.ObjectKey,

					NewVersioned: false,
				},
				ErrClass: &metabase.ErrMethodNotAllowed,
				ErrText:  "copying delete marker is not allowed",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(deletionResult.Markers[0]),
					metabase.RawObject(unversionedObject),
					metabase.RawObject(versionedObject),
				},
			}.Check(ctx, t, db)
		})

		t.Run("versioned delete marker targets unversioned and versioned", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()
			obj.Version = 12000
			unversionedObject := metabasetest.CreateObject(ctx, t, db, obj, 0)
			obj.Version = 13000
			versionedObject := metabasetest.CreateObjectVersioned(ctx, t, db, obj, 0)

			sourceStream := metabasetest.RandObjectStream()
			sourceStream.ProjectID = obj.ProjectID
			sourceStream.Version = 13001
			sourceObject := metabasetest.CreateObjectVersioned(ctx, t, db, sourceStream, 0)

			deletionResult := metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: sourceObject.Location(),
					Versioned:      true,
				},
				Result: metabase.DeleteObjectResult{
					Markers: []metabase.Object{
						{
							ObjectStream: metabase.ObjectStream{
								ProjectID:  sourceObject.ProjectID,
								BucketName: sourceObject.BucketName,
								ObjectKey:  sourceObject.ObjectKey,
								StreamID:   sourceStream.StreamID,
							},
							Status:    metabase.DeleteMarkerVersioned,
							CreatedAt: time.Now(),
						},
					},
				},
			}.Check(ctx, t, db)

			// copy of delete marker should fail
			metabasetest.FinishCopyObject{
				Opts: metabase.FinishCopyObject{
					ObjectStream:          deletionResult.Markers[0].ObjectStream,
					NewBucket:             obj.BucketName,
					NewStreamID:           testrand.UUID(),
					NewEncryptedObjectKey: obj.ObjectKey,

					NewVersioned: true,
				},
				ErrClass: &metabase.ErrMethodNotAllowed,
				ErrText:  "copying delete marker is not allowed",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(deletionResult.Markers[0]),
					metabase.RawObject(sourceObject),
					metabase.RawObject(unversionedObject),
					metabase.RawObject(versionedObject),
				},
			}.Check(ctx, t, db)
		})

		t.Run("copy object from versioned source object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj.Version = 1
			sourceObject, sourceSegments := metabasetest.CreateTestObject{}.Run(ctx, t, db, obj, 1)
			obj.Version = 2
			latestObject := metabasetest.CreateObjectVersioned(ctx, t, db, obj, 0)

			results, err := db.BeginCopyObject(ctx, metabase.BeginCopyObject{
				ObjectLocation: sourceObject.Location(),
				Version:        sourceObject.Version,
				SegmentLimit:   10,
			})
			require.NoError(t, err)

			expectedCopiedObject := metabase.Object{
				ObjectStream: metabase.ObjectStream{
					ProjectID:  sourceObject.ProjectID,
					BucketName: sourceObject.BucketName,
					ObjectKey:  metabase.ObjectKey("new key"),
					StreamID:   testrand.UUID(),
					Version:    0,
				},
				Status:       metabase.CommittedVersioned,
				SegmentCount: 1,

				CreatedAt:          time.Now(),
				TotalPlainSize:     sourceObject.TotalPlainSize,
				TotalEncryptedSize: sourceObject.TotalEncryptedSize,
				FixedSegmentSize:   sourceObject.FixedSegmentSize,
				Encryption:         sourceObject.Encryption,
			}

			expectedTargetSegment := sourceSegments[0]
			expectedTargetSegment.StreamID = expectedCopiedObject.StreamID
			expectedTargetSegment.EncryptedETag = nil

			copyObjectResult := metabasetest.FinishCopyObject{
				Opts: metabase.FinishCopyObject{
					ObjectStream:          sourceObject.ObjectStream,
					NewBucket:             sourceObject.BucketName,
					NewEncryptedObjectKey: metabase.ObjectKey("new key"),
					NewStreamID:           expectedCopiedObject.StreamID,
					NewVersioned:          true,

					NewSegmentKeys: results.EncryptedKeysNonces,
				},
				Result: expectedCopiedObject,
			}.Check(ctx, t, db)

			require.NotZero(t, copyObjectResult.Version)
			expectedCopiedObject.Version = copyObjectResult.Version

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(sourceObject),
					metabase.RawObject(latestObject),
					metabase.RawObject(expectedCopiedObject),
				},
				Segments: metabasetest.SegmentsToRaw(append(sourceSegments, expectedTargetSegment)),
			}.Check(ctx, t, db)
		})

		t.Run("copy with TTL (object) and object lock", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()
			nowPlusHour := now.Add(time.Hour)

			unversionedObject := metabasetest.CreateExpiredObject(ctx, t, db, metabasetest.RandObjectStream(), 0, nowPlusHour)
			errText := "Object Lock settings must not be placed on an object with an expiration date"

			finishCopyObject := metabase.FinishCopyObject{
				ObjectStream:          unversionedObject.ObjectStream,
				NewBucket:             unversionedObject.BucketName,
				NewEncryptedObjectKey: metabase.ObjectKey("new key"),
				NewStreamID:           testrand.UUID(),

				NewVersioned: true,
				Retention: metabase.Retention{
					Mode:        storj.ComplianceMode,
					RetainUntil: now,
				},
			}

			// retention and no legal hold
			metabasetest.FinishCopyObject{
				Opts:     finishCopyObject,
				ErrClass: &metabase.ErrObjectExpiration,
				ErrText:  errText,
			}.Check(ctx, t, db)

			// retention and legal hold
			finishCopyObject.LegalHold = true
			metabasetest.FinishCopyObject{
				Opts:     finishCopyObject,
				ErrClass: &metabase.ErrObjectExpiration,
				ErrText:  errText,
			}.Check(ctx, t, db)

			// no retention and legal hold
			finishCopyObject.Retention = metabase.Retention{}
			metabasetest.FinishCopyObject{
				Opts:     finishCopyObject,
				ErrClass: &metabase.ErrObjectExpiration,
				ErrText:  errText,
			}.Check(ctx, t, db)
		})

		t.Run("copy with TTL (segments) and object lock", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()
			nowPlusHour := now.Add(time.Hour)

			unversionedObject := metabasetest.CreateExpiredObject(ctx, t, db, metabasetest.RandObjectStream(), 12, nowPlusHour)
			errText := "Object Lock settings must not be placed on an object with segments having an expiration date"

			finishCopyObject := metabase.FinishCopyObject{
				ObjectStream:          unversionedObject.ObjectStream,
				NewBucket:             unversionedObject.BucketName,
				NewEncryptedObjectKey: metabase.ObjectKey("new key"),
				NewStreamID:           testrand.UUID(),

				NewSegmentKeys: make([]metabase.EncryptedKeyAndNonce, 12),

				NewVersioned: true,

				Retention: metabase.Retention{
					Mode:        storj.ComplianceMode,
					RetainUntil: now,
				},
			}

			// retention and no legal hold
			metabasetest.FinishCopyObject{
				Opts:     finishCopyObject,
				ErrClass: &metabase.ErrObjectExpiration,
				ErrText:  errText,
			}.Check(ctx, t, db)

			// retention and legal hold
			finishCopyObject.LegalHold = true
			metabasetest.FinishCopyObject{
				Opts:     finishCopyObject,
				ErrClass: &metabase.ErrObjectExpiration,
				ErrText:  errText,
			}.Check(ctx, t, db)

			// no retention and legal hold
			finishCopyObject.Retention = metabase.Retention{}
			metabasetest.FinishCopyObject{
				Opts:     finishCopyObject,
				ErrClass: &metabase.ErrObjectExpiration,
				ErrText:  errText,
			}.Check(ctx, t, db)
		})

		t.Run("copy unversioned without version and with object lock", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			unversionedObject := metabasetest.CreateObject(ctx, t, db, metabasetest.RandObjectStream(), 0)
			errText := "Object Lock settings must not be placed on unversioned objects"

			finishCopyObject := metabase.FinishCopyObject{
				ObjectStream:          unversionedObject.ObjectStream,
				NewBucket:             unversionedObject.BucketName,
				NewEncryptedObjectKey: metabase.ObjectKey("new key"),
				NewStreamID:           testrand.UUID(),

				NewVersioned: false,

				Retention: metabase.Retention{
					Mode:        storj.ComplianceMode,
					RetainUntil: time.Date(1912, time.April, 15, 0, 0, 0, 0, time.UTC),
				},
			}

			// retention and no legal hold
			metabasetest.FinishCopyObject{
				Opts:     finishCopyObject,
				ErrClass: &metabase.ErrObjectStatus,
				ErrText:  errText,
			}.Check(ctx, t, db)

			// retention and legal hold
			finishCopyObject.LegalHold = true
			metabasetest.FinishCopyObject{
				Opts:     finishCopyObject,
				ErrClass: &metabase.ErrObjectStatus,
				ErrText:  errText,
			}.Check(ctx, t, db)

			// no retention and legal hold
			finishCopyObject.Retention = metabase.Retention{}
			metabasetest.FinishCopyObject{
				Opts:     finishCopyObject,
				ErrClass: &metabase.ErrObjectStatus,
				ErrText:  errText,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(unversionedObject),
				},
			}.Check(ctx, t, db)
		})

		t.Run("copy versioned without version and with object lock", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()
			obj.Version = metabase.DefaultVersion
			obj1 := metabasetest.CreateObject(ctx, t, db, obj, 0)
			obj.Version = 2
			obj2 := metabasetest.CreateObjectVersioned(ctx, t, db, obj, 0)
			obj.Version = 3
			obj3 := metabasetest.CreateObjectVersioned(ctx, t, db, obj, 0)

			errText := "Object Lock settings must not be placed on unversioned objects"

			finishCopyObject := metabase.FinishCopyObject{
				ObjectStream:          obj2.ObjectStream,
				NewBucket:             obj2.BucketName,
				NewEncryptedObjectKey: metabase.ObjectKey("new key"),
				NewStreamID:           testrand.UUID(),

				NewVersioned: false,

				Retention: metabase.Retention{
					Mode:        storj.ComplianceMode,
					RetainUntil: time.Date(1912, time.April, 15, 0, 0, 0, 0, time.UTC),
				},
			}

			// retention and no legal hold
			metabasetest.FinishCopyObject{
				Opts:     finishCopyObject,
				ErrClass: &metabase.ErrObjectStatus,
				ErrText:  errText,
			}.Check(ctx, t, db)

			// retention and legal hold
			finishCopyObject.LegalHold = true
			metabasetest.FinishCopyObject{
				Opts:     finishCopyObject,
				ErrClass: &metabase.ErrObjectStatus,
				ErrText:  errText,
			}.Check(ctx, t, db)

			// no retention and legal hold
			finishCopyObject.Retention = metabase.Retention{}
			metabasetest.FinishCopyObject{
				Opts:     finishCopyObject,
				ErrClass: &metabase.ErrObjectStatus,
				ErrText:  errText,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(obj1),
					metabase.RawObject(obj2),
					metabase.RawObject(obj3),
				},
			}.Check(ctx, t, db)
		})

		t.Run("copy unversioned with version and with object lock", func(t *testing.T) {
			test := func(t *testing.T, expectedRetention metabase.Retention, legalHold bool) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				unversionedObject := metabasetest.CreateObject(ctx, t, db, metabasetest.RandObjectStream(), 0)

				copyObject, _, _ := metabasetest.CreateObjectCopy{
					OriginalObject: unversionedObject,
					NewVersioned:   true,
					Retention:      expectedRetention,
					LegalHold:      legalHold,
				}.Run(ctx, t, db)

				require.Equal(t, unversionedObject.ProjectID, copyObject.ProjectID)
				require.NotZero(t, copyObject.Version)
				require.Equal(t, unversionedObject.ExpiresAt, copyObject.ExpiresAt)
				require.Equal(t, metabase.CommittedVersioned, copyObject.Status)
				require.Equal(t, unversionedObject.SegmentCount, copyObject.SegmentCount)
				require.Equal(t, unversionedObject.EncryptedMetadata, copyObject.EncryptedMetadata)
				require.Equal(t, unversionedObject.TotalPlainSize, copyObject.TotalPlainSize)
				require.Equal(t, unversionedObject.TotalEncryptedSize, copyObject.TotalEncryptedSize)
				require.Equal(t, unversionedObject.FixedSegmentSize, copyObject.FixedSegmentSize)
				require.Equal(t, unversionedObject.Encryption, copyObject.Encryption)
				require.Equal(t, unversionedObject.ZombieDeletionDeadline, copyObject.ZombieDeletionDeadline)
				require.Equal(t, expectedRetention, copyObject.Retention)
				require.Equal(t, legalHold, copyObject.LegalHold)

				metabasetest.Verify{
					Objects: []metabase.RawObject{
						metabase.RawObject(unversionedObject),
						metabase.RawObject(copyObject),
					},
				}.Check(ctx, t, db)
			}

			// retention and no legal hold
			test(t, metabase.Retention{
				Mode:        storj.ComplianceMode,
				RetainUntil: time.Date(1912, time.April, 15, 0, 0, 0, 0, time.UTC),
			}, false)

			// retention and legal hold
			test(t, metabase.Retention{
				Mode:        storj.ComplianceMode,
				RetainUntil: time.Date(1912, time.April, 15, 0, 0, 0, 0, time.UTC),
			}, true)

			// no retention and legal hold
			test(t, metabase.Retention{}, true)
		})

		t.Run("copy versioned with version and with object lock", func(t *testing.T) {
			test := func(t *testing.T, expectedRetention metabase.Retention, legalHold bool) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				obj := metabasetest.RandObjectStream()
				obj.Version = metabase.DefaultVersion
				obj1 := metabasetest.CreateObject(ctx, t, db, obj, 0)
				obj.Version = 2
				obj2 := metabasetest.CreateObjectVersioned(ctx, t, db, obj, 0)
				obj.Version = 3
				obj3 := metabasetest.CreateObjectVersioned(ctx, t, db, obj, 0)

				copyObject, _, _ := metabasetest.CreateObjectCopy{
					OriginalObject: obj2,
					NewVersioned:   true,
					Retention:      expectedRetention,
					LegalHold:      legalHold,
				}.Run(ctx, t, db)

				require.Equal(t, obj2.ProjectID, copyObject.ProjectID)
				require.NotZero(t, copyObject.Version)
				require.Equal(t, obj2.ExpiresAt, copyObject.ExpiresAt)
				require.Equal(t, obj2.Status, copyObject.Status)
				require.Equal(t, obj2.SegmentCount, copyObject.SegmentCount)
				require.Equal(t, obj2.EncryptedMetadata, copyObject.EncryptedMetadata)
				require.Equal(t, obj2.TotalPlainSize, copyObject.TotalPlainSize)
				require.Equal(t, obj2.TotalEncryptedSize, copyObject.TotalEncryptedSize)
				require.Equal(t, obj2.FixedSegmentSize, copyObject.FixedSegmentSize)
				require.Equal(t, obj2.Encryption, copyObject.Encryption)
				require.Equal(t, obj2.ZombieDeletionDeadline, copyObject.ZombieDeletionDeadline)
				require.Equal(t, expectedRetention, copyObject.Retention)
				require.Equal(t, legalHold, copyObject.LegalHold)

				metabasetest.Verify{
					Objects: []metabase.RawObject{
						metabase.RawObject(obj1),
						metabase.RawObject(obj2),
						metabase.RawObject(obj3),
						metabase.RawObject(copyObject),
					},
				}.Check(ctx, t, db)
			}

			// retention and no legal hold
			test(t, metabase.Retention{
				Mode:        storj.ComplianceMode,
				RetainUntil: time.Date(1912, time.April, 15, 0, 0, 0, 0, time.UTC),
			}, false)

			// retention and legal hold
			test(t, metabase.Retention{
				Mode:        storj.ComplianceMode,
				RetainUntil: time.Date(1912, time.April, 15, 0, 0, 0, 0, time.UTC),
			}, true)

			// no retention and legal hold
			test(t, metabase.Retention{}, true)
		})
	}, metabasetest.WithTimestampVersioning)
}
