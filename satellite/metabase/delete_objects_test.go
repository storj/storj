// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestDeleteObjects(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		projectID := testrand.UUID()
		bucketName := metabase.BucketName(testrand.BucketName())

		createObject := func(t *testing.T, objStream metabase.ObjectStream) (metabase.Object, []metabase.Segment) {
			return metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream: objStream,
				},
			}.Run(ctx, t, db, objStream, 2)
		}

		randVersion := func() metabase.Version {
			return metabase.Version(1 + (testrand.Int63n(math.MaxInt64) - 2))
		}

		randObjectStream := func() metabase.ObjectStream {
			return metabase.ObjectStream{
				ProjectID:  projectID,
				BucketName: bucketName,
				ObjectKey:  metabase.ObjectKey(testrand.Path()),
				Version:    randVersion(),
				StreamID:   testrand.UUID(),
			}
		}

		t.Run("Unversioned", func(t *testing.T) {
			t.Run("Basic", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				obj1, _ := createObject(t, randObjectStream())
				obj2, _ := createObject(t, randObjectStream())

				// These objects are added to ensure that we don't accidentally
				// delete objects residing in different projects or buckets.
				differentBucketObj, differentBucketSegs := createObject(t, metabase.ObjectStream{
					ProjectID:  obj1.ProjectID,
					BucketName: metabase.BucketName(testrand.BucketName()),
					ObjectKey:  obj1.ObjectKey,
					Version:    obj1.Version,
					StreamID:   testrand.UUID(),
				})

				differentProjectObj, differentProjectSegs := createObject(t, metabase.ObjectStream{
					ProjectID:  testrand.UUID(),
					BucketName: obj1.BucketName,
					ObjectKey:  obj1.ObjectKey,
					Version:    obj1.Version,
					StreamID:   testrand.UUID(),
				})

				obj1StreamVersionID := obj1.StreamVersionID()

				metabasetest.DeleteObjects{
					Opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Items: []metabase.DeleteObjectsItem{
							{
								ObjectKey:       obj1.ObjectKey,
								StreamVersionID: obj1.StreamVersionID(),
							}, {
								ObjectKey: obj2.ObjectKey,
							},
						},
					},
					Result: metabase.DeleteObjectsResult{
						Items: []metabase.DeleteObjectsResultItem{
							{
								ObjectKey:                obj1.ObjectKey,
								RequestedStreamVersionID: obj1StreamVersionID,
								Removed: &metabase.DeleteObjectsInfo{
									StreamVersionID: obj1StreamVersionID,
									Status:          metabase.CommittedUnversioned,
								},
								Status: metabase.DeleteStatusOK,
							}, {
								ObjectKey: obj2.ObjectKey,
								Removed: &metabase.DeleteObjectsInfo{
									StreamVersionID: obj2.StreamVersionID(),
									Status:          metabase.CommittedUnversioned,
								},
								Status: metabase.DeleteStatusOK,
							},
						},
						DeletedSegmentCount: int64(obj1.SegmentCount + obj2.SegmentCount),
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects:  metabasetest.ObjectsToRaw(differentBucketObj, differentProjectObj),
					Segments: metabasetest.SegmentsToRaw(concat(differentBucketSegs, differentProjectSegs)),
				}.Check(ctx, t, db)
			})

			t.Run("Not found", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				key1, key2 := metabase.ObjectKey(testrand.Path()), metabase.ObjectKey(testrand.Path())
				streamVersionID1 := metabase.NewStreamVersionID(randVersion(), testrand.UUID())

				// Ensure that an object is not deleted if only one of the object's version and stream ID is correct.
				obj, segments := createObject(t, randObjectStream())
				objStreamVersionID1 := metabase.NewStreamVersionID(obj.Version, testrand.UUID())
				objStreamVersionID2 := metabase.NewStreamVersionID(randVersion(), obj.StreamID)

				metabasetest.DeleteObjects{
					Opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Items: []metabase.DeleteObjectsItem{
							{
								ObjectKey:       key1,
								StreamVersionID: streamVersionID1,
							}, {
								ObjectKey: key2,
							}, {
								ObjectKey:       obj.ObjectKey,
								StreamVersionID: objStreamVersionID1,
							}, {
								ObjectKey:       obj.ObjectKey,
								StreamVersionID: objStreamVersionID2,
							},
						},
					},
					Result: metabase.DeleteObjectsResult{
						Items: []metabase.DeleteObjectsResultItem{
							{
								ObjectKey:                key1,
								RequestedStreamVersionID: streamVersionID1,
								Status:                   metabase.DeleteStatusNotFound,
							}, {
								ObjectKey: key2,
								Status:    metabase.DeleteStatusNotFound,
							}, {
								ObjectKey:                obj.ObjectKey,
								RequestedStreamVersionID: objStreamVersionID1,
								Status:                   metabase.DeleteStatusNotFound,
							}, {
								ObjectKey:                obj.ObjectKey,
								RequestedStreamVersionID: objStreamVersionID2,
								Status:                   metabase.DeleteStatusNotFound,
							},
						},
						DeletedSegmentCount: 0,
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects:  metabasetest.ObjectsToRaw(obj),
					Segments: metabasetest.SegmentsToRaw(segments),
				}.Check(ctx, t, db)
			})

			t.Run("Pending object", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				obj := metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: randObjectStream(),
						Encryption:   metabasetest.DefaultEncryption,
					},
				}.Check(ctx, t, db)

				segments := metabasetest.CreateSegments(ctx, t, db, obj.ObjectStream, nil, 2)

				metabasetest.DeleteObjects{
					Opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Items: []metabase.DeleteObjectsItem{{
							ObjectKey: obj.ObjectKey,
						}},
					},
					Result: metabase.DeleteObjectsResult{
						Items: []metabase.DeleteObjectsResultItem{{
							ObjectKey: obj.ObjectKey,
							Status:    metabase.DeleteStatusNotFound,
						}},
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects:  metabasetest.ObjectsToRaw(obj),
					Segments: metabasetest.SegmentsToRaw(segments),
				}.Check(ctx, t, db)

				sv := obj.StreamVersionID()

				metabasetest.DeleteObjects{
					Opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Items: []metabase.DeleteObjectsItem{{
							ObjectKey:       obj.ObjectKey,
							StreamVersionID: sv,
						}},
					},
					Result: metabase.DeleteObjectsResult{
						Items: []metabase.DeleteObjectsResultItem{{
							ObjectKey:                obj.ObjectKey,
							RequestedStreamVersionID: sv,
							Removed: &metabase.DeleteObjectsInfo{
								StreamVersionID: sv,
								Status:          metabase.Pending,
							},
							Status: metabase.DeleteStatusOK,
						}},
						DeletedSegmentCount: int64(len(segments)),
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{}.Check(ctx, t, db)
			})

			t.Run("Duplicate deletion", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				obj, _ := createObject(t, randObjectStream())
				sv := obj.StreamVersionID()
				reqItem := metabase.DeleteObjectsItem{
					ObjectKey:       obj.ObjectKey,
					StreamVersionID: sv,
				}

				metabasetest.DeleteObjects{
					Opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Items:      []metabase.DeleteObjectsItem{reqItem, reqItem},
					},
					Result: metabase.DeleteObjectsResult{
						Items: []metabase.DeleteObjectsResultItem{{
							ObjectKey:                obj.ObjectKey,
							RequestedStreamVersionID: sv,
							Removed: &metabase.DeleteObjectsInfo{
								StreamVersionID: sv,
								Status:          metabase.CommittedUnversioned,
							},
							Status: metabase.DeleteStatusOK,
						}},
						DeletedSegmentCount: int64(obj.SegmentCount),
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{}.Check(ctx, t, db)
			})

			// This tests the case where an object's last committed version is specified
			// in the deletion request both indirectly and explicitly.
			t.Run("Duplicate deletion (indirect)", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				obj, _ := createObject(t, randObjectStream())
				sv := obj.StreamVersionID()

				expectedRemoved := &metabase.DeleteObjectsInfo{
					StreamVersionID: sv,
					Status:          metabase.CommittedUnversioned,
				}

				metabasetest.DeleteObjects{
					Opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Items: []metabase.DeleteObjectsItem{
							{
								ObjectKey:       obj.ObjectKey,
								StreamVersionID: sv,
							}, {
								ObjectKey: obj.ObjectKey,
							},
						},
					},
					Result: metabase.DeleteObjectsResult{
						Items: []metabase.DeleteObjectsResultItem{
							{
								ObjectKey:                obj.ObjectKey,
								RequestedStreamVersionID: sv,
								Removed:                  expectedRemoved,
								Status:                   metabase.DeleteStatusOK,
							}, {
								ObjectKey: obj.ObjectKey,
								Removed:   expectedRemoved,
								Status:    metabase.DeleteStatusOK,
							},
						},
						DeletedSegmentCount: int64(obj.SegmentCount),
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{}.Check(ctx, t, db)
			})
		})

		t.Run("Versioned", func(t *testing.T) {
			t.Run("Basic", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				// We create 4 objects to ensure that the method can handle multiple
				// of each kind of deletion (exact version deletion or delete marker insertion).
				obj1, _ := createObject(t, randObjectStream())
				obj2, _ := createObject(t, randObjectStream())

				obj1StreamVersionID := obj1.StreamVersionID()
				obj2StreamVersionID := obj2.StreamVersionID()

				obj3, obj3Segments := createObject(t, randObjectStream())
				obj4, obj4Segments := createObject(t, randObjectStream())

				differentBucketObj, differentBucketSegs := createObject(t, metabase.ObjectStream{
					ProjectID:  obj1.ProjectID,
					BucketName: metabase.BucketName(testrand.BucketName()),
					ObjectKey:  obj1.ObjectKey,
					Version:    obj1.Version,
					StreamID:   testrand.UUID(),
				})

				differentProjectObj, differentProjectSegs := createObject(t, metabase.ObjectStream{
					ProjectID:  testrand.UUID(),
					BucketName: obj1.BucketName,
					ObjectKey:  obj1.ObjectKey,
					Version:    obj1.Version,
					StreamID:   testrand.UUID(),
				})

				metabasetest.DeleteObjects{
					Opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Versioned:  true,
						Items: []metabase.DeleteObjectsItem{
							{
								ObjectKey:       obj1.ObjectKey,
								StreamVersionID: obj1StreamVersionID,
							}, {
								ObjectKey:       obj2.ObjectKey,
								StreamVersionID: obj2StreamVersionID,
							}, {
								ObjectKey: obj3.ObjectKey,
							}, {
								ObjectKey: obj4.ObjectKey,
							},
						},
					},
					Result: metabase.DeleteObjectsResult{
						Items: []metabase.DeleteObjectsResultItem{
							{
								ObjectKey:                obj1.ObjectKey,
								RequestedStreamVersionID: obj1StreamVersionID,
								Removed: &metabase.DeleteObjectsInfo{
									StreamVersionID: obj1StreamVersionID,
									Status:          metabase.CommittedUnversioned,
								},
								Status: metabase.DeleteStatusOK,
							}, {
								ObjectKey:                obj2.ObjectKey,
								RequestedStreamVersionID: obj2StreamVersionID,
								Removed: &metabase.DeleteObjectsInfo{
									StreamVersionID: obj2StreamVersionID,
									Status:          metabase.CommittedUnversioned,
								},
								Status: metabase.DeleteStatusOK,
							}, {
								ObjectKey: obj3.ObjectKey,
								Marker: &metabase.DeleteObjectsInfo{
									StreamVersionID: metabase.NewStreamVersionID(obj3.Version+1, uuid.UUID{}),
									Status:          metabase.DeleteMarkerVersioned,
								},
								Status: metabase.DeleteStatusOK,
							}, {
								ObjectKey: obj4.ObjectKey,
								Marker: &metabase.DeleteObjectsInfo{
									StreamVersionID: metabase.NewStreamVersionID(obj4.Version+1, uuid.UUID{}),
									Status:          metabase.DeleteMarkerVersioned,
								},
								Status: metabase.DeleteStatusOK,
							},
						},
						DeletedSegmentCount: int64(obj1.SegmentCount + obj2.SegmentCount),
					},
				}.Check(ctx, t, db)

				obj3DeleteMarker, err := db.GetObjectExactVersion(ctx, metabase.GetObjectExactVersion{
					ObjectLocation: obj3.Location(),
					Version:        obj3.Version + 1,
				})
				require.NoError(t, err)

				obj4DeleteMarker, err := db.GetObjectExactVersion(ctx, metabase.GetObjectExactVersion{
					ObjectLocation: obj4.Location(),
					Version:        obj4.Version + 1,
				})
				require.NoError(t, err)

				metabasetest.Verify{
					Objects: metabasetest.ObjectsToRaw(
						obj3,
						obj3DeleteMarker,
						obj4,
						obj4DeleteMarker,
						differentBucketObj,
						differentProjectObj,
					),
					Segments: metabasetest.SegmentsToRaw(concat(
						obj3Segments,
						obj4Segments,
						differentBucketSegs,
						differentProjectSegs,
					)),
				}.Check(ctx, t, db)
			})

			t.Run("Not found", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				missingObjStream1, missingObjStream2 := randObjectStream(), randObjectStream()
				missingStreamVersionID := metabase.NewStreamVersionID(missingObjStream1.Version, missingObjStream1.StreamID)

				// Ensure that an object is not deleted if only one of the object's version and stream ID is correct.
				obj, segments := createObject(t, randObjectStream())
				badStreamVersionID1 := metabase.NewStreamVersionID(obj.Version, testrand.UUID())
				badStreamVersionID2 := metabase.NewStreamVersionID(randVersion(), obj.StreamID)

				metabasetest.DeleteObjects{
					Opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Versioned:  true,
						Items: []metabase.DeleteObjectsItem{
							{
								ObjectKey:       missingObjStream1.ObjectKey,
								StreamVersionID: missingStreamVersionID,
							}, {
								ObjectKey: missingObjStream2.ObjectKey,
							}, {
								ObjectKey:       obj.ObjectKey,
								StreamVersionID: badStreamVersionID1,
							}, {
								ObjectKey:       obj.ObjectKey,
								StreamVersionID: badStreamVersionID2,
							},
						},
					},
					Result: metabase.DeleteObjectsResult{
						Items: []metabase.DeleteObjectsResultItem{
							{
								ObjectKey:                missingObjStream1.ObjectKey,
								RequestedStreamVersionID: missingStreamVersionID,
								Status:                   metabase.DeleteStatusNotFound,
							}, {
								ObjectKey: missingObjStream2.ObjectKey,
								Marker: &metabase.DeleteObjectsInfo{
									StreamVersionID: metabase.NewStreamVersionID(1, uuid.UUID{}),
									Status:          metabase.DeleteMarkerVersioned,
								},
								Status: metabase.DeleteStatusOK,
							}, {
								ObjectKey:                obj.ObjectKey,
								RequestedStreamVersionID: badStreamVersionID1,
								Status:                   metabase.DeleteStatusNotFound,
							}, {
								ObjectKey:                obj.ObjectKey,
								RequestedStreamVersionID: badStreamVersionID2,
								Status:                   metabase.DeleteStatusNotFound,
							},
						},
					},
				}.Check(ctx, t, db)

				deleteMarker, err := db.GetObjectExactVersion(ctx, metabase.GetObjectExactVersion{
					ObjectLocation: missingObjStream2.Location(),
					Version:        1,
				})
				require.NoError(t, err)

				metabasetest.Verify{
					Objects:  metabasetest.ObjectsToRaw(obj, deleteMarker),
					Segments: metabasetest.SegmentsToRaw(segments),
				}.Check(ctx, t, db)
			})

			t.Run("Pending object", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				pending := metabasetest.BeginObjectExactVersion{
					Opts: metabase.BeginObjectExactVersion{
						ObjectStream: randObjectStream(),
						Encryption:   metabasetest.DefaultEncryption,
					},
				}.Check(ctx, t, db)

				segments := metabasetest.CreateSegments(ctx, t, db, pending.ObjectStream, nil, 2)

				metabasetest.DeleteObjects{
					Opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Versioned:  true,
						Items: []metabase.DeleteObjectsItem{{
							ObjectKey: pending.ObjectKey,
						}},
					},
					Result: metabase.DeleteObjectsResult{
						Items: []metabase.DeleteObjectsResultItem{{
							ObjectKey: pending.ObjectKey,
							Marker: &metabase.DeleteObjectsInfo{
								StreamVersionID: metabase.NewStreamVersionID(pending.Version+1, uuid.UUID{}),
								Status:          metabase.DeleteMarkerVersioned,
							},
							Status: metabase.DeleteStatusOK,
						}},
					},
				}.Check(ctx, t, db)

				deleteMarker, err := db.GetObjectExactVersion(ctx, metabase.GetObjectExactVersion{
					ObjectLocation: pending.Location(),
					Version:        pending.Version + 1,
				})
				require.NoError(t, err)

				metabasetest.Verify{
					Objects:  metabasetest.ObjectsToRaw(pending, deleteMarker),
					Segments: metabasetest.SegmentsToRaw(segments),
				}.Check(ctx, t, db)

				sv := pending.StreamVersionID()

				metabasetest.DeleteObjects{
					Opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Versioned:  true,
						Items: []metabase.DeleteObjectsItem{{
							ObjectKey:       pending.ObjectKey,
							StreamVersionID: sv,
						}},
					},
					Result: metabase.DeleteObjectsResult{
						Items: []metabase.DeleteObjectsResultItem{{
							ObjectKey:                pending.ObjectKey,
							RequestedStreamVersionID: sv,
							Removed: &metabase.DeleteObjectsInfo{
								StreamVersionID: sv,
								Status:          metabase.Pending,
							},
							Status: metabase.DeleteStatusOK,
						}},
						DeletedSegmentCount: int64(len(segments)),
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects: metabasetest.ObjectsToRaw(deleteMarker),
				}.Check(ctx, t, db)
			})

			t.Run("Duplicate deletion", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				obj1, _ := createObject(t, randObjectStream())
				obj1StreamVersionID := obj1.StreamVersionID()
				reqItem1 := metabase.DeleteObjectsItem{
					ObjectKey:       obj1.ObjectKey,
					StreamVersionID: obj1StreamVersionID,
				}

				obj2, obj2Segments := createObject(t, randObjectStream())
				reqItem2 := metabase.DeleteObjectsItem{
					ObjectKey: obj2.ObjectKey,
				}

				metabasetest.DeleteObjects{
					Opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Versioned:  true,
						Items:      []metabase.DeleteObjectsItem{reqItem1, reqItem1, reqItem2, reqItem2},
					},
					Result: metabase.DeleteObjectsResult{
						Items: []metabase.DeleteObjectsResultItem{
							{
								ObjectKey:                obj1.ObjectKey,
								RequestedStreamVersionID: obj1StreamVersionID,
								Removed: &metabase.DeleteObjectsInfo{
									StreamVersionID: obj1StreamVersionID,
									Status:          metabase.CommittedUnversioned,
								},
								Status: metabase.DeleteStatusOK,
							}, {
								ObjectKey: obj2.ObjectKey,
								Marker: &metabase.DeleteObjectsInfo{
									StreamVersionID: metabase.NewStreamVersionID(obj2.Version+1, uuid.UUID{}),
									Status:          metabase.DeleteMarkerVersioned,
								},
								Status: metabase.DeleteStatusOK,
							},
						},
						DeletedSegmentCount: int64(obj1.SegmentCount),
					},
				}.Check(ctx, t, db)

				deleteMarker, err := db.GetObjectExactVersion(ctx, metabase.GetObjectExactVersion{
					ObjectLocation: obj2.Location(),
					Version:        obj2.Version + 1,
				})
				require.NoError(t, err)

				metabasetest.Verify{
					Objects:  metabasetest.ObjectsToRaw(obj2, deleteMarker),
					Segments: metabasetest.SegmentsToRaw(obj2Segments),
				}.Check(ctx, t, db)
			})

			t.Run("Duplicate deletion (indirect)", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				obj, _ := createObject(t, randObjectStream())
				sv := obj.StreamVersionID()

				metabasetest.DeleteObjects{
					Opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Versioned:  true,
						Items: []metabase.DeleteObjectsItem{
							{
								ObjectKey:       obj.ObjectKey,
								StreamVersionID: sv,
							}, {
								ObjectKey: obj.ObjectKey,
							},
						},
					},
					Result: metabase.DeleteObjectsResult{
						Items: []metabase.DeleteObjectsResultItem{
							{
								ObjectKey:                obj.ObjectKey,
								RequestedStreamVersionID: sv,
								Removed: &metabase.DeleteObjectsInfo{
									StreamVersionID: sv,
									Status:          metabase.CommittedUnversioned,
								},
								Status: metabase.DeleteStatusOK,
							}, {
								ObjectKey: obj.ObjectKey,
								Marker: &metabase.DeleteObjectsInfo{
									StreamVersionID: metabase.NewStreamVersionID(1, uuid.UUID{}),
									Status:          metabase.DeleteMarkerVersioned,
								},
								Status: metabase.DeleteStatusOK,
							},
						},
						DeletedSegmentCount: int64(obj.SegmentCount),
					},
				}.Check(ctx, t, db)

				deleteMarker, err := db.GetObjectExactVersion(ctx, metabase.GetObjectExactVersion{
					ObjectLocation: obj.Location(),
					Version:        1,
				})
				require.NoError(t, err)

				metabasetest.Verify{
					Objects: metabasetest.ObjectsToRaw(deleteMarker),
				}.Check(ctx, t, db)
			})
		})

		t.Run("Invalid options", func(t *testing.T) {
			validItem := metabase.DeleteObjectsItem{
				ObjectKey:       metabase.ObjectKey(testrand.Path()),
				StreamVersionID: metabase.NewStreamVersionID(randVersion(), testrand.UUID()),
			}

			for _, tt := range []struct {
				name   string
				opts   metabase.DeleteObjects
				errMsg string
			}{
				{
					name: "Project ID missing",
					opts: metabase.DeleteObjects{
						BucketName: bucketName,
						Items:      []metabase.DeleteObjectsItem{validItem},
					},
					errMsg: "ProjectID missing",
				}, {
					name: "Bucket name missing",
					opts: metabase.DeleteObjects{
						ProjectID: projectID,
						Items:     []metabase.DeleteObjectsItem{validItem},
					},
					errMsg: "BucketName missing",
				}, {
					name: "Items missing",
					opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
					},
					errMsg: "Items missing",
				}, {
					name: "Too many items",
					opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Items:      make([]metabase.DeleteObjectsItem, 1001),
					},
					errMsg: "Items is too long; expected <= 1000, but got 1001",
				}, {
					name: "Missing object key",
					opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Items: []metabase.DeleteObjectsItem{{
							StreamVersionID: validItem.StreamVersionID,
						}},
					},
					errMsg: "Items[0].ObjectKey missing",
				}, {
					name: "Invalid version",
					opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Items: []metabase.DeleteObjectsItem{{
							ObjectKey:       validItem.ObjectKey,
							StreamVersionID: metabase.NewStreamVersionID(-1, testrand.UUID()),
						}},
					},
					errMsg: "Items[0].StreamVersionID invalid: version is -1",
				},
			} {
				t.Run(tt.name, func(t *testing.T) {
					defer metabasetest.DeleteAll{}.Check(ctx, t, db)

					metabasetest.DeleteObjects{
						Opts:     tt.opts,
						ErrClass: &metabase.ErrInvalidRequest,
						ErrText:  tt.errMsg,
					}.Check(ctx, t, db)

					metabasetest.Verify{}.Check(ctx, t, db)
				})
			}
		})
	})
}
