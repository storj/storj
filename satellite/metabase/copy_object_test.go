// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

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
	})
}

func TestFinishCopyObject(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()
		newBucketName := "New bucket name"

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
					ObjectStream:                 obj,
					NewEncryptedObjectKey:        metabasetest.RandObjectKey(),
					NewEncryptedMetadataKey:      []byte{1, 2, 3},
					NewEncryptedMetadataKeyNonce: testrand.Nonce(),
					NewStreamID:                  newStreamID,
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
					ObjectStream:                 obj,
					NewBucket:                    newBucketName,
					NewEncryptedObjectKey:        metabasetest.RandObjectKey(),
					NewEncryptedMetadataKey:      []byte{1, 2, 3},
					NewEncryptedMetadataKeyNonce: testrand.Nonce(),
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

		t.Run("invalid EncryptedMetadataKeyNonce", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.FinishCopyObject{
				Opts: metabase.FinishCopyObject{
					NewBucket:               newBucketName,
					ObjectStream:            obj,
					NewEncryptedObjectKey:   metabasetest.RandObjectKey(),
					NewStreamID:             newStreamID,
					NewEncryptedMetadataKey: []byte{0},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedMetadataKeyNonce is missing",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("invalid EncryptedMetadataKey", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.FinishCopyObject{
				Opts: metabase.FinishCopyObject{
					NewBucket:                    newBucketName,
					ObjectStream:                 obj,
					NewEncryptedObjectKey:        metabasetest.RandObjectKey(),
					NewEncryptedMetadataKeyNonce: testrand.Nonce(),
					NewStreamID:                  newStreamID,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedMetadataKey is missing",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("empty EncryptedMetadataKey and EncryptedMetadataKeyNonce", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.FinishCopyObject{
				Opts: metabase.FinishCopyObject{
					NewBucket:             newBucketName,
					ObjectStream:          obj,
					NewEncryptedObjectKey: metabasetest.RandObjectKey(),
					NewStreamID:           newStreamID,
				},
				// validation pass without EncryptedMetadataKey and EncryptedMetadataKeyNonce
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

					OverrideMetadata:             true,
					NewEncryptedMetadataKey:      []byte{1},
					NewEncryptedMetadataKeyNonce: testrand.Nonce(),
					NewStreamID:                  newStreamID,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must be not set if EncryptedMetadata is not set",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("empty NewEncryptedMetadataKey and NewEncryptedMetadataKeyNonce with OverrideMetadata=true", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.FinishCopyObject{
				Opts: metabase.FinishCopyObject{
					NewBucket:             newBucketName,
					ObjectStream:          obj,
					NewEncryptedObjectKey: metabasetest.RandObjectKey(),
					NewStreamID:           newStreamID,

					OverrideMetadata:     true,
					NewEncryptedMetadata: testrand.BytesInt(256),
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedMetadataNonce and EncryptedMetadataEncryptedKey must be set if EncryptedMetadata is set",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("object does not exist", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			newObj := metabasetest.RandObjectStream()

			newEncryptedMetadataKeyNonce := testrand.Nonce()
			newEncryptedMetadataKey := testrand.Bytes(32)
			newEncryptedKeysNonces := make([]metabase.EncryptedKeyAndNonce, 10)

			metabasetest.FinishCopyObject{
				Opts: metabase.FinishCopyObject{
					NewBucket:                    newBucketName,
					NewStreamID:                  newStreamID,
					ObjectStream:                 newObj,
					NewSegmentKeys:               newEncryptedKeysNonces,
					NewEncryptedObjectKey:        metabasetest.RandObjectKey(),
					NewEncryptedMetadataKeyNonce: newEncryptedMetadataKeyNonce,
					NewEncryptedMetadataKey:      newEncryptedMetadataKey,
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
					ObjectStream:                  obj,
					OverrideEncryptedMetadata:     true,
					EncryptedMetadata:             testrand.Bytes(64),
					EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
					EncryptedMetadataEncryptedKey: testrand.Bytes(265),
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
					NewEncryptedObjectKey:        metabasetest.RandObjectKey(),
					NewEncryptedMetadataKeyNonce: testrand.Nonce(),
					NewEncryptedMetadataKey:      testrand.Bytes(32),
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
					ObjectStream:                  obj,
					EncryptedMetadata:             testrand.Bytes(64),
					EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
					EncryptedMetadataEncryptedKey: testrand.Bytes(265),
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
					NewBucket:                    newBucketName,
					ObjectStream:                 obj,
					NewStreamID:                  newStreamID,
					NewSegmentKeys:               newEncryptedKeysNonces,
					NewEncryptedObjectKey:        metabasetest.RandObjectKey(),
					NewEncryptedMetadataKeyNonce: newEncryptedMetadataKeyNonce,
					NewEncryptedMetadataKey:      newEncryptedMetadataKey,
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
					ObjectStream:                  obj,
					EncryptedMetadata:             testrand.Bytes(64),
					EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
					EncryptedMetadataEncryptedKey: testrand.Bytes(265),
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
					NewStreamID:                  newStreamID,
					NewBucket:                    newBucketName,
					ObjectStream:                 obj,
					NewSegmentKeys:               newEncryptedKeysNonces,
					NewEncryptedObjectKey:        metabasetest.RandObjectKey(),
					NewEncryptedMetadataKeyNonce: newEncryptedMetadataKeyNonce,
					NewEncryptedMetadataKey:      newEncryptedMetadataKey,
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
						ObjectStream:                  objStream,
						EncryptedMetadata:             testrand.Bytes(64),
						EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
						EncryptedMetadataEncryptedKey: testrand.Bytes(265),
					},
				}.Run(ctx, t, db, objStream, 0)

				metadataNonce := testrand.Nonce()
				expectedCopyObject := originalObj
				expectedCopyObject.ObjectKey = copyStream.ObjectKey
				expectedCopyObject.StreamID = copyStream.StreamID
				expectedCopyObject.Version = metabase.DefaultVersion // it will always copy into first available version
				expectedCopyObject.EncryptedMetadataEncryptedKey = testrand.Bytes(32)
				expectedCopyObject.EncryptedMetadataNonce = metadataNonce.Bytes()

				objectCopy := metabasetest.FinishCopyObject{
					Opts: metabase.FinishCopyObject{
						ObjectStream:                 objStream,
						NewBucket:                    copyStream.BucketName,
						NewStreamID:                  copyStream.StreamID,
						NewEncryptedObjectKey:        copyStream.ObjectKey,
						NewEncryptedMetadataKey:      expectedCopyObject.EncryptedMetadataEncryptedKey,
						NewEncryptedMetadataKeyNonce: metadataNonce,
					},
					Result: expectedCopyObject,
				}.Check(ctx, t, db)

				require.NotEqual(t, originalObj.CreatedAt, objectCopy.CreatedAt)

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
					ObjectStream:                  obj,
					EncryptedMetadata:             testrand.Bytes(64),
					EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
					EncryptedMetadataEncryptedKey: testrand.Bytes(265),
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

			originalMetadata := testrand.Bytes(64)
			originalMetadataNonce := testrand.Nonce().Bytes()
			originalMetadataEncryptedKey := testrand.Bytes(265)

			originalObj, _ := metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:                  obj,
					OverrideEncryptedMetadata:     true,
					EncryptedMetadata:             originalMetadata,
					EncryptedMetadataNonce:        originalMetadataNonce,
					EncryptedMetadataEncryptedKey: originalMetadataEncryptedKey,
				},
			}.Run(ctx, t, db, obj, 0)

			newMetadata := testrand.Bytes(256)
			newMetadataKey := testrand.Bytes(32)
			newMetadataKeyNonce := testrand.Nonce()

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

					OverrideMetadata:             false,
					NewEncryptedMetadata:         newMetadata,
					NewEncryptedMetadataKeyNonce: newMetadataKeyNonce,
					NewEncryptedMetadataKey:      newMetadataKey,
				},
			}.Run(ctx, t, db)

			require.Equal(t, originalMetadata, copyObjNoOverride.EncryptedMetadata)
			require.Equal(t, newMetadataKey, copyObjNoOverride.EncryptedMetadataEncryptedKey)
			require.Equal(t, newMetadataKeyNonce.Bytes(), copyObjNoOverride.EncryptedMetadataNonce)

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

					OverrideMetadata:             true,
					NewEncryptedMetadata:         newMetadata,
					NewEncryptedMetadataKeyNonce: newMetadataKeyNonce,
					NewEncryptedMetadataKey:      newMetadataKey,
				},
			}.Run(ctx, t, db)

			require.Equal(t, newMetadata, copyObj.EncryptedMetadata)
			require.Equal(t, newMetadataKey, copyObj.EncryptedMetadataEncryptedKey)
			require.Equal(t, newMetadataKeyNonce.Bytes(), copyObj.EncryptedMetadataNonce)

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
					ObjectStream:                  objStreamA,
					EncryptedMetadata:             testrand.Bytes(64),
					EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
					EncryptedMetadataEncryptedKey: testrand.Bytes(265),
				},
			}.Run(ctx, t, db, objStreamA, 4)

			objB, _ := metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:                  objStreamB,
					EncryptedMetadata:             testrand.Bytes(64),
					EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
					EncryptedMetadataEncryptedKey: testrand.Bytes(265),
				},
			}.Run(ctx, t, db, objStreamB, 3)

			objC, segmentsOfC := metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:                  objStreamC,
					EncryptedMetadata:             testrand.Bytes(64),
					EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
					EncryptedMetadataEncryptedKey: testrand.Bytes(265),
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
					ObjectStream:                  originalObjStream,
					EncryptedMetadata:             testrand.Bytes(64),
					EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
					EncryptedMetadataEncryptedKey: testrand.Bytes(265),
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
			metabasetest.CreateObjectCopy{
				OriginalObject:   copyObj,
				CopyObjectStream: &copyBackObjStream,
				FinishObject:     &opts,
			}.Run(ctx, t, db)

			// expected object at the location which was previously the original object
			copyBackObj := originalObj
			copyBackObj.Version = originalObj.Version + 1 // copy is placed into next version
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
					ObjectStream:                  originalObjStream,
					EncryptedMetadata:             testrand.Bytes(64),
					EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
					EncryptedMetadataEncryptedKey: testrand.Bytes(265),
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
			metabasetest.CreateObjectCopy{
				OriginalObject:   originalObj,
				CopyObjectStream: &copyBackObjStream,
				FinishObject:     &opts,
			}.Run(ctx, t, db)

			copyBackObj := originalObj
			copyBackObj.Version = originalObj.Version + 1 // copy is placed into next version
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
			expectedCopyObject.Version = 1 // it'll assign the next available version
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
					ObjectStream:                 objStream,
					NewBucket:                    copyStream.BucketName,
					NewStreamID:                  copyStream.StreamID,
					NewEncryptedObjectKey:        copyStream.ObjectKey,
					NewEncryptedMetadataKey:      expectedCopyObject.EncryptedMetadataEncryptedKey,
					NewEncryptedMetadataKeyNonce: metadataNonce,
					NewSegmentKeys:               newEncryptedKeysNonces,
				},
				Result: expectedCopyObject,
			}.Check(ctx, t, db)

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
					ObjectStream:                  obj,
					EncryptedMetadata:             testrand.Bytes(64),
					EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
					EncryptedMetadataEncryptedKey: testrand.Bytes(265),
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
					ObjectStream:                  obj,
					EncryptedMetadata:             testrand.Bytes(64),
					EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
					EncryptedMetadataEncryptedKey: testrand.Bytes(265),
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

			now := time.Now()
			zombieDeadline := now.Add(24 * time.Hour)

			sourceObjStream := metabasetest.RandObjectStream()
			destinationObjStream := metabasetest.RandObjectStream()
			destinationObjStream.ProjectID = sourceObjStream.ProjectID
			// testcases:
			// - versions of pending objects
			// - version of committed object
			// - expected copy version

			testCases := []struct {
				Bucket                      string
				Key                         metabase.ObjectKey
				NewBucket                   string
				NewKey                      metabase.ObjectKey
				sourcePendingVersions       []metabase.Version
				sourceCommittedVersion      metabase.Version
				destinationPendingVersions  []metabase.Version
				destinationCommittedVersion metabase.Version
				expectedCopyVersion         metabase.Version
			}{
				// the same bucket
				{"testbucket", "object", "testbucket", "new-object",
					[]metabase.Version{}, 2,
					[]metabase.Version{}, 1,
					2},
				{"testbucket", "object", "testbucket", "new-object",
					[]metabase.Version{}, 1,
					[]metabase.Version{1}, 2,
					3},
				{"testbucket", "object", "testbucket", "new-object",
					[]metabase.Version{}, 1,
					[]metabase.Version{1, 3}, 2,
					4},
				{"testbucket", "object", "testbucket", "new-object",
					[]metabase.Version{1, 5}, 2,
					[]metabase.Version{1, 3}, 2,
					4},
				{"testbucket", "object", "newbucket", "object",
					[]metabase.Version{2, 3}, 1,
					[]metabase.Version{1, 5}, 2,
					6},
			}

			for _, tc := range testCases {
				metabasetest.DeleteAll{}.Check(ctx, t, db)
				sourceObjStream.BucketName = tc.Bucket
				sourceObjStream.ObjectKey = tc.Key
				destinationObjStream.BucketName = tc.NewBucket
				destinationObjStream.ObjectKey = tc.NewKey

				var rawObjects []metabase.RawObject
				for _, version := range tc.sourcePendingVersions {
					sourceObjStream.Version = version
					sourceObjStream.StreamID = testrand.UUID()
					metabasetest.CreatePendingObject(ctx, t, db, sourceObjStream, 0)

					rawObjects = append(rawObjects, metabase.RawObject{
						ObjectStream: sourceObjStream,
						CreatedAt:    now,
						Status:       metabase.Pending,

						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieDeadline,
					})
				}
				sourceObjStream.Version = tc.sourceCommittedVersion
				sourceObjStream.StreamID = testrand.UUID()
				sourceObj, _ := metabasetest.CreateTestObject{
					BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
						ObjectStream: sourceObjStream,
						Encryption:   metabasetest.DefaultEncryption,
					},
					CommitObject: &metabase.CommitObject{
						ObjectStream:                  sourceObjStream,
						OverrideEncryptedMetadata:     true,
						EncryptedMetadata:             testrand.Bytes(64),
						EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
						EncryptedMetadataEncryptedKey: testrand.Bytes(265),
					},
				}.Run(ctx, t, db, sourceObjStream, 0)

				rawObjects = append(rawObjects, metabase.RawObject(sourceObj))

				for _, version := range tc.destinationPendingVersions {
					destinationObjStream.Version = version
					destinationObjStream.StreamID = testrand.UUID()
					metabasetest.CreatePendingObject(ctx, t, db, destinationObjStream, 0)

					rawObjects = append(rawObjects, metabase.RawObject{
						ObjectStream: destinationObjStream,
						CreatedAt:    now,
						Status:       metabase.Pending,

						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieDeadline,
					})
				}

				if tc.destinationCommittedVersion != 0 {
					destinationObjStream.StreamID = testrand.UUID()
					destinationObjStream.Version = tc.destinationCommittedVersion
					_, _ = metabasetest.CreateTestObject{
						BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
							ObjectStream: destinationObjStream,
							Encryption:   metabasetest.DefaultEncryption,
						},
						CommitObject: &metabase.CommitObject{
							ObjectStream:                  destinationObjStream,
							OverrideEncryptedMetadata:     true,
							EncryptedMetadata:             testrand.Bytes(64),
							EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
							EncryptedMetadataEncryptedKey: testrand.Bytes(265),
						},
					}.Run(ctx, t, db, destinationObjStream, 0)
				}

				copyObj, expectedOriginalSegments, _ := metabasetest.CreateObjectCopy{
					OriginalObject:   sourceObj,
					CopyObjectStream: &destinationObjStream,
				}.Run(ctx, t, db)

				require.Equal(t, tc.expectedCopyVersion, copyObj.Version)

				rawObjects = append(rawObjects, metabase.RawObject(copyObj))

				metabasetest.Verify{
					Objects:  rawObjects,
					Segments: expectedOriginalSegments,
				}.Check(ctx, t, db)
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
					Version:    conflictObjStream.Version + 1,
				},
				CreatedAt:                     now,
				Status:                        metabase.CommittedUnversioned,
				Encryption:                    initialObject.Encryption,
				EncryptedMetadataNonce:        newNonce[:],
				EncryptedMetadataEncryptedKey: newMetadataKey,
			}

			metabasetest.FinishCopyObject{
				Opts: metabase.FinishCopyObject{
					NewBucket:                    conflictObjStream.BucketName,
					NewStreamID:                  newUUID,
					ObjectStream:                 initialStream,
					NewEncryptedObjectKey:        conflictObjStream.ObjectKey,
					NewEncryptedMetadataKeyNonce: newNonce,
					NewEncryptedMetadataKey:      newMetadataKey,
				},
				Result: copiedObject,
			}.Check(ctx, t, db)

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

					NewEncryptedObjectKey:        conflictObjStream.ObjectKey,
					NewEncryptedMetadataKeyNonce: newNonce,
					NewEncryptedMetadataKey:      newMetadataKey,

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
			copiedObject.ObjectStream.Version = 13001
			copiedObject.ObjectStream.StreamID = newStreamID
			copiedObject.Status = metabase.CommittedVersioned

			// versioned copy should leave everything else as is
			metabasetest.FinishCopyObject{
				Opts: metabase.FinishCopyObject{
					ObjectStream:          sourceStream,
					NewBucket:             obj.BucketName,
					NewStreamID:           newStreamID,
					NewEncryptedObjectKey: obj.ObjectKey,

					NewVersioned: true,
				},
				Result: copiedObject,
			}.Check(ctx, t, db)

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
			copiedObject.ObjectStream.Version = 13001
			copiedObject.ObjectStream.StreamID = newStreamID
			copiedObject.Status = metabase.CommittedUnversioned

			// unversioned copy should only delete the unversioned object
			metabasetest.FinishCopyObject{
				Opts: metabase.FinishCopyObject{
					ObjectStream:          sourceStream,
					NewBucket:             obj.BucketName,
					NewStreamID:           newStreamID,
					NewEncryptedObjectKey: obj.ObjectKey,
					NewVersioned:          false,
				},
				Result: copiedObject,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(versionedObject),
					metabase.RawObject(sourceObject),
					metabase.RawObject(copiedObject),
				},
			}.Check(ctx, t, db)
		})

	})
}
