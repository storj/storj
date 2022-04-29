// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestBeginMoveObject(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		for _, test := range metabasetest.InvalidObjectLocations(obj.Location()) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)
				metabasetest.BeginMoveObject{
					Opts: metabase.BeginMoveObject{
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

			metabasetest.BeginMoveObject{
				Opts: metabase.BeginMoveObject{
					ObjectLocation: obj.Location(),
					Version:        0,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Version invalid: 0",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("begin move object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			expectedMetadataNonce := testrand.Nonce()
			expectedMetadataKey := testrand.Bytes(265)
			expectedObject, _ := metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:                  obj,
					OverrideEncryptedMetadata:     true,
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

			metabasetest.BeginMoveObject{
				Opts: metabase.BeginMoveObject{
					Version:        expectedObject.Version,
					ObjectLocation: obj.Location(),
				},
				Result: metabase.BeginMoveObjectResult{
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

func TestFinishMoveObject(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()
		newBucketName := "New bucket name"

		for _, test := range metabasetest.InvalidObjectStreams(obj) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)
				metabasetest.FinishMoveObject{
					Opts: metabase.FinishMoveObject{
						NewBucket:    newBucketName,
						ObjectStream: test.ObjectStream,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)

				metabasetest.Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("invalid NewBucket", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.FinishMoveObject{
				Opts: metabase.FinishMoveObject{
					ObjectStream:                 obj,
					NewEncryptedObjectKey:        []byte{1, 2, 3},
					NewEncryptedMetadataKey:      []byte{1, 2, 3},
					NewEncryptedMetadataKeyNonce: testrand.Nonce(),
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "NewBucket is missing",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("invalid NewEncryptedObjectKey", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.FinishMoveObject{
				Opts: metabase.FinishMoveObject{
					NewBucket:    newBucketName,
					ObjectStream: obj,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "NewEncryptedObjectKey is missing",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("invalid EncryptedMetadataKeyNonce", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.FinishMoveObject{
				Opts: metabase.FinishMoveObject{
					NewBucket:               newBucketName,
					ObjectStream:            obj,
					NewEncryptedObjectKey:   []byte{0},
					NewEncryptedMetadataKey: []byte{0},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedMetadataKeyNonce is missing",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("invalid EncryptedMetadataKey", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.FinishMoveObject{
				Opts: metabase.FinishMoveObject{
					NewBucket:                    newBucketName,
					ObjectStream:                 obj,
					NewEncryptedObjectKey:        []byte{0},
					NewEncryptedMetadataKeyNonce: testrand.Nonce(),
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedMetadataKey is missing",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("empty EncryptedMetadataKey and EncryptedMetadataKeyNonce", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.FinishMoveObject{
				Opts: metabase.FinishMoveObject{
					NewBucket:             newBucketName,
					ObjectStream:          obj,
					NewEncryptedObjectKey: []byte{0},
				},
				// validation pass without EncryptedMetadataKey and EncryptedMetadataKeyNonce
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "object not found",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("object already exists", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			moveObjStream := metabasetest.RandObjectStream()
			metabasetest.CreateObject(ctx, t, db, moveObjStream, 0)

			conflictObjStream := metabasetest.RandObjectStream()
			conflictObjStream.ProjectID = moveObjStream.ProjectID
			metabasetest.CreateObject(ctx, t, db, conflictObjStream, 0)

			metabasetest.FinishMoveObject{
				Opts: metabase.FinishMoveObject{
					NewBucket:                    conflictObjStream.BucketName,
					ObjectStream:                 moveObjStream,
					NewEncryptedObjectKey:        []byte(conflictObjStream.ObjectKey),
					NewEncryptedMetadataKeyNonce: testrand.Nonce(),
					NewEncryptedMetadataKey:      testrand.Bytes(265),
				},
				ErrClass: &metabase.ErrObjectAlreadyExists,
			}.Check(ctx, t, db)
		})

		t.Run("object does not exist", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			newObj := metabasetest.RandObjectStream()

			newEncryptedMetadataKeyNonce := testrand.Nonce()
			newEncryptedMetadataKey := testrand.Bytes(32)
			newEncryptedKeysNonces := make([]metabase.EncryptedKeyAndNonce, 10)
			newObjectKey := testrand.Bytes(32)

			metabasetest.FinishMoveObject{
				Opts: metabase.FinishMoveObject{
					NewBucket:                    newBucketName,
					ObjectStream:                 newObj,
					NewSegmentKeys:               newEncryptedKeysNonces,
					NewEncryptedObjectKey:        newObjectKey,
					NewEncryptedMetadataKeyNonce: newEncryptedMetadataKeyNonce,
					NewEncryptedMetadataKey:      newEncryptedMetadataKey,
				},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "object not found",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("less amount of segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			numberOfSegments := 10
			newObjectKey := testrand.Bytes(32)

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
			expectedEncryptedSize := 1060
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
				expectedSegments[i].EncryptedSize = int32(expectedEncryptedSize)
			}

			metabasetest.FinishMoveObject{
				Opts: metabase.FinishMoveObject{
					NewBucket:                    newBucketName,
					ObjectStream:                 obj,
					NewSegmentKeys:               newEncryptedKeysNonces,
					NewEncryptedObjectKey:        newObjectKey,
					NewEncryptedMetadataKeyNonce: newEncryptedMetadataKeyNonce,
					NewEncryptedMetadataKey:      newEncryptedMetadataKey,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "wrong amount of segments keys received",
			}.Check(ctx, t, db)
		})

		t.Run("wrong segment indexes", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			numberOfSegments := 10
			newObjectKey := testrand.Bytes(32)

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

			metabasetest.FinishMoveObject{
				Opts: metabase.FinishMoveObject{
					NewBucket:                    newBucketName,
					ObjectStream:                 obj,
					NewSegmentKeys:               newEncryptedKeysNonces,
					NewEncryptedObjectKey:        newObjectKey,
					NewEncryptedMetadataKeyNonce: newEncryptedMetadataKeyNonce,
					NewEncryptedMetadataKey:      newEncryptedMetadataKey,
				},
				ErrClass: &metabase.Error,
				ErrText:  "segment is missing",
			}.Check(ctx, t, db)
		})

		t.Run("finish move object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			numberOfSegments := 10
			newObjectKey := testrand.Bytes(32)

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
					Position:          metabase.SegmentPosition{Index: uint32(i)},
					EncryptedKeyNonce: testrand.Nonce().Bytes(),
					EncryptedKey:      testrand.Bytes(32),
				}

				expectedSegments[i] = metabasetest.DefaultRawSegment(newObj.ObjectStream, metabase.SegmentPosition{Index: uint32(i)})
				expectedSegments[i].EncryptedKeyNonce = newEncryptedKeysNonces[i].EncryptedKeyNonce
				expectedSegments[i].EncryptedKey = newEncryptedKeysNonces[i].EncryptedKey
				// TODO: place this calculation in metabasetest.
				expectedSegments[i].PlainOffset = int64(int32(i) * expectedSegments[i].PlainSize)
				// TODO: we should use the same value for encrypted size in both test methods.
				expectedSegments[i].EncryptedSize = int32(expectedEncryptedSize)
			}

			metabasetest.FinishMoveObject{
				Opts: metabase.FinishMoveObject{
					NewBucket:                    newBucketName,
					ObjectStream:                 obj,
					NewSegmentKeys:               newEncryptedKeysNonces,
					NewEncryptedObjectKey:        newObjectKey,
					NewEncryptedMetadataKeyNonce: newEncryptedMetadataKeyNonce,
					NewEncryptedMetadataKey:      newEncryptedMetadataKey,
				},
				ErrText: "",
			}.Check(ctx, t, db)

			newObj.ObjectKey = metabase.ObjectKey(newObjectKey)
			newObj.EncryptedMetadataEncryptedKey = newEncryptedMetadataKey
			newObj.EncryptedMetadataNonce = newEncryptedMetadataKeyNonce[:]
			newObj.BucketName = newBucketName

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(newObj),
				},
				Segments: expectedSegments,
			}.Check(ctx, t, db)
		})
	})
}
