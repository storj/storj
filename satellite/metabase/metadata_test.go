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

		t.Run("Invalid metadata", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			for i, scenario := range metabasetest.InvalidEncryptedUserDataScenarios() {
				t.Log(i)

				metabasetest.UpdateObjectLastCommittedMetadata{
					Opts: metabase.UpdateObjectLastCommittedMetadata{
						ObjectLocation:    obj.Location(),
						StreamID:          obj.StreamID,
						EncryptedUserData: scenario.EncryptedUserData,
					},
					ErrClass: &metabase.ErrInvalidRequest,
					ErrText:  scenario.ErrText,
				}.Check(ctx, t, db)
			}

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Missing includes", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: metabase.UpdateObjectLastCommittedMetadata{
					ObjectLocation:    obj.Location(),
					StreamID:          obj.StreamID,
					EncryptedUserData: metabasetest.RandEncryptedUserDataWithChecksum(),
					Includes:          metabase.EncryptedUserDataIncludes{},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "Includes is missing",
			}.Check(ctx, t, db)
		})

		t.Run("Missing object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()
			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: metabase.UpdateObjectLastCommittedMetadata{
					ObjectLocation: obj.Location(),
					StreamID:       obj.StreamID,
					Includes:       metabase.EncryptedUserDataIncludesAll(),
				},
				ErrClass: &metabase.ErrObjectNotFound,
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Update metadata", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()
			object := metabasetest.CreateObject(ctx, t, db, obj, 0)

			userData := metabasetest.RandEncryptedUserDataWithChecksum()

			opts := metabase.UpdateObjectLastCommittedMetadata{
				ObjectLocation:    object.Location(),
				StreamID:          object.StreamID,
				EncryptedUserData: userData,
				Includes: metabase.EncryptedUserDataIncludes{
					Metadata: true,
				},
			}

			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: opts,
			}.Check(ctx, t, db)

			object.EncryptedUserData = userData
			object.EncryptedUserData.EncryptedETag = nil
			object.EncryptedUserData.Checksum = metabase.Checksum{}

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(object),
				},
			}.Check(ctx, t, db)

			opts.Includes.ETag = true

			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: opts,
			}.Check(ctx, t, db)

			object.EncryptedUserData.EncryptedETag = userData.EncryptedETag

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(object),
				},
			}.Check(ctx, t, db)

			opts.Includes.Checksum = true

			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: opts,
			}.Check(ctx, t, db)

			object.EncryptedUserData.Checksum = userData.Checksum

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

			userData := metabasetest.RandEncryptedUserDataWithChecksum()

			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: metabase.UpdateObjectLastCommittedMetadata{
					ObjectLocation:    object2.Location(),
					StreamID:          object2.StreamID,
					EncryptedUserData: userData,
					Includes:          metabase.EncryptedUserDataIncludesAll(),
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

			userData := metabasetest.RandEncryptedUserDataWithChecksum()

			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: metabase.UpdateObjectLastCommittedMetadata{
					ObjectLocation:    object.Location(),
					StreamID:          object.StreamID,
					EncryptedUserData: userData,
					Includes:          metabase.EncryptedUserDataIncludesAll(),
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

			// Confirm that we cannot update any of the deleted object's user data.
			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: metabase.UpdateObjectLastCommittedMetadata{
					ObjectLocation:    object.Location(),
					StreamID:          object.StreamID,
					EncryptedUserData: userData,
					Includes:          metabase.EncryptedUserDataIncludesAll(),
				},
				ErrClass: &metabase.ErrObjectNotFound,
			}.Check(ctx, t, db)

			// Confirm that we cannot update any of the delete marker's user data, either.
			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: metabase.UpdateObjectLastCommittedMetadata{
					ObjectLocation:    marker.Location(),
					StreamID:          marker.StreamID,
					EncryptedUserData: userData,
					Includes:          metabase.EncryptedUserDataIncludesAll(),
				},
				ErrClass: &metabase.ErrObjectNotFound,
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

			// Confirm that we cannot update any of the deleted object's user data.
			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: metabase.UpdateObjectLastCommittedMetadata{
					ObjectLocation:    object2.Location(),
					StreamID:          object2.StreamID,
					EncryptedUserData: userData,
					Includes:          metabase.EncryptedUserDataIncludesAll(),
				},
				ErrClass: &metabase.ErrObjectNotFound,
			}.Check(ctx, t, db)

			// Confirm that we cannot update any of the delete marker's user data, either.
			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: metabase.UpdateObjectLastCommittedMetadata{
					ObjectLocation:    marker.Location(),
					StreamID:          marker.StreamID,
					EncryptedUserData: userData,
					Includes:          metabase.EncryptedUserDataIncludesAll(),
				},
				ErrClass: &metabase.ErrObjectNotFound,
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
					Includes: metabase.EncryptedUserDataIncludes{
						Metadata: true,
					},
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
					Includes: metabase.EncryptedUserDataIncludes{
						Metadata: true,
						ETag:     true,
					},
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
					Includes: metabase.EncryptedUserDataIncludes{
						Metadata: true,
					},
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
					Includes: metabase.EncryptedUserDataIncludes{
						Metadata: true,
						ETag:     true,
					},
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

		for _, tt := range []struct {
			name      string
			versioned bool
		}{
			{"unversioned", false},
			{"versioned", true},
		} {
			t.Run("disallow accidental dismissal of metadata fields ("+tt.name+")", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				objStream := metabasetest.RandObjectStream()
				fullUserData := metabasetest.RandEncryptedUserDataWithChecksum()

				object, _ := metabasetest.CreateTestObject{
					CommitObject: &metabase.CommitObject{
						ObjectStream:         objStream,
						Encryption:           metabasetest.DefaultEncryption,
						EncryptedUserData:    fullUserData,
						SetEncryptedMetadata: true,
						Versioned:            tt.versioned,
					},
				}.Run(ctx, t, db, objStream, 0)

				test := func(userData metabase.EncryptedUserData, includes metabase.EncryptedUserDataIncludes) {
					metabasetest.UpdateObjectLastCommittedMetadata{
						Opts: metabase.UpdateObjectLastCommittedMetadata{
							ObjectLocation:    object.Location(),
							StreamID:          object.StreamID,
							EncryptedUserData: userData,
							Includes:          includes,
						},
						ErrClass: &metabase.ErrInsufficientMetadataIncludes,
						ErrText:  "the object's metadata contains populated fields not included in the provided includes",
					}.Check(ctx, t, db)

					metabasetest.Verify{
						Objects: []metabase.RawObject{metabase.RawObject(object)},
					}.Check(ctx, t, db)
				}

				includeAll := metabase.EncryptedUserDataIncludesAll()

				noMetadataUserData := fullUserData
				noMetadataUserData.EncryptedMetadata = nil
				test(noMetadataUserData, includeAll.Without(metabase.EncryptedUserDataIncludes{
					Metadata: true,
				}))

				noETagUserData := fullUserData
				noETagUserData.EncryptedETag = nil
				test(noETagUserData, includeAll.Without(metabase.EncryptedUserDataIncludes{
					ETag: true,
				}))

				noChecksumUserData := fullUserData
				noChecksumUserData.Checksum = metabase.Checksum{}
				test(noChecksumUserData, includeAll.Without(metabase.EncryptedUserDataIncludes{
					Checksum: true,
				}))
			})
		}
	})
}

func TestUpdateObjectLastCommittedMetadata_Encoding(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		testObjectEncoding(ctx, t, db, func(t *testing.T, testCase objectEncodingTestCase) metabase.ObjectStream {
			objStream := metabasetest.RandObjectStream()
			metabasetest.CreateObject(ctx, t, db, objStream, 0)

			metabasetest.UpdateObjectLastCommittedMetadata{
				Opts: metabase.UpdateObjectLastCommittedMetadata{
					ObjectLocation:    objStream.Location(),
					StreamID:          objStream.StreamID,
					EncryptedUserData: testCase.userData,
					Includes:          metabase.EncryptedUserDataIncludesAll(),
				},
			}.Check(ctx, t, db)

			return objStream
		})
	})
}

func TestGetPendingObjectMetadata(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		objStream := metabasetest.RandObjectStream()
		userData := metabasetest.RandEncryptedUserDataWithChecksum()

		for _, test := range metabasetest.InvalidObjectStreams(objStream) {
			t.Run(test.Name, func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				metabasetest.GetPendingObjectMetadata{
					Opts: metabase.GetPendingObjectMetadata{
						ObjectStream: test.ObjectStream,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)

				metabasetest.Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("Object missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.GetPendingObjectMetadata{
				Opts: metabase.GetPendingObjectMetadata{
					ObjectStream: objStream,
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "metabase: sql: no rows in result set",
			}.Check(ctx, t, db)

			object := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream:      objStream,
					Encryption:        metabasetest.DefaultEncryption,
					EncryptedUserData: userData,
				},
			}.Check(ctx, t, db)

			objStream := objStream
			objStream.StreamID = testrand.UUID()

			// Even if all of the fields comprising the object's primary key match,
			// an error should be returned if the stream ID doesn't match.
			metabasetest.GetPendingObjectMetadata{
				Opts: metabase.GetPendingObjectMetadata{
					ObjectStream: objStream,
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "metabase: sql: no rows in result set",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{metabase.RawObject(object)},
			}.Check(ctx, t, db)
		})

		t.Run("Pending object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream:      objStream,
					Encryption:        metabasetest.DefaultEncryption,
					EncryptedUserData: userData,
				},
			}.Check(ctx, t, db)

			metabasetest.GetPendingObjectMetadata{
				Opts: metabase.GetPendingObjectMetadata{
					ObjectStream: objStream,
				},
				Result: metabase.GetPendingObjectMetadataResult{
					EncryptedUserData: userData,
					Encryption:        object.Encryption,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{metabase.RawObject(object)},
			}.Check(ctx, t, db)
		})

		t.Run("Committed object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object, _ := metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:         objStream,
					EncryptedUserData:    userData,
					SetEncryptedMetadata: true,
				},
			}.Run(ctx, t, db, objStream, 0)

			metabasetest.GetPendingObjectMetadata{
				Opts: metabase.GetPendingObjectMetadata{
					ObjectStream: objStream,
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "metabase: sql: no rows in result set",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{metabase.RawObject(object)},
			}.Check(ctx, t, db)
		})
	})
}
