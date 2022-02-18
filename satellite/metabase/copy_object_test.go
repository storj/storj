// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"

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

		t.Run("invalid version", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginCopyObject{
				Opts: metabase.BeginCopyObject{
					ObjectLocation: obj.Location(),
					Version:        0,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Version invalid: 0",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("begin copy object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			expectedMetadataNonce := testrand.Nonce()
			expectedMetadataKey := testrand.Bytes(265)
			expectedObject, _ := metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:                  obj,
					EncryptedMetadata:             testrand.Bytes(64),
					EncryptedMetadataNonce:        expectedMetadataNonce[:],
					EncryptedMetadataEncryptedKey: expectedMetadataKey,
				},
			}.Run(ctx, t, db, obj, 10)

			var encKeyAndNonces []metabase.EncryptedKeyAndNonce
			expectedRawSegments := make([]metabase.RawSegment, 10)
			for i := range expectedRawSegments {
				expectedRawSegments[i] = metabasetest.DefaultRawSegment(expectedObject.ObjectStream, metabase.SegmentPosition{
					Index: uint32(i),
				})
				expectedRawSegments[i].PlainOffset = int64(i) * int64(expectedRawSegments[i].PlainSize)
				expectedRawSegments[i].EncryptedSize = 1060

				encKeyAndNonces = append(encKeyAndNonces, metabase.EncryptedKeyAndNonce{
					EncryptedKeyNonce: expectedRawSegments[i].EncryptedKeyNonce,
					EncryptedKey:      expectedRawSegments[i].EncryptedKey,
					Position:          expectedRawSegments[i].Position,
				})
			}

			metabasetest.BeginCopyObject{
				Opts: metabase.BeginCopyObject{
					Version:        expectedObject.Version,
					ObjectLocation: obj.Location(),
				},
				Result: metabase.BeginCopyObjectResult{
					StreamID:                  expectedObject.StreamID,
					EncryptedMetadata:         expectedObject.EncryptedMetadata,
					EncryptedMetadataKey:      expectedMetadataKey,
					EncryptedMetadataKeyNonce: expectedMetadataNonce[:],
					EncryptedKeysNonces:       encKeyAndNonces,
					EncryptionParameters:      expectedObject.Encryption,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(expectedObject),
				},
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
					NewEncryptedMetadataKeyNonce: []byte{1, 2, 3},
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
					NewEncryptedMetadataKeyNonce: []byte{1, 2, 3},
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

		t.Run("copy to the same EncryptedObjectKey", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.FinishCopyObject{
				Opts: metabase.FinishCopyObject{
					NewBucket:             newBucketName,
					NewEncryptedObjectKey: obj.ObjectKey,
					ObjectStream:          obj,
					NewStreamID:           newStreamID,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "source and destination encrypted object key are identical",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("invalid EncryptedMetadataKeyNonce", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.FinishCopyObject{
				Opts: metabase.FinishCopyObject{
					NewBucket:             newBucketName,
					ObjectStream:          obj,
					NewEncryptedObjectKey: metabasetest.RandObjectKey(),
					NewStreamID:           newStreamID,
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
					NewEncryptedMetadataKeyNonce: []byte{0},
					NewStreamID:                  newStreamID,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedMetadataKey is missing",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
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
					NewEncryptedMetadataKeyNonce: []byte{1},
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
					NewEncryptedMetadataKeyNonce: newEncryptedMetadataKeyNonce.Bytes(),
					NewEncryptedMetadataKey:      newEncryptedMetadataKey,
				},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: sql: no rows in result set",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("less amount of segments", func(t *testing.T) {
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
					NewEncryptedMetadataKeyNonce: newEncryptedMetadataKeyNonce.Bytes(),
					NewEncryptedMetadataKey:      newEncryptedMetadataKey,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "wrong amount of segments keys received (received 10, need 9)",
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
					NewEncryptedMetadataKeyNonce: newEncryptedMetadataKeyNonce.Bytes(),
					NewEncryptedMetadataKey:      newEncryptedMetadataKey,
				},
				ErrClass: &metabase.Error,
				ErrText:  "missing new segment keys for segment 0",
			}.Check(ctx, t, db)
		})

		t.Run("returned object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			objStream := metabasetest.RandObjectStream()
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

			expectedCopyObject := originalObj
			expectedCopyObject.ObjectKey = copyStream.ObjectKey
			expectedCopyObject.StreamID = copyStream.StreamID
			expectedCopyObject.EncryptedMetadataEncryptedKey = testrand.Bytes(32)
			expectedCopyObject.EncryptedMetadataNonce = testrand.Nonce().Bytes()

			objectCopy := metabasetest.FinishCopyObject{
				Opts: metabase.FinishCopyObject{
					ObjectStream:                 objStream,
					NewBucket:                    copyStream.BucketName,
					NewStreamID:                  copyStream.StreamID,
					NewEncryptedObjectKey:        copyStream.ObjectKey,
					NewEncryptedMetadataKey:      expectedCopyObject.EncryptedMetadataEncryptedKey,
					NewEncryptedMetadataKeyNonce: expectedCopyObject.EncryptedMetadataNonce,
				},
				Result: expectedCopyObject,
			}.Check(ctx, t, db)

			require.NotEqual(t, originalObj.CreatedAt, objectCopy.CreatedAt)
			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(originalObj),
					metabase.RawObject(expectedCopyObject),
				},
				Copies: []metabase.RawCopy{
					{
						StreamID:         copyStream.StreamID,
						AncestorStreamID: objStream.StreamID,
					},
				},
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
				Copies: []metabase.RawCopy{{
					StreamID:         copyObj.StreamID,
					AncestorStreamID: originalObj.StreamID,
				}},
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
				Copies: []metabase.RawCopy{{
					StreamID:         copyStream.StreamID,
					AncestorStreamID: originalObj.StreamID,
				}, {
					StreamID:         copyOfCopyObj.StreamID,
					AncestorStreamID: originalObj.StreamID,
				}},
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
					EncryptedMetadata:             originalMetadata,
					EncryptedMetadataNonce:        originalMetadataNonce,
					EncryptedMetadataEncryptedKey: originalMetadataEncryptedKey,
				},
			}.Run(ctx, t, db, obj, 0)

			newMetadata := testrand.Bytes(256)
			newMetadataKey := testrand.Bytes(32)
			newMetadataKeyNonce := testrand.Nonce().Bytes()

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
			require.Equal(t, newMetadataKeyNonce, copyObjNoOverride.EncryptedMetadataNonce)

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
			require.Equal(t, newMetadataKeyNonce, copyObj.EncryptedMetadataNonce)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(originalObj),
					metabase.RawObject(copyObj),
					metabase.RawObject(copyObjNoOverride),
				},
				Copies: []metabase.RawCopy{
					{
						StreamID:         copyStream.StreamID,
						AncestorStreamID: originalObj.StreamID,
					},
					{
						StreamID:         copyObjNoOverride.StreamID,
						AncestorStreamID: originalObj.StreamID,
					},
				},
			}.Check(ctx, t, db)
		})
	})
}
