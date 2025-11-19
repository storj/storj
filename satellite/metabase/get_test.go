// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestGetObjectExactVersion(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		location := obj.Location()

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

			object := metabasetest.CreateObject(ctx, t, db, obj, 0)

			metabasetest.GetObjectExactVersion{
				Opts: metabase.GetObjectExactVersion{
					ObjectLocation: location,
					Version:        11,
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "metabase: sql: no rows in result set",
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("Get pending object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,

					Encryption: metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.GetObjectExactVersion{
				Opts: metabase.GetObjectExactVersion{
					ObjectLocation: location,
					Version:        1,
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "metabase: sql: no rows in result set",
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("Get negative pending object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := obj
			obj.Version = -1

			object := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.GetObjectExactVersion{
				Opts: metabase.GetObjectExactVersion{
					ObjectLocation: obj.Location(),
					Version:        obj.Version,
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "metabase: sql: no rows in result set",
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("Get expired object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()
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
					Status:       metabase.CommittedUnversioned,
					ExpiresAt:    &expiresAt,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}}.Check(ctx, t, db)
		})

		t.Run("get committed/deletemarker unversioned/versioned", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			unversionedLocation := obj
			unversioned := metabasetest.CreateObject(ctx, t, db, unversionedLocation, 0)

			metabasetest.GetObjectExactVersion{
				Opts: metabase.GetObjectExactVersion{
					ObjectLocation: unversioned.Location(),
					Version:        unversioned.Version,
				},
				Result: unversioned,
			}.Check(ctx, t, db)

			versionedLocation := obj
			versionedLocation.Version++
			versioned := metabasetest.CreateObjectVersioned(ctx, t, db, versionedLocation, 0)

			metabasetest.GetObjectExactVersion{
				Opts: metabase.GetObjectExactVersion{
					ObjectLocation: versioned.Location(),
					Version:        versioned.Version,
				},
				Result: versioned,
			}.Check(ctx, t, db)

			markerLocation := obj
			markerLocation.StreamID = uuid.UUID{}
			markerLocation.Version = versioned.Version + 1
			versionedMarker := metabase.Object{
				ObjectStream: markerLocation,
				CreatedAt:    time.Now(),
				Status:       metabase.DeleteMarkerVersioned,
			}

			// this creates a versioned delete marker
			metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: location,
					Versioned:      true,
				},
				Result: metabase.DeleteObjectResult{
					Markers: []metabase.Object{versionedMarker},
				},
				OutputMarkerStreamID: &versionedMarker.StreamID,
			}.Check(ctx, t, db)

			metabasetest.GetObjectExactVersion{
				Opts: metabase.GetObjectExactVersion{
					ObjectLocation: versionedMarker.Location(),
					Version:        versionedMarker.Version,
				},
				Result: versionedMarker,
			}.Check(ctx, t, db)

			unversionedMarkerLocation := obj
			unversionedMarkerLocation.StreamID = uuid.UUID{}
			unversionedMarkerLocation.Version = versionedMarker.Version + 1
			unversionedMarker := metabase.Object{
				ObjectStream: unversionedMarkerLocation,
				CreatedAt:    time.Now(),
				Status:       metabase.DeleteMarkerUnversioned,
			}

			// this creates an unversioned delete marker and replace unversioned object
			metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: location,
					Suspended:      true,
				},
				Result: metabase.DeleteObjectResult{
					Removed: []metabase.Object{unversioned},
					Markers: []metabase.Object{unversionedMarker},
				},
				OutputMarkerStreamID: &unversionedMarker.StreamID,
			}.Check(ctx, t, db)

			metabasetest.GetObjectExactVersion{
				Opts: metabase.GetObjectExactVersion{
					ObjectLocation: unversionedMarker.Location(),
					Version:        unversionedMarker.Version,
				},
				Result: unversionedMarker,
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: []metabase.RawObject{
				metabase.RawObject(versionedMarker),
				metabase.RawObject(unversionedMarker),
				metabase.RawObject(versioned),
			}}.Check(ctx, t, db)
		})

		t.Run("Retention", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()

			retention := metabase.Retention{
				Mode:        storj.ComplianceMode,
				RetainUntil: now.Add(time.Hour),
			}

			metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
					Retention:    retention,
				},
			}.Run(ctx, t, db, obj, 0)

			metabasetest.GetObjectExactVersion{
				Opts: metabase.GetObjectExactVersion{
					ObjectLocation: obj.Location(),
					Version:        obj.Version,
				},
				Result: metabase.Object{
					ObjectStream: obj,
					CreatedAt:    now,
					Status:       metabase.CommittedUnversioned,
					Encryption:   metabasetest.DefaultEncryption,
					Retention:    retention,
				},
			}.Check(ctx, t, db)
		})

		t.Run("Legal hold", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()

			metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
					LegalHold:    true,
				},
			}.Run(ctx, t, db, obj, 0)

			metabasetest.GetObjectExactVersion{
				Opts: metabase.GetObjectExactVersion{
					ObjectLocation: obj.Location(),
					Version:        obj.Version,
				},
				Result: metabase.Object{
					ObjectStream: obj,
					CreatedAt:    now,
					Status:       metabase.CommittedUnversioned,
					Encryption:   metabasetest.DefaultEncryption,
					LegalHold:    true,
				},
			}.Check(ctx, t, db)
		})
	})
}

func TestGetObjectLastCommitted(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()
		location := obj.Location()

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

			object := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.GetObjectLastCommitted{
				Opts: metabase.GetObjectLastCommitted{
					ObjectLocation: location,
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "metabase: sql: no rows in result set",
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("Get object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()

			userData := metabasetest.RandEncryptedUserData()
			retention := metabase.Retention{
				Mode:        storj.ComplianceMode,
				RetainUntil: now.Add(time.Hour),
			}

			metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
					Retention:    retention,
				},
				CommitObject: &metabase.CommitObject{
					ObjectStream: obj,

					OverrideEncryptedMetadata: true,
					EncryptedUserData:         userData,
				},
			}.Run(ctx, t, db, obj, 0)

			metabasetest.GetObjectLastCommitted{
				Opts: metabase.GetObjectLastCommitted{
					ObjectLocation: location,
				},
				Result: metabase.Object{
					ObjectStream:      obj,
					CreatedAt:         now,
					Status:            metabase.CommittedUnversioned,
					Encryption:        metabasetest.DefaultEncryption,
					EncryptedUserData: userData,
					Retention:         retention,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: []metabase.RawObject{
				{
					ObjectStream:      obj,
					CreatedAt:         now,
					Status:            metabase.CommittedUnversioned,
					Encryption:        metabasetest.DefaultEncryption,
					EncryptedUserData: userData,
					Retention:         retention,
				},
			}}.Check(ctx, t, db)
		})

		t.Run("Get object last committed version from multiple", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			firstObject := obj
			firstObject.Version = metabase.Version(1)
			createdObject := metabasetest.CreateObject(ctx, t, db, firstObject, 0)

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
				Result: createdObject,
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: []metabase.RawObject{
				metabase.RawObject(createdObject),
				metabase.RawObject(secondObject),
			}}.Check(ctx, t, db)
		})

		t.Run("Get object last committed version, multiple versions", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			first := obj
			first.Version = metabase.Version(10)
			firstObject := metabasetest.CreateObjectVersioned(ctx, t, db, first, 0)

			second := obj
			second.Version = metabase.Version(11)
			secondObject := metabasetest.CreateObjectVersioned(ctx, t, db, second, 0)

			metabasetest.GetObjectLastCommitted{
				Opts: metabase.GetObjectLastCommitted{
					ObjectLocation: location,
				},
				Result: secondObject,
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: []metabase.RawObject{
				metabase.RawObject(firstObject),
				metabase.RawObject(secondObject),
			}}.Check(ctx, t, db)
		})

		t.Run("Get object delete marker, multiple versions", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			first := obj
			first.Version = metabase.Version(5)
			firstObject := metabasetest.CreateObjectVersioned(ctx, t, db, first, 0)

			second := obj
			second.Version = metabase.Version(8)
			secondObject := metabasetest.CreateObjectVersioned(ctx, t, db, second, 0)

			result, err := db.DeleteObjectLastCommitted(ctx, metabase.DeleteObjectLastCommitted{
				ObjectLocation: second.Location(),
				Versioned:      true,
			})
			require.NoError(t, err)

			metabasetest.GetObjectLastCommitted{
				Opts: metabase.GetObjectLastCommitted{
					ObjectLocation: second.Location(),
				},
				ErrClass: &metabase.ErrObjectNotFound,
			}.Check(ctx, t, db)

			third := obj
			third.Version = metabase.Version(10)
			thirdObject := metabasetest.CreateObjectVersioned(ctx, t, db, third, 0)

			metabasetest.GetObjectLastCommitted{
				Opts: metabase.GetObjectLastCommitted{
					ObjectLocation: second.Location(),
				},
				Result: thirdObject,
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: []metabase.RawObject{
				metabase.RawObject(result.Markers[0]),
				metabase.RawObject(firstObject),
				metabase.RawObject(secondObject),
				metabase.RawObject(thirdObject),
			}}.Check(ctx, t, db)
		})

		t.Run("Get latest copied object version with duplicate metadata", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()

			copyObjStream := metabasetest.RandObjectStream()
			copyObjStream.Version = 1 // auto assigned the first available version
			originalObject := metabasetest.CreateObject(ctx, t, db, obj, 0)

			copiedObj, _, _ := metabasetest.CreateObjectCopy{
				OriginalObject:   originalObject,
				CopyObjectStream: &copyObjStream,
			}.Run(ctx, t, db)

			metabasetest.DeleteObjectExactVersion{
				Opts: metabase.DeleteObjectExactVersion{
					Version:        obj.Version,
					ObjectLocation: obj.Location(),
				},
				Result: metabase.DeleteObjectResult{
					Removed: []metabase.Object{originalObject},
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
					CreatedAt:         now,
					Status:            metabase.CommittedUnversioned,
					Encryption:        metabasetest.DefaultEncryption,
					EncryptedUserData: copiedObj.EncryptedUserData,
				},
			}}.Check(ctx, t, db)
		})
	})
}

func TestGetSegmentByPosition(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

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

			now := time.Now()

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
						Status:       metabase.CommittedUnversioned,
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
						Status:       metabase.CommittedUnversioned,
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

			now := time.Now()

			objStream := metabasetest.RandObjectStream()
			copyObjStream := metabasetest.RandObjectStream()
			copyObjStream.ProjectID = objStream.ProjectID
			copyObjStream.Version = 1 // auto assigned the first available version

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
					SegmentLimit:   10,
				},
				Result: metabase.BeginCopyObjectResult{
					StreamID:             obj.StreamID,
					Version:              obj.Version,
					EncryptedUserData:    obj.EncryptedUserData,
					EncryptedKeysNonces:  encryptedKeyNonces,
					EncryptionParameters: obj.Encryption,
				},
			}.Check(ctx, t, db)

			newEncryptedMetadataKeyNonce := testrand.Nonce()
			newEncryptedMetadataKey := testrand.Bytes(32)

			_, err := db.FinishCopyObject(ctx, metabase.FinishCopyObject{
				NewStreamID:           copyObjStream.StreamID,
				NewBucket:             copyObjStream.BucketName,
				ObjectStream:          obj.ObjectStream,
				NewSegmentKeys:        newEncryptedKeyNonces,
				NewEncryptedObjectKey: copyObjStream.ObjectKey,
				NewEncryptedUserData: metabase.EncryptedUserData{
					EncryptedMetadataNonce:        newEncryptedMetadataKeyNonce.Bytes(),
					EncryptedMetadataEncryptedKey: newEncryptedMetadataKey,
				},
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

			expectedCopiedSegment := expectedSegment
			expectedCopiedSegment.StreamID = copyObjStream.StreamID
			expectedCopiedSegment.EncryptedETag = nil
			expectedCopiedSegment.EncryptedKey = newEncryptedKeyNonces[0].EncryptedKey
			expectedCopiedSegment.EncryptedKeyNonce = newEncryptedKeyNonces[0].EncryptedKeyNonce
			expectedCopiedSegment.InlineData = []byte{}

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
				Result: expectedCopiedSegment,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj.ObjectStream,
						CreatedAt:    now,
						Status:       metabase.CommittedUnversioned,
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
						Status:       metabase.CommittedUnversioned,
						SegmentCount: 1,

						TotalPlainSize:     512,
						TotalEncryptedSize: 1024,
						FixedSegmentSize:   512,

						EncryptedUserData: metabase.EncryptedUserData{
							EncryptedMetadataNonce:        newEncryptedMetadataKeyNonce[:],
							EncryptedMetadataEncryptedKey: newEncryptedMetadataKey,
						},
						Encryption: metabasetest.DefaultEncryption,
					},
				},
				Segments: []metabase.RawSegment{
					metabase.RawSegment(expectedSegment),
					metabase.RawSegment(expectedCopiedSegment),
				},
			}.Check(ctx, t, db)
		})

		t.Run("Get empty inline segment copy", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()

			objStream := metabasetest.RandObjectStream()
			copyObjStream := metabasetest.RandObjectStream()
			copyObjStream.ProjectID = objStream.ProjectID
			copyObjStream.Version = 1 // auto assigned the first available version

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
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
					SegmentLimit:   10,
				},
				Result: metabase.BeginCopyObjectResult{
					StreamID:             obj.StreamID,
					Version:              obj.Version,
					EncryptedUserData:    obj.EncryptedUserData,
					EncryptedKeysNonces:  encryptedKeyNonces,
					EncryptionParameters: obj.Encryption,
				},
			}.Check(ctx, t, db)

			newEncryptedMetadataKeyNonce := testrand.Nonce()
			newEncryptedMetadataKey := testrand.Bytes(32)

			_, err := db.FinishCopyObject(ctx, metabase.FinishCopyObject{
				ObjectStream:          obj.ObjectStream,
				NewStreamID:           copyObjStream.StreamID,
				NewBucket:             copyObjStream.BucketName,
				NewSegmentKeys:        newEncryptedKeyNonces,
				NewEncryptedObjectKey: copyObjStream.ObjectKey,
				NewEncryptedUserData: metabase.EncryptedUserData{
					EncryptedMetadataNonce:        newEncryptedMetadataKeyNonce.Bytes(),
					EncryptedMetadataEncryptedKey: newEncryptedMetadataKey,
				},
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
						Status:       metabase.CommittedUnversioned,
						SegmentCount: 1,

						Encryption: metabasetest.DefaultEncryption,
					},
					{
						ObjectStream: copyObjStream,
						CreatedAt:    now,
						ExpiresAt:    obj.ExpiresAt,
						Status:       metabase.CommittedUnversioned,
						SegmentCount: 1,

						EncryptedUserData: metabase.EncryptedUserData{
							EncryptedMetadataNonce:        newEncryptedMetadataKeyNonce[:],
							EncryptedMetadataEncryptedKey: newEncryptedMetadataKey,
						},
						Encryption: metabasetest.DefaultEncryption,
					},
				},
				Segments: []metabase.RawSegment{
					metabase.RawSegment(expectedSegment),
					metabase.RawSegment(expectedCopiedSegmentRaw),
				},
			}.Check(ctx, t, db)
		})

		t.Run("Get inline segment copy", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()

			objStream := metabasetest.RandObjectStream()
			copyObjStream := metabasetest.RandObjectStream()
			data := testrand.Bytes(1024)
			copyObjStream.ProjectID = objStream.ProjectID
			copyObjStream.Version = 1 // auto assigned the first available version

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
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
					SegmentLimit:   10,
				},
				Result: metabase.BeginCopyObjectResult{
					StreamID:             obj.StreamID,
					Version:              obj.Version,
					EncryptedUserData:    obj.EncryptedUserData,
					EncryptedKeysNonces:  encryptedKeyNonces,
					EncryptionParameters: obj.Encryption,
				},
			}.Check(ctx, t, db)

			newEncryptedMetadataKeyNonce := testrand.Nonce()
			newEncryptedMetadataKey := testrand.Bytes(32)

			_, err := db.FinishCopyObject(ctx, metabase.FinishCopyObject{
				ObjectStream:          obj.ObjectStream,
				NewStreamID:           copyObjStream.StreamID,
				NewBucket:             copyObjStream.BucketName,
				NewSegmentKeys:        newEncryptedKeyNonces,
				NewEncryptedObjectKey: copyObjStream.ObjectKey,
				NewEncryptedUserData: metabase.EncryptedUserData{
					EncryptedMetadataNonce:        newEncryptedMetadataKeyNonce.Bytes(),
					EncryptedMetadataEncryptedKey: newEncryptedMetadataKey,
				},
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
						Status:       metabase.CommittedUnversioned,
						SegmentCount: 1,

						Encryption: metabasetest.DefaultEncryption,

						TotalEncryptedSize: 1024,
					},
					{
						ObjectStream: copyObjStream,
						CreatedAt:    now,
						ExpiresAt:    obj.ExpiresAt,
						Status:       metabase.CommittedUnversioned,
						SegmentCount: 1,

						EncryptedUserData: metabase.EncryptedUserData{
							EncryptedMetadataNonce:        newEncryptedMetadataKeyNonce[:],
							EncryptedMetadataEncryptedKey: newEncryptedMetadataKey,
						},
						Encryption: metabasetest.DefaultEncryption,

						TotalEncryptedSize: 1024,
					},
				},
				Segments: []metabase.RawSegment{
					metabase.RawSegment(expectedSegment),
					metabase.RawSegment(expectedCopiedSegmentRaw),
				},
			}.Check(ctx, t, db)
		})
	})
}

func TestGetLatestObjectLastSegment(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()
		location := obj.Location()

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

			now := time.Now()

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
						Status:       metabase.CommittedUnversioned,
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
					ObjectStream:      objStream,
					EncryptedUserData: metabasetest.RandEncryptedUserDataWithoutETag(),
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
			}.Check(ctx, t, db)
		})

		t.Run("Get empty inline segment copy", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()

			objStream := metabasetest.RandObjectStream()
			copyObjStream := metabasetest.RandObjectStream()
			copyObjStream.ProjectID = objStream.ProjectID
			copyObjStream.Version = 1 // auto assigned the first available version
			objLocation := objStream.Location()
			copyLocation := copyObjStream.Location()

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
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
					SegmentLimit:   10,
				},
				Result: metabase.BeginCopyObjectResult{
					StreamID:             obj.StreamID,
					Version:              obj.Version,
					EncryptedUserData:    obj.EncryptedUserData,
					EncryptedKeysNonces:  encryptedKeyNonces,
					EncryptionParameters: obj.Encryption,
				},
			}.Check(ctx, t, db)

			newEncryptedMetadataKeyNonce := testrand.Nonce()
			newEncryptedMetadataKey := testrand.Bytes(32)

			_, err := db.FinishCopyObject(ctx, metabase.FinishCopyObject{
				ObjectStream:          obj.ObjectStream,
				NewStreamID:           copyObjStream.StreamID,
				NewBucket:             copyObjStream.BucketName,
				NewSegmentKeys:        newEncryptedKeyNonces,
				NewEncryptedObjectKey: copyObjStream.ObjectKey,
				NewEncryptedUserData: metabase.EncryptedUserData{
					EncryptedMetadataNonce:        newEncryptedMetadataKeyNonce.Bytes(),
					EncryptedMetadataEncryptedKey: newEncryptedMetadataKey,
				},
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
						Status:       metabase.CommittedUnversioned,
						SegmentCount: 1,

						Encryption: metabasetest.DefaultEncryption,
					},
					{
						ObjectStream: copyObjStream,
						CreatedAt:    now,
						ExpiresAt:    obj.ExpiresAt,
						Status:       metabase.CommittedUnversioned,
						SegmentCount: 1,

						EncryptedUserData: metabase.EncryptedUserData{
							EncryptedMetadataNonce:        newEncryptedMetadataKeyNonce[:],
							EncryptedMetadataEncryptedKey: newEncryptedMetadataKey,
						},
						Encryption: metabasetest.DefaultEncryption,
					},
				},
				Segments: []metabase.RawSegment{
					metabase.RawSegment(expectedSegment),
					metabase.RawSegment(expectedCopiedSegmentRaw),
				},
			}.Check(ctx, t, db)
		})

		t.Run("Get inline segment copy", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()

			objStream := metabasetest.RandObjectStream()
			copyObjStream := metabasetest.RandObjectStream()
			data := testrand.Bytes(1024)
			copyObjStream.ProjectID = objStream.ProjectID
			copyObjStream.Version = 1 // auto assigned the first available version
			objLocation := objStream.Location()
			copyLocation := copyObjStream.Location()

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
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
					SegmentLimit:   10,
				},
				Result: metabase.BeginCopyObjectResult{
					StreamID:             obj.StreamID,
					Version:              obj.Version,
					EncryptedUserData:    obj.EncryptedUserData,
					EncryptedKeysNonces:  encryptedKeyNonces,
					EncryptionParameters: obj.Encryption,
				},
			}.Check(ctx, t, db)

			newEncryptedMetadataKeyNonce := testrand.Nonce()
			newEncryptedMetadataKey := testrand.Bytes(32)

			_, err := db.FinishCopyObject(ctx, metabase.FinishCopyObject{
				ObjectStream:          obj.ObjectStream,
				NewStreamID:           copyObjStream.StreamID,
				NewBucket:             copyObjStream.BucketName,
				NewSegmentKeys:        newEncryptedKeyNonces,
				NewEncryptedObjectKey: copyObjStream.ObjectKey,
				NewEncryptedUserData: metabase.EncryptedUserData{
					EncryptedMetadataNonce:        newEncryptedMetadataKeyNonce.Bytes(),
					EncryptedMetadataEncryptedKey: newEncryptedMetadataKey,
				},
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
						Status:       metabase.CommittedUnversioned,
						SegmentCount: 1,

						Encryption: metabasetest.DefaultEncryption,

						TotalEncryptedSize: 1024,
					},
					{
						ObjectStream: copyObjStream,
						CreatedAt:    now,
						ExpiresAt:    obj.ExpiresAt,
						Status:       metabase.CommittedUnversioned,
						SegmentCount: 1,

						EncryptedUserData: metabase.EncryptedUserData{
							EncryptedMetadataNonce:        newEncryptedMetadataKeyNonce[:],
							EncryptedMetadataEncryptedKey: newEncryptedMetadataKey,
						},
						Encryption: metabasetest.DefaultEncryption,

						TotalEncryptedSize: 1024,
					},
				},
				Segments: []metabase.RawSegment{
					metabase.RawSegment(expectedSegment),
					metabase.RawSegment(expectedCopiedSegmentRaw),
				},
			}.Check(ctx, t, db)
		})

		t.Run("versioned", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.CreateObjectVersioned(ctx, t, db, obj, 2)

			segments, err := db.TestingAllSegments(ctx)
			require.NoError(t, err)
			require.Len(t, segments, 2)

			metabasetest.GetLatestObjectLastSegment{
				Opts: metabase.GetLatestObjectLastSegment{
					ObjectLocation: location,
				},
				Result: segments[1],
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(object),
				},
				Segments: metabasetest.SegmentsToRaw(segments),
			}.Check(ctx, t, db)
		})

		t.Run("versioned delete marker", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.CreateObjectVersioned(ctx, t, db, obj, 2)

			segments, err := db.TestingAllSegments(ctx)
			require.NoError(t, err)
			require.Len(t, segments, 2)

			markerLocation := obj
			markerLocation.StreamID = uuid.UUID{}
			markerLocation.Version = object.Version + 1
			marker := metabase.Object{
				ObjectStream: markerLocation,
				CreatedAt:    time.Now(),
				Status:       metabase.DeleteMarkerVersioned,
			}

			// this creates a versioned delete marker
			metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: location,
					Versioned:      true,
				},
				Result: metabase.DeleteObjectResult{
					Markers: []metabase.Object{marker},
				},
				OutputMarkerStreamID: &marker.StreamID,
			}.Check(ctx, t, db)

			metabasetest.GetLatestObjectLastSegment{
				Opts: metabase.GetLatestObjectLastSegment{
					ObjectLocation: location,
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "metabase: object or segment missing",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(object),
					metabase.RawObject(marker),
				},
				Segments: metabasetest.SegmentsToRaw(segments),
			}.Check(ctx, t, db)
		})
		t.Run("unversioned delete marker", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			unversioned := metabasetest.CreateObject(ctx, t, db, obj, 2)
			versionedobj := obj
			versionedobj.Version++
			versionedobj.StreamID = testrand.UUID()
			versioned := metabasetest.CreateObjectVersioned(ctx, t, db, versionedobj, 2)

			segments, err := db.TestingAllSegments(ctx)
			require.NoError(t, err)
			require.Len(t, segments, 4)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(unversioned),
					metabase.RawObject(versioned),
				},
				Segments: metabasetest.SegmentsToRaw(segments),
			}.Check(ctx, t, db)

			markerLocation := obj
			markerLocation.StreamID = uuid.UUID{}
			markerLocation.Version = unversioned.Version + 2
			marker := metabase.Object{
				ObjectStream: markerLocation,
				CreatedAt:    time.Now(),
				Status:       metabase.DeleteMarkerUnversioned,
			}

			// this creates a versioned delete marker
			metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: location,
					Versioned:      false,
					Suspended:      true,
				},
				Result: metabase.DeleteObjectResult{
					Markers:             []metabase.Object{marker},
					Removed:             []metabase.Object{unversioned},
					DeletedSegmentCount: 2,
				},
				OutputMarkerStreamID: &marker.StreamID,
			}.Check(ctx, t, db)

			metabasetest.GetLatestObjectLastSegment{
				Opts: metabase.GetLatestObjectLastSegment{
					ObjectLocation: location,
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "metabase: object or segment missing",
			}.Check(ctx, t, db)

			segments = slices.DeleteFunc(segments, func(seg metabase.Segment) bool {
				return seg.StreamID == unversioned.StreamID
			})

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(versioned),
					metabase.RawObject(marker),
				},
				Segments: metabasetest.SegmentsToRaw(segments),
			}.Check(ctx, t, db)
		})
	})
}

func TestBucketEmpty(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

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

			object := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,

					Encryption: metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.BucketEmpty{
				Opts: metabase.BucketEmpty{
					ProjectID:  obj.ProjectID,
					BucketName: obj.BucketName,
				},
				Result: false,
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
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

func TestGetObjectExactVersionLegalHold(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		objStream := metabasetest.RandObjectStream()

		t.Run("Success", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj1, _ := metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
				CommitObject: &metabase.CommitObject{
					ObjectStream: objStream,
					Versioned:    true,
				},
			}.Run(ctx, t, db, objStream, 0)

			objStream2 := objStream
			objStream2.Version++
			obj2, _ := metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: objStream2,
					Encryption:   metabasetest.DefaultEncryption,
					LegalHold:    true,
				},
				CommitObject: &metabase.CommitObject{
					ObjectStream: objStream2,
					Versioned:    true,
				},
			}.Run(ctx, t, db, objStream, 0)

			metabasetest.GetObjectExactVersionLegalHold{
				Opts: metabase.GetObjectExactVersionLegalHold{
					ObjectLocation: objStream.Location(),
					Version:        objStream.Version,
				},
				Result: false,
			}.Check(ctx, t, db)

			metabasetest.GetObjectExactVersionLegalHold{
				Opts: metabase.GetObjectExactVersionLegalHold{
					ObjectLocation: objStream2.Location(),
					Version:        objStream2.Version,
				},
				Result: true,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{metabase.RawObject(obj1), metabase.RawObject(obj2)},
			}.Check(ctx, t, db)
		})

		t.Run("Missing object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.GetObjectExactVersionLegalHold{
				Opts: metabase.GetObjectExactVersionLegalHold{
					ObjectLocation: objStream.Location(),
					Version:        objStream.Version,
				},
				ErrClass: &metabase.ErrObjectNotFound,
			}.Check(ctx, t, db)

			obj, _ := metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
				CommitObject: &metabase.CommitObject{
					ObjectStream: objStream,
					Versioned:    true,
				},
			}.Run(ctx, t, db, objStream, 0)

			metabasetest.GetObjectExactVersionLegalHold{
				Opts: metabase.GetObjectExactVersionLegalHold{
					ObjectLocation: objStream.Location(),
					Version:        objStream.Version + 1,
				},
				ErrClass: &metabase.ErrObjectNotFound,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{metabase.RawObject(obj)},
			}.Check(ctx, t, db)
		})

		t.Run("Delete marker", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
				CommitObject: &metabase.CommitObject{
					ObjectStream: objStream,
					Versioned:    true,
				},
			}.Run(ctx, t, db, objStream, 0)

			markerStream := objStream
			markerStream.StreamID = uuid.UUID{}
			markerStream.Version++
			marker := metabase.Object{
				ObjectStream: markerStream,
				CreatedAt:    time.Now(),
				Status:       metabase.DeleteMarkerVersioned,
			}

			metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: markerStream.Location(),
					Versioned:      true,
				},
				Result: metabase.DeleteObjectResult{
					Markers: []metabase.Object{marker},
				},
				OutputMarkerStreamID: &markerStream.StreamID,
			}.Check(ctx, t, db)

			metabasetest.GetObjectExactVersionLegalHold{
				Opts: metabase.GetObjectExactVersionLegalHold{
					ObjectLocation: markerStream.Location(),
					Version:        markerStream.Version,
				},
				ErrClass: &metabase.ErrMethodNotAllowed,
			}.Check(ctx, t, db)
		})

		t.Run("Pending object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			pending := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.GetObjectExactVersionLegalHold{
				Opts: metabase.GetObjectExactVersionLegalHold{
					ObjectLocation: objStream.Location(),
					Version:        objStream.Version,
				},
				ErrClass: &metabase.ErrMethodNotAllowed,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{metabase.RawObject(pending)},
			}.Check(ctx, t, db)
		})
	})
}

func TestGetObjectLastCommittedLegalHold(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		objStream := metabasetest.RandObjectStream()

		t.Run("Success", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj1, _ := metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
				CommitObject: &metabase.CommitObject{
					ObjectStream: objStream,
					Versioned:    true,
				},
			}.Run(ctx, t, db, objStream, 0)

			metabasetest.GetObjectLastCommittedLegalHold{
				Opts: metabase.GetObjectLastCommittedLegalHold{
					ObjectLocation: objStream.Location(),
				},
			}.Check(ctx, t, db)

			objStream2 := objStream
			objStream2.Version++
			obj2, _ := metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: objStream2,
					Encryption:   metabasetest.DefaultEncryption,
					LegalHold:    true,
				},
				CommitObject: &metabase.CommitObject{
					ObjectStream: objStream2,
					Versioned:    true,
				},
			}.Run(ctx, t, db, objStream2, 0)

			metabasetest.GetObjectLastCommittedLegalHold{
				Opts: metabase.GetObjectLastCommittedLegalHold{
					ObjectLocation: objStream2.Location(),
				},
				Result: true,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{metabase.RawObject(obj1), metabase.RawObject(obj2)},
			}.Check(ctx, t, db)
		})

		t.Run("Missing object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.GetObjectLastCommittedLegalHold{
				Opts: metabase.GetObjectLastCommittedLegalHold{
					ObjectLocation: objStream.Location(),
				},
				ErrClass: &metabase.ErrObjectNotFound,
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete marker", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
				CommitObject: &metabase.CommitObject{
					ObjectStream: objStream,
					Versioned:    true,
				},
			}.Run(ctx, t, db, objStream, 0)

			markerStream := objStream
			markerStream.StreamID = uuid.UUID{}
			markerStream.Version++
			marker := metabase.Object{
				ObjectStream: markerStream,
				CreatedAt:    time.Now(),
				Status:       metabase.DeleteMarkerVersioned,
			}

			metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: markerStream.Location(),
					Versioned:      true,
				},
				Result: metabase.DeleteObjectResult{
					Markers: []metabase.Object{marker},
				},
				OutputMarkerStreamID: &markerStream.StreamID,
			}.Check(ctx, t, db)

			metabasetest.GetObjectLastCommittedLegalHold{
				Opts: metabase.GetObjectLastCommittedLegalHold{
					ObjectLocation: markerStream.Location(),
				},
				ErrClass: &metabase.ErrMethodNotAllowed,
			}.Check(ctx, t, db)
		})

		t.Run("Pending object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			loc := objStream.Location()

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
					LegalHold:    true,
				},
			}.Check(ctx, t, db)

			metabasetest.GetObjectLastCommittedLegalHold{
				Opts: metabase.GetObjectLastCommittedLegalHold{
					ObjectLocation: loc,
				},
				ErrClass: &metabase.ErrObjectNotFound,
			}.Check(ctx, t, db)

			metabasetest.DeleteAll{}.Check(ctx, t, db)

			committed, _ := metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
				CommitObject: &metabase.CommitObject{
					ObjectStream: objStream,
					Versioned:    true,
				},
			}.Run(ctx, t, db, objStream, 0)

			pendingObjStream := objStream
			pendingObjStream.Version++
			pending := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: pendingObjStream,
					Encryption:   metabasetest.DefaultEncryption,
					LegalHold:    true,
				},
			}.Check(ctx, t, db)

			metabasetest.GetObjectLastCommittedLegalHold{
				Opts: metabase.GetObjectLastCommittedLegalHold{
					ObjectLocation: loc,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{metabase.RawObject(pending), metabase.RawObject(committed)},
			}.Check(ctx, t, db)
		})
	})
}

func TestGetObjectExactVersionRetention(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		objStream := metabasetest.RandObjectStream()

		t.Run("Success", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj1, _ := metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
				CommitObject: &metabase.CommitObject{
					ObjectStream: objStream,
					Versioned:    true,
				},
			}.Run(ctx, t, db, objStream, 0)

			retention := metabase.Retention{
				Mode:        storj.ComplianceMode,
				RetainUntil: time.Now().Add(time.Hour),
			}

			objStream2 := objStream
			objStream2.Version++
			obj2, _ := metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: objStream2,
					Encryption:   metabasetest.DefaultEncryption,
					Retention:    retention,
				},
				CommitObject: &metabase.CommitObject{
					ObjectStream: objStream2,
					Versioned:    true,
				},
			}.Run(ctx, t, db, objStream, 0)

			metabasetest.GetObjectExactVersionRetention{
				Opts: metabase.GetObjectExactVersionRetention{
					ObjectLocation: objStream.Location(),
					Version:        objStream.Version,
				},
			}.Check(ctx, t, db)

			metabasetest.GetObjectExactVersionRetention{
				Opts: metabase.GetObjectExactVersionRetention{
					ObjectLocation: objStream2.Location(),
					Version:        objStream2.Version,
				},
				Result: retention,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{metabase.RawObject(obj1), metabase.RawObject(obj2)},
			}.Check(ctx, t, db)
		})

		t.Run("Missing object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.GetObjectExactVersionRetention{
				Opts: metabase.GetObjectExactVersionRetention{
					ObjectLocation: objStream.Location(),
					Version:        objStream.Version,
				},
				ErrClass: &metabase.ErrObjectNotFound,
			}.Check(ctx, t, db)

			obj, _ := metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
				CommitObject: &metabase.CommitObject{
					ObjectStream: objStream,
					Versioned:    true,
				},
			}.Run(ctx, t, db, objStream, 0)

			metabasetest.GetObjectExactVersionRetention{
				Opts: metabase.GetObjectExactVersionRetention{
					ObjectLocation: objStream.Location(),
					Version:        objStream.Version + 1,
				},
				ErrClass: &metabase.ErrObjectNotFound,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{metabase.RawObject(obj)},
			}.Check(ctx, t, db)
		})

		t.Run("Delete marker", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
				CommitObject: &metabase.CommitObject{
					ObjectStream: objStream,
					Versioned:    true,
				},
			}.Run(ctx, t, db, objStream, 0)

			markerStream := objStream
			markerStream.StreamID = uuid.UUID{}
			markerStream.Version++
			marker := metabase.Object{
				ObjectStream: markerStream,
				CreatedAt:    time.Now(),
				Status:       metabase.DeleteMarkerVersioned,
			}

			metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: markerStream.Location(),
					Versioned:      true,
				},
				Result: metabase.DeleteObjectResult{
					Markers: []metabase.Object{marker},
				},
				OutputMarkerStreamID: &markerStream.StreamID,
			}.Check(ctx, t, db)

			metabasetest.GetObjectExactVersionRetention{
				Opts: metabase.GetObjectExactVersionRetention{
					ObjectLocation: markerStream.Location(),
					Version:        markerStream.Version,
				},
				ErrClass: &metabase.ErrMethodNotAllowed,
			}.Check(ctx, t, db)
		})

		t.Run("Pending object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			pending := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.GetObjectExactVersionRetention{
				Opts: metabase.GetObjectExactVersionRetention{
					ObjectLocation: objStream.Location(),
					Version:        objStream.Version,
				},
				ErrClass: &metabase.ErrMethodNotAllowed,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{metabase.RawObject(pending)},
			}.Check(ctx, t, db)
		})
	})
}

func TestGetObjectLastCommittedRetention(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		objStream := metabasetest.RandObjectStream()
		retention := metabase.Retention{
			Mode:        storj.ComplianceMode,
			RetainUntil: time.Now().Add(time.Hour),
		}

		t.Run("Success", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj1, _ := metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
				CommitObject: &metabase.CommitObject{
					ObjectStream: objStream,
					Versioned:    true,
				},
			}.Run(ctx, t, db, objStream, 0)

			metabasetest.GetObjectLastCommittedRetention{
				Opts: metabase.GetObjectLastCommittedRetention{
					ObjectLocation: objStream.Location(),
				},
			}.Check(ctx, t, db)

			objStream2 := objStream
			objStream2.Version++
			obj2, _ := metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: objStream2,
					Encryption:   metabasetest.DefaultEncryption,
					Retention:    retention,
				},
				CommitObject: &metabase.CommitObject{
					ObjectStream: objStream2,
					Versioned:    true,
				},
			}.Run(ctx, t, db, objStream2, 0)

			metabasetest.GetObjectLastCommittedRetention{
				Opts: metabase.GetObjectLastCommittedRetention{
					ObjectLocation: objStream2.Location(),
				},
				Result: retention,
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{metabase.RawObject(obj1), metabase.RawObject(obj2)},
			}.Check(ctx, t, db)
		})

		t.Run("Missing object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.GetObjectLastCommittedRetention{
				Opts: metabase.GetObjectLastCommittedRetention{
					ObjectLocation: objStream.Location(),
				},
				ErrClass: &metabase.ErrObjectNotFound,
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete marker", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
				CommitObject: &metabase.CommitObject{
					ObjectStream: objStream,
					Versioned:    true,
				},
			}.Run(ctx, t, db, objStream, 0)

			markerStream := objStream
			markerStream.StreamID = uuid.UUID{}
			markerStream.Version++
			marker := metabase.Object{
				ObjectStream: markerStream,
				CreatedAt:    time.Now(),
				Status:       metabase.DeleteMarkerVersioned,
			}

			metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: markerStream.Location(),
					Versioned:      true,
				},
				Result: metabase.DeleteObjectResult{
					Markers: []metabase.Object{marker},
				},
				OutputMarkerStreamID: &markerStream.StreamID,
			}.Check(ctx, t, db)

			metabasetest.GetObjectLastCommittedRetention{
				Opts: metabase.GetObjectLastCommittedRetention{
					ObjectLocation: markerStream.Location(),
				},
				ErrClass: &metabase.ErrMethodNotAllowed,
			}.Check(ctx, t, db)
		})

		t.Run("Pending object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			loc := objStream.Location()

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
					Retention:    retention,
				},
			}.Check(ctx, t, db)

			metabasetest.GetObjectLastCommittedRetention{
				Opts: metabase.GetObjectLastCommittedRetention{
					ObjectLocation: loc,
				},
				ErrClass: &metabase.ErrObjectNotFound,
			}.Check(ctx, t, db)

			metabasetest.DeleteAll{}.Check(ctx, t, db)

			committed, _ := metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
				},
				CommitObject: &metabase.CommitObject{
					ObjectStream: objStream,
					Versioned:    true,
				},
			}.Run(ctx, t, db, objStream, 0)

			pendingObjStream := objStream
			pendingObjStream.Version++
			pending := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: pendingObjStream,
					Encryption:   metabasetest.DefaultEncryption,
					Retention:    retention,
				},
			}.Check(ctx, t, db)

			metabasetest.GetObjectLastCommittedRetention{
				Opts: metabase.GetObjectLastCommittedRetention{
					ObjectLocation: loc,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{metabase.RawObject(pending), metabase.RawObject(committed)},
			}.Check(ctx, t, db)
		})
	})
}
