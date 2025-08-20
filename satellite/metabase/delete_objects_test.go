// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
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

		createObject := func(t *testing.T, objStream metabase.ObjectStream, versioned bool) (metabase.Object, []metabase.Segment) {
			return metabasetest.CreateTestObject{
				CommitObject: &metabase.CommitObject{
					ObjectStream: objStream,
					Versioned:    versioned,
				},
			}.Run(ctx, t, db, objStream, 2)
		}

		createLockedObject := func(t *testing.T, testCase metabasetest.ObjectLockDeletionTestCase) (metabase.Object, []metabase.Segment) {
			objStream := randObjectStream()
			return metabasetest.CreateTestObject{
				BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
					ObjectStream: objStream,
					Encryption:   metabasetest.DefaultEncryption,
					Retention:    testCase.Retention,
					LegalHold:    testCase.LegalHold,
				},
			}.Run(ctx, t, db, objStream, 2)
		}

		runUnversionedTests := func(t *testing.T, objectLockEnabled bool) {
			t.Run("Basic", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				obj1, _ := createObject(t, randObjectStream(), false)
				obj2, _ := createObject(t, randObjectStream(), false)

				// These objects are added to ensure that we don't accidentally
				// delete objects residing in different projects or buckets.
				differentBucketObj, differentBucketSegs := createObject(t, metabase.ObjectStream{
					ProjectID:  obj1.ProjectID,
					BucketName: metabase.BucketName(testrand.BucketName()),
					ObjectKey:  obj1.ObjectKey,
					Version:    obj1.Version,
					StreamID:   testrand.UUID(),
				}, false)

				differentProjectObj, differentProjectSegs := createObject(t, metabase.ObjectStream{
					ProjectID:  testrand.UUID(),
					BucketName: obj1.BucketName,
					ObjectKey:  obj1.ObjectKey,
					Version:    obj1.Version,
					StreamID:   testrand.UUID(),
				}, false)

				obj1StreamVersionID := obj1.StreamVersionID()

				metabasetest.DeleteObjects{
					Opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						ObjectLock: metabase.ObjectLockDeleteOptions{
							Enabled: objectLockEnabled,
						},
						Items: []metabase.DeleteObjectsItem{
							{
								ObjectKey:       obj1.ObjectKey,
								StreamVersionID: obj1.StreamVersionID(),
							},
							{
								ObjectKey: obj2.ObjectKey,
							},
						},
					},
					Result: metabase.DeleteObjectsResult{
						Items: []metabase.DeleteObjectsResultItem{
							{
								ObjectKey: obj2.ObjectKey,
								Removed: &metabase.DeleteObjectsInfo{
									StreamVersionID: obj2.StreamVersionID(),
									Status:          metabase.CommittedUnversioned,
								},
								Status: storj.DeleteObjectsStatusOK,
							},
							{
								ObjectKey:                obj1.ObjectKey,
								RequestedStreamVersionID: obj1StreamVersionID,
								Removed: &metabase.DeleteObjectsInfo{
									StreamVersionID: obj1StreamVersionID,
									Status:          metabase.CommittedUnversioned,
								},
								Status: storj.DeleteObjectsStatusOK,
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
				obj, segments := createObject(t, randObjectStream(), false)
				objStreamVersionID1 := metabase.NewStreamVersionID(obj.Version, testrand.UUID())
				objStreamVersionID2 := metabase.NewStreamVersionID(randVersion(), obj.StreamID)

				metabasetest.DeleteObjects{
					Opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						ObjectLock: metabase.ObjectLockDeleteOptions{
							Enabled: objectLockEnabled,
						},
						Items: []metabase.DeleteObjectsItem{
							{
								ObjectKey:       key1,
								StreamVersionID: streamVersionID1,
							},
							{
								ObjectKey: key2,
							},
							{
								ObjectKey:       obj.ObjectKey,
								StreamVersionID: objStreamVersionID1,
							},
							{
								ObjectKey:       obj.ObjectKey,
								StreamVersionID: objStreamVersionID2,
							},
						},
					},
					Result: metabase.DeleteObjectsResult{
						Items: []metabase.DeleteObjectsResultItem{
							{
								ObjectKey: key2,
								Status:    storj.DeleteObjectsStatusNotFound,
							},
							{
								ObjectKey:                key1,
								RequestedStreamVersionID: streamVersionID1,
								Status:                   storj.DeleteObjectsStatusNotFound,
							},
							{
								ObjectKey:                obj.ObjectKey,
								RequestedStreamVersionID: objStreamVersionID1,
								Status:                   storj.DeleteObjectsStatusNotFound,
							},
							{
								ObjectKey:                obj.ObjectKey,
								RequestedStreamVersionID: objStreamVersionID2,
								Status:                   storj.DeleteObjectsStatusNotFound,
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
						ObjectLock: metabase.ObjectLockDeleteOptions{
							Enabled: objectLockEnabled,
						},
						Items: []metabase.DeleteObjectsItem{{
							ObjectKey: obj.ObjectKey,
						}},
					},
					Result: metabase.DeleteObjectsResult{
						Items: []metabase.DeleteObjectsResultItem{{
							ObjectKey: obj.ObjectKey,
							Status:    storj.DeleteObjectsStatusNotFound,
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
						ObjectLock: metabase.ObjectLockDeleteOptions{
							Enabled: objectLockEnabled,
						},
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
							Status: storj.DeleteObjectsStatusOK,
						}},
						DeletedSegmentCount: int64(len(segments)),
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{}.Check(ctx, t, db)
			})

			t.Run("Duplicate deletion", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				obj, _ := createObject(t, randObjectStream(), false)
				sv := obj.StreamVersionID()
				reqItem := metabase.DeleteObjectsItem{
					ObjectKey:       obj.ObjectKey,
					StreamVersionID: sv,
				}

				metabasetest.DeleteObjects{
					Opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						ObjectLock: metabase.ObjectLockDeleteOptions{
							Enabled: objectLockEnabled,
						},
						Items: []metabase.DeleteObjectsItem{reqItem, reqItem},
					},
					Result: metabase.DeleteObjectsResult{
						Items: []metabase.DeleteObjectsResultItem{{
							ObjectKey:                obj.ObjectKey,
							RequestedStreamVersionID: sv,
							Removed: &metabase.DeleteObjectsInfo{
								StreamVersionID: sv,
								Status:          metabase.CommittedUnversioned,
							},
							Status: storj.DeleteObjectsStatusOK,
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

				obj, _ := createObject(t, randObjectStream(), false)
				sv := obj.StreamVersionID()

				expectedRemoved := &metabase.DeleteObjectsInfo{
					StreamVersionID: sv,
					Status:          metabase.CommittedUnversioned,
				}

				opts := metabasetest.DeleteObjects{
					Opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						ObjectLock: metabase.ObjectLockDeleteOptions{
							Enabled: objectLockEnabled,
						},
						Items: []metabase.DeleteObjectsItem{
							{
								ObjectKey:       obj.ObjectKey,
								StreamVersionID: sv,
							},
							{
								ObjectKey: obj.ObjectKey,
							},
						},
					},
					Result: metabase.DeleteObjectsResult{
						Items: []metabase.DeleteObjectsResultItem{
							{
								ObjectKey: obj.ObjectKey,
								Removed:   expectedRemoved,
								Status:    storj.DeleteObjectsStatusOK,
							},
							{
								ObjectKey:                obj.ObjectKey,
								RequestedStreamVersionID: sv,
								Removed:                  expectedRemoved,
								Status:                   storj.DeleteObjectsStatusOK,
							},
						},
						DeletedSegmentCount: int64(obj.SegmentCount),
					},
				}
				opts.Check(ctx, t, db)

				metabasetest.Verify{}.Check(ctx, t, db)

				metabasetest.DeleteAll{}.Check(ctx, t, db)

				createObject(t, obj.ObjectStream, false)
				opts.Opts.Items[0], opts.Opts.Items[1] = opts.Opts.Items[1], opts.Opts.Items[0]
				opts.Check(ctx, t, db)

				metabasetest.Verify{}.Check(ctx, t, db)
			})
		}

		t.Run("Unversioned", func(t *testing.T) {
			runUnversionedTests(t, false)
		})

		t.Run("Unversioned - Object Lock", func(t *testing.T) {
			runUnversionedTests(t, true)

			metabasetest.ObjectLockDeletionTestRunner{
				TestProtected: func(t *testing.T, testCase metabasetest.ObjectLockDeletionTestCase) {
					defer metabasetest.DeleteAll{}.Check(ctx, t, db)

					exactVersionObj, exactVersionSegments := createLockedObject(t, testCase)
					exactVersionStreamVersionID := exactVersionObj.StreamVersionID()

					lastCommittedObj, lastCommittedSegments := createLockedObject(t, testCase)

					metabasetest.DeleteObjects{
						Opts: metabase.DeleteObjects{
							ProjectID:  projectID,
							BucketName: bucketName,
							ObjectLock: metabase.ObjectLockDeleteOptions{
								Enabled:          true,
								BypassGovernance: testCase.BypassGovernance,
							},
							Items: []metabase.DeleteObjectsItem{
								{
									ObjectKey:       exactVersionObj.ObjectKey,
									StreamVersionID: exactVersionStreamVersionID,
								},
								{
									ObjectKey: lastCommittedObj.ObjectKey,
								},
							},
						},
						Result: metabase.DeleteObjectsResult{
							Items: []metabase.DeleteObjectsResultItem{
								{
									ObjectKey: lastCommittedObj.ObjectKey,
									Status:    storj.DeleteObjectsStatusLocked,
								},
								{
									ObjectKey:                exactVersionObj.ObjectKey,
									RequestedStreamVersionID: exactVersionStreamVersionID,
									Status:                   storj.DeleteObjectsStatusLocked,
								},
							},
						},
					}.Check(ctx, t, db)

					metabasetest.Verify{
						Objects:  metabasetest.ObjectsToRaw(exactVersionObj, lastCommittedObj),
						Segments: metabasetest.SegmentsToRaw(concat(exactVersionSegments, lastCommittedSegments)),
					}.Check(ctx, t, db)
				},
				TestRemovable: func(t *testing.T, testCase metabasetest.ObjectLockDeletionTestCase) {
					defer metabasetest.DeleteAll{}.Check(ctx, t, db)

					exactVersionObj, _ := createLockedObject(t, testCase)
					exactVersionStreamVersionID := exactVersionObj.StreamVersionID()

					lastCommittedObj, _ := createLockedObject(t, testCase)

					metabasetest.DeleteObjects{
						Opts: metabase.DeleteObjects{
							ProjectID:  projectID,
							BucketName: bucketName,
							Items: []metabase.DeleteObjectsItem{
								{
									ObjectKey:       exactVersionObj.ObjectKey,
									StreamVersionID: exactVersionStreamVersionID,
								},
								{
									ObjectKey: lastCommittedObj.ObjectKey,
								},
							},
						},
						Result: metabase.DeleteObjectsResult{
							Items: []metabase.DeleteObjectsResultItem{
								{
									ObjectKey: lastCommittedObj.ObjectKey,
									Removed: &metabase.DeleteObjectsInfo{
										StreamVersionID: lastCommittedObj.StreamVersionID(),
										Status:          metabase.CommittedUnversioned,
									},
									Status: storj.DeleteObjectsStatusOK,
								},
								{
									ObjectKey:                exactVersionObj.ObjectKey,
									RequestedStreamVersionID: exactVersionStreamVersionID,
									Removed: &metabase.DeleteObjectsInfo{
										StreamVersionID: exactVersionStreamVersionID,
										Status:          metabase.CommittedUnversioned,
									},
									Status: storj.DeleteObjectsStatusOK,
								},
							},
							DeletedSegmentCount: int64(exactVersionObj.SegmentCount + lastCommittedObj.SegmentCount),
						},
					}.Check(ctx, t, db)

					metabasetest.Verify{}.Check(ctx, t, db)
				},
			}.Run(t)
		})

		runVersionedTests := func(t *testing.T, objectLockEnabled bool) {
			t.Run("Basic", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				// We create 4 objects to ensure that the method can handle multiple
				// of each kind of deletion (exact version deletion or delete marker insertion).
				obj1, _ := createObject(t, randObjectStream(), true)
				obj2, _ := createObject(t, randObjectStream(), true)

				obj1StreamVersionID := obj1.StreamVersionID()
				obj2StreamVersionID := obj2.StreamVersionID()

				obj3, obj3Segments := createObject(t, randObjectStream(), true)
				obj4, obj4Segments := createObject(t, randObjectStream(), true)

				differentBucketObj, differentBucketSegs := createObject(t, metabase.ObjectStream{
					ProjectID:  obj1.ProjectID,
					BucketName: metabase.BucketName(testrand.BucketName()),
					ObjectKey:  obj1.ObjectKey,
					Version:    obj1.Version,
					StreamID:   testrand.UUID(),
				}, true)

				differentProjectObj, differentProjectSegs := createObject(t, metabase.ObjectStream{
					ProjectID:  testrand.UUID(),
					BucketName: obj1.BucketName,
					ObjectKey:  obj1.ObjectKey,
					Version:    obj1.Version,
					StreamID:   testrand.UUID(),
				}, true)

				metabasetest.DeleteObjects{
					Opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Versioned:  true,
						ObjectLock: metabase.ObjectLockDeleteOptions{
							Enabled: objectLockEnabled,
						},
						Items: []metabase.DeleteObjectsItem{
							{
								ObjectKey:       obj1.ObjectKey,
								StreamVersionID: obj1StreamVersionID,
							},
							{
								ObjectKey:       obj2.ObjectKey,
								StreamVersionID: obj2StreamVersionID,
							},
							{
								ObjectKey: obj3.ObjectKey,
							},
							{
								ObjectKey: obj4.ObjectKey,
							},
						},
					},
					Result: metabase.DeleteObjectsResult{
						Items: []metabase.DeleteObjectsResultItem{
							{
								ObjectKey: obj3.ObjectKey,
								Marker: &metabase.DeleteObjectsInfo{
									StreamVersionID: metabase.NewStreamVersionID(obj3.Version+1, uuid.UUID{}),
									Status:          metabase.DeleteMarkerVersioned,
								},
								Status: storj.DeleteObjectsStatusOK,
							},
							{
								ObjectKey: obj4.ObjectKey,
								Marker: &metabase.DeleteObjectsInfo{
									StreamVersionID: metabase.NewStreamVersionID(obj4.Version+1, uuid.UUID{}),
									Status:          metabase.DeleteMarkerVersioned,
								},
								Status: storj.DeleteObjectsStatusOK,
							},
							{
								ObjectKey:                obj1.ObjectKey,
								RequestedStreamVersionID: obj1StreamVersionID,
								Removed: &metabase.DeleteObjectsInfo{
									StreamVersionID: obj1StreamVersionID,
									Status:          metabase.CommittedVersioned,
								},
								Status: storj.DeleteObjectsStatusOK,
							},
							{
								ObjectKey:                obj2.ObjectKey,
								RequestedStreamVersionID: obj2StreamVersionID,
								Removed: &metabase.DeleteObjectsInfo{
									StreamVersionID: obj2StreamVersionID,
									Status:          metabase.CommittedVersioned,
								},
								Status: storj.DeleteObjectsStatusOK,
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
				obj, segments := createObject(t, randObjectStream(), true)
				badStreamVersionID1 := metabase.NewStreamVersionID(obj.Version, testrand.UUID())
				badStreamVersionID2 := metabase.NewStreamVersionID(randVersion(), obj.StreamID)

				metabasetest.DeleteObjects{
					Opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Versioned:  true,
						ObjectLock: metabase.ObjectLockDeleteOptions{
							Enabled: objectLockEnabled,
						},
						Items: []metabase.DeleteObjectsItem{
							{
								ObjectKey:       missingObjStream1.ObjectKey,
								StreamVersionID: missingStreamVersionID,
							},
							{
								ObjectKey: missingObjStream2.ObjectKey,
							},
							{
								ObjectKey:       obj.ObjectKey,
								StreamVersionID: badStreamVersionID1,
							},
							{
								ObjectKey:       obj.ObjectKey,
								StreamVersionID: badStreamVersionID2,
							},
						},
					},
					Result: metabase.DeleteObjectsResult{
						Items: []metabase.DeleteObjectsResultItem{
							{
								ObjectKey: missingObjStream2.ObjectKey,
								Marker: &metabase.DeleteObjectsInfo{
									StreamVersionID: metabase.NewStreamVersionID(1, uuid.UUID{}),
									Status:          metabase.DeleteMarkerVersioned,
								},
								Status: storj.DeleteObjectsStatusOK,
							},
							{
								ObjectKey:                missingObjStream1.ObjectKey,
								RequestedStreamVersionID: missingStreamVersionID,
								Status:                   storj.DeleteObjectsStatusNotFound,
							},
							{
								ObjectKey:                obj.ObjectKey,
								RequestedStreamVersionID: badStreamVersionID1,
								Status:                   storj.DeleteObjectsStatusNotFound,
							},
							{
								ObjectKey:                obj.ObjectKey,
								RequestedStreamVersionID: badStreamVersionID2,
								Status:                   storj.DeleteObjectsStatusNotFound,
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
						ObjectLock: metabase.ObjectLockDeleteOptions{
							Enabled: objectLockEnabled,
						},
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
							Status: storj.DeleteObjectsStatusOK,
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
						ObjectLock: metabase.ObjectLockDeleteOptions{
							Enabled: objectLockEnabled,
						},
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
							Status: storj.DeleteObjectsStatusOK,
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

				obj1, _ := createObject(t, randObjectStream(), true)
				obj1StreamVersionID := obj1.StreamVersionID()
				reqItem1 := metabase.DeleteObjectsItem{
					ObjectKey:       obj1.ObjectKey,
					StreamVersionID: obj1StreamVersionID,
				}

				obj2, obj2Segments := createObject(t, randObjectStream(), true)
				reqItem2 := metabase.DeleteObjectsItem{
					ObjectKey: obj2.ObjectKey,
				}

				metabasetest.DeleteObjects{
					Opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Versioned:  true,
						ObjectLock: metabase.ObjectLockDeleteOptions{
							Enabled: objectLockEnabled,
						},
						Items: []metabase.DeleteObjectsItem{reqItem1, reqItem1, reqItem2, reqItem2},
					},
					Result: metabase.DeleteObjectsResult{
						Items: []metabase.DeleteObjectsResultItem{
							{
								ObjectKey: obj2.ObjectKey,
								Marker: &metabase.DeleteObjectsInfo{
									StreamVersionID: metabase.NewStreamVersionID(obj2.Version+1, uuid.UUID{}),
									Status:          metabase.DeleteMarkerVersioned,
								},
								Status: storj.DeleteObjectsStatusOK,
							},
							{
								ObjectKey:                obj1.ObjectKey,
								RequestedStreamVersionID: obj1StreamVersionID,
								Removed: &metabase.DeleteObjectsInfo{
									StreamVersionID: obj1StreamVersionID,
									Status:          metabase.CommittedVersioned,
								},
								Status: storj.DeleteObjectsStatusOK,
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

				obj, _ := createObject(t, randObjectStream(), true)
				sv := obj.StreamVersionID()

				opts := metabasetest.DeleteObjects{
					Opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Versioned:  true,
						ObjectLock: metabase.ObjectLockDeleteOptions{
							Enabled: objectLockEnabled,
						},
						Items: []metabase.DeleteObjectsItem{
							{
								ObjectKey:       obj.ObjectKey,
								StreamVersionID: sv,
							},
							{
								ObjectKey: obj.ObjectKey,
							},
						},
					},
					Result: metabase.DeleteObjectsResult{
						Items: []metabase.DeleteObjectsResultItem{
							{
								ObjectKey: obj.ObjectKey,
								Marker: &metabase.DeleteObjectsInfo{
									StreamVersionID: metabase.NewStreamVersionID(obj.Version+1, uuid.UUID{}),
									Status:          metabase.DeleteMarkerVersioned,
								},
								Status: storj.DeleteObjectsStatusOK,
							},
							{
								ObjectKey:                obj.ObjectKey,
								RequestedStreamVersionID: sv,
								Removed: &metabase.DeleteObjectsInfo{
									StreamVersionID: sv,
									Status:          metabase.CommittedVersioned,
								},
								Status: storj.DeleteObjectsStatusOK,
							},
						},
						DeletedSegmentCount: int64(obj.SegmentCount),
					},
				}
				opts.Check(ctx, t, db)

				deleteMarker, err := db.GetObjectExactVersion(ctx, metabase.GetObjectExactVersion{
					ObjectLocation: obj.Location(),
					Version:        obj.Version + 1,
				})
				require.NoError(t, err)

				metabasetest.Verify{
					Objects: metabasetest.ObjectsToRaw(deleteMarker),
				}.Check(ctx, t, db)

				metabasetest.DeleteAll{}.Check(ctx, t, db)

				createObject(t, obj.ObjectStream, true)
				opts.Opts.Items[0], opts.Opts.Items[1] = opts.Opts.Items[1], opts.Opts.Items[0]
				opts.Check(ctx, t, db)

				deleteMarker, err = db.GetObjectExactVersion(ctx, metabase.GetObjectExactVersion{
					ObjectLocation: obj.Location(),
					Version:        obj.Version + 1,
				})
				require.NoError(t, err)

				metabasetest.Verify{
					Objects: metabasetest.ObjectsToRaw(deleteMarker),
				}.Check(ctx, t, db)
			})
		}

		t.Run("Versioned", func(t *testing.T) {
			runVersionedTests(t, false)
		})

		t.Run("Versioned - Object Lock", func(t *testing.T) {
			runVersionedTests(t, true)

			metabasetest.ObjectLockDeletionTestRunner{
				TestProtected: func(t *testing.T, testCase metabasetest.ObjectLockDeletionTestCase) {
					defer metabasetest.DeleteAll{}.Check(ctx, t, db)

					exactVersionObj, exactVersionSegments := createLockedObject(t, testCase)
					exactVersionStreamVersionID := exactVersionObj.StreamVersionID()

					lastCommittedObj, lastCommittedSegments := createLockedObject(t, testCase)

					metabasetest.DeleteObjects{
						Opts: metabase.DeleteObjects{
							ProjectID:  projectID,
							BucketName: bucketName,
							Versioned:  true,
							ObjectLock: metabase.ObjectLockDeleteOptions{
								Enabled:          true,
								BypassGovernance: testCase.BypassGovernance,
							},
							Items: []metabase.DeleteObjectsItem{
								{
									ObjectKey:       exactVersionObj.ObjectKey,
									StreamVersionID: exactVersionStreamVersionID,
								},
								{
									ObjectKey: lastCommittedObj.ObjectKey,
								},
							},
						},
						Result: metabase.DeleteObjectsResult{
							Items: []metabase.DeleteObjectsResultItem{
								{
									ObjectKey: lastCommittedObj.ObjectKey,
									Marker: &metabase.DeleteObjectsInfo{
										StreamVersionID: metabase.NewStreamVersionID(lastCommittedObj.Version+1, uuid.UUID{}),
										Status:          metabase.DeleteMarkerVersioned,
									},
									Status: storj.DeleteObjectsStatusOK,
								},
								{
									ObjectKey:                exactVersionObj.ObjectKey,
									RequestedStreamVersionID: exactVersionStreamVersionID,
									Status:                   storj.DeleteObjectsStatusLocked,
								},
							},
						},
					}.Check(ctx, t, db)

					marker, err := db.GetObjectExactVersion(ctx, metabase.GetObjectExactVersion{
						ObjectLocation: lastCommittedObj.Location(),
						Version:        lastCommittedObj.Version + 1,
					})
					require.NoError(t, err)

					metabasetest.Verify{
						Objects:  metabasetest.ObjectsToRaw(exactVersionObj, lastCommittedObj, marker),
						Segments: metabasetest.SegmentsToRaw(concat(exactVersionSegments, lastCommittedSegments)),
					}.Check(ctx, t, db)
				},
				TestRemovable: func(t *testing.T, testCase metabasetest.ObjectLockDeletionTestCase) {
					defer metabasetest.DeleteAll{}.Check(ctx, t, db)

					exactVersionObj, _ := createLockedObject(t, testCase)
					exactVersionStreamVersionID := exactVersionObj.StreamVersionID()

					lastCommittedObj, lastCommittedSegments := createLockedObject(t, testCase)

					metabasetest.DeleteObjects{
						Opts: metabase.DeleteObjects{
							ProjectID:  projectID,
							BucketName: bucketName,
							Versioned:  true,
							ObjectLock: metabase.ObjectLockDeleteOptions{
								Enabled:          true,
								BypassGovernance: testCase.BypassGovernance,
							},
							Items: []metabase.DeleteObjectsItem{
								{
									ObjectKey:       exactVersionObj.ObjectKey,
									StreamVersionID: exactVersionStreamVersionID,
								},
								{
									ObjectKey: lastCommittedObj.ObjectKey,
								},
							},
						},
						Result: metabase.DeleteObjectsResult{
							Items: []metabase.DeleteObjectsResultItem{
								{
									ObjectKey: lastCommittedObj.ObjectKey,
									Marker: &metabase.DeleteObjectsInfo{
										StreamVersionID: metabase.NewStreamVersionID(lastCommittedObj.Version+1, uuid.UUID{}),
										Status:          metabase.DeleteMarkerVersioned,
									},
									Status: storj.DeleteObjectsStatusOK,
								},
								{
									ObjectKey:                exactVersionObj.ObjectKey,
									RequestedStreamVersionID: exactVersionStreamVersionID,
									Removed: &metabase.DeleteObjectsInfo{
										StreamVersionID: exactVersionStreamVersionID,
										Status:          metabase.CommittedUnversioned,
									},
									Status: storj.DeleteObjectsStatusOK,
								},
							},
							DeletedSegmentCount: int64(exactVersionObj.SegmentCount),
						},
					}.Check(ctx, t, db)

					marker, err := db.GetObjectExactVersion(ctx, metabase.GetObjectExactVersion{
						ObjectLocation: lastCommittedObj.Location(),
						Version:        lastCommittedObj.Version + 1,
					})
					require.NoError(t, err)

					metabasetest.Verify{
						Objects:  metabasetest.ObjectsToRaw(lastCommittedObj, marker),
						Segments: metabasetest.SegmentsToRaw(lastCommittedSegments),
					}.Check(ctx, t, db)
				},
			}.Run(t)
		})

		runSuspendedTests := func(t *testing.T, objectLockEnabled bool) {
			t.Run("Basic", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				exactVersionObj1, _ := createObject(t, randObjectStream(), true)
				exactVersionObj1SVID := exactVersionObj1.StreamVersionID()

				exactVersionObj2, _ := createObject(t, randObjectStream(), true)
				exactVersionObj2SVID := exactVersionObj2.StreamVersionID()

				// Ensure that if an unversioned object version exists at this location,
				// it should be deleted, and a delete marker is inserted as the last version.
				lastCommittedObj1, _ := createObject(t, randObjectStream(), false)

				// Ensure that if a versioned object exists at this location,
				// it isn't deleted, and a delete marker is inserted as the last version.
				lastCommittedObj2, lastCommittedObj2Segs := createObject(t, randObjectStream(), true)

				differentBucketObj, differentBucketSegs := createObject(t, metabase.ObjectStream{
					ProjectID:  exactVersionObj1.ProjectID,
					BucketName: metabase.BucketName(testrand.BucketName()),
					ObjectKey:  exactVersionObj1.ObjectKey,
					Version:    exactVersionObj1.Version,
					StreamID:   testrand.UUID(),
				}, true)

				differentProjectObj, differentProjectSegs := createObject(t, metabase.ObjectStream{
					ProjectID:  testrand.UUID(),
					BucketName: exactVersionObj1.BucketName,
					ObjectKey:  exactVersionObj1.ObjectKey,
					Version:    exactVersionObj1.Version,
					StreamID:   testrand.UUID(),
				}, true)

				metabasetest.DeleteObjects{
					Opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Suspended:  true,
						ObjectLock: metabase.ObjectLockDeleteOptions{
							Enabled: objectLockEnabled,
						},
						Items: []metabase.DeleteObjectsItem{
							{
								ObjectKey:       exactVersionObj1.ObjectKey,
								StreamVersionID: exactVersionObj1SVID,
							},
							{
								ObjectKey:       exactVersionObj2.ObjectKey,
								StreamVersionID: exactVersionObj2SVID,
							},
							{
								ObjectKey: lastCommittedObj1.ObjectKey,
							},
							{
								ObjectKey: lastCommittedObj2.ObjectKey,
							},
						},
					},
					Result: metabase.DeleteObjectsResult{
						Items: []metabase.DeleteObjectsResultItem{
							{
								ObjectKey: lastCommittedObj1.ObjectKey,
								Removed: &metabase.DeleteObjectsInfo{
									StreamVersionID: lastCommittedObj1.StreamVersionID(),
									Status:          metabase.CommittedUnversioned,
								},
								Marker: &metabase.DeleteObjectsInfo{
									StreamVersionID: metabase.NewStreamVersionID(lastCommittedObj1.Version+1, uuid.UUID{}),
									Status:          metabase.DeleteMarkerUnversioned,
								},
								Status: storj.DeleteObjectsStatusOK,
							},
							{
								ObjectKey: lastCommittedObj2.ObjectKey,
								Marker: &metabase.DeleteObjectsInfo{
									StreamVersionID: metabase.NewStreamVersionID(lastCommittedObj2.Version+1, uuid.UUID{}),
									Status:          metabase.DeleteMarkerUnversioned,
								},
								Status: storj.DeleteObjectsStatusOK,
							},
							{
								ObjectKey:                exactVersionObj1.ObjectKey,
								RequestedStreamVersionID: exactVersionObj1SVID,
								Removed: &metabase.DeleteObjectsInfo{
									StreamVersionID: exactVersionObj1SVID,
									Status:          metabase.CommittedVersioned,
								},
								Status: storj.DeleteObjectsStatusOK,
							},
							{
								ObjectKey:                exactVersionObj2.ObjectKey,
								RequestedStreamVersionID: exactVersionObj2SVID,
								Removed: &metabase.DeleteObjectsInfo{
									StreamVersionID: exactVersionObj2SVID,
									Status:          metabase.CommittedVersioned,
								},
								Status: storj.DeleteObjectsStatusOK,
							},
						},
						DeletedSegmentCount: int64(exactVersionObj1.SegmentCount + exactVersionObj2.SegmentCount + lastCommittedObj1.SegmentCount),
					},
				}.Check(ctx, t, db)

				obj3DeleteMarker, err := db.GetObjectExactVersion(ctx, metabase.GetObjectExactVersion{
					ObjectLocation: lastCommittedObj1.Location(),
					Version:        lastCommittedObj1.Version + 1,
				})
				require.NoError(t, err)

				obj4DeleteMarker, err := db.GetObjectExactVersion(ctx, metabase.GetObjectExactVersion{
					ObjectLocation: lastCommittedObj2.Location(),
					Version:        lastCommittedObj2.Version + 1,
				})
				require.NoError(t, err)

				metabasetest.Verify{
					Objects: metabasetest.ObjectsToRaw(
						lastCommittedObj2,
						obj3DeleteMarker,
						obj4DeleteMarker,
						differentBucketObj,
						differentProjectObj,
					),
					Segments: metabasetest.SegmentsToRaw(concat(
						lastCommittedObj2Segs,
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
				obj, segments := createObject(t, randObjectStream(), true)
				badStreamVersionID1 := metabase.NewStreamVersionID(obj.Version, testrand.UUID())
				badStreamVersionID2 := metabase.NewStreamVersionID(randVersion(), obj.StreamID)

				metabasetest.DeleteObjects{
					Opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Suspended:  true,
						ObjectLock: metabase.ObjectLockDeleteOptions{
							Enabled: objectLockEnabled,
						},
						Items: []metabase.DeleteObjectsItem{
							{
								ObjectKey:       missingObjStream1.ObjectKey,
								StreamVersionID: missingStreamVersionID,
							},
							{
								ObjectKey: missingObjStream2.ObjectKey,
							},
							{
								ObjectKey:       obj.ObjectKey,
								StreamVersionID: badStreamVersionID1,
							},
							{
								ObjectKey:       obj.ObjectKey,
								StreamVersionID: badStreamVersionID2,
							},
						},
					},
					Result: metabase.DeleteObjectsResult{
						Items: []metabase.DeleteObjectsResultItem{
							{
								ObjectKey: missingObjStream2.ObjectKey,
								Status:    storj.DeleteObjectsStatusNotFound,
							},
							{
								ObjectKey:                missingObjStream1.ObjectKey,
								RequestedStreamVersionID: missingStreamVersionID,
								Status:                   storj.DeleteObjectsStatusNotFound,
							},
							{
								ObjectKey:                obj.ObjectKey,
								RequestedStreamVersionID: badStreamVersionID1,
								Status:                   storj.DeleteObjectsStatusNotFound,
							},
							{
								ObjectKey:                obj.ObjectKey,
								RequestedStreamVersionID: badStreamVersionID2,
								Status:                   storj.DeleteObjectsStatusNotFound,
							},
						},
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects:  metabasetest.ObjectsToRaw(obj),
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
				sv := pending.StreamVersionID()

				segments := metabasetest.CreateSegments(ctx, t, db, pending.ObjectStream, nil, 2)

				metabasetest.DeleteObjects{
					Opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Suspended:  true,
						ObjectLock: metabase.ObjectLockDeleteOptions{
							Enabled: objectLockEnabled,
						},
						Items: []metabase.DeleteObjectsItem{{
							ObjectKey: pending.ObjectKey,
						}},
					},
					Result: metabase.DeleteObjectsResult{
						Items: []metabase.DeleteObjectsResultItem{{
							ObjectKey: pending.ObjectKey,
							Status:    storj.DeleteObjectsStatusNotFound,
						}},
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{
					Objects:  metabasetest.ObjectsToRaw(pending),
					Segments: metabasetest.SegmentsToRaw(segments),
				}.Check(ctx, t, db)

				metabasetest.DeleteObjects{
					Opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Suspended:  true,
						ObjectLock: metabase.ObjectLockDeleteOptions{
							Enabled: objectLockEnabled,
						},
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
							Status: storj.DeleteObjectsStatusOK,
						}},
						DeletedSegmentCount: int64(len(segments)),
					},
				}.Check(ctx, t, db)

				metabasetest.Verify{}.Check(ctx, t, db)
			})

			t.Run("Duplicate deletion", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				obj1, _ := createObject(t, randObjectStream(), true)
				obj1StreamVersionID := obj1.StreamVersionID()
				reqItem1 := metabase.DeleteObjectsItem{
					ObjectKey:       obj1.ObjectKey,
					StreamVersionID: obj1StreamVersionID,
				}

				obj2, obj2Segments := createObject(t, randObjectStream(), true)
				reqItem2 := metabase.DeleteObjectsItem{
					ObjectKey: obj2.ObjectKey,
				}

				metabasetest.DeleteObjects{
					Opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Suspended:  true,
						ObjectLock: metabase.ObjectLockDeleteOptions{
							Enabled: objectLockEnabled,
						},
						Items: []metabase.DeleteObjectsItem{reqItem1, reqItem1, reqItem2, reqItem2},
					},
					Result: metabase.DeleteObjectsResult{
						Items: []metabase.DeleteObjectsResultItem{
							{
								ObjectKey: obj2.ObjectKey,
								Marker: &metabase.DeleteObjectsInfo{
									StreamVersionID: metabase.NewStreamVersionID(obj2.Version+1, uuid.UUID{}),
									Status:          metabase.DeleteMarkerUnversioned,
								},
								Status: storj.DeleteObjectsStatusOK,
							},
							{
								ObjectKey:                obj1.ObjectKey,
								RequestedStreamVersionID: obj1StreamVersionID,
								Removed: &metabase.DeleteObjectsInfo{
									StreamVersionID: obj1StreamVersionID,
									Status:          metabase.CommittedVersioned,
								},
								Status: storj.DeleteObjectsStatusOK,
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

				obj, _ := createObject(t, randObjectStream(), true)
				sv := obj.StreamVersionID()

				opts := metabasetest.DeleteObjects{
					Opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Suspended:  true,
						ObjectLock: metabase.ObjectLockDeleteOptions{
							Enabled: objectLockEnabled,
						},
						Items: []metabase.DeleteObjectsItem{
							{
								ObjectKey:       obj.ObjectKey,
								StreamVersionID: sv,
							},
							{
								ObjectKey: obj.ObjectKey,
							},
						},
					},
					Result: metabase.DeleteObjectsResult{
						Items: []metabase.DeleteObjectsResultItem{
							{
								ObjectKey: obj.ObjectKey,
								Marker: &metabase.DeleteObjectsInfo{
									StreamVersionID: metabase.NewStreamVersionID(obj.Version+1, uuid.UUID{}),
									Status:          metabase.DeleteMarkerUnversioned,
								},
								Status: storj.DeleteObjectsStatusOK,
							},
							{
								ObjectKey:                obj.ObjectKey,
								RequestedStreamVersionID: sv,
								Removed: &metabase.DeleteObjectsInfo{
									StreamVersionID: sv,
									Status:          metabase.CommittedVersioned,
								},
								Status: storj.DeleteObjectsStatusOK,
							},
						},
						DeletedSegmentCount: int64(obj.SegmentCount),
					},
				}
				opts.Check(ctx, t, db)

				deleteMarker, err := db.GetObjectExactVersion(ctx, metabase.GetObjectExactVersion{
					ObjectLocation: obj.Location(),
					Version:        obj.Version + 1,
				})
				require.NoError(t, err)

				metabasetest.Verify{
					Objects: metabasetest.ObjectsToRaw(deleteMarker),
				}.Check(ctx, t, db)

				metabasetest.DeleteAll{}.Check(ctx, t, db)

				createObject(t, obj.ObjectStream, true)
				opts.Opts.Items[0], opts.Opts.Items[1] = opts.Opts.Items[1], opts.Opts.Items[0]
				opts.Check(ctx, t, db)

				deleteMarker, err = db.GetObjectExactVersion(ctx, metabase.GetObjectExactVersion{
					ObjectLocation: obj.Location(),
					Version:        obj.Version + 1,
				})
				require.NoError(t, err)

				metabasetest.Verify{
					Objects: metabasetest.ObjectsToRaw(deleteMarker),
				}.Check(ctx, t, db)
			})
		}

		t.Run("Suspended", func(t *testing.T) {
			runSuspendedTests(t, false)
		})

		t.Run("Suspended - Object Lock", func(t *testing.T) {
			runSuspendedTests(t, false)

			metabasetest.ObjectLockDeletionTestRunner{
				TestProtected: func(t *testing.T, testCase metabasetest.ObjectLockDeletionTestCase) {
					defer metabasetest.DeleteAll{}.Check(ctx, t, db)

					exactVersionObj, exactVersionSegments := createLockedObject(t, testCase)
					exactVersionStreamVersionID := exactVersionObj.StreamVersionID()

					lastCommittedObj, lastCommittedSegments := createLockedObject(t, testCase)

					metabasetest.DeleteObjects{
						Opts: metabase.DeleteObjects{
							ProjectID:  projectID,
							BucketName: bucketName,
							Suspended:  true,
							ObjectLock: metabase.ObjectLockDeleteOptions{
								Enabled:          true,
								BypassGovernance: testCase.BypassGovernance,
							},
							Items: []metabase.DeleteObjectsItem{
								{
									ObjectKey:       exactVersionObj.ObjectKey,
									StreamVersionID: exactVersionStreamVersionID,
								},
								{
									ObjectKey: lastCommittedObj.ObjectKey,
								},
							},
						},
						Result: metabase.DeleteObjectsResult{
							Items: []metabase.DeleteObjectsResultItem{
								{
									ObjectKey: lastCommittedObj.ObjectKey,
									Status:    storj.DeleteObjectsStatusLocked,
								},
								{
									ObjectKey:                exactVersionObj.ObjectKey,
									RequestedStreamVersionID: exactVersionStreamVersionID,
									Status:                   storj.DeleteObjectsStatusLocked,
								},
							},
						},
					}.Check(ctx, t, db)

					metabasetest.Verify{
						Objects:  metabasetest.ObjectsToRaw(exactVersionObj, lastCommittedObj),
						Segments: metabasetest.SegmentsToRaw(concat(exactVersionSegments, lastCommittedSegments)),
					}.Check(ctx, t, db)
				},
				TestRemovable: func(t *testing.T, testCase metabasetest.ObjectLockDeletionTestCase) {
					defer metabasetest.DeleteAll{}.Check(ctx, t, db)

					exactVersionObj, _ := createLockedObject(t, testCase)
					exactVersionStreamVersionID := exactVersionObj.StreamVersionID()

					lastCommittedObj, _ := createLockedObject(t, testCase)

					metabasetest.DeleteObjects{
						Opts: metabase.DeleteObjects{
							ProjectID:  projectID,
							BucketName: bucketName,
							Suspended:  true,
							ObjectLock: metabase.ObjectLockDeleteOptions{
								Enabled:          true,
								BypassGovernance: testCase.BypassGovernance,
							},
							Items: []metabase.DeleteObjectsItem{
								{
									ObjectKey:       exactVersionObj.ObjectKey,
									StreamVersionID: exactVersionStreamVersionID,
								},
								{
									ObjectKey: lastCommittedObj.ObjectKey,
								},
							},
						},
						Result: metabase.DeleteObjectsResult{
							Items: []metabase.DeleteObjectsResultItem{
								{
									ObjectKey: lastCommittedObj.ObjectKey,
									Removed: &metabase.DeleteObjectsInfo{
										StreamVersionID: lastCommittedObj.StreamVersionID(),
										Status:          metabase.CommittedUnversioned,
									},
									Marker: &metabase.DeleteObjectsInfo{
										StreamVersionID: metabase.NewStreamVersionID(lastCommittedObj.Version+1, uuid.UUID{}),
										Status:          metabase.DeleteMarkerUnversioned,
									},
									Status: storj.DeleteObjectsStatusOK,
								},
								{
									ObjectKey:                exactVersionObj.ObjectKey,
									RequestedStreamVersionID: exactVersionStreamVersionID,
									Removed: &metabase.DeleteObjectsInfo{
										StreamVersionID: exactVersionStreamVersionID,
										Status:          metabase.CommittedUnversioned,
									},
									Status: storj.DeleteObjectsStatusOK,
								},
							},
							DeletedSegmentCount: int64(exactVersionObj.SegmentCount + lastCommittedObj.SegmentCount),
						},
					}.Check(ctx, t, db)

					deleteMarker, err := db.GetObjectExactVersion(ctx, metabase.GetObjectExactVersion{
						ObjectLocation: lastCommittedObj.Location(),
						Version:        lastCommittedObj.Version + 1,
					})
					require.NoError(t, err)

					metabasetest.Verify{
						Objects: metabasetest.ObjectsToRaw(deleteMarker),
					}.Check(ctx, t, db)
				},
			}.Run(t)
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
				},
				{
					name: "Bucket name missing",
					opts: metabase.DeleteObjects{
						ProjectID: projectID,
						Items:     []metabase.DeleteObjectsItem{validItem},
					},
					errMsg: "BucketName missing",
				},
				{
					name: "Items missing",
					opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
					},
					errMsg: "Items missing",
				},
				{
					name: "Too many items",
					opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Items:      make([]metabase.DeleteObjectsItem, 1001),
					},
					errMsg: "Items is too long; expected <= 1000, but got 1001",
				},
				{
					name: "Missing object key",
					opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Items: []metabase.DeleteObjectsItem{{
							StreamVersionID: validItem.StreamVersionID,
						}},
					},
					errMsg: "Items[0].ObjectKey missing",
				},
				{
					name: "Invalid version",
					opts: metabase.DeleteObjects{
						ProjectID:  projectID,
						BucketName: bucketName,
						Items: []metabase.DeleteObjectsItem{{
							ObjectKey:       validItem.ObjectKey,
							StreamVersionID: metabase.NewStreamVersionID(0, testrand.UUID()),
						}},
					},
					errMsg: "Items[0].StreamVersionID invalid: version is 0",
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
