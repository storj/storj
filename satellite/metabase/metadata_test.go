// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"
	"time"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestUpdateObjectMetadata(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()
		now := time.Now()
		zombieDeadline := now.Add(24 * time.Hour)

		for _, test := range metabasetest.InvalidObjectLocations(obj.Location()) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)
				metabasetest.UpdateObjectMetadata{
					Opts: metabase.UpdateObjectMetadata{
						ProjectID:  test.ObjectLocation.ProjectID,
						BucketName: test.ObjectLocation.BucketName,
						ObjectKey:  test.ObjectLocation.ObjectKey,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)
				metabasetest.Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("StreamID missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			metabasetest.UpdateObjectMetadata{
				Opts: metabase.UpdateObjectMetadata{
					ProjectID:  obj.ProjectID,
					BucketName: obj.BucketName,
					ObjectKey:  obj.ObjectKey,
					StreamID:   uuid.UUID{},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "StreamID missing",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Metadata missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.UpdateObjectMetadata{
				Opts: metabase.UpdateObjectMetadata{
					ProjectID:  obj.ProjectID,
					BucketName: obj.BucketName,
					ObjectKey:  obj.ObjectKey,
					StreamID:   obj.StreamID,
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "object with specified version and committed status is missing",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Update metadata", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.CreateTestObject{}.Run(ctx, t, db, obj, 0)

			encryptedMetadata := testrand.Bytes(1024)
			encryptedMetadataNonce := testrand.Nonce()
			encryptedMetadataKey := testrand.Bytes(265)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Committed,
						Encryption:   metabasetest.DefaultEncryption,
					},
				},
			}.Check(ctx, t, db)

			metabasetest.UpdateObjectMetadata{
				Opts: metabase.UpdateObjectMetadata{
					ProjectID:                     obj.ProjectID,
					BucketName:                    obj.BucketName,
					ObjectKey:                     obj.ObjectKey,
					StreamID:                      obj.StreamID,
					EncryptedMetadata:             encryptedMetadata,
					EncryptedMetadataNonce:        encryptedMetadataNonce[:],
					EncryptedMetadataEncryptedKey: encryptedMetadataKey,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.Committed,
						Encryption:   metabasetest.DefaultEncryption,

						EncryptedMetadata:             encryptedMetadata,
						EncryptedMetadataNonce:        encryptedMetadataNonce[:],
						EncryptedMetadataEncryptedKey: encryptedMetadataKey,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("Update metadata with version != 1", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			newObj := metabasetest.RandObjectStream()
			metabasetest.CreatePendingObject(ctx, t, db, newObj, 0)

			newObjDiffVersion := newObj
			newObjDiffVersion.Version = newObj.Version + 10000
			metabasetest.CreateTestObject{}.Run(ctx, t, db, newObjDiffVersion, 0)

			encryptedMetadata := testrand.Bytes(1024)
			encryptedMetadataNonce := testrand.Nonce()
			encryptedMetadataKey := testrand.Bytes(265)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream:           newObj,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieDeadline,
					},
					{
						ObjectStream: newObjDiffVersion,
						CreatedAt:    now,
						Status:       metabase.Committed,
						Encryption:   metabasetest.DefaultEncryption,
					},
				},
			}.Check(ctx, t, db)

			metabasetest.UpdateObjectMetadata{
				Opts: metabase.UpdateObjectMetadata{
					ProjectID:                     newObj.ProjectID,
					BucketName:                    newObj.BucketName,
					ObjectKey:                     newObj.ObjectKey,
					StreamID:                      newObj.StreamID,
					EncryptedMetadata:             encryptedMetadata,
					EncryptedMetadataNonce:        encryptedMetadataNonce[:],
					EncryptedMetadataEncryptedKey: encryptedMetadataKey,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream:           newObj,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						Encryption:             metabasetest.DefaultEncryption,
						ZombieDeletionDeadline: &zombieDeadline,
					},
					{
						ObjectStream: newObjDiffVersion,
						CreatedAt:    now,
						Status:       metabase.Committed,
						Encryption:   metabasetest.DefaultEncryption,

						EncryptedMetadata:             encryptedMetadata,
						EncryptedMetadataNonce:        encryptedMetadataNonce[:],
						EncryptedMetadataEncryptedKey: encryptedMetadataKey,
					},
				},
			}.Check(ctx, t, db)
		})
	})
}
