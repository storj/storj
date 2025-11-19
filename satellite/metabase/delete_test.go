// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestDeletePendingObject(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		for _, test := range metabasetest.InvalidObjectStreams(obj) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)
				metabasetest.DeletePendingObject{
					Opts: metabase.DeletePendingObject{
						ObjectStream: test.ObjectStream,
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)
				metabasetest.Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("object missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.DeletePendingObject{
				Opts: metabase.DeletePendingObject{
					ObjectStream: obj,
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "metabase: no rows deleted",
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("non existing object version", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.DeletePendingObject{
				Opts: metabase.DeletePendingObject{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  obj.ProjectID,
						BucketName: obj.BucketName,
						ObjectKey:  obj.ObjectKey,
						Version:    33,
						StreamID:   obj.StreamID,
					},
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "metabase: no rows deleted",
			}.Check(ctx, t, db)
			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("delete committed object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()
			object := metabasetest.CreateObject(ctx, t, db, obj, 0)

			metabasetest.DeletePendingObject{
				Opts: metabase.DeletePendingObject{
					ObjectStream: object.ObjectStream,
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "metabase: no rows deleted",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: obj,
						CreatedAt:    now,
						Status:       metabase.CommittedUnversioned,

						Encryption: metabasetest.DefaultEncryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("without segments with wrong StreamID", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.DeletePendingObject{
				Opts: metabase.DeletePendingObject{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  obj.ProjectID,
						BucketName: obj.BucketName,
						ObjectKey:  obj.ObjectKey,
						Version:    obj.Version,
						StreamID:   uuid.UUID{33},
					},
				},
				Result:   metabase.DeleteObjectResult{},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "metabase: no rows deleted",
			}.Check(ctx, t, db)

			metabasetest.Verify{Objects: metabasetest.ObjectsToRaw(object)}.Check(ctx, t, db)
		})

		t.Run("without segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			object := metabase.RawObject{
				ObjectStream: obj,
				Status:       metabase.Pending,
			}
			metabasetest.DeletePendingObject{
				Opts: metabase.DeletePendingObject{
					ObjectStream: obj,
				},
				Result: metabase.DeleteObjectResult{
					Removed: []metabase.Object{metabase.Object(object)},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("with segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.CreatePendingObject(ctx, t, db, obj, 2)

			metabasetest.DeletePendingObject{
				Opts: metabase.DeletePendingObject{
					ObjectStream: obj,
				},
				Result: metabase.DeleteObjectResult{
					Removed: []metabase.Object{
						{
							ObjectStream: obj,
							Status:       metabase.Pending,
						},
					},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("with inline segment", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitInlineSegment{
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

			metabasetest.DeletePendingObject{
				Opts: metabase.DeletePendingObject{
					ObjectStream: obj,
				},
				Result: metabase.DeleteObjectResult{
					Removed: []metabase.Object{
						{
							ObjectStream: obj,
							Status:       metabase.Pending,
						},
					},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})
	})
}

func TestDeleteObjectExactVersion(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		location := obj.Location()

		for _, test := range metabasetest.InvalidObjectLocations(location) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)
				metabasetest.DeleteObjectExactVersion{
					Opts: metabase.DeleteObjectExactVersion{
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

			metabasetest.DeleteObjectExactVersion{
				Opts: metabase.DeleteObjectExactVersion{
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

			metabasetest.DeleteObjectExactVersion{
				Opts: metabase.DeleteObjectExactVersion{
					ObjectLocation: location,
					Version:        1,
				},
				Result: metabase.DeleteObjectResult{
					Removed: []metabase.Object{},
				},
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete non existing object version", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.DeleteObjectExactVersion{
				Opts: metabase.DeleteObjectExactVersion{
					ObjectLocation: location,
					Version:        33,
				},
				Result: metabase.DeleteObjectResult{
					Removed: []metabase.Object{},
				},
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete partial object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.DeleteObjectExactVersion{
				Opts: metabase.DeleteObjectExactVersion{
					ObjectLocation: location,
					Version:        obj.Version,
				},
				Result: metabase.DeleteObjectResult{
					Removed: []metabase.Object{{
						ObjectStream: obj,
						CreatedAt:    now,
						Encryption:   metabasetest.DefaultEncryption,
						Status:       metabase.Pending,
					}},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete pending negative object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()
			obj := obj
			obj.Version = -1

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.DeleteObjectExactVersion{
				Opts: metabase.DeleteObjectExactVersion{
					ObjectLocation: obj.Location(),
					Version:        obj.Version,
				},
				Result: metabase.DeleteObjectResult{
					Removed: []metabase.Object{{
						ObjectStream: obj,
						CreatedAt:    now,
						Encryption:   metabasetest.DefaultEncryption,
						Status:       metabase.Pending,
					}},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete object without segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object, _ := metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:      obj,
					EncryptedUserData: metabasetest.RandEncryptedUserData(),
				},
			}.Run(ctx, t, db, obj, 0)

			metabasetest.DeleteObjectExactVersion{
				Opts: metabase.DeleteObjectExactVersion{
					ObjectLocation: location,
					Version:        obj.Version,
				},
				Result: metabase.DeleteObjectResult{
					Removed: []metabase.Object{object},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete object with segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.CreateObject(ctx, t, db, obj, 2)

			metabasetest.DeleteObjectExactVersion{
				Opts: metabase.DeleteObjectExactVersion{
					ObjectLocation: location,
					Version:        obj.Version,
				},
				Result: metabase.DeleteObjectResult{
					Removed:             []metabase.Object{object},
					DeletedSegmentCount: 2,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete object with inline segment", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitInlineSegment{
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

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
				},
			}.Check(ctx, t, db)

			metabasetest.DeleteObjectExactVersion{
				Opts: metabase.DeleteObjectExactVersion{
					ObjectLocation: location,
					Version:        obj.Version,
				},
				Result: metabase.DeleteObjectResult{
					Removed:             []metabase.Object{object},
					DeletedSegmentCount: 1,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Using stream ID suffix", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object, segments := metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Run(ctx, t, db, obj, 2)

			metabasetest.DeleteObjectExactVersion{
				Opts: metabase.DeleteObjectExactVersion{
					ObjectLocation: location,
					Version:        obj.Version,
					StreamIDSuffix: metabase.StreamIDSuffix(testrand.Bytes(8)),
				},
				Result: metabase.DeleteObjectResult{
					Removed: []metabase.Object{},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects:  metabasetest.ObjectsToRaw(object),
				Segments: metabasetest.SegmentsToRaw(segments),
			}.Check(ctx, t, db)

			metabasetest.DeleteObjectExactVersion{
				Opts: metabase.DeleteObjectExactVersion{
					ObjectLocation: location,
					Version:        obj.Version,
					StreamIDSuffix: obj.StreamIDSuffix(),
				},
				Result: metabase.DeleteObjectResult{
					Removed:             []metabase.Object{object},
					DeletedSegmentCount: 2,
				},
			}.Check(ctx, t, db)
		})

		t.Run("Delete committed object with retention", func(t *testing.T) {
			metabasetest.ObjectLockDeletionTestRunner{
				TestProtected: func(t *testing.T, testCase metabasetest.ObjectLockDeletionTestCase) {
					defer metabasetest.DeleteAll{}.Check(ctx, t, db)

					object, segments := metabasetest.CreateTestObject{
						BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
							ObjectStream: obj,
							Encryption:   metabasetest.DefaultEncryption,
							Retention:    testCase.Retention,
							LegalHold:    testCase.LegalHold,
						},
					}.Run(ctx, t, db, obj, 1)

					var errMsg string
					if testCase.LegalHold {
						errMsg = "object is protected by a legal hold"
					} else {
						errMsg = "object is protected by a retention period"
					}

					metabasetest.DeleteObjectExactVersion{
						Opts: metabase.DeleteObjectExactVersion{
							ObjectLocation: location,
							Version:        obj.Version,
							ObjectLock: metabase.ObjectLockDeleteOptions{
								Enabled:          true,
								BypassGovernance: testCase.BypassGovernance,
							},
						},
						ErrClass: &metabase.ErrObjectLock,
						ErrText:  errMsg,
					}.Check(ctx, t, db)

					metabasetest.Verify{
						Objects:  []metabase.RawObject{metabase.RawObject(object)},
						Segments: metabasetest.SegmentsToRaw(segments),
					}.Check(ctx, t, db)
				},
				TestRemovable: func(t *testing.T, params metabasetest.ObjectLockDeletionTestCase) {
					defer metabasetest.DeleteAll{}.Check(ctx, t, db)

					object, _ := metabasetest.CreateObjectWithRetention(ctx, t, db, obj, 1, params.Retention)

					metabasetest.DeleteObjectExactVersion{
						Opts: metabase.DeleteObjectExactVersion{
							ObjectLocation: location,
							Version:        obj.Version,
							ObjectLock: metabase.ObjectLockDeleteOptions{
								Enabled:          true,
								BypassGovernance: params.BypassGovernance,
							},
						},
						Result: metabase.DeleteObjectResult{
							Removed:             []metabase.Object{object},
							DeletedSegmentCount: 1,
						},
					}.Check(ctx, t, db)

					metabasetest.Verify{}.Check(ctx, t, db)
				},
			}.Run(t)
		})

		t.Run("Delete partial object with retention", func(t *testing.T) {
			test := func(t *testing.T, testCase metabasetest.ObjectLockDeletionTestCase) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				object := metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: obj,
						Encryption:   metabasetest.DefaultEncryption,
						Retention:    testCase.Retention,
						LegalHold:    testCase.LegalHold,
					},
				}.Check(ctx, t, db)

				expected := object
				expected.ZombieDeletionDeadline = nil

				metabasetest.DeleteObjectExactVersion{
					Opts: metabase.DeleteObjectExactVersion{
						ObjectLocation: location,
						Version:        obj.Version,
						ObjectLock: metabase.ObjectLockDeleteOptions{
							Enabled:          true,
							BypassGovernance: testCase.BypassGovernance,
						},
					},
					Result: metabase.DeleteObjectResult{
						Removed: []metabase.Object{expected},
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{}.Check(ctx, t, db)
			}

			metabasetest.ObjectLockDeletionTestRunner{
				TestProtected: test,
				TestRemovable: test,
			}.Run(t)
		})
	})
}

func TestDeleteObjectVersioning(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()
		location := obj.Location()

		t.Run("delete non existing object version", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()
			marker := obj
			marker.Version = 0

			result := metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: location,
					Versioned:      true,
				},
				Result: metabase.DeleteObjectResult{
					Markers: []metabase.Object{
						{
							ObjectStream: marker,
							CreatedAt:    now,
							Status:       metabase.DeleteMarkerVersioned,
						},
					},
				},
			}.Check(ctx, t, db)
			require.NotZero(t, result.Markers[0].Version)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(result.Markers[0]),
				},
			}.Check(ctx, t, db)
		})

		t.Run("Delete partial object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			pending := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			now := time.Now()
			marker := obj
			marker.Version = 0

			result := metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: location,
					Versioned:      true,
				},
				Result: metabase.DeleteObjectResult{
					Markers: []metabase.Object{
						{
							ObjectStream: marker,
							CreatedAt:    now,
							Status:       metabase.DeleteMarkerVersioned,
						},
					},
				},
			}.Check(ctx, t, db)

			require.Greater(t, result.Markers[0].Version, pending.Version)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(pending),
					metabase.RawObject(result.Markers[0]),
				},
			}.Check(ctx, t, db)

			marker2 := marker
			marker2.Version = 0
			result2 := metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: location,
					Versioned:      true,
				},
				Result: metabase.DeleteObjectResult{
					Markers: []metabase.Object{
						{
							ObjectStream: marker2,
							CreatedAt:    now,
							Status:       metabase.DeleteMarkerVersioned,
						},
					},
				},
			}.Check(ctx, t, db)

			require.Greater(t, result2.Markers[0].Version, result.Markers[0].Version)

			// Not quite sure whether this is the appropriate behavior,
			// but let's leave the pending object in place and not insert a deletion marker.
			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(pending),
					metabase.RawObject(result.Markers[0]),
					metabase.RawObject(result2.Markers[0]),
				},
			}.Check(ctx, t, db)
		})

		t.Run("Create a delete marker", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			committed, _ := metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream: obj,
				},
			}.Run(ctx, t, db, obj, 0)

			marker := committed.ObjectStream
			marker.Version = 0

			now := time.Now()
			result := metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: location,
					Versioned:      true,
				},
				Result: metabase.DeleteObjectResult{
					Markers: []metabase.Object{
						{
							ObjectStream: marker,
							CreatedAt:    now,
							Status:       metabase.DeleteMarkerVersioned,
						},
					},
				},
				OutputMarkerStreamID: &marker.StreamID,
			}.Check(ctx, t, db)

			require.Greater(t, result.Markers[0].Version, committed.Version)
			marker.Version = result.Markers[0].Version

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: marker,
						CreatedAt:    now,
						Status:       metabase.DeleteMarkerVersioned,
					},
					{
						ObjectStream: obj,
						CreatedAt:    committed.CreatedAt,
						Status:       metabase.CommittedUnversioned,
						Encryption:   committed.Encryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("multiple delete markers", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			committed, _ := metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream: obj,
				},
			}.Run(ctx, t, db, obj, 0)

			marker := committed.ObjectStream
			marker.Version = 0 // ignore version check

			now := time.Now()
			result1 := metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: location,
					Versioned:      true,
				},
				Result: metabase.DeleteObjectResult{
					Markers: []metabase.Object{
						{
							ObjectStream: marker,
							CreatedAt:    now,
							Status:       metabase.DeleteMarkerVersioned,
						},
					},
				},
				OutputMarkerStreamID: &marker.StreamID,
			}.Check(ctx, t, db)

			require.Greater(t, result1.Markers[0].Version, committed.Version)
			marker.Version = result1.Markers[0].Version

			marker2 := marker
			marker2.Version = 0 // ignore version check
			result2 := metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: location,
					Versioned:      true,
				},
				Result: metabase.DeleteObjectResult{
					Markers: []metabase.Object{
						{
							ObjectStream: marker2,
							CreatedAt:    now,
							Status:       metabase.DeleteMarkerVersioned,
						},
					},
				},
				OutputMarkerStreamID: &marker2.StreamID,
			}.Check(ctx, t, db)

			require.Greater(t, result2.Markers[0].Version, result1.Markers[0].Version)
			marker2.Version = result2.Markers[0].Version

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream: marker,
						CreatedAt:    now,
						Status:       metabase.DeleteMarkerVersioned,
					},
					{
						ObjectStream: marker2,
						CreatedAt:    now,
						Status:       metabase.DeleteMarkerVersioned,
					},
					{
						ObjectStream: obj,
						CreatedAt:    committed.CreatedAt,
						Status:       metabase.CommittedUnversioned,
						Encryption:   committed.Encryption,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("delete last committed unversioned with suspended", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()

			obj := metabasetest.RandObjectStream()
			object := metabasetest.CreateObject(ctx, t, db, obj, 0)

			marker := metabase.Object{
				ObjectStream: metabase.ObjectStream{
					ProjectID:  obj.ProjectID,
					BucketName: obj.BucketName,
					ObjectKey:  obj.ObjectKey,
					Version:    0,
				},
				Status:    metabase.DeleteMarkerUnversioned,
				CreatedAt: now,
			}

			result := metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: metabase.ObjectLocation{
						ProjectID:  obj.ProjectID,
						BucketName: obj.BucketName,
						ObjectKey:  obj.ObjectKey,
					},
					Versioned: false,
					Suspended: true,
				},
				Result: metabase.DeleteObjectResult{
					Markers: []metabase.Object{marker},
					Removed: []metabase.Object{
						object,
					},
				},
				OutputMarkerStreamID: &marker.StreamID,
			}.Check(ctx, t, db)

			require.Greater(t, result.Markers[0].Version, object.Version)
			marker.ObjectStream.Version = result.Markers[0].Version

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(marker),
				},
			}.Check(ctx, t, db)
		})

		t.Run("delete last committed versioned with suspended", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()

			obj := metabasetest.RandObjectStream()
			initial := metabasetest.CreateObjectVersioned(ctx, t, db, obj, 0)

			marker := metabase.Object{
				ObjectStream: metabase.ObjectStream{
					ProjectID:  obj.ProjectID,
					BucketName: obj.BucketName,
					ObjectKey:  obj.ObjectKey,
					Version:    0,
				},
				Status:    metabase.DeleteMarkerUnversioned,
				CreatedAt: now,
			}

			result := metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: metabase.ObjectLocation{
						ProjectID:  obj.ProjectID,
						BucketName: obj.BucketName,
						ObjectKey:  obj.ObjectKey,
					},
					Versioned: false,
					Suspended: true,
				},
				Result: metabase.DeleteObjectResult{
					Markers: []metabase.Object{marker},
				},
				OutputMarkerStreamID: &marker.StreamID,
			}.Check(ctx, t, db)

			require.Greater(t, result.Markers[0].Version, initial.Version)
			marker.ObjectStream.Version = result.Markers[0].Version

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(initial),
					metabase.RawObject(marker),
				},
			}.Check(ctx, t, db)
		})

		t.Run("delete last pending with suspended", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj := metabasetest.RandObjectStream()
			pending := metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: metabase.ObjectLocation{
						ProjectID:  obj.ProjectID,
						BucketName: obj.BucketName,
						ObjectKey:  obj.ObjectKey,
					},
					Versioned: false,
					Suspended: true,
				},
				ErrClass: &metabase.ErrObjectNotFound,
				ErrText:  "unable to delete object",
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					metabase.RawObject(pending),
				},
			}.Check(ctx, t, db)
		})
	}, metabasetest.WithTimestampVersioning)
}

func TestDeleteCopyWithDuplicateMetadata(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		for _, numberOfSegments := range []int{0, 1, 3} {
			t.Run(fmt.Sprintf("%d segments", numberOfSegments), func(t *testing.T) {
				t.Run("delete copy", func(t *testing.T) {
					defer metabasetest.DeleteAll{}.Check(ctx, t, db)
					originalObjStream := metabasetest.RandObjectStream()

					originalObj, originalSegments := metabasetest.CreateTestObject{
						CommitObject: &metabase.CommitObject{
							ObjectStream:      originalObjStream,
							EncryptedUserData: metabasetest.RandEncryptedUserData(),
						},
					}.Run(ctx, t, db, originalObjStream, byte(numberOfSegments))

					copyObj, _, copySegments := metabasetest.CreateObjectCopy{
						OriginalObject: originalObj,
					}.Run(ctx, t, db)

					// check that copy went OK
					metabasetest.Verify{
						Objects: []metabase.RawObject{
							metabase.RawObject(originalObj),
							metabase.RawObject(copyObj),
						},
						Segments: append(metabasetest.SegmentsToRaw(originalSegments), copySegments...),
					}.Check(ctx, t, db)

					metabasetest.DeleteObjectExactVersion{
						Opts: metabase.DeleteObjectExactVersion{
							ObjectLocation: copyObj.Location(),
							Version:        copyObj.Version,
						},
						Result: metabase.DeleteObjectResult{
							Removed:             []metabase.Object{copyObj},
							DeletedSegmentCount: numberOfSegments,
						},
					}.Check(ctx, t, db)

					// Verify that we are back at the original single object
					metabasetest.Verify{
						Objects: []metabase.RawObject{
							metabase.RawObject(originalObj),
						},
						Segments: metabasetest.SegmentsToRaw(originalSegments),
					}.Check(ctx, t, db)
				})

				t.Run("delete one of two copies", func(t *testing.T) {
					defer metabasetest.DeleteAll{}.Check(ctx, t, db)
					originalObjectStream := metabasetest.RandObjectStream()

					originalObj, originalSegments := metabasetest.CreateTestObject{
						CommitObject: &metabase.CommitObject{
							ObjectStream:      originalObjectStream,
							EncryptedUserData: metabasetest.RandEncryptedUserData(),
						},
					}.Run(ctx, t, db, originalObjectStream, byte(numberOfSegments))

					copyObject1, _, _ := metabasetest.CreateObjectCopy{
						OriginalObject: originalObj,
					}.Run(ctx, t, db)
					copyObject2, _, copySegments2 := metabasetest.CreateObjectCopy{
						OriginalObject: originalObj,
					}.Run(ctx, t, db)

					metabasetest.DeleteObjectExactVersion{
						Opts: metabase.DeleteObjectExactVersion{
							ObjectLocation: copyObject1.Location(),
							Version:        copyObject1.Version,
						},
						Result: metabase.DeleteObjectResult{
							Removed:             []metabase.Object{copyObject1},
							DeletedSegmentCount: numberOfSegments,
						},
					}.Check(ctx, t, db)

					// Verify that only one of the copies is deleted
					metabasetest.Verify{
						Objects: []metabase.RawObject{
							metabase.RawObject(originalObj),
							metabase.RawObject(copyObject2),
						},
						Segments: append(metabasetest.SegmentsToRaw(originalSegments), copySegments2...),
					}.Check(ctx, t, db)
				})

				t.Run("delete original", func(t *testing.T) {
					defer metabasetest.DeleteAll{}.Check(ctx, t, db)
					originalObjectStream := metabasetest.RandObjectStream()

					originalObj, originalSegments := metabasetest.CreateTestObject{
						CommitObject: &metabase.CommitObject{
							ObjectStream:      originalObjectStream,
							EncryptedUserData: metabasetest.RandEncryptedUserData(),
						},
					}.Run(ctx, t, db, originalObjectStream, byte(numberOfSegments))

					copyObject, _, copySegments := metabasetest.CreateObjectCopy{
						OriginalObject: originalObj,
					}.Run(ctx, t, db)

					metabasetest.DeleteObjectExactVersion{
						Opts: metabase.DeleteObjectExactVersion{
							ObjectLocation: originalObj.Location(),
							Version:        originalObj.Version,
						},
						Result: metabase.DeleteObjectResult{
							Removed:             []metabase.Object{originalObj},
							DeletedSegmentCount: numberOfSegments,
						},
					}.Check(ctx, t, db)

					for i := range copySegments {
						copySegments[i].Pieces = originalSegments[i].Pieces
					}

					// verify that the copy is left
					metabasetest.Verify{
						Objects: []metabase.RawObject{
							metabase.RawObject(copyObject),
						},
						Segments: copySegments,
					}.Check(ctx, t, db)
				})

				t.Run("delete original and leave two copies", func(t *testing.T) {
					defer metabasetest.DeleteAll{}.Check(ctx, t, db)
					originalObjectStream := metabasetest.RandObjectStream()

					originalObj, originalSegments := metabasetest.CreateTestObject{
						CommitObject: &metabase.CommitObject{
							ObjectStream:      originalObjectStream,
							EncryptedUserData: metabasetest.RandEncryptedUserData(),
						},
					}.Run(ctx, t, db, originalObjectStream, byte(numberOfSegments))

					copyObject1, _, copySegments1 := metabasetest.CreateObjectCopy{
						OriginalObject: originalObj,
					}.Run(ctx, t, db)
					copyObject2, _, copySegments2 := metabasetest.CreateObjectCopy{
						OriginalObject: originalObj,
					}.Run(ctx, t, db)

					_, err := db.DeleteObjectExactVersion(ctx, metabase.DeleteObjectExactVersion{
						Version:        originalObj.Version,
						ObjectLocation: originalObj.Location(),
					})
					require.NoError(t, err)

					var expectedAncestorStreamID uuid.UUID

					if numberOfSegments > 0 {
						segments, err := db.TestingAllSegments(ctx)
						require.NoError(t, err)
						require.NotEmpty(t, segments)

						if segments[0].StreamID == copyObject1.StreamID {
							expectedAncestorStreamID = copyObject1.StreamID
						} else {
							expectedAncestorStreamID = copyObject2.StreamID
						}
					}

					// set pieces in expected ancestor for verifcation
					for _, segments := range [][]metabase.RawSegment{copySegments1, copySegments2} {
						for i := range segments {
							if segments[i].StreamID == expectedAncestorStreamID {
								segments[i].Pieces = originalSegments[i].Pieces
							}
						}
					}

					// verify that two functioning copies are left and the original object is gone
					metabasetest.Verify{
						Objects: []metabase.RawObject{
							metabase.RawObject(copyObject1),
							metabase.RawObject(copyObject2),
						},
						Segments: append(copySegments1, copySegments2...),
					}.Check(ctx, t, db)
				})
			})
		}
	})
}

func TestDeleteObjectLastCommitted(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		location := obj.Location()

		for _, test := range metabasetest.InvalidObjectLocations(obj.Location()) {
			test := test
			t.Run(test.Name, func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				metabasetest.DeleteObjectLastCommitted{
					Opts: metabase.DeleteObjectLastCommitted{
						ObjectLocation: metabase.ObjectLocation{
							ProjectID:  test.ObjectLocation.ProjectID,
							BucketName: test.ObjectLocation.BucketName,
							ObjectKey:  test.ObjectLocation.ObjectKey,
						},
					},
					ErrClass: test.ErrClass,
					ErrText:  test.ErrText,
				}.Check(ctx, t, db)
				metabasetest.Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("Object missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: location,
				},
				Result: metabase.DeleteObjectResult{},
			}.Check(ctx, t, db)
			metabasetest.Verify{}.Check(ctx, t, db)
		})
		t.Run("Delete object without segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object, _ := metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream:      obj,
					EncryptedUserData: metabasetest.RandEncryptedUserData(),
				},
			}.Run(ctx, t, db, obj, 0)

			metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: location,
				},
				Result: metabase.DeleteObjectResult{
					Removed: []metabase.Object{object},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete object with segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			object := metabasetest.CreateObject(ctx, t, db, obj, 2)

			metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: location,
				},
				Result: metabase.DeleteObjectResult{
					Removed:             []metabase.Object{object},
					DeletedSegmentCount: 2,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete object with inline segment", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: obj,
					Encryption:   metabasetest.DefaultEncryption,
				},
			}.Check(ctx, t, db)

			metabasetest.CommitInlineSegment{
				Opts: metabase.CommitInlineSegment{
					ObjectStream:      obj,
					Position:          metabase.SegmentPosition{Part: 0, Index: 0},
					EncryptedKey:      testrand.Bytes(32),
					EncryptedKeyNonce: testrand.Bytes(32),
					InlineData:        testrand.Bytes(1024),
					PlainSize:         512,
					PlainOffset:       0,
				},
			}.Check(ctx, t, db)

			object := metabasetest.CommitObject{
				Opts: metabase.CommitObject{
					ObjectStream: obj,
				},
			}.Check(ctx, t, db)

			metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: location,
				},
				Result: metabase.DeleteObjectResult{
					Removed:             []metabase.Object{object},
					DeletedSegmentCount: 1,
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("Delete last committed from several versions", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			now := time.Now()
			newObj := metabasetest.RandObjectStream()

			metabasetest.BeginObjectExactVersion{
				Opts: metabase.BeginObjectExactVersion{
					ObjectStream: metabase.ObjectStream{
						ProjectID:  newObj.ProjectID,
						BucketName: newObj.BucketName,
						ObjectKey:  newObj.ObjectKey,
						Version:    newObj.Version,
						StreamID:   newObj.StreamID,
					},
					ZombieDeletionDeadline: &now,
				},
			}.Check(ctx, t, db)

			newObjDiffVersion := newObj
			newObjDiffVersion.Version = newObj.Version * 2

			committedObject, _ := metabasetest.CreateTestObject{}.Run(ctx, t, db, newObjDiffVersion, 0)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream:           newObj,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						ZombieDeletionDeadline: &now,
					},
					{
						ObjectStream: newObjDiffVersion,
						CreatedAt:    now,
						Status:       metabase.CommittedUnversioned,
						Encryption:   metabasetest.DefaultEncryption,
					},
				},
			}.Check(ctx, t, db)

			metabasetest.DeleteObjectLastCommitted{
				Opts: metabase.DeleteObjectLastCommitted{
					ObjectLocation: metabase.ObjectLocation{
						ProjectID:  newObj.ProjectID,
						BucketName: newObj.BucketName,
						ObjectKey:  newObj.ObjectKey,
					}},
				Result: metabase.DeleteObjectResult{
					Removed: []metabase.Object{committedObject},
				},
			}.Check(ctx, t, db)

			metabasetest.Verify{
				Objects: []metabase.RawObject{
					{
						ObjectStream:           newObj,
						CreatedAt:              now,
						Status:                 metabase.Pending,
						ZombieDeletionDeadline: &now,
					},
				},
			}.Check(ctx, t, db)
		})

		t.Run("Delete object with retention", func(t *testing.T) {
			t.Run("Versioned", func(t *testing.T) {
				test := func(t *testing.T, testCase metabasetest.ObjectLockDeletionTestCase) {
					defer metabasetest.DeleteAll{}.Check(ctx, t, db)

					object, segments := metabasetest.CreateTestObject{
						BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
							ObjectStream: obj,
							Encryption:   metabasetest.DefaultEncryption,
							Retention:    testCase.Retention,
							LegalHold:    testCase.LegalHold,
						},
					}.Run(ctx, t, db, obj, 1)

					markerObjStream := obj
					markerObjStream.Version = 0

					deleted := metabasetest.DeleteObjectLastCommitted{
						Opts: metabase.DeleteObjectLastCommitted{
							ObjectLocation: obj.Location(),
							ObjectLock: metabase.ObjectLockDeleteOptions{
								Enabled:          true,
								BypassGovernance: testCase.BypassGovernance,
							},
							Versioned: true,
						},
						Result: metabase.DeleteObjectResult{
							Markers: []metabase.Object{{
								ObjectStream: markerObjStream,
								CreatedAt:    time.Now(),
								Status:       metabase.DeleteMarkerVersioned,
							}},
						},
					}.Check(ctx, t, db)

					require.Greater(t, deleted.Markers[0].Version, object.Version)

					metabasetest.Verify{
						Objects:  metabasetest.ObjectsToRaw(object, deleted.Markers[0]),
						Segments: metabasetest.SegmentsToRaw(segments),
					}.Check(ctx, t, db)
				}

				metabasetest.ObjectLockDeletionTestRunner{
					TestProtected: test,
					TestRemovable: test,
				}.Run(t)
			})

			t.Run("Suspended", func(t *testing.T) {
				metabasetest.ObjectLockDeletionTestRunner{
					TestProtected: func(t *testing.T, testCase metabasetest.ObjectLockDeletionTestCase) {
						defer metabasetest.DeleteAll{}.Check(ctx, t, db)

						object, segments := metabasetest.CreateTestObject{
							BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
								ObjectStream: obj,
								Encryption:   metabasetest.DefaultEncryption,
								Retention:    testCase.Retention,
								LegalHold:    testCase.LegalHold,
							},
						}.Run(ctx, t, db, obj, 1)

						var errMsg string
						if testCase.LegalHold {
							errMsg = "object is protected by a legal hold"
						} else {
							errMsg = "object is protected by a retention period"
						}

						metabasetest.DeleteObjectLastCommitted{
							Opts: metabase.DeleteObjectLastCommitted{
								ObjectLocation: obj.Location(),
								ObjectLock: metabase.ObjectLockDeleteOptions{
									Enabled:          true,
									BypassGovernance: testCase.BypassGovernance,
								},
								Suspended: true,
							},
							ErrClass: &metabase.ErrObjectLock,
							ErrText:  errMsg,
						}.Check(ctx, t, db)

						metabasetest.Verify{
							Objects:  []metabase.RawObject{metabase.RawObject(object)},
							Segments: metabasetest.SegmentsToRaw(segments),
						}.Check(ctx, t, db)
					},
					TestRemovable: func(t *testing.T, testCase metabasetest.ObjectLockDeletionTestCase) {
						defer metabasetest.DeleteAll{}.Check(ctx, t, db)

						object, _ := metabasetest.CreateObjectWithRetention(ctx, t, db, obj, 1, testCase.Retention)

						markerObjStream := obj
						markerObjStream.Version = 0

						deleted := metabasetest.DeleteObjectLastCommitted{
							Opts: metabase.DeleteObjectLastCommitted{
								ObjectLocation: object.Location(),
								ObjectLock: metabase.ObjectLockDeleteOptions{
									Enabled:          true,
									BypassGovernance: testCase.BypassGovernance,
								},
								Suspended: true,
							},
							Result: metabase.DeleteObjectResult{
								Removed: []metabase.Object{object},
								Markers: []metabase.Object{{
									ObjectStream: markerObjStream,
									CreatedAt:    time.Now(),
									Status:       metabase.DeleteMarkerUnversioned,
								}},
								DeletedSegmentCount: 1,
							},
						}.Check(ctx, t, db)

						require.Greater(t, deleted.Markers[0].Version, object.Version)

						metabasetest.Verify{
							Objects: []metabase.RawObject{metabase.RawObject(deleted.Markers[0])},
						}.Check(ctx, t, db)
					},
				}.Run(t)
			})

			t.Run("Unversioned", func(t *testing.T) {
				metabasetest.ObjectLockDeletionTestRunner{
					TestProtected: func(t *testing.T, testCase metabasetest.ObjectLockDeletionTestCase) {
						defer metabasetest.DeleteAll{}.Check(ctx, t, db)

						objStream := metabasetest.RandObjectStream()
						object, segments := metabasetest.CreateTestObject{
							BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
								ObjectStream: objStream,
								Encryption:   metabasetest.DefaultEncryption,
								Retention:    testCase.Retention,
								LegalHold:    testCase.LegalHold,
							},
						}.Run(ctx, t, db, objStream, 1)

						var errMsg string
						if testCase.LegalHold {
							errMsg = "object is protected by a legal hold"
						} else {
							errMsg = "object is protected by a retention period"
						}

						metabasetest.DeleteObjectLastCommitted{
							Opts: metabase.DeleteObjectLastCommitted{
								ObjectLocation: objStream.Location(),
								ObjectLock: metabase.ObjectLockDeleteOptions{
									Enabled:          true,
									BypassGovernance: testCase.BypassGovernance,
								},
							},
							ErrClass: &metabase.ErrObjectLock,
							ErrText:  errMsg,
						}.Check(ctx, t, db)

						metabasetest.Verify{
							Objects:  []metabase.RawObject{metabase.RawObject(object)},
							Segments: metabasetest.SegmentsToRaw(segments),
						}.Check(ctx, t, db)
					},
					TestRemovable: func(t *testing.T, testCase metabasetest.ObjectLockDeletionTestCase) {
						defer metabasetest.DeleteAll{}.Check(ctx, t, db)

						object, _ := metabasetest.CreateObjectWithRetention(ctx, t, db, obj, 1, testCase.Retention)

						metabasetest.DeleteObjectLastCommitted{
							Opts: metabase.DeleteObjectLastCommitted{
								ObjectLocation: object.Location(),
								ObjectLock: metabase.ObjectLockDeleteOptions{
									Enabled:          true,
									BypassGovernance: testCase.BypassGovernance,
								},
							},
							Result: metabase.DeleteObjectResult{
								Removed:             []metabase.Object{object},
								DeletedSegmentCount: 1,
							},
						}.Check(ctx, t, db)

						metabasetest.Verify{}.Check(ctx, t, db)
					},
				}.Run(t)
			})
		})
	}, metabasetest.WithTimestampVersioning)
}
