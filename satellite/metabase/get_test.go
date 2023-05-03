// Copyright (C) 2020 Storj Labs, Inc.
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

func TestGetObjectExactVersion(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		location := obj.Location()

		now := time.Now()
		zombieDeadline := now.Add(24 * time.Hour)

		for _, test := range metabasetest.InvalidObjectLocations(location) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)
				metabasetest.GetObjectExactVersion{
					Opts: metabase.GetObjectExactVersion{
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

			metabasetest.GetObjectExactVersion{
				Opts: metabase.GetObjectExactVersion{
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

			metabasetest.GetObjectExactVersion{
				Opts: metabase.GetObjectExactVersion{
					ObjectLocation: location,
					Version:        1,
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "metabase: sql: no rows in result set",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Get not existing version", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.CreateObject(ctx, t, db, obj, 0)

			metabasetest.GetObjectExactVersion{
				Opts: metabase.GetObjectExactVersion{
					ObjectLocation: location,
					Version:        11,
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "metabase: sql: no rows in result set",
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

		t.Run("Get pending object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,

					Encryption: metabasetest.DefaultEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			metabasetest.GetObjectExactVersion{
				Opts: metabase.GetObjectExactVersion{
					ObjectLocation: location,
					Version:        1,
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "metabase: sql: no rows in result set",
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

		t.Run("Get expired object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			expiresAt := now.Add(-2 * time.Hour)
			metabasetest.CreateExpiredObject(ctx, t, db, obj, 0, expiresAt)
			metabasetest.GetObjectExactVersion{
				Opts: metabase.GetObjectExactVersion{
					ObjectLocation: location,
					Version:        1,
				},
				ErrClass: &metabase.ErrObjectNotFound,
			}.Check(ctx, t, db)
			metabasetest.Verify{Objects: []metabase.RawObject{
				{
					ObjectStream: obj,
					CreatedAt:    now,
					Status:       metabase.Committed,
					ExpiresAt:    &expiresAt,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}}.Check(ctx, t, db)
		})

		t.Run("Get object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.CreateObject(ctx, t, db, obj, 0)

			metabasetest.GetObjectExactVersion{
				Opts: metabase.GetObjectExactVersion{
					ObjectLocation: location,
					Version:        1,
				},
				Result: metabase.Object{
					ObjectStream: obj,
					CreatedAt:    now,
					Status:       metabase.Committed,

					Encryption: metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: []metabase.RawObject{
				{
					ObjectStream: obj,
					CreatedAt:    now,
					Status:       metabase.Committed,

					Encryption: metabasetest.DefaultEncryption,
				},
			}}.Check(ctx, t, db)
		})
	})
}

func TestGetObjectLastCommitted(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()
		location := obj.Location()
		now := time.Now()
		zombieDeadline := now.Add(24 * time.Hour)

		for _, test := range metabasetest.InvalidObjectLocations(location) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)
				metabasetest.GetObjectLastCommitted{
					Opts: metabase.GetObjectLastCommitted{
						ObjectLocation: test.ObjectLocation,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)
				metabasetest.Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("Object missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			metabasetest.GetObjectLastCommitted{
				Opts: metabase.GetObjectLastCommitted{
					ObjectLocation: location,
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "metabase: sql: no rows in result set",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Get pending object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			metabasetest.GetObjectLastCommitted{
				Opts: metabase.GetObjectLastCommitted{
					ObjectLocation: location,
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "metabase: sql: no rows in result set",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream:           obj,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieDeadline,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("Get object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			encryptedMetadata := testrand.Bytes(1024)
			encryptedMetadataNonce := testrand.Nonce()
			encryptedMetadataKey := testrand.Bytes(265)

			metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:                  obj,
					EncryptedMetadataNonce:        encryptedMetadataNonce[:],
					EncryptedMetadata:             encryptedMetadata,
					EncryptedMetadataEncryptedKey: encryptedMetadataKey,
					OverrideEncryptedMetadata:     true,
				},
			}.Run(ctx, t, db, obj, 0)

			metabasetest.GetObjectLastCommitted{
				Opts: metabase.GetObjectLastCommitted{
					ObjectLocation: location,
				},
				Result: metabase.Object{
					ObjectStream:                  obj,
					CreatedAt:                     now,
					Status:                        metabase.Committed,
					Encryption:                    metabasetest.DefaultEncryption,
					EncryptedMetadataNonce:        encryptedMetadataNonce[:],
					EncryptedMetadata:             encryptedMetadata,
					EncryptedMetadataEncryptedKey: encryptedMetadataKey,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: []metabase.RawObject{
				{
					ObjectStream:                  obj,
					CreatedAt:                     now,
					Status:                        metabase.Committed,
					Encryption:                    metabasetest.DefaultEncryption,
					EncryptedMetadataNonce:        encryptedMetadataNonce[:],
					EncryptedMetadata:             encryptedMetadata,
					EncryptedMetadataEncryptedKey: encryptedMetadataKey,
				},
			}}.Check(ctx, t, db)
		})

		t.Run("Get object last committed version from multiple", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			firstObject := obj
			firstObject.Version = metabase.Version(1)
			metabasetest.CreateObject(ctx, t, db, firstObject, 0)

			secondObject, err := db.BeginObjectNextVersion(ctx, metabase.BeginObjectNextVersion{
				ObjectStream: metabase.ObjectStream{
					ProjectID:  firstObject.ProjectID,
					BucketName: firstObject.BucketName,
					ObjectKey:  firstObject.ObjectKey,
					Version:    metabase.NextVersion,
					StreamID:   testrand.UUID(),
				},
			})
			require.NoError(t, err)

			metabasetest.GetObjectLastCommitted{
				Opts: metabase.GetObjectLastCommitted{
					ObjectLocation: location,
				},
				Result: metabase.Object{
					ObjectStream: firstObject,
					CreatedAt:    now,
					Status:       metabase.Committed,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: []metabase.RawObject{
				{
					ObjectStream: firstObject,
					CreatedAt:    now,
					Status:       metabase.Committed,
					Encryption:   metabasetest.DefaultEncryption,
				},
				metabase.RawObject(secondObject),
			}}.Check(ctx, t, db)
		})

		t.Run("Get latest copied object version", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			copyObjStream := metabasetest.RandObjectStream()
			originalObject := metabasetest.CreateObject(ctx, t, db, obj, 0)

			copiedObj, _, _ := metabasetest.CreateObjectCopy{
				OriginalObject:   originalObject,
				CopyObjectStream: &copyObjStream,
			}.Run(ctx, t, db)

			metabasetest.DeleteObjectExactVersion{
				Opts: metabase.DeleteObjectExactVersion{
					Version:        1,
					ObjectLocation: obj.Location(),
				},
				Result: metabase.DeleteObjectResult{
					Objects: []metabase.Object{originalObject},
				},
			}.Check(ctx, t, db)

			metabasetest.GetObjectLastCommitted{
				Opts: metabase.GetObjectLastCommitted{
					ObjectLocation: copiedObj.Location(),
				},
				Result: copiedObj,
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: []metabase.RawObject{
				{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  copiedObj.ProjectID,
						BucketName: copiedObj.BucketName,
						ObjectKey:  copiedObj.ObjectKey,
						Version:    copiedObj.Version,
						StreamID:   copiedObj.StreamID,
					},
					CreatedAt:                     now,
					Status:                        metabase.Committed,
					Encryption:                    metabasetest.DefaultEncryption,
					EncryptedMetadata:             copiedObj.EncryptedMetadata,
					EncryptedMetadataNonce:        copiedObj.EncryptedMetadataNonce,
					EncryptedMetadataEncryptedKey: copiedObj.EncryptedMetadataEncryptedKey,
				},
			}}.Check(ctx, t, db)
		})
	})
}

func TestGetSegmentByPosition(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		now := time.Now()

		t.Run("StreamID missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.GetSegmentByPosition{
				Opts:     metabase.GetSegmentByPosition{},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "StreamID missing",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Segment missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.GetSegmentByPosition{
				Opts: metabase.GetSegmentByPosition{
					StreamID: obj.StreamID,
				},
				ErrClass: &metabase.ErrSegmentNotFound,
				ErrText:  "segment missing",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Get segment", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj1 := metabasetest.CreateObject(ctx, t, db, metabasetest.RandObjectStream(), 1)

			expectedExpiresAt := now.Add(5 * time.Hour)
			obj2 := metabasetest.CreateExpiredObject(ctx, t, db, metabasetest.RandObjectStream(), 1, expectedExpiresAt)

			segments := make([]metabase.Segment, 0, 2)
			for _, obj := range []metabase.Object{obj1, obj2} {
				expectedSegment := metabase.Segment{
					StreamID: obj.StreamID,
					Position: metabase.SegmentPosition{
						Index: 0,
					},
					CreatedAt:         obj.CreatedAt,
					ExpiresAt:         obj.ExpiresAt,
					RootPieceID:       storj.PieceID{1},
					EncryptedKey:      []byte{3},
					EncryptedKeyNonce: []byte{4},
					EncryptedETag:     []byte{5},
					EncryptedSize:     1024,
					PlainSize:         512,
					Pieces:            metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
					Redundancy:        metabasetest.DefaultRedundancy,
				}

				metabasetest.GetSegmentByPosition{
					Opts: metabase.GetSegmentByPosition{
						StreamID: obj.StreamID,
						Position: metabase.SegmentPosition{
							Index: 0,
						},
					},
					Result: expectedSegment,
				}.Check(ctx, t, db)

				segments = append(segments, expectedSegment)
			}

			// check non existing segment in existing object
			metabasetest.GetSegmentByPosition{
				Opts: metabase.GetSegmentByPosition{
					StreamID: obj.StreamID,
					Position: metabase.SegmentPosition{
						Index: 1,
					},
				},
				ErrClass: &metabase.ErrSegmentNotFound,
				ErrText:  "segment missing",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj1.ObjectStream,
						CreatedAt:    now,
						Status:       metabase.Committed,
						SegmentCount: 1,

						TotalPlainSize:     512,
						TotalEncryptedSize: 1024,
						FixedSegmentSize:   512,

						Encryption: metabasetest.DefaultEncryption,
					},
					{
						ObjectStream: obj2.ObjectStream,
						CreatedAt:    now,
						ExpiresAt:    obj2.ExpiresAt,
						Status:       metabase.Committed,
						SegmentCount: 1,

						TotalPlainSize:     512,
						TotalEncryptedSize: 1024,
						FixedSegmentSize:   512,

						Encryption: metabasetest.DefaultEncryption,
					},
				},
				Segments: []metabase.RawSegment{
					metabase.RawSegment(segments[0]),
					metabase.RawSegment(segments[1]),
				},
			}.Check(ctx, t, db)
		})

		t.Run("Get segment copy", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			objStream := metabasetest.RandObjectStream()
			copyObjStream := metabasetest.RandObjectStream()
			copyObjStream.ProjectID = objStream.ProjectID

			obj := metabasetest.CreateObject(ctx, t, db, objStream, 1)

			encryptedKeyNonces := []metabase.EncryptedKeyAndNonce{{
				EncryptedKeyNonce: []byte{4},
				EncryptedKey:      []byte{3},
				Position: metabase.SegmentPosition{
					Index: 0,
				},
			}}

			newEncryptedKeyNonces := []metabase.EncryptedKeyAndNonce{{
				EncryptedKeyNonce: []byte{3},
				EncryptedKey:      []byte{4},
				Position: metabase.SegmentPosition{
					Index: 0,
				},
			}}

			metabasetest.BeginCopyObject{
				Opts: metabase.BeginCopyObject{
					ObjectLocation: obj.Location(),
				},
				Result: metabase.BeginCopyObjectResult{
					StreamID:                  obj.StreamID,
					Version:                   obj.Version,
					EncryptedMetadata:         obj.EncryptedMetadata,
					EncryptedMetadataKey:      obj.EncryptedMetadataEncryptedKey,
					EncryptedMetadataKeyNonce: obj.EncryptedMetadataNonce,
					EncryptedKeysNonces:       encryptedKeyNonces,
					EncryptionParameters:      obj.Encryption,
				},
			}.Check(ctx, t, db)

			newEncryptedMetadataKeyNonce := testrand.Nonce()
			newEncryptedMetadataKey := testrand.Bytes(32)

			_, err := db.FinishCopyObject(ctx, metabase.FinishCopyObject{
				NewStreamID:                  copyObjStream.StreamID,
				NewBucket:                    copyObjStream.BucketName,
				ObjectStream:                 obj.ObjectStream,
				NewSegmentKeys:               newEncryptedKeyNonces,
				NewEncryptedObjectKey:        copyObjStream.ObjectKey,
				NewEncryptedMetadataKeyNonce: newEncryptedMetadataKeyNonce,
				NewEncryptedMetadataKey:      newEncryptedMetadataKey,
			})
			require.NoError(t, err)

			expectedSegment := metabase.Segment{
				StreamID: obj.StreamID,
				Position: metabase.SegmentPosition{
					Index: 0,
				},
				CreatedAt:         obj.CreatedAt,
				ExpiresAt:         obj.ExpiresAt,
				RootPieceID:       storj.PieceID{1},
				EncryptedKey:      []byte{3},
				EncryptedKeyNonce: []byte{4},
				EncryptedETag:     []byte{5},
				EncryptedSize:     1024,
				PlainSize:         512,
				Pieces:            metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
				Redundancy:        metabasetest.DefaultRedundancy,
			}

			expectedCopiedSegmentRaw := metabase.Segment{
				StreamID: copyObjStream.StreamID,
				Position: metabase.SegmentPosition{
					Index: 0,
				},
				CreatedAt:   obj.CreatedAt,
				ExpiresAt:   obj.ExpiresAt,
				RootPieceID: storj.PieceID{1},

				Pieces: metabase.Pieces{},

				EncryptedKey:      newEncryptedKeyNonces[0].EncryptedKey,
				EncryptedKeyNonce: newEncryptedKeyNonces[0].EncryptedKeyNonce,
				EncryptedSize:     1024,
				PlainSize:         512,

				Redundancy: metabasetest.DefaultRedundancy,
				InlineData: []byte{},
			}

			expectedCopiedSegmentGet := expectedSegment

			expectedCopiedSegmentGet.EncryptedETag = nil
			expectedCopiedSegmentGet.StreamID = copyObjStream.StreamID
			expectedCopiedSegmentGet.EncryptedKey = newEncryptedKeyNonces[0].EncryptedKey
			expectedCopiedSegmentGet.EncryptedKeyNonce = newEncryptedKeyNonces[0].EncryptedKeyNonce
			expectedCopiedSegmentGet.InlineData = []byte{}

			metabasetest.GetSegmentByPosition{
				Opts: metabase.GetSegmentByPosition{
					StreamID: obj.StreamID,
					Position: metabase.SegmentPosition{
						Index: 0,
					},
				},
				Result: expectedSegment,
			}.Check(ctx, t, db)

			metabasetest.GetSegmentByPosition{
				Opts: metabase.GetSegmentByPosition{
					StreamID: copyObjStream.StreamID,
					Position: metabase.SegmentPosition{
						Index: 0,
					},
				},
				Result: expectedCopiedSegmentGet,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj.ObjectStream,
						CreatedAt:    now,
						Status:       metabase.Committed,
						SegmentCount: 1,

						TotalPlainSize:     512,
						TotalEncryptedSize: 1024,
						FixedSegmentSize:   512,

						Encryption: metabasetest.DefaultEncryption,
					},
					{
						ObjectStream: copyObjStream,
						CreatedAt:    now,
						ExpiresAt:    obj.ExpiresAt,
						Status:       metabase.Committed,
						SegmentCount: 1,

						TotalPlainSize:     512,
						TotalEncryptedSize: 1024,
						FixedSegmentSize:   512,

						EncryptedMetadataNonce:        newEncryptedMetadataKeyNonce[:],
						EncryptedMetadataEncryptedKey: newEncryptedMetadataKey,
						Encryption:                    metabasetest.DefaultEncryption,
					},
				},
				Segments: []metabase.RawSegment{
					metabase.RawSegment(expectedSegment),
					metabase.RawSegment(expectedCopiedSegmentRaw),
				},
				Copies: []metabase.RawCopy{
					{
						StreamID:         copyObjStream.StreamID,
						AncestorStreamID: objStream.StreamID,
					}},
			}.Check(ctx, t, db)
		})

		t.Run("Get empty inline segment copy", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			objStream := metabasetest.RandObjectStream()
			copyObjStream := metabasetest.RandObjectStream()
			copyObjStream.ProjectID = objStream.ProjectID

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
				Version: obj.Version,
			}.Check(ctx, t, db)

			metabasetest.CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: objStream,
					Position:     metabase.SegmentPosition{Part: 0, Index: uint32(0)},

					EncryptedKey:      []byte{3},
					EncryptedKeyNonce: []byte{4},

					PlainSize:   0,
					PlainOffset: 0,
				},
			}.Check(ctx, t, db)

			obj := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: objStream,
				},
			}.Check(ctx, t, db)

			encryptedKeyNonces := []metabase.EncryptedKeyAndNonce{{
				EncryptedKeyNonce: []byte{4},
				EncryptedKey:      []byte{3},
				Position: metabase.SegmentPosition{
					Index: 0,
				},
			}}

			newEncryptedKeyNonces := []metabase.EncryptedKeyAndNonce{{
				EncryptedKeyNonce: []byte{3},
				EncryptedKey:      []byte{4},
				Position: metabase.SegmentPosition{
					Index: 0,
				},
			}}

			metabasetest.BeginCopyObject{
				Opts: metabase.BeginCopyObject{
					ObjectLocation: obj.Location(),
				},
				Result: metabase.BeginCopyObjectResult{
					StreamID:                  obj.StreamID,
					Version:                   obj.Version,
					EncryptedMetadata:         obj.EncryptedMetadata,
					EncryptedMetadataKey:      obj.EncryptedMetadataEncryptedKey,
					EncryptedMetadataKeyNonce: obj.EncryptedMetadataNonce,
					EncryptedKeysNonces:       encryptedKeyNonces,
					EncryptionParameters:      obj.Encryption,
				},
			}.Check(ctx, t, db)

			newEncryptedMetadataKeyNonce := testrand.Nonce()
			newEncryptedMetadataKey := testrand.Bytes(32)

			_, err := db.FinishCopyObject(ctx, metabase.FinishCopyObject{
				ObjectStream:                 obj.ObjectStream,
				NewStreamID:                  copyObjStream.StreamID,
				NewBucket:                    copyObjStream.BucketName,
				NewSegmentKeys:               newEncryptedKeyNonces,
				NewEncryptedObjectKey:        copyObjStream.ObjectKey,
				NewEncryptedMetadataKeyNonce: newEncryptedMetadataKeyNonce,
				NewEncryptedMetadataKey:      newEncryptedMetadataKey,
			})
			require.NoError(t, err)

			expectedSegment := metabase.Segment{
				StreamID: obj.StreamID,
				Position: metabase.SegmentPosition{
					Index: 0,
				},
				CreatedAt:         obj.CreatedAt,
				ExpiresAt:         obj.ExpiresAt,
				EncryptedKey:      []byte{3},
				EncryptedKeyNonce: []byte{4},
				EncryptedSize:     0,
				PlainSize:         0,
			}

			expectedCopiedSegmentRaw := metabase.Segment{
				StreamID: copyObjStream.StreamID,
				Position: metabase.SegmentPosition{
					Index: 0,
				},
				CreatedAt: obj.CreatedAt,
				ExpiresAt: obj.ExpiresAt,

				Pieces: metabase.Pieces{},

				EncryptedKey:      newEncryptedKeyNonces[0].EncryptedKey,
				EncryptedKeyNonce: newEncryptedKeyNonces[0].EncryptedKeyNonce,
				InlineData:        []byte{},
			}

			expectedCopiedSegmentGet := expectedSegment
			expectedCopiedSegmentGet.StreamID = copyObjStream.StreamID

			expectedCopiedSegmentGet.EncryptedKey = newEncryptedKeyNonces[0].EncryptedKey
			expectedCopiedSegmentGet.EncryptedKeyNonce = newEncryptedKeyNonces[0].EncryptedKeyNonce
			expectedCopiedSegmentGet.InlineData = []byte{}

			metabasetest.GetSegmentByPosition{
				Opts: metabase.GetSegmentByPosition{
					StreamID: obj.StreamID,
					Position: metabase.SegmentPosition{
						Index: 0,
					},
				},
				Result: expectedSegment,
			}.Check(ctx, t, db)

			metabasetest.GetSegmentByPosition{
				Opts: metabase.GetSegmentByPosition{
					StreamID: copyObjStream.StreamID,
					Position: metabase.SegmentPosition{
						Index: 0,
					},
				},
				Result: expectedCopiedSegmentGet,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj.ObjectStream,
						CreatedAt:    now,
						Status:       metabase.Committed,
						SegmentCount: 1,

						Encryption: metabasetest.DefaultEncryption,
					},
					{
						ObjectStream: copyObjStream,
						CreatedAt:    now,
						ExpiresAt:    obj.ExpiresAt,
						Status:       metabase.Committed,
						SegmentCount: 1,

						EncryptedMetadataNonce:        newEncryptedMetadataKeyNonce[:],
						EncryptedMetadataEncryptedKey: newEncryptedMetadataKey,
						Encryption:                    metabasetest.DefaultEncryption,
					},
				},
				Segments: []metabase.RawSegment{
					metabase.RawSegment(expectedSegment),
					metabase.RawSegment(expectedCopiedSegmentRaw),
				},
				Copies: nil,
			}.Check(ctx, t, db)
		})

		t.Run("Get inline segment copy", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			objStream := metabasetest.RandObjectStream()
			copyObjStream := metabasetest.RandObjectStream()
			data := testrand.Bytes(1024)
			copyObjStream.ProjectID = objStream.ProjectID

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
				Version: obj.Version,
			}.Check(ctx, t, db)

			metabasetest.CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: objStream,
					Position:     metabase.SegmentPosition{Part: 0, Index: uint32(0)},

					EncryptedKey:      []byte{3},
					EncryptedKeyNonce: []byte{4},

					PlainSize:   0,
					PlainOffset: 0,

					InlineData: data,
				},
			}.Check(ctx, t, db)

			obj := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: objStream,
				},
			}.Check(ctx, t, db)

			encryptedKeyNonces := []metabase.EncryptedKeyAndNonce{{
				EncryptedKeyNonce: []byte{4},
				EncryptedKey:      []byte{3},
				Position: metabase.SegmentPosition{
					Index: 0,
				},
			}}

			newEncryptedKeyNonces := []metabase.EncryptedKeyAndNonce{{
				EncryptedKeyNonce: []byte{3},
				EncryptedKey:      []byte{4},
				Position: metabase.SegmentPosition{
					Index: 0,
				},
			}}

			metabasetest.BeginCopyObject{
				Opts: metabase.BeginCopyObject{
					ObjectLocation: obj.Location(),
				},
				Result: metabase.BeginCopyObjectResult{
					StreamID:                  obj.StreamID,
					Version:                   obj.Version,
					EncryptedMetadata:         obj.EncryptedMetadata,
					EncryptedMetadataKey:      obj.EncryptedMetadataEncryptedKey,
					EncryptedMetadataKeyNonce: obj.EncryptedMetadataNonce,
					EncryptedKeysNonces:       encryptedKeyNonces,
					EncryptionParameters:      obj.Encryption,
				},
			}.Check(ctx, t, db)

			newEncryptedMetadataKeyNonce := testrand.Nonce()
			newEncryptedMetadataKey := testrand.Bytes(32)

			_, err := db.FinishCopyObject(ctx, metabase.FinishCopyObject{
				ObjectStream:                 obj.ObjectStream,
				NewStreamID:                  copyObjStream.StreamID,
				NewBucket:                    copyObjStream.BucketName,
				NewSegmentKeys:               newEncryptedKeyNonces,
				NewEncryptedObjectKey:        copyObjStream.ObjectKey,
				NewEncryptedMetadataKeyNonce: newEncryptedMetadataKeyNonce,
				NewEncryptedMetadataKey:      newEncryptedMetadataKey,
			})
			require.NoError(t, err)

			expectedSegment := metabase.Segment{
				StreamID: obj.StreamID,
				Position: metabase.SegmentPosition{
					Index: 0,
				},
				CreatedAt:         obj.CreatedAt,
				ExpiresAt:         obj.ExpiresAt,
				EncryptedKey:      []byte{3},
				EncryptedKeyNonce: []byte{4},

				EncryptedSize: 1024,
				PlainSize:     0,

				InlineData: data,
			}

			expectedCopiedSegmentRaw := metabase.Segment{
				StreamID: copyObjStream.StreamID,
				Position: metabase.SegmentPosition{
					Index: 0,
				},
				CreatedAt: obj.CreatedAt,
				ExpiresAt: obj.ExpiresAt,

				Pieces: metabase.Pieces{},

				EncryptedKey:      newEncryptedKeyNonces[0].EncryptedKey,
				EncryptedKeyNonce: newEncryptedKeyNonces[0].EncryptedKeyNonce,

				EncryptedSize: 1024,

				InlineData: data,
			}

			expectedCopiedSegmentGet := expectedSegment
			expectedCopiedSegmentGet.StreamID = copyObjStream.StreamID

			expectedCopiedSegmentGet.EncryptedKey = newEncryptedKeyNonces[0].EncryptedKey
			expectedCopiedSegmentGet.EncryptedKeyNonce = newEncryptedKeyNonces[0].EncryptedKeyNonce

			metabasetest.GetSegmentByPosition{
				Opts: metabase.GetSegmentByPosition{
					StreamID: obj.StreamID,
					Position: metabase.SegmentPosition{
						Index: 0,
					},
				},
				Result: expectedSegment,
			}.Check(ctx, t, db)

			metabasetest.GetSegmentByPosition{
				Opts: metabase.GetSegmentByPosition{
					StreamID: copyObjStream.StreamID,
					Position: metabase.SegmentPosition{
						Index: 0,
					},
				},
				Result: expectedCopiedSegmentGet,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj.ObjectStream,
						CreatedAt:    now,
						Status:       metabase.Committed,
						SegmentCount: 1,

						Encryption: metabasetest.DefaultEncryption,

						TotalEncryptedSize: 1024,
					},
					{
						ObjectStream: copyObjStream,
						CreatedAt:    now,
						ExpiresAt:    obj.ExpiresAt,
						Status:       metabase.Committed,
						SegmentCount: 1,

						EncryptedMetadataNonce:        newEncryptedMetadataKeyNonce[:],
						EncryptedMetadataEncryptedKey: newEncryptedMetadataKey,
						Encryption:                    metabasetest.DefaultEncryption,

						TotalEncryptedSize: 1024,
					},
				},
				Segments: []metabase.RawSegment{
					metabase.RawSegment(expectedSegment),
					metabase.RawSegment(expectedCopiedSegmentRaw),
				},
				Copies: nil,
			}.Check(ctx, t, db)
		})
	})
}

func TestGetLatestObjectLastSegment(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()
		location := obj.Location()
		now := time.Now()

		for _, test := range metabasetest.InvalidObjectLocations(location) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)
				metabasetest.GetLatestObjectLastSegment{
					Opts: metabase.GetLatestObjectLastSegment{
						ObjectLocation: test.ObjectLocation,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)

				metabasetest.Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("Object or segment missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.GetLatestObjectLastSegment{
				Opts: metabase.GetLatestObjectLastSegment{
					ObjectLocation: location,
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "metabase: object or segment missing",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Get last segment", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.CreateObject(ctx, t, db, obj, 2)

			expectedSegmentSecond := metabase.Segment{
				StreamID: obj.StreamID,
				Position: metabase.SegmentPosition{
					Index: 1,
				},
				CreatedAt:         now,
				RootPieceID:       storj.PieceID{1},
				EncryptedKey:      []byte{3},
				EncryptedKeyNonce: []byte{4},
				EncryptedETag:     []byte{5},
				EncryptedSize:     1024,
				PlainSize:         512,
				PlainOffset:       512,
				Pieces:            metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
				Redundancy:        metabasetest.DefaultRedundancy,
			}

			expectedSegmentFirst := expectedSegmentSecond
			expectedSegmentFirst.Position.Index = 0
			expectedSegmentFirst.PlainOffset = 0

			metabasetest.GetLatestObjectLastSegment{
				Opts: metabase.GetLatestObjectLastSegment{
					ObjectLocation: location,
				},
				Result: expectedSegmentSecond,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Committed,
						SegmentCount: 2,

						TotalPlainSize:     1024,
						TotalEncryptedSize: 2048,
						FixedSegmentSize:   512,

						Encryption: metabasetest.DefaultEncryption,
					},
				},
				Segments: []metabase.RawSegment{
					metabase.RawSegment(expectedSegmentFirst),
					metabase.RawSegment(expectedSegmentSecond),
				},
			}.Check(ctx, t, db)
		})

		t.Run("Get segment copy", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			objStream := metabasetest.RandObjectStream()

			originalObj, originalSegments := metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:                  objStream,
					EncryptedMetadata:             testrand.Bytes(64),
					EncryptedMetadataNonce:        testrand.Nonce().Bytes(),
					EncryptedMetadataEncryptedKey: testrand.Bytes(265),
				},
			}.Run(ctx, t, db, objStream, 1)

			copyObj, _, newSegments := metabasetest.CreateObjectCopy{
				OriginalObject: originalObj,
			}.Run(ctx, t, db)

			metabasetest.GetLatestObjectLastSegment{
				Opts: metabase.GetLatestObjectLastSegment{
					ObjectLocation: originalObj.Location(),
				},
				Result: originalSegments[0],
			}.Check(ctx, t, db)

			copySegmentGet := originalSegments[0]
			copySegmentGet.StreamID = copyObj.StreamID
			copySegmentGet.EncryptedETag = nil
			copySegmentGet.InlineData = []byte{}
			copySegmentGet.EncryptedKey = newSegments[0].EncryptedKey
			copySegmentGet.EncryptedKeyNonce = newSegments[0].EncryptedKeyNonce

			metabasetest.GetLatestObjectLastSegment{
				Opts: metabase.GetLatestObjectLastSegment{
					ObjectLocation: copyObj.Location(),
				},
				Result: copySegmentGet,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(originalObj),
					metabase.RawObject(copyObj),
				},
				Segments: append(metabasetest.SegmentsToRaw(originalSegments), newSegments...),
				Copies: []metabase.RawCopy{{
					StreamID:         copyObj.StreamID,
					AncestorStreamID: originalObj.StreamID,
				}},
			}.Check(ctx, t, db)
		})

		t.Run("Get empty inline segment copy", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			objStream := metabasetest.RandObjectStream()
			copyObjStream := metabasetest.RandObjectStream()
			copyObjStream.ProjectID = objStream.ProjectID
			objLocation := objStream.Location()
			copyLocation := copyObjStream.Location()

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
				Version: obj.Version,
			}.Check(ctx, t, db)

			metabasetest.CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: objStream,
					Position:     metabase.SegmentPosition{Part: 0, Index: uint32(0)},

					EncryptedKey:      []byte{3},
					EncryptedKeyNonce: []byte{4},

					PlainSize:   0,
					PlainOffset: 0,
				},
			}.Check(ctx, t, db)

			obj := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: objStream,
				},
			}.Check(ctx, t, db)

			encryptedKeyNonces := []metabase.EncryptedKeyAndNonce{{
				EncryptedKeyNonce: []byte{4},
				EncryptedKey:      []byte{3},
				Position: metabase.SegmentPosition{
					Index: 0,
				},
			}}

			newEncryptedKeyNonces := []metabase.EncryptedKeyAndNonce{{
				EncryptedKeyNonce: []byte{3},
				EncryptedKey:      []byte{4},
				Position: metabase.SegmentPosition{
					Index: 0,
				},
			}}

			metabasetest.BeginCopyObject{
				Opts: metabase.BeginCopyObject{
					ObjectLocation: obj.Location(),
				},
				Result: metabase.BeginCopyObjectResult{
					StreamID:                  obj.StreamID,
					Version:                   obj.Version,
					EncryptedMetadata:         obj.EncryptedMetadata,
					EncryptedMetadataKey:      obj.EncryptedMetadataEncryptedKey,
					EncryptedMetadataKeyNonce: obj.EncryptedMetadataNonce,
					EncryptedKeysNonces:       encryptedKeyNonces,
					EncryptionParameters:      obj.Encryption,
				},
			}.Check(ctx, t, db)

			newEncryptedMetadataKeyNonce := testrand.Nonce()
			newEncryptedMetadataKey := testrand.Bytes(32)

			_, err := db.FinishCopyObject(ctx, metabase.FinishCopyObject{
				ObjectStream:                 obj.ObjectStream,
				NewStreamID:                  copyObjStream.StreamID,
				NewBucket:                    copyObjStream.BucketName,
				NewSegmentKeys:               newEncryptedKeyNonces,
				NewEncryptedObjectKey:        copyObjStream.ObjectKey,
				NewEncryptedMetadataKeyNonce: newEncryptedMetadataKeyNonce,
				NewEncryptedMetadataKey:      newEncryptedMetadataKey,
			})
			require.NoError(t, err)

			expectedSegment := metabase.Segment{
				StreamID: obj.StreamID,
				Position: metabase.SegmentPosition{
					Index: 0,
				},
				CreatedAt:         obj.CreatedAt,
				ExpiresAt:         obj.ExpiresAt,
				EncryptedKey:      []byte{3},
				EncryptedKeyNonce: []byte{4},
				EncryptedSize:     0,
				PlainSize:         0,
			}

			expectedCopiedSegmentRaw := metabase.Segment{
				StreamID: copyObjStream.StreamID,
				Position: metabase.SegmentPosition{
					Index: 0,
				},
				CreatedAt: obj.CreatedAt,
				ExpiresAt: obj.ExpiresAt,

				Pieces: metabase.Pieces{},

				EncryptedKey:      newEncryptedKeyNonces[0].EncryptedKey,
				EncryptedKeyNonce: newEncryptedKeyNonces[0].EncryptedKeyNonce,

				InlineData: []byte{},
			}

			expectedCopiedSegmentGet := expectedSegment
			expectedCopiedSegmentGet.StreamID = copyObjStream.StreamID

			expectedCopiedSegmentGet.EncryptedKey = newEncryptedKeyNonces[0].EncryptedKey
			expectedCopiedSegmentGet.EncryptedKeyNonce = newEncryptedKeyNonces[0].EncryptedKeyNonce
			expectedCopiedSegmentGet.InlineData = []byte{}

			metabasetest.GetLatestObjectLastSegment{
				Opts: metabase.GetLatestObjectLastSegment{
					ObjectLocation: objLocation,
				},
				Result: expectedSegment,
			}.Check(ctx, t, db)

			metabasetest.GetLatestObjectLastSegment{
				Opts: metabase.GetLatestObjectLastSegment{
					ObjectLocation: copyLocation,
				},
				Result: expectedCopiedSegmentGet,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj.ObjectStream,
						CreatedAt:    now,
						Status:       metabase.Committed,
						SegmentCount: 1,

						Encryption: metabasetest.DefaultEncryption,
					},
					{
						ObjectStream: copyObjStream,
						CreatedAt:    now,
						ExpiresAt:    obj.ExpiresAt,
						Status:       metabase.Committed,
						SegmentCount: 1,

						EncryptedMetadataNonce:        newEncryptedMetadataKeyNonce[:],
						EncryptedMetadataEncryptedKey: newEncryptedMetadataKey,
						Encryption:                    metabasetest.DefaultEncryption,
					},
				},
				Segments: []metabase.RawSegment{
					metabase.RawSegment(expectedSegment),
					metabase.RawSegment(expectedCopiedSegmentRaw),
				},
				Copies: nil,
			}.Check(ctx, t, db)
		})

		t.Run("Get inline segment copy", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			objStream := metabasetest.RandObjectStream()
			copyObjStream := metabasetest.RandObjectStream()
			data := testrand.Bytes(1024)
			copyObjStream.ProjectID = objStream.ProjectID
			objLocation := objStream.Location()
			copyLocation := copyObjStream.Location()

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
				Version: obj.Version,
			}.Check(ctx, t, db)

			metabasetest.CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream: objStream,
					Position:     metabase.SegmentPosition{Part: 0, Index: uint32(0)},

					EncryptedKey:      []byte{3},
					EncryptedKeyNonce: []byte{4},

					PlainSize:   0,
					PlainOffset: 0,

					InlineData: data,
				},
			}.Check(ctx, t, db)

			obj := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: objStream,
				},
			}.Check(ctx, t, db)

			encryptedKeyNonces := []metabase.EncryptedKeyAndNonce{{
				EncryptedKeyNonce: []byte{4},
				EncryptedKey:      []byte{3},
				Position: metabase.SegmentPosition{
					Index: 0,
				},
			}}

			newEncryptedKeyNonces := []metabase.EncryptedKeyAndNonce{{
				EncryptedKeyNonce: []byte{3},
				EncryptedKey:      []byte{4},
				Position: metabase.SegmentPosition{
					Index: 0,
				},
			}}

			metabasetest.BeginCopyObject{
				Opts: metabase.BeginCopyObject{
					ObjectLocation: obj.Location(),
				},
				Result: metabase.BeginCopyObjectResult{
					StreamID:                  obj.StreamID,
					Version:                   obj.Version,
					EncryptedMetadata:         obj.EncryptedMetadata,
					EncryptedMetadataKey:      obj.EncryptedMetadataEncryptedKey,
					EncryptedMetadataKeyNonce: obj.EncryptedMetadataNonce,
					EncryptedKeysNonces:       encryptedKeyNonces,
					EncryptionParameters:      obj.Encryption,
				},
			}.Check(ctx, t, db)

			newEncryptedMetadataKeyNonce := testrand.Nonce()
			newEncryptedMetadataKey := testrand.Bytes(32)

			_, err := db.FinishCopyObject(ctx, metabase.FinishCopyObject{
				ObjectStream:                 obj.ObjectStream,
				NewStreamID:                  copyObjStream.StreamID,
				NewBucket:                    copyObjStream.BucketName,
				NewSegmentKeys:               newEncryptedKeyNonces,
				NewEncryptedObjectKey:        copyObjStream.ObjectKey,
				NewEncryptedMetadataKeyNonce: newEncryptedMetadataKeyNonce,
				NewEncryptedMetadataKey:      newEncryptedMetadataKey,
			})
			require.NoError(t, err)

			expectedSegment := metabase.Segment{
				StreamID: obj.StreamID,
				Position: metabase.SegmentPosition{
					Index: 0,
				},
				CreatedAt:         obj.CreatedAt,
				ExpiresAt:         obj.ExpiresAt,
				EncryptedKey:      []byte{3},
				EncryptedKeyNonce: []byte{4},

				EncryptedSize: 1024,
				PlainSize:     0,

				InlineData: data,
			}

			expectedCopiedSegmentRaw := metabase.Segment{
				StreamID: copyObjStream.StreamID,
				Position: metabase.SegmentPosition{
					Index: 0,
				},
				CreatedAt: obj.CreatedAt,
				ExpiresAt: obj.ExpiresAt,

				Pieces: metabase.Pieces{},

				EncryptedKey:      newEncryptedKeyNonces[0].EncryptedKey,
				EncryptedKeyNonce: newEncryptedKeyNonces[0].EncryptedKeyNonce,

				EncryptedSize: 1024,

				InlineData: data,
			}

			expectedCopiedSegmentGet := expectedSegment
			expectedCopiedSegmentGet.StreamID = copyObjStream.StreamID

			expectedCopiedSegmentGet.EncryptedKey = newEncryptedKeyNonces[0].EncryptedKey
			expectedCopiedSegmentGet.EncryptedKeyNonce = newEncryptedKeyNonces[0].EncryptedKeyNonce

			metabasetest.GetLatestObjectLastSegment{
				Opts: metabase.GetLatestObjectLastSegment{
					ObjectLocation: objLocation,
				},
				Result: expectedSegment,
			}.Check(ctx, t, db)

			metabasetest.GetLatestObjectLastSegment{
				Opts: metabase.GetLatestObjectLastSegment{
					ObjectLocation: copyLocation,
				},
				Result: expectedCopiedSegmentGet,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj.ObjectStream,
						CreatedAt:    now,
						Status:       metabase.Committed,
						SegmentCount: 1,

						Encryption: metabasetest.DefaultEncryption,

						TotalEncryptedSize: 1024,
					},
					{
						ObjectStream: copyObjStream,
						CreatedAt:    now,
						ExpiresAt:    obj.ExpiresAt,
						Status:       metabase.Committed,
						SegmentCount: 1,

						EncryptedMetadataNonce:        newEncryptedMetadataKeyNonce[:],
						EncryptedMetadataEncryptedKey: newEncryptedMetadataKey,
						Encryption:                    metabasetest.DefaultEncryption,

						TotalEncryptedSize: 1024,
					},
				},
				Segments: []metabase.RawSegment{
					metabase.RawSegment(expectedSegment),
					metabase.RawSegment(expectedCopiedSegmentRaw),
				},
				Copies: nil,
			}.Check(ctx, t, db)
		})
	})
}

func TestBucketEmpty(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()
		now := time.Now()
		zombieDeadline := now.Add(24 * time.Hour)

		t.Run("ProjectID missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BucketEmpty{
				Opts:     metabase.BucketEmpty{},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "ProjectID missing",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("BucketName missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BucketEmpty{
				Opts: metabase.BucketEmpty{
					ProjectID: obj.ProjectID,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "BucketName missing",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("BucketEmpty true", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BucketEmpty{
				Opts: metabase.BucketEmpty{
					ProjectID:  obj.ProjectID,
					BucketName: obj.BucketName,
				},
				Result: true,
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("BucketEmpty false with pending object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,

					Encryption: metabasetest.DefaultEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			metabasetest.BucketEmpty{
				Opts: metabase.BucketEmpty{
					ProjectID:  obj.ProjectID,
					BucketName: obj.BucketName,
				},
				Result: false,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream:           obj,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieDeadline,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("BucketEmpty false with committed object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.CreateObject(ctx, t, db, obj, 0)

			metabasetest.BucketEmpty{
				Opts: metabase.BucketEmpty{
					ProjectID:  obj.ProjectID,
					BucketName: obj.BucketName,
				},
				Result: false,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(object),
				},
			}.Check(ctx, t, db)
		})
	})
}
