// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestListAllBuckets(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		// no buckets
		list, err := db.Buckets().ListAllBuckets(ctx, metainfo.ListAllBucketsOptions{})
		require.NoError(t, err)
		require.Equal(t, 0, len(list.Items))

		first, err := db.Console().Projects().Insert(ctx, &console.Project{
			Name: "first",
			ID:   testrand.UUID(),
		})
		require.NoError(t, err)

		second, err := db.Console().Projects().Insert(ctx, &console.Project{
			Name: "second",
			ID:   testrand.UUID(),
		})
		require.NoError(t, err)

		projects := []*console.Project{first, second}
		if bytes.Compare(first.ID[:], second.ID[:]) > 0 {
			projects = []*console.Project{second, first}
		}

		buckets := make([]storj.Bucket, 10)
		for i, project := range projects {
			for index := 0; index < (len(buckets) / 2); index++ {
				var err error
				buckets[index+(i*5)], err = db.Buckets().CreateBucket(ctx, storj.Bucket{
					ID:        testrand.UUID(),
					Name:      "bucket-test-" + strconv.Itoa(index),
					ProjectID: project.ID,
				})
				require.NoError(t, err)
			}
		}

		list, err = db.Buckets().ListAllBuckets(ctx, metainfo.ListAllBucketsOptions{})
		require.NoError(t, err)
		require.Equal(t, len(buckets), len(list.Items))
		require.False(t, list.More)
		require.Zero(t, cmp.Diff(buckets, list.Items))

		list, err = db.Buckets().ListAllBuckets(ctx, metainfo.ListAllBucketsOptions{
			Cursor: metainfo.ListAllBucketsCursor{
				ProjectID: projects[1].ID,
			},
		})
		require.NoError(t, err)
		require.Equal(t, len(buckets)/2, len(list.Items))
		require.Zero(t, cmp.Diff(buckets[len(buckets)/2:], list.Items))

		list, err = db.Buckets().ListAllBuckets(ctx, metainfo.ListAllBucketsOptions{
			Cursor: metainfo.ListAllBucketsCursor{
				ProjectID:  projects[1].ID,
				BucketName: []byte("bucket-test-2"),
			},
		})
		require.NoError(t, err)
		require.Equal(t, 2, len(list.Items))
		require.False(t, list.More)
		require.Zero(t, cmp.Diff(buckets[8:], list.Items))

		list, err = db.Buckets().ListAllBuckets(ctx, metainfo.ListAllBucketsOptions{
			Cursor: metainfo.ListAllBucketsCursor{
				ProjectID:  projects[1].ID,
				BucketName: []byte("bucket-test-4"),
			},
		})
		require.NoError(t, err)
		require.Equal(t, 0, len(list.Items))

		list, err = db.Buckets().ListAllBuckets(ctx, metainfo.ListAllBucketsOptions{
			Limit: 2,
		})
		require.NoError(t, err)
		require.Equal(t, 2, len(list.Items))
		require.True(t, list.More)
		require.Zero(t, cmp.Diff(buckets[:2], list.Items))
	})
}
