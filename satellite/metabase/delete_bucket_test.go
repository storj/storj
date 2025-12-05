// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestDeleteAllBucketObjects(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj1 := metabasetest.RandObjectStream()
		obj2 := metabasetest.RandObjectStream()
		obj3 := metabasetest.RandObjectStream()
		obj2.ProjectID, obj2.BucketName = obj1.ProjectID, obj1.BucketName
		obj3.ProjectID, obj3.BucketName = obj1.ProjectID, obj1.BucketName

		t.Run("invalid options", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.DeleteAllBucketObjects{
				Opts: metabase.DeleteAllBucketObjects{
					Bucket: metabase.BucketLocation{
						ProjectID:  uuid.UUID{},
						BucketName: "",
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "ProjectID missing",
			}.Check(ctx, t, db)

			metabasetest.DeleteAllBucketObjects{
				Opts: metabase.DeleteAllBucketObjects{
					Bucket: metabase.BucketLocation{
						ProjectID:  uuid.UUID{1},
						BucketName: "",
					},
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "BucketName missing",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("empty bucket", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.DeleteAllBucketObjects{
				Opts: metabase.DeleteAllBucketObjects{
					Bucket: obj1.Location().Bucket(),
				},
				Deleted: 0,
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("one object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.CreateObject(ctx, t, db, obj1, 2)

			metabasetest.DeleteAllBucketObjects{
				Opts: metabase.DeleteAllBucketObjects{
					Bucket: obj1.Location().Bucket(),
				},
				Deleted: 1,
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("empty object", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.CreateObject(ctx, t, db, obj1, 0)

			metabasetest.DeleteAllBucketObjects{
				Opts: metabase.DeleteAllBucketObjects{
					Bucket: obj1.Location().Bucket(),
				},
				Deleted: 1,
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("three objects", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.CreateObject(ctx, t, db, obj1, 2)
			metabasetest.CreateObject(ctx, t, db, obj2, 2)
			metabasetest.CreateObject(ctx, t, db, obj3, 2)

			metabasetest.DeleteAllBucketObjects{
				Opts: metabase.DeleteAllBucketObjects{
					Bucket:    obj1.Location().Bucket(),
					BatchSize: 2,
				},
				Deleted: 3,
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("don't delete non-exact match", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			objDifferentBucket := metabasetest.RandObjectStream()
			objDifferentBucket.ProjectID = obj1.ProjectID

			objDifferentProject := metabasetest.RandObjectStream()
			objDifferentProject.BucketName = obj1.BucketName

			metabasetest.CreateObject(ctx, t, db, objDifferentBucket, 1)
			metabasetest.CreateObject(ctx, t, db, objDifferentProject, 1)

			snapshot := metabasetest.Snapshot(ctx, t, db)

			metabasetest.CreateObject(ctx, t, db, obj1, 1)

			metabasetest.DeleteAllBucketObjects{
				Opts: metabase.DeleteAllBucketObjects{
					Bucket: obj1.Location().Bucket(),
				},
				Deleted: 1,
			}.Check(ctx, t, db)

			snapshot.Check(ctx, t, db)
		})

		t.Run("object with multiple segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.CreateObject(ctx, t, db, obj1, 37)

			metabasetest.DeleteAllBucketObjects{
				Opts: metabase.DeleteAllBucketObjects{
					Bucket:    obj1.Location().Bucket(),
					BatchSize: 2,
				},
				Deleted: 1,
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("multiple objects", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			root := metabasetest.RandObjectStream()
			for i := 0; i < 5; i++ {
				obj := metabasetest.RandObjectStream()
				obj.ProjectID = root.ProjectID
				obj.BucketName = root.BucketName
				metabasetest.CreateObject(ctx, t, db, obj, 5)
			}

			metabasetest.DeleteAllBucketObjects{
				Opts: metabase.DeleteAllBucketObjects{
					Bucket:    root.Location().Bucket(),
					BatchSize: 1,
				},
				Deleted: 5,
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})
	})
}

func TestDeleteAllBucketObjectsParallel(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		defer metabasetest.DeleteAll{}.Check(ctx, t, db)

		root := metabasetest.RandObjectStream()
		for i := 0; i < 5; i++ {
			obj := metabasetest.RandObjectStream()
			obj.ProjectID = root.ProjectID
			obj.BucketName = root.BucketName
			metabasetest.CreateObject(ctx, t, db, obj, 50)
		}

		objects, err := db.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Equal(t, 5, len(objects))

		var errgroup errgroup.Group
		for i := 0; i < 3; i++ {
			errgroup.Go(func() error {
				_, err := db.DeleteAllBucketObjects(ctx, metabase.DeleteAllBucketObjects{
					Bucket:    root.Location().Bucket(),
					BatchSize: 2,
				})
				return err
			})
		}
		require.NoError(t, errgroup.Wait())

		metabasetest.Verify{}.Check(ctx, t, db)
	})
}

func TestDeleteAllBucketObjectsCancel(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		defer metabasetest.DeleteAll{}.Check(ctx, t, db)

		object := metabasetest.CreateObject(ctx, t, db, metabasetest.RandObjectStream(), 1)

		testCtx, cancel := context.WithCancel(ctx)
		cancel()
		_, err := db.DeleteAllBucketObjects(testCtx, metabase.DeleteAllBucketObjects{
			Bucket:    object.Location().Bucket(),
			BatchSize: 2,
		})
		require.Error(t, err)

		metabasetest.Verify{
			Objects: []metabase.RawObject{metabase.RawObject(object)},
			Segments: []metabase.RawSegment{
				metabasetest.DefaultRawSegment(object.ObjectStream, metabase.SegmentPosition{}),
			},
		}.Check(ctx, t, db)
	})
}

func TestDeleteBucketWithCopies(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		for _, numberOfSegments := range []int{0, 1, 3} {
			t.Run(fmt.Sprintf("%d segments", numberOfSegments), func(t *testing.T) {
				t.Run("delete bucket with copy", func(t *testing.T) {
					defer metabasetest.DeleteAll{}.Check(ctx, t, db)
					originalObjStream := metabasetest.RandObjectStream()
					originalObjStream.BucketName = "original-bucket"

					originalObj, originalSegments := metabasetest.CreateTestObject{
						CommitObject: &metabase.CommitObject{
							ObjectStream:      originalObjStream,
							EncryptedUserData: metabasetest.RandEncryptedUserDataWithoutETag(),
						},
					}.Run(ctx, t, db, originalObjStream, byte(numberOfSegments))

					copyObjectStream := metabasetest.RandObjectStream()
					copyObjectStream.ProjectID = originalObjStream.ProjectID
					copyObjectStream.BucketName = "copy-bucket"

					metabasetest.CreateObjectCopy{
						OriginalObject:   originalObj,
						CopyObjectStream: &copyObjectStream,
					}.Run(ctx, t, db)

					_, err := db.DeleteAllBucketObjects(ctx, metabase.DeleteAllBucketObjects{
						Bucket: metabase.BucketLocation{
							ProjectID:  originalObjStream.ProjectID,
							BucketName: "copy-bucket",
						},
						BatchSize: 2,
					})
					require.NoError(t, err)

					// Verify that we are back at the original single object
					metabasetest.Verify{
						Objects: []metabase.RawObject{
							metabase.RawObject(originalObj),
						},
						Segments: metabasetest.SegmentsToRaw(originalSegments),
					}.Check(ctx, t, db)
				})

				t.Run("delete bucket with ancestor", func(t *testing.T) {
					defer metabasetest.DeleteAll{}.Check(ctx, t, db)
					originalObjStream := metabasetest.RandObjectStream()
					originalObjStream.BucketName = "original-bucket"

					originalObj, originalSegments := metabasetest.CreateTestObject{
						CommitObject: &metabase.CommitObject{
							ObjectStream:      originalObjStream,
							EncryptedUserData: metabasetest.RandEncryptedUserDataWithoutETag(),
						},
					}.Run(ctx, t, db, originalObjStream, byte(numberOfSegments))

					copyObjectStream := metabasetest.RandObjectStream()
					copyObjectStream.ProjectID = originalObjStream.ProjectID
					copyObjectStream.BucketName = "copy-bucket"

					copyObj, _, copySegments := metabasetest.CreateObjectCopy{
						OriginalObject:   originalObj,
						CopyObjectStream: &copyObjectStream,
					}.Run(ctx, t, db)

					_, err := db.DeleteAllBucketObjects(ctx, metabase.DeleteAllBucketObjects{
						Bucket: metabase.BucketLocation{
							ProjectID:  originalObjStream.ProjectID,
							BucketName: "original-bucket",
						},
						BatchSize: 2,
					})
					require.NoError(t, err)

					for i := range copySegments {
						copySegments[i].Pieces = originalSegments[i].Pieces
					}

					// Verify that we are back at the original single object
					metabasetest.Verify{
						Objects: []metabase.RawObject{
							metabase.RawObject(copyObj),
						},
						Segments: copySegments,
					}.Check(ctx, t, db)
				})

				t.Run("delete bucket which has one ancestor and one copy", func(t *testing.T) {
					defer metabasetest.DeleteAll{}.Check(ctx, t, db)
					originalObjStream1 := metabasetest.RandObjectStream()
					originalObjStream1.BucketName = "bucket1"

					projectID := originalObjStream1.ProjectID

					originalObjStream2 := metabasetest.RandObjectStream()
					originalObjStream2.ProjectID = projectID
					originalObjStream2.BucketName = "bucket2"

					originalObj1, originalSegments1 := metabasetest.CreateTestObject{
						CommitObject: &metabase.CommitObject{
							ObjectStream: originalObjStream1,
						},
					}.Run(ctx, t, db, originalObjStream1, byte(numberOfSegments))

					originalObj2, originalSegments2 := metabasetest.CreateTestObject{
						CommitObject: &metabase.CommitObject{
							ObjectStream: originalObjStream2,
						},
					}.Run(ctx, t, db, originalObjStream2, byte(numberOfSegments))

					copyObjectStream1 := metabasetest.RandObjectStream()
					copyObjectStream1.ProjectID = projectID
					copyObjectStream1.BucketName = "bucket2" // copy from bucket 1 to bucket 2

					copyObjectStream2 := metabasetest.RandObjectStream()
					copyObjectStream2.ProjectID = projectID
					copyObjectStream2.BucketName = "bucket1" // copy from bucket 2 to bucket 1

					metabasetest.CreateObjectCopy{
						OriginalObject:   originalObj1,
						CopyObjectStream: &copyObjectStream1,
					}.Run(ctx, t, db)

					copyObj2, _, copySegments2 := metabasetest.CreateObjectCopy{
						OriginalObject:   originalObj2,
						CopyObjectStream: &copyObjectStream2,
					}.Run(ctx, t, db)

					// done preparing, delete bucket 2
					_, err := db.DeleteAllBucketObjects(ctx, metabase.DeleteAllBucketObjects{
						Bucket: metabase.BucketLocation{
							ProjectID:  projectID,
							BucketName: "bucket2",
						},
						BatchSize: 2,
					})
					require.NoError(t, err)

					// Prepare for check.
					// obj1 is the same as before, copyObj2 should now be the original
					for i := range copySegments2 {
						copySegments2[i].Pieces = originalSegments2[i].Pieces
					}

					metabasetest.Verify{
						Objects: []metabase.RawObject{
							metabase.RawObject(originalObj1),
							metabase.RawObject(copyObj2),
						},
						Segments: append(copySegments2, metabasetest.SegmentsToRaw(originalSegments1)...),
					}.Check(ctx, t, db)
				})

				// TODO: check that DeletePieces callback is called with the correct arguments

				// scenario: delete original bucket with 2 copies

				// scenario: delete copy bucket with 2 copies

				// scenario: delete bucket with 2 internal copies
			})
		}
	})
}
