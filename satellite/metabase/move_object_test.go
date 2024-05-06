// Copyright (C) 2021 Storj Labs, Inc.
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

		t.Run("begin move object", func(t *testing.T) {
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

				metabasetest.BeginMoveObject{
					Opts: metabase.BeginMoveObject{
						ObjectLocation: obj.Location(),
					},
					Result: metabase.BeginMoveObjectResult{
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
	}, metabasetest.WithSpanner())
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
					NewEncryptedObjectKey:        metabase.ObjectKey("\x01\x02\x03"),
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

			numberOfSegments := 10
			newObjectKey := metabasetest.RandObjectKey()
			newEncryptedMetadataKey := testrand.Bytes(32)

			newObj, segments := metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:                  obj,
					OverrideEncryptedMetadata:     true,
					EncryptedMetadata:             testrand.Bytes(64),
					EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
					EncryptedMetadataEncryptedKey: testrand.Bytes(265),
				},
			}.Run(ctx, t, db, obj, byte(numberOfSegments))

			newEncryptedKeysNonces := make([]metabase.EncryptedKeyAndNonce, newObj.SegmentCount)

			metabasetest.FinishMoveObject{
				Opts: metabase.FinishMoveObject{
					NewBucket:                    newBucketName,
					ObjectStream:                 obj,
					NewSegmentKeys:               newEncryptedKeysNonces,
					NewEncryptedObjectKey:        newObjectKey,
					NewEncryptedMetadataKeyNonce: storj.Nonce{},
					NewEncryptedMetadataKey:      newEncryptedMetadataKey,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedMetadataKeyNonce is missing",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(newObj),
				},
				Segments: metabasetest.SegmentsToRaw(segments),
			}.Check(ctx, t, db)
		})

		t.Run("invalid EncryptedMetadataKey", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			numberOfSegments := 10
			newObjectKey := metabasetest.RandObjectKey()

			newObj, segments := metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:                  obj,
					OverrideEncryptedMetadata:     true,
					EncryptedMetadata:             testrand.Bytes(64),
					EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
					EncryptedMetadataEncryptedKey: testrand.Bytes(265),
				},
			}.Run(ctx, t, db, obj, byte(numberOfSegments))

			newEncryptedKeysNonces := make([]metabase.EncryptedKeyAndNonce, newObj.SegmentCount)
			newEncryptedMetadataKeyNonce := testrand.Nonce()

			metabasetest.FinishMoveObject{
				Opts: metabase.FinishMoveObject{
					NewBucket:                    newBucketName,
					ObjectStream:                 obj,
					NewSegmentKeys:               newEncryptedKeysNonces,
					NewEncryptedObjectKey:        newObjectKey,
					NewEncryptedMetadataKeyNonce: newEncryptedMetadataKeyNonce,
					NewEncryptedMetadataKey:      nil,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "EncryptedMetadataKey is missing",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(newObj),
				},
				Segments: metabasetest.SegmentsToRaw(segments),
			}.Check(ctx, t, db)
		})

		t.Run("empty EncryptedMetadataKey and EncryptedMetadataKeyNonce", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.FinishMoveObject{
				Opts: metabase.FinishMoveObject{
					NewBucket:             newBucketName,
					ObjectStream:          obj,
					NewEncryptedObjectKey: metabase.ObjectKey("\x00"),
				},
				// validation pass without EncryptedMetadataKey and EncryptedMetadataKeyNonce
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "object not found",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("existing object is overwritten", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			moveObjStream := metabasetest.RandObjectStream()
			initialObject := metabasetest.CreateObject(ctx, t, db, moveObjStream, 0)

			conflictObjStream := metabasetest.RandObjectStream()
			conflictObjStream.ProjectID = moveObjStream.ProjectID
			metabasetest.CreateObject(ctx, t, db, conflictObjStream, 0)

			newNonce := testrand.Nonce()
			newMetadataKey := testrand.Bytes(265)

			now := time.Now()
			metabasetest.FinishMoveObject{
				Opts: metabase.FinishMoveObject{
					NewBucket:                    conflictObjStream.BucketName,
					ObjectStream:                 moveObjStream,
					NewEncryptedObjectKey:        conflictObjStream.ObjectKey,
					NewEncryptedMetadataKeyNonce: newNonce,
					NewEncryptedMetadataKey:      newMetadataKey,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: metabase.ObjectStream{
							ProjectID:  conflictObjStream.ProjectID,
							BucketName: conflictObjStream.BucketName,
							ObjectKey:  conflictObjStream.ObjectKey,
							StreamID:   initialObject.StreamID,
							Version:    conflictObjStream.Version + 1,
						},
						CreatedAt:  now,
						Status:     metabase.CommittedUnversioned,
						Encryption: initialObject.Encryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("existing object is not overwritten, permission denied", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			moveObjStream := metabasetest.RandObjectStream()
			initialObject := metabasetest.CreateObject(ctx, t, db, moveObjStream, 0)

			conflictObjStream := metabasetest.RandObjectStream()
			conflictObjStream.ProjectID = moveObjStream.ProjectID
			conflictObject := metabasetest.CreateObject(ctx, t, db, conflictObjStream, 0)

			newNonce := testrand.Nonce()
			newMetadataKey := testrand.Bytes(265)

			metabasetest.FinishMoveObject{
				Opts: metabase.FinishMoveObject{
					NewBucket:                    conflictObjStream.BucketName,
					ObjectStream:                 moveObjStream,
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

		t.Run("object does not exist", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			newObj := metabasetest.RandObjectStream()

			newEncryptedMetadataKeyNonce := testrand.Nonce()
			newEncryptedMetadataKey := testrand.Bytes(32)
			newEncryptedKeysNonces := make([]metabase.EncryptedKeyAndNonce, 10)
			newObjectKey := metabasetest.RandObjectKey()

			metabasetest.FinishMoveObject{
				Opts: metabase.FinishMoveObject{
					NewBucket:                    newBucketName,
					ObjectStream:                 newObj,
					NewSegmentKeys:               newEncryptedKeysNonces,
					NewEncryptedObjectKey:        newObjectKey,
					NewEncryptedMetadataKeyNonce: newEncryptedMetadataKeyNonce,
					NewEncryptedMetadataKey:      newEncryptedMetadataKey,
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "object not found",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("not enough segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			numberOfSegments := 10
			newObjectKey := metabasetest.RandObjectKey()

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
				ErrText:  "wrong number of segments keys received",
			}.Check(ctx, t, db)
		})

		t.Run("wrong segment indexes", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			numberOfSegments := 10
			newObjectKey := metabasetest.RandObjectKey()

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

		// Assert that an error occurs when  a new object is put at the key (different stream_id)
		// between BeginMoveObject and FinishMoveObject,
		t.Run("source object changed", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			newObj, newSegments := metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:                  obj,
					OverrideEncryptedMetadata:     true,
					EncryptedMetadata:             testrand.Bytes(64),
					EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
					EncryptedMetadataEncryptedKey: testrand.Bytes(265),
				},
			}.Run(ctx, t, db, obj, 2)

			metabasetest.FinishMoveObject{
				Opts: metabase.FinishMoveObject{
					NewBucket: newBucketName,
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
				ErrText:  "object was changed during move",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects:  []metabase.RawObject{metabase.RawObject(newObj)},
				Segments: metabasetest.SegmentsToRaw(newSegments),
			}.Check(ctx, t, db)
		})

		t.Run("finish move object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			expectedRawObjects := []metabase.RawObject{}
			expectedRawSegments := []metabase.RawSegment{}

			for _, expectedVersion := range []metabase.Version{1, 2, 3, 11} {
				obj := metabasetest.RandObjectStream()
				obj.Version = expectedVersion
				object, segments := metabasetest.CreateTestObject{
					CommitObject: &metabase.CommitObject{
						ObjectStream: obj,
					},
				}.Run(ctx, t, db, obj, 10)

				newEncryptedKeysNonces := make([]metabase.EncryptedKeyAndNonce, object.SegmentCount)

				for i, segment := range segments {

					newEncryptedKeysNonces[i] = metabase.EncryptedKeyAndNonce{
						Position:          segment.Position,
						EncryptedKeyNonce: testrand.Nonce().Bytes(),
						EncryptedKey:      testrand.Bytes(32),
					}

					segment.EncryptedKey = newEncryptedKeysNonces[i].EncryptedKey
					segment.EncryptedKeyNonce = newEncryptedKeysNonces[i].EncryptedKeyNonce
					expectedRawSegments = append(expectedRawSegments, metabase.RawSegment(segment))
				}

				newObjectKey := metabasetest.RandObjectKey()
				metabasetest.FinishMoveObject{
					Opts: metabase.FinishMoveObject{
						NewBucket:             newBucketName,
						ObjectStream:          obj,
						NewSegmentKeys:        newEncryptedKeysNonces,
						NewEncryptedObjectKey: newObjectKey,
					},
					ErrText: "",
				}.Check(ctx, t, db)

				object.ObjectKey = newObjectKey
				object.BucketName = newBucketName
				object.Version = 1 // there are no overwritten object, hence it should start from 1

				expectedRawObjects = append(expectedRawObjects, metabase.RawObject(object))
			}

			metabasetest.Verify{
				Objects:  expectedRawObjects,
				Segments: expectedRawSegments,
			}.Check(ctx, t, db)
		})

		t.Run("finish move object with empty metadata, key, nonce object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			numberOfSegments := 10
			newObjectKey := metabasetest.RandObjectKey()

			newObj, _ := metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:              obj,
					OverrideEncryptedMetadata: true,
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
					EncryptedKeyNonce: nil,
					EncryptedKey:      nil,
				}

				expectedSegments[i] = metabasetest.DefaultRawSegment(newObj.ObjectStream, metabase.SegmentPosition{Index: uint32(i)})
				expectedSegments[i].EncryptedKeyNonce = nil
				expectedSegments[i].EncryptedKey = nil
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
				ErrText: "",
			}.Check(ctx, t, db)

			newObj.ObjectKey = newObjectKey
			newObj.BucketName = newBucketName
			newObj.Version = 1

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(newObj),
				},
				Segments: expectedSegments,
			}.Check(ctx, t, db)
		})

		t.Run("finish move object - different versions reject when NewDisallowDelete", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			committedTargetStreams := []metabase.ObjectStream{}
			obj := metabasetest.RandObjectStream()
			for _, version := range []metabase.Version{1, 2} {
				obj.Version = version
				object, _ := metabasetest.CreateTestObject{}.Run(ctx, t, db, obj, 1)
				committedTargetStreams = append(committedTargetStreams, object.ObjectStream)
			}

			sourceStream := metabasetest.RandObjectStream()
			sourceStream.ProjectID = obj.ProjectID
			_, _ = metabasetest.CreateTestObject{}.Run(ctx, t, db, sourceStream, 1)

			// it's not possible to move if under location were we have committed version
			for _, targetStream := range committedTargetStreams {
				metabasetest.FinishMoveObject{
					Opts: metabase.FinishMoveObject{
						ObjectStream:          sourceStream,
						NewBucket:             targetStream.BucketName,
						NewEncryptedObjectKey: targetStream.ObjectKey,

						NewDisallowDelete: true,
					},
					ErrClass: &metabase.ErrPermissionDenied,
				}.Check(ctx, t, db)
			}
		})

		t.Run("finish move object - target pending object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()

			metabasetest.CreatePendingObject(ctx, t, db, obj, 1)

			sourceStream := metabasetest.RandObjectStream()
			sourceStream.ProjectID = obj.ProjectID
			_, _ = metabasetest.CreateTestObject{}.Run(ctx, t, db, sourceStream, 0)

			// it's possible to move if under location were we have only pending version
			metabasetest.FinishMoveObject{
				Opts: metabase.FinishMoveObject{
					ObjectStream:          sourceStream,
					NewBucket:             obj.BucketName,
					NewEncryptedObjectKey: obj.ObjectKey,
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

			movedObject := sourceObject
			movedObject.ObjectStream.ProjectID = obj.ProjectID
			movedObject.ObjectStream.BucketName = obj.BucketName
			movedObject.ObjectStream.ObjectKey = obj.ObjectKey
			movedObject.ObjectStream.Version = 13001
			movedObject.Status = metabase.CommittedVersioned

			// versioned copy should leave everything else as is
			metabasetest.FinishMoveObject{
				Opts: metabase.FinishMoveObject{
					ObjectStream:          sourceStream,
					NewBucket:             obj.BucketName,
					NewEncryptedObjectKey: obj.ObjectKey,

					NewVersioned: true,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(unversionedObject),
					metabase.RawObject(versionedObject),
					metabase.RawObject(movedObject),
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

			movedObject := sourceObject
			movedObject.ObjectStream.ProjectID = obj.ProjectID
			movedObject.ObjectStream.BucketName = obj.BucketName
			movedObject.ObjectStream.ObjectKey = obj.ObjectKey
			movedObject.ObjectStream.Version = 13001
			movedObject.Status = metabase.CommittedUnversioned

			// unversioned copy should only delete the unversioned object
			metabasetest.FinishMoveObject{
				Opts: metabase.FinishMoveObject{
					ObjectStream:          sourceStream,
					NewBucket:             obj.BucketName,
					NewEncryptedObjectKey: obj.ObjectKey,
					NewVersioned:          false,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(versionedObject),
					metabase.RawObject(movedObject),
				},
			}.Check(ctx, t, db)
		})

		t.Run("unversioned delete marker targets unversioned and versioned", func(t *testing.T) {
			if _, ok := db.ChooseAdapter(uuid.UUID{}).(*metabase.SpannerAdapter); ok {
				t.Skip("not ready for spanner until DeleteObject is implemented")
			}
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
								Version:    12346,
							},
							Status:    metabase.DeleteMarkerUnversioned,
							CreatedAt: time.Now(),
						},
					},
				},
			}.Check(ctx, t, db)

			// move of delete marker should fail
			metabasetest.FinishMoveObject{
				Opts: metabase.FinishMoveObject{
					ObjectStream:          deletionResult.Markers[0].ObjectStream,
					NewBucket:             obj.BucketName,
					NewEncryptedObjectKey: obj.ObjectKey,

					NewVersioned: false,
				},
				ErrClass: &metabase.ErrMethodNotAllowed,
				ErrText:  "moving delete marker is not allowed",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(unversionedObject),
					metabase.RawObject(versionedObject),
					metabase.RawObject(deletionResult.Markers[0]),
				},
			}.Check(ctx, t, db)
		})

		t.Run("versioned delete marker targets unversioned and versioned", func(t *testing.T) {
			if _, ok := db.ChooseAdapter(uuid.UUID{}).(*metabase.SpannerAdapter); ok {
				t.Skip("not ready for spanner until DeleteObject is implemented")
			}
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
								Version:    13002,
							},
							Status:    metabase.DeleteMarkerVersioned,
							CreatedAt: time.Now(),
						},
					},
				},
			}.Check(ctx, t, db)

			// copy of delete marker should fail
			metabasetest.FinishMoveObject{
				Opts: metabase.FinishMoveObject{
					ObjectStream:          deletionResult.Markers[0].ObjectStream,
					NewBucket:             obj.BucketName,
					NewEncryptedObjectKey: obj.ObjectKey,

					NewVersioned: true,
				},
				ErrClass: &metabase.ErrMethodNotAllowed,
				ErrText:  "moving delete marker is not allowed",
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
	}, metabasetest.WithSpanner())
}
