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
	"storj.io/storj/satellite/metainfo/metabase"
)

func TestDeletePendingObject(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := randObjectStream()

		location := obj.Location()

		now := time.Now()

		for _, test := range invalidObjectLocations(location) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer DeleteAll{}.Check(ctx, t, db)
				DeletePendingObject{
					Opts: metabase.DeletePendingObject{
						StreamID:       obj.StreamID,
						Version:        1,
						ObjectLocation: test.ObjectLocation,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)
				Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("Version invalid", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			DeletePendingObject{
				Opts: metabase.DeletePendingObject{
					StreamID:       obj.StreamID,
					Version:        0,
					ObjectLocation: location,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Version invalid: 0",
			}.Check(ctx, t, db)
			Verify{}.Check(ctx, t, db)
		})

		t.Run("StreamID missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			DeletePendingObject{
				Opts: metabase.DeletePendingObject{
					StreamID:       uuid.UUID{},
					Version:        1,
					ObjectLocation: location,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "StreamID missing",
			}.Check(ctx, t, db)
			Verify{}.Check(ctx, t, db)
		})

		t.Run("Object missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			DeletePendingObject{
				Opts: metabase.DeletePendingObject{
					StreamID:       obj.StreamID,
					Version:        1,
					ObjectLocation: location,
				},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: no rows deleted",
			}.Check(ctx, t, db)
			Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete non existing object version", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   defaultTestEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			DeletePendingObject{
				Opts: metabase.DeletePendingObject{
					StreamID:       obj.StreamID,
					Version:        33,
					ObjectLocation: location,
				},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: no rows deleted",
			}.Check(ctx, t, db)
			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Pending,

						Encryption: defaultTestEncryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("Delete committed object", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			object := createObject(ctx, t, db, obj, 0)

			DeletePendingObject{
				Opts: metabase.DeletePendingObject{
					StreamID:       object.StreamID,
					Version:        1,
					ObjectLocation: object.Location(),
				},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: no rows deleted",
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Committed,

						Encryption: defaultTestEncryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("Delete pending object without segments with wrong StreamID", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   defaultTestEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			DeletePendingObject{
				Opts: metabase.DeletePendingObject{
					StreamID:       uuid.UUID{33},
					Version:        1,
					ObjectLocation: location,
				},
				Result:   metabase.DeleteObjectResult{},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: no rows deleted",
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Pending,

						Encryption: defaultTestEncryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("Delete pending object without segments", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   defaultTestEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			object := metabase.RawObject{
				ObjectStream: obj,
				CreatedAt:    now,
				Status:       metabase.Pending,
				Encryption:   defaultTestEncryption,
			}
			DeletePendingObject{
				Opts: metabase.DeletePendingObject{
					StreamID:       obj.StreamID,
					Version:        1,
					ObjectLocation: location,
				},
				Result: metabase.DeleteObjectResult{
					Objects: []metabase.Object{metabase.Object(object)},
				},
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete pending object with segments", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			createPendingObject(ctx, t, db, obj, 2)

			expectedSegmentInfo := metabase.DeletedSegmentInfo{
				RootPieceID: storj.PieceID{1},
				Pieces:      metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
			}

			DeletePendingObject{
				Opts: metabase.DeletePendingObject{
					StreamID:       obj.StreamID,
					Version:        1,
					ObjectLocation: location,
				},
				Result: metabase.DeleteObjectResult{
					Objects: []metabase.Object{
						{
							ObjectStream: obj,
							CreatedAt:    now,
							Status:       metabase.Pending,
							Encryption:   defaultTestEncryption,
						},
					},
					Segments: []metabase.DeletedSegmentInfo{expectedSegmentInfo, expectedSegmentInfo},
				},
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete pending object with inline segment", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   defaultTestEncryption,
				},
				Version: obj.Version,
			}.Check(ctx, t, db)

			CommitInlineSegment{
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

			DeletePendingObject{
				Opts: metabase.DeletePendingObject{
					StreamID:       obj.StreamID,
					Version:        1,
					ObjectLocation: location,
				},
				Result: metabase.DeleteObjectResult{
					Objects: []metabase.Object{
						{
							ObjectStream: obj,
							CreatedAt:    now,
							Status:       metabase.Pending,
							Encryption:   defaultTestEncryption,
						},
					},
				},
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})
	})
}

func TestDeleteObjectExactVersion(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := randObjectStream()

		location := obj.Location()

		now := time.Now()

		for _, test := range invalidObjectLocations(location) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer DeleteAll{}.Check(ctx, t, db)
				DeleteObjectExactVersion{
					Opts: metabase.DeleteObjectExactVersion{
						ObjectLocation: test.ObjectLocation,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)
				Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("Version invalid", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			DeleteObjectExactVersion{
				Opts: metabase.DeleteObjectExactVersion{
					ObjectLocation: location,
					Version:        0,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Version invalid: 0",
			}.Check(ctx, t, db)
			Verify{}.Check(ctx, t, db)
		})

		t.Run("Object missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			DeleteObjectExactVersion{
				Opts: metabase.DeleteObjectExactVersion{
					ObjectLocation: location,
					Version:        1,
				},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: no rows deleted",
			}.Check(ctx, t, db)
			Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete non existing object version", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			DeleteObjectExactVersion{
				Opts: metabase.DeleteObjectExactVersion{
					ObjectLocation: location,
					Version:        33,
				},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: no rows deleted",
			}.Check(ctx, t, db)
			Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete partial object", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   defaultTestEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			DeleteObjectExactVersion{
				Opts: metabase.DeleteObjectExactVersion{
					ObjectLocation: location,
					Version:        1,
				},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: no rows deleted",
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Pending,

						Encryption: defaultTestEncryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("Delete object without segments", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			encryptedMetadata := testrand.Bytes(1024)
			encryptedMetadataNonce := testrand.Nonce()
			encryptedMetadataKey := testrand.Bytes(265)

			object := CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:                  obj,
					EncryptedMetadataNonce:        encryptedMetadataNonce[:],
					EncryptedMetadata:             encryptedMetadata,
					EncryptedMetadataEncryptedKey: encryptedMetadataKey,
				},
			}.Run(ctx, t, db, obj, 0)

			DeleteObjectExactVersion{
				Opts: metabase.DeleteObjectExactVersion{
					ObjectLocation: location,
					Version:        1,
				},
				Result: metabase.DeleteObjectResult{
					Objects: []metabase.Object{object},
				},
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete object with segments", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			object := createObject(ctx, t, db, obj, 2)

			expectedSegmentInfo := metabase.DeletedSegmentInfo{
				RootPieceID: storj.PieceID{1},
				Pieces:      metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
			}

			DeleteObjectExactVersion{
				Opts: metabase.DeleteObjectExactVersion{
					ObjectLocation: location,
					Version:        1,
				},
				Result: metabase.DeleteObjectResult{
					Objects:  []metabase.Object{object},
					Segments: []metabase.DeletedSegmentInfo{expectedSegmentInfo, expectedSegmentInfo},
				},
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete object with inline segment", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   defaultTestEncryption,
				},
				Version: obj.Version,
			}.Check(ctx, t, db)

			CommitInlineSegment{
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

			object := CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
				},
			}.Check(ctx, t, db)

			DeleteObjectExactVersion{
				Opts: metabase.DeleteObjectExactVersion{
					ObjectLocation: location,
					Version:        1,
				},
				Result: metabase.DeleteObjectResult{
					Objects: []metabase.Object{object},
				},
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})
	})
}

func TestDeleteObjectLatestVersion(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := randObjectStream()

		location := obj.Location()

		now := time.Now()

		for _, test := range invalidObjectLocations(location) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer DeleteAll{}.Check(ctx, t, db)
				DeleteObjectLatestVersion{
					Opts: metabase.DeleteObjectLatestVersion{
						ObjectLocation: test.ObjectLocation,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)
				Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("Object missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			DeleteObjectLatestVersion{
				Opts:     metabase.DeleteObjectLatestVersion{ObjectLocation: location},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: no rows deleted",
			}.Check(ctx, t, db)
			Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete non existing object version", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			DeleteObjectLatestVersion{
				Opts:     metabase.DeleteObjectLatestVersion{ObjectLocation: location},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: no rows deleted",
			}.Check(ctx, t, db)
			Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete partial object", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   defaultTestEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			DeleteObjectLatestVersion{
				Opts:     metabase.DeleteObjectLatestVersion{ObjectLocation: obj.Location()},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: no rows deleted",
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Pending,

						Encryption: defaultTestEncryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("Delete object without segments", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			encryptedMetadata := testrand.Bytes(1024)
			encryptedMetadataNonce := testrand.Nonce()
			encryptedMetadataKey := testrand.Bytes(265)

			object := CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:                  obj,
					EncryptedMetadataNonce:        encryptedMetadataNonce[:],
					EncryptedMetadata:             encryptedMetadata,
					EncryptedMetadataEncryptedKey: encryptedMetadataKey,
				},
			}.Run(ctx, t, db, obj, 0)

			DeleteObjectLatestVersion{
				Opts: metabase.DeleteObjectLatestVersion{
					ObjectLocation: obj.Location(),
				},
				Result: metabase.DeleteObjectResult{
					Objects: []metabase.Object{object},
				},
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete object with segments", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			object := createObject(ctx, t, db, obj, 2)

			expectedSegmentInfo := metabase.DeletedSegmentInfo{
				RootPieceID: storj.PieceID{1},
				Pieces:      metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
			}

			DeleteObjectLatestVersion{
				Opts: metabase.DeleteObjectLatestVersion{
					ObjectLocation: location,
				},
				Result: metabase.DeleteObjectResult{
					Objects:  []metabase.Object{object},
					Segments: []metabase.DeletedSegmentInfo{expectedSegmentInfo, expectedSegmentInfo},
				},
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete object with inline segment", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   defaultTestEncryption,
				},
				Version: obj.Version,
			}.Check(ctx, t, db)

			CommitInlineSegment{
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

			object := CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
				},
			}.Check(ctx, t, db)

			DeleteObjectLatestVersion{
				Opts: metabase.DeleteObjectLatestVersion{
					ObjectLocation: obj.Location(),
				},
				Result: metabase.DeleteObjectResult{
					Objects: []metabase.Object{object},
				},
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete latest from multiple versions", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			obj := randObjectStream()

			// first version
			obj.Version = metabase.Version(10)
			createObject(ctx, t, db, obj, 1)

			// second version, to delete
			secondObject := metabase.ObjectStream{
				ProjectID:  obj.ProjectID,
				BucketName: obj.BucketName,
				ObjectKey:  obj.ObjectKey,
				Version:    11,
				StreamID:   testrand.UUID(),
			}
			object := createObject(ctx, t, db, secondObject, 1)

			expectedSegmentInfo := metabase.DeletedSegmentInfo{
				RootPieceID: storj.PieceID{1},
				Pieces:      metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
			}

			DeleteObjectLatestVersion{
				Opts: metabase.DeleteObjectLatestVersion{
					ObjectLocation: obj.Location(),
				},
				Result: metabase.DeleteObjectResult{
					Objects: []metabase.Object{object},
					Segments: []metabase.DeletedSegmentInfo{
						expectedSegmentInfo,
					},
				},
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Committed,
						SegmentCount: 1,

						TotalPlainSize:     512,
						TotalEncryptedSize: 1024,
						FixedSegmentSize:   512,
						Encryption:         defaultTestEncryption,
					},
				},
				Segments: []metabase.RawSegment{
					{
						StreamID:  obj.StreamID,
						Position:  metabase.SegmentPosition{Part: 0, Index: 0},
						CreatedAt: &now,

						RootPieceID:       storj.PieceID{1},
						Pieces:            metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
						EncryptedKey:      []byte{3},
						EncryptedKeyNonce: []byte{4},
						EncryptedETag:     []byte{5},

						EncryptedSize: 1024,
						PlainSize:     512,
						PlainOffset:   0,

						Redundancy: defaultTestRedundancy,
					},
				},
			}.Check(ctx, t, db)
		})
	})
}

func TestDeleteObjectAnyStatusAllVersions(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := randObjectStream()

		location := obj.Location()

		now := time.Now()

		for _, test := range invalidObjectLocations(location) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer DeleteAll{}.Check(ctx, t, db)
				DeleteObjectAnyStatusAllVersions{
					Opts:     metabase.DeleteObjectAnyStatusAllVersions{ObjectLocation: test.ObjectLocation},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)
				Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("Object missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			DeleteObjectAnyStatusAllVersions{
				Opts:     metabase.DeleteObjectAnyStatusAllVersions{ObjectLocation: obj.Location()},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: no rows deleted",
			}.Check(ctx, t, db)
			Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete non existing object version", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			DeleteObjectAnyStatusAllVersions{
				Opts:     metabase.DeleteObjectAnyStatusAllVersions{ObjectLocation: obj.Location()},
				ErrClass: &storj.ErrObjectNotFound,
				ErrText:  "metabase: no rows deleted",
			}.Check(ctx, t, db)
			Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete partial object", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   defaultTestEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			DeleteObjectAnyStatusAllVersions{
				Opts: metabase.DeleteObjectAnyStatusAllVersions{ObjectLocation: obj.Location()},
				Result: metabase.DeleteObjectResult{
					Objects: []metabase.Object{{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Pending,

						Encryption: defaultTestEncryption,
					}},
				},
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete object without segments", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			encryptedMetadata := testrand.Bytes(1024)
			encryptedMetadataNonce := testrand.Nonce()
			encryptedMetadataKey := testrand.Bytes(265)

			object := CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:                  obj,
					EncryptedMetadataNonce:        encryptedMetadataNonce[:],
					EncryptedMetadata:             encryptedMetadata,
					EncryptedMetadataEncryptedKey: encryptedMetadataKey,
				},
			}.Run(ctx, t, db, obj, 0)

			DeleteObjectAnyStatusAllVersions{
				Opts: metabase.DeleteObjectAnyStatusAllVersions{ObjectLocation: obj.Location()},
				Result: metabase.DeleteObjectResult{
					Objects: []metabase.Object{object},
				},
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete object with segments", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			object := createObject(ctx, t, db, obj, 2)

			expectedSegmentInfo := metabase.DeletedSegmentInfo{
				RootPieceID: storj.PieceID{1},
				Pieces:      metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
			}

			DeleteObjectAnyStatusAllVersions{
				Opts: metabase.DeleteObjectAnyStatusAllVersions{
					ObjectLocation: location,
				},
				Result: metabase.DeleteObjectResult{
					Objects:  []metabase.Object{object},
					Segments: []metabase.DeletedSegmentInfo{expectedSegmentInfo, expectedSegmentInfo},
				},
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete object with inline segment", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   defaultTestEncryption,
				},
				Version: obj.Version,
			}.Check(ctx, t, db)

			CommitInlineSegment{
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

			object := CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
				},
			}.Check(ctx, t, db)

			DeleteObjectAnyStatusAllVersions{
				Opts: metabase.DeleteObjectAnyStatusAllVersions{ObjectLocation: obj.Location()},
				Result: metabase.DeleteObjectResult{
					Objects: []metabase.Object{object},
				},
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete multiple versions of the same object at once", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			expected := metabase.DeleteObjectResult{}

			obj := randObjectStream()
			for i := 1; i <= 10; i++ {
				obj.StreamID = testrand.UUID()
				obj.Version = metabase.Version(i)
				expected.Objects = append(expected.Objects, createObject(ctx, t, db, obj, 1))
				expected.Segments = append(expected.Segments, metabase.DeletedSegmentInfo{
					RootPieceID: storj.PieceID{1},
					Pieces:      metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
				})
			}

			DeleteObjectAnyStatusAllVersions{
				Opts:   metabase.DeleteObjectAnyStatusAllVersions{ObjectLocation: obj.Location()},
				Result: expected,
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})
	})
}

func TestDeleteObjectsAllVersions(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := randObjectStream()

		location := obj.Location()

		now := time.Now()

		for _, test := range invalidObjectLocations(location) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer DeleteAll{}.Check(ctx, t, db)
				DeleteObjectsAllVersions{
					Opts: metabase.DeleteObjectsAllVersions{
						Locations: []metabase.ObjectLocation{test.ObjectLocation},
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)
				Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("Delete two objects from different projects", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			obj2 := randObjectStream()

			DeleteObjectsAllVersions{
				Opts: metabase.DeleteObjectsAllVersions{
					Locations: []metabase.ObjectLocation{location, obj2.Location()},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "all objects must be in the same bucket",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete two objects from same project, but different buckets", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			obj2 := randObjectStream()
			obj2.ProjectID = obj.ProjectID

			DeleteObjectsAllVersions{
				Opts: metabase.DeleteObjectsAllVersions{
					Locations: []metabase.ObjectLocation{location, obj2.Location()},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "all objects must be in the same bucket",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete empty list of objects", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			DeleteObjectsAllVersions{}.Check(ctx, t, db)
			Verify{}.Check(ctx, t, db)
		})

		t.Run("Object missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			DeleteObjectsAllVersions{
				Opts: metabase.DeleteObjectsAllVersions{
					Locations: []metabase.ObjectLocation{location},
				},
			}.Check(ctx, t, db)
			Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete partial object", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   defaultTestEncryption,
				},
				Version: 1,
			}.Check(ctx, t, db)

			DeleteObjectsAllVersions{
				Opts: metabase.DeleteObjectsAllVersions{
					Locations: []metabase.ObjectLocation{location},
				},
			}.Check(ctx, t, db)

			Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Pending,

						Encryption: defaultTestEncryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("Delete object without segments", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			object := createObject(ctx, t, db, obj, 0)

			DeleteObjectsAllVersions{
				Opts: metabase.DeleteObjectsAllVersions{
					Locations: []metabase.ObjectLocation{location},
				},
				Result: metabase.DeleteObjectResult{
					Objects: []metabase.Object{object},
				},
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete object with segments", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			object := createObject(ctx, t, db, obj, 2)

			expectedSegmentInfo := metabase.DeletedSegmentInfo{
				RootPieceID: storj.PieceID{1},
				Pieces:      metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
			}

			DeleteObjectsAllVersions{
				Opts: metabase.DeleteObjectsAllVersions{
					Locations: []metabase.ObjectLocation{location},
				},
				Result: metabase.DeleteObjectResult{
					Objects:  []metabase.Object{object},
					Segments: []metabase.DeletedSegmentInfo{expectedSegmentInfo, expectedSegmentInfo},
				},
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete two objects with segments from same bucket", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			obj2 := randObjectStream()
			obj2.ProjectID = obj.ProjectID
			obj2.BucketName = obj.BucketName

			object1 := createObject(ctx, t, db, obj, 1)
			object2 := createObject(ctx, t, db, obj2, 2)

			expectedSegmentInfo := metabase.DeletedSegmentInfo{
				RootPieceID: storj.PieceID{1},
				Pieces:      metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
			}

			DeleteObjectsAllVersions{
				Opts: metabase.DeleteObjectsAllVersions{
					Locations: []metabase.ObjectLocation{location, obj2.Location()},
				},
				Result: metabase.DeleteObjectResult{
					Objects:  []metabase.Object{object1, object2},
					Segments: []metabase.DeletedSegmentInfo{expectedSegmentInfo, expectedSegmentInfo, expectedSegmentInfo},
				},
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete object with inline segment", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   defaultTestEncryption,
				},
				Version: obj.Version,
			}.Check(ctx, t, db)

			CommitInlineSegment{
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

			object := CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
				},
			}.Check(ctx, t, db)

			DeleteObjectsAllVersions{
				Opts: metabase.DeleteObjectsAllVersions{
					Locations: []metabase.ObjectLocation{location},
				},
				Result: metabase.DeleteObjectResult{
					Objects: []metabase.Object{object},
				},
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete object with inline segment and object with remote segments", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   defaultTestEncryption,
				},
				Version: obj.Version,
			}.Check(ctx, t, db)

			CommitInlineSegment{
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

			object1 := CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
				},
			}.Check(ctx, t, db)

			obj2 := randObjectStream()
			obj2.ProjectID = obj.ProjectID
			obj2.BucketName = obj.BucketName

			object2 := createObject(ctx, t, db, obj2, 2)

			expectedSegmentInfo := metabase.DeletedSegmentInfo{
				RootPieceID: storj.PieceID{1},
				Pieces:      metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
			}

			DeleteObjectsAllVersions{
				Opts: metabase.DeleteObjectsAllVersions{
					Locations: []metabase.ObjectLocation{location, object2.Location()},
				},
				Result: metabase.DeleteObjectResult{
					Objects:  []metabase.Object{object1, object2},
					Segments: []metabase.DeletedSegmentInfo{expectedSegmentInfo, expectedSegmentInfo},
				},
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete multiple versions of the same object at once", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			expected := metabase.DeleteObjectResult{}

			for i := 1; i <= 10; i++ {
				obj.StreamID = testrand.UUID()
				obj.Version = metabase.Version(i)
				expected.Objects = append(expected.Objects, createObject(ctx, t, db, obj, 1))
				expected.Segments = append(expected.Segments, metabase.DeletedSegmentInfo{
					RootPieceID: storj.PieceID{1},
					Pieces:      metabase.Pieces{{Number: 0, StorageNode: storj.NodeID{2}}},
				})
			}

			DeleteObjectsAllVersions{
				Opts: metabase.DeleteObjectsAllVersions{
					Locations: []metabase.ObjectLocation{location},
				},
				Result: expected,
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})
	})
}

func createPendingObject(ctx *testcontext.Context, t *testing.T, db *metabase.DB, obj metabase.ObjectStream, numberOfSegments byte) {
	BeginObjectExactVersion{
		Opts: metabase.BeginObjectExactVersion{
			ObjectStream: obj,
			Encryption:   defaultTestEncryption,
		},
		Version: obj.Version,
	}.Check(ctx, t, db)

	for i := byte(0); i < numberOfSegments; i++ {
		BeginSegment{
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

		CommitSegment{
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
				Redundancy:    defaultTestRedundancy,
			},
		}.Check(ctx, t, db)
	}
}

func createObject(ctx *testcontext.Context, t *testing.T, db *metabase.DB, obj metabase.ObjectStream, numberOfSegments byte) metabase.Object {
	BeginObjectExactVersion{
		Opts: metabase.BeginObjectExactVersion{
			ObjectStream: obj,
			Encryption:   defaultTestEncryption,
		},
		Version: obj.Version,
	}.Check(ctx, t, db)

	for i := byte(0); i < numberOfSegments; i++ {
		BeginSegment{
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

		CommitSegment{
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
				Redundancy:    defaultTestRedundancy,
			},
		}.Check(ctx, t, db)
	}

	return CommitObject{
		Opts: metabase.CommitObject{
			ObjectStream: obj,
		},
	}.Check(ctx, t, db)
}
