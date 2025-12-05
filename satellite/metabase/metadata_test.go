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

func TestUpdateObjectLastCommittedMetadata(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()
		for _, test := range metabasetest.InvalidObjectLocations(obj.Location()) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)
				metabasetest.UpdateObjectLastCommittedMetadata{
					Opts: metabase.UpdateObjectLastCommittedMetadata{
						ObjectLocation: test.ObjectLocation,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)
				metabasetest.Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("StreamID missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()
			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: metabase.UpdateObjectLastCommittedMetadata{
					ObjectLocation: obj.Location(),
					StreamID:       uuid.UUID{},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "StreamID missing",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Metadata missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()
			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: metabase.UpdateObjectLastCommittedMetadata{
					ObjectLocation: obj.Location(),
					StreamID:       obj.StreamID,
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "object with specified version and committed status is missing",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Update metadata", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()
			object := metabasetest.CreateObject(ctx, t, db, obj, 0)

			userData := metabasetest.RandEncryptedUserDataWithoutETag()

			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: metabase.UpdateObjectLastCommittedMetadata{
					ObjectLocation:    object.Location(),
					StreamID:          object.StreamID,
					EncryptedUserData: userData,
				},
			}.Check(ctx, t, db)

			object.EncryptedUserData = userData

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(object),
				},
			}.Check(ctx, t, db)

			userData.EncryptedETag = testrand.Bytes(32)

			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: metabase.UpdateObjectLastCommittedMetadata{
					ObjectLocation:    object.Location(),
					StreamID:          object.StreamID,
					EncryptedUserData: userData,
					SetEncryptedETag:  true,
				},
			}.Check(ctx, t, db)

			object.EncryptedUserData = userData

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(object),
				},
			}.Check(ctx, t, db)
		})

		t.Run("Update metadata with version != 1", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()
			object := metabasetest.CreatePendingObject(ctx, t, db, obj, 0)

			obj2 := obj
			obj2.Version++
			object2 := metabasetest.CreateObject(ctx, t, db, obj2, 0)

			userData := metabasetest.RandEncryptedUserData()

			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: metabase.UpdateObjectLastCommittedMetadata{
					ObjectLocation:    object2.Location(),
					StreamID:          object2.StreamID,
					EncryptedUserData: userData,
					SetEncryptedETag:  true,
				},
			}.Check(ctx, t, db)

			object2.EncryptedUserData = userData

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(object),
					metabase.RawObject(object2),
				},
			}.Check(ctx, t, db)
		})

		t.Run("update metadata of versioned object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()
			object := metabasetest.CreateObjectVersioned(ctx, t, db, obj, 0)

			userData := metabasetest.RandEncryptedUserData()

			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: metabase.UpdateObjectLastCommittedMetadata{
					ObjectLocation:    object.Location(),
					StreamID:          object.StreamID,
					EncryptedUserData: userData,
					SetEncryptedETag:  true,
				},
			}.Check(ctx, t, db)

			object.EncryptedUserData = userData

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(object),
				},
			}.Check(ctx, t, db)
		})

		t.Run("update metadata of versioned delete marker", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()
			object := metabasetest.CreateObjectVersioned(ctx, t, db, obj, 0)

			userData := metabasetest.RandEncryptedUserDataWithoutETag()

			marker := metabase.Object{
				ObjectStream: object.ObjectStream,
				Status:       metabase.DeleteMarkerVersioned,
				CreatedAt:    time.Now(),
			}
			marker.Version++

			metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: object.Location(),
					Versioned:      true,
				},
				Result: metabase.DeleteObjectResult{
					Markers: []metabase.Object{marker},
				},
				OutputMarkerStreamID: &marker.StreamID,
			}.Check(ctx, t, db)

			// verify we cannot update the metadata of a deleted object
			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: metabase.UpdateObjectLastCommittedMetadata{
					ObjectLocation:    object.Location(),
					StreamID:          object.StreamID,
					EncryptedUserData: userData,
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "object with specified version and committed status is missing",
			}.Check(ctx, t, db)

			// verify cannot update the metadata of the delete marker either
			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: metabase.UpdateObjectLastCommittedMetadata{
					ObjectLocation:    marker.Location(),
					StreamID:          marker.StreamID,
					EncryptedUserData: userData,
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "object with specified version and committed status is missing",
			}.Check(ctx, t, db)

			userDataWithETag := metabasetest.RandEncryptedUserData()

			// verify we cannot update the metadata with set etag of a deleted object
			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: metabase.UpdateObjectLastCommittedMetadata{
					ObjectLocation:    object.Location(),
					StreamID:          object.StreamID,
					EncryptedUserData: userDataWithETag,
					SetEncryptedETag:  true,
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "object with specified version and committed status is missing",
			}.Check(ctx, t, db)

			// verify cannot update the metadata with etag of the delete marker either
			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: metabase.UpdateObjectLastCommittedMetadata{
					ObjectLocation:    marker.Location(),
					StreamID:          marker.StreamID,
					EncryptedUserData: userDataWithETag,
					SetEncryptedETag:  true,
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "object with specified version and committed status is missing",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(object),
					metabase.RawObject(marker),
				},
			}.Check(ctx, t, db)
		})

		t.Run("update metadata of unversioned delete marker", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()
			object := metabasetest.CreateObjectVersioned(ctx, t, db, obj, 0)

			obj2 := obj
			obj2.Version++

			object2 := metabasetest.CreateObject(ctx, t, db, obj2, 0)

			marker := metabase.Object{
				ObjectStream: object2.ObjectStream,
				Status:       metabase.DeleteMarkerUnversioned,
				CreatedAt:    time.Now(),
			}
			marker.Version++

			metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: object2.Location(),
					Versioned:      false,
					Suspended:      true,
				},
				Result: metabase.DeleteObjectResult{
					Markers: []metabase.Object{marker},
					Removed: []metabase.Object{object2},
				},
				OutputMarkerStreamID: &marker.StreamID,
			}.Check(ctx, t, db)

			userData := metabasetest.RandEncryptedUserDataWithoutETag()

			// verify we cannot update the metadata of a deleted object
			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: metabase.UpdateObjectLastCommittedMetadata{
					ObjectLocation:    object2.Location(),
					StreamID:          object2.StreamID,
					EncryptedUserData: userData,
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "object with specified version and committed status is missing",
			}.Check(ctx, t, db)

			// verify cannot update the metadata of the delete marker either
			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: metabase.UpdateObjectLastCommittedMetadata{
					ObjectLocation:    marker.Location(),
					StreamID:          marker.StreamID,
					EncryptedUserData: userData,
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "object with specified version and committed status is missing",
			}.Check(ctx, t, db)

			userDataWithETag := metabasetest.RandEncryptedUserData()

			// verify we cannot update the metadata with etag of a deleted object
			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: metabase.UpdateObjectLastCommittedMetadata{
					ObjectLocation:    object2.Location(),
					StreamID:          object2.StreamID,
					EncryptedUserData: userDataWithETag,
					SetEncryptedETag:  true,
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "object with specified version and committed status is missing",
			}.Check(ctx, t, db)

			// verify cannot update the metadata with etag of the delete marker either
			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: metabase.UpdateObjectLastCommittedMetadata{
					ObjectLocation:    marker.Location(),
					StreamID:          marker.StreamID,
					EncryptedUserData: userDataWithETag,
					SetEncryptedETag:  true,
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "object with specified version and committed status is missing",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(object),
					metabase.RawObject(marker),
				},
			}.Check(ctx, t, db)
		})

		t.Run("update metadata of versioned object with previous delete marker", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()
			object := metabasetest.CreateObjectVersioned(ctx, t, db, obj, 0)

			marker := metabase.Object{
				ObjectStream: object.ObjectStream,
				Status:       metabase.DeleteMarkerVersioned,
				CreatedAt:    time.Now(),
			}
			marker.Version++

			metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: object.Location(),
					Versioned:      true,
				},
				Result: metabase.DeleteObjectResult{
					Markers: []metabase.Object{marker},
				},
				OutputMarkerStreamID: &marker.StreamID,
			}.Check(ctx, t, db)

			obj2 := obj
			obj2.StreamID = testrand.UUID()
			obj2.Version = marker.Version + 1
			object2 := metabasetest.CreateObjectVersioned(ctx, t, db, obj2, 0)

			userData := metabasetest.RandEncryptedUserDataWithoutETag()

			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: metabase.UpdateObjectLastCommittedMetadata{
					ObjectLocation:    object2.Location(),
					StreamID:          object2.StreamID,
					EncryptedUserData: userData,
				},
			}.Check(ctx, t, db)

			object2.EncryptedUserData = userData

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(object),
					metabase.RawObject(marker),
					metabase.RawObject(object2),
				},
			}.Check(ctx, t, db)

			userDataWithETag := metabasetest.RandEncryptedUserData()

			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: metabase.UpdateObjectLastCommittedMetadata{
					ObjectLocation:    object2.Location(),
					StreamID:          object2.StreamID,
					EncryptedUserData: userDataWithETag,
					SetEncryptedETag:  true,
				},
			}.Check(ctx, t, db)

			object2.EncryptedUserData = userDataWithETag

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(object),
					metabase.RawObject(marker),
					metabase.RawObject(object2),
				},
			}.Check(ctx, t, db)
		})

		t.Run("update metadata of unversioned object with previous version", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()
			object := metabasetest.CreateObjectVersioned(ctx, t, db, obj, 0)

			obj2 := obj
			obj2.StreamID = testrand.UUID()
			obj2.Version = obj.Version + 1
			object2 := metabasetest.CreateObjectVersioned(ctx, t, db, obj2, 0)

			obj3 := obj
			obj3.StreamID = testrand.UUID()
			obj3.Version = obj2.Version + 1
			object3 := metabasetest.CreateObject(ctx, t, db, obj3, 0)

			userData := metabasetest.RandEncryptedUserDataWithoutETag()

			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: metabase.UpdateObjectLastCommittedMetadata{
					ObjectLocation:    object3.Location(),
					StreamID:          object3.StreamID,
					EncryptedUserData: userData,
				},
			}.Check(ctx, t, db)

			object3.EncryptedUserData = userData

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(object),
					metabase.RawObject(object2),
					metabase.RawObject(object3),
				},
			}.Check(ctx, t, db)

			userDataWithETag := metabasetest.RandEncryptedUserData()
			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: metabase.UpdateObjectLastCommittedMetadata{
					ObjectLocation:    object3.Location(),
					StreamID:          object3.StreamID,
					EncryptedUserData: userDataWithETag,
					SetEncryptedETag:  true,
				},
			}.Check(ctx, t, db)

			object3.EncryptedUserData = userDataWithETag

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(object),
					metabase.RawObject(object2),
					metabase.RawObject(object3),
				},
			}.Check(ctx, t, db)
		})

		t.Run("disallow accidental dismissal of encryptedETag", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj1 := metabasetest.RandObjectStream()
			obj2 := metabasetest.RandObjectStream()
			object1 := metabasetest.CreateObjectVersioned(ctx, t, db, obj1, 0)
			object2 := metabasetest.CreateObject(ctx, t, db, obj2, 0)

			userData := metabasetest.RandEncryptedUserData()

			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: metabase.UpdateObjectLastCommittedMetadata{
					ObjectLocation:    object1.Location(),
					StreamID:          object1.StreamID,
					EncryptedUserData: userData,
					SetEncryptedETag:  true,
				},
			}.Check(ctx, t, db)

			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: metabase.UpdateObjectLastCommittedMetadata{
					ObjectLocation:    object2.Location(),
					StreamID:          object2.StreamID,
					EncryptedUserData: userData,
					SetEncryptedETag:  true,
				},
			}.Check(ctx, t, db)

			object1.EncryptedUserData = userData
			object2.EncryptedUserData = userData

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(object1),
					metabase.RawObject(object2),
				},
			}.Check(ctx, t, db)

			userDataWithoutEtag := metabasetest.RandEncryptedUserDataWithoutETag()

			// check that we cannot update when "SetEncryptedETag = false"
			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: metabase.UpdateObjectLastCommittedMetadata{
					ObjectLocation:    object1.Location(),
					StreamID:          object1.StreamID,
					EncryptedUserData: userDataWithoutEtag,
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "object with specified version and committed status is missing",
			}.Check(ctx, t, db)

			// check that we cannot update when "SetEncryptedETag = false"
			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: metabase.UpdateObjectLastCommittedMetadata{
					ObjectLocation:    object2.Location(),
					StreamID:          object2.StreamID,
					EncryptedUserData: userDataWithoutEtag,
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "object with specified version and committed status is missing",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(object1),
					metabase.RawObject(object2),
				},
			}.Check(ctx, t, db)
		})
	})
}
