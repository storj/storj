// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"
	"time"

	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metainfo/metabase"
)

func TestListBucket(t *testing.T) {
	All(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		// obj := randObjectStream()

		// location := obj.Location()

		// now := time.Now()

		// for _, test := range invalidObjectLocations(location) {
		// 	test := test
		// 	t.Run(test.Name, func(t *testing.T) {
		// 		defer DeleteAll{}.Check(ctx, t, db)
		// 		GetObjectExactVersion{
		// 			Opts: metabase.GetObjectExactVersion{
		// 				ObjectLocation: test.ObjectLocation,
		// 			},
		// 			ErrClass: test.ErrClass,
		// 			ErrText:  test.ErrText,
		// 		}.Check(ctx, t, db)

		// 		Verify{}.Check(ctx, t, db)
		// 	})
		// }

		t.Run("BucketName missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			ListBucket{
				Opts: metabase.ListBucket{
					ProjectID:  uuid.UUID{1},
					BucketName: "",
					Recursive:  true,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "BucketName missing",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		t.Run("ProjectID missing", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			ListBucket{
				Opts: metabase.ListBucket{
					ProjectID:  uuid.UUID{},
					BucketName: "sj://mybucket",
					Recursive:  true,
				},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "ProjectID missing",
			}.Check(ctx, t, db)

			Verify{}.Check(ctx, t, db)
		})

		// TODO: for now we cannot distinguish between empty bucket and non-existing bucket
		// t.Run("List empty bucket", func(t *testing.T) {
		// 	defer DeleteAll{}.Check(ctx, t, db)

		// 	Verify{}.Check(ctx, t, db)
		// })

		t.Run("List recursively", func(t *testing.T) {
			defer DeleteAll{}.Check(ctx, t, db)

			obj := randObjectStream()
			now := time.Now()
			createObject(ctx, t, db, obj, 0)
			expected := []metabase.Object{{
				ObjectStream: obj,
				CreatedAt:    now,
				Status:       metabase.Committed,
				Encryption:   defaultTestEncryption,
			}}

			ListBucket{
				Opts: metabase.ListBucket{
					ProjectID:  obj.ProjectID,
					BucketName: obj.BucketName,
					Recursive:  true,
				},
				Result: expected,
			}.Check(ctx, t, db)

			Verify{Objects: []metabase.RawObject{
				{
					ObjectStream: obj,
					CreatedAt:    now,
					Status:       metabase.Committed,

					Encryption: defaultTestEncryption,
				},
			}}.Check(ctx, t, db)
		})
	})
}
