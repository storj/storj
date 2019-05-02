// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestBasicBucket(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, masterDB satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		project := &console.Project{
			Name: "TestProject",
		}
		project, err := masterDB.Console().Projects().Insert(ctx, project)
		require.NoError(t, err)

		bucketsDB := masterDB.MetainfoBuckets()

		bucketID, err := uuid.New()
		require.NoError(t, err)

		// DB is not keeping nanoseconds
		createdAt := time.Now().UTC()
		createdAt = time.Date(createdAt.Year(), createdAt.Month(), createdAt.Day(), createdAt.Hour(), createdAt.Minute(), createdAt.Second(), 0, createdAt.Location())

		// TODO test AttributionID
		expectedBucket := &metainfo.Bucket{
			ID:                 *bucketID,
			Name:               "test-bucket",
			ProjectID:          project.ID,
			CreatedAt:          createdAt,
			DefaultSegmentSize: 256,
			DefaultRedundancy: storj.RedundancyScheme{
				Algorithm:      storj.ReedSolomon,
				ShareSize:      9,
				RequiredShares: 10,
				RepairShares:   11,
				OptimalShares:  12,
				TotalShares:    13,
			},
			DefaultEncryption: storj.EncryptionParameters{
				CipherSuite: storj.EncAESGCM,
				BlockSize:   32,
			},
		}
		err = bucketsDB.Create(ctx, expectedBucket)
		require.NoError(t, err)

		bucket, err := bucketsDB.Get(ctx, project.ID, "test-bucket")
		require.NoError(t, err)
		require.Equal(t, expectedBucket, bucket)

		err = bucketsDB.Delete(ctx, project.ID, "test-bucket")
		require.NoError(t, err)
	})
}

func TestListBucketsEmpty(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, masterDB satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		project := &console.Project{
			Name: "TestProject",
		}
		project, err := masterDB.Console().Projects().Insert(ctx, project)
		require.NoError(t, err)

		bucketsDB := masterDB.MetainfoBuckets()
		_, err = bucketsDB.List(ctx, project.ID, metainfo.BucketListOptions{})
		assert.EqualError(t, err, "unknown list direction")

		for _, direction := range []storj.ListDirection{
			storj.Before,
			storj.Backward,
			storj.Forward,
			storj.After,
		} {
			bucketList, err := bucketsDB.List(ctx, project.ID, metainfo.BucketListOptions{Direction: direction})
			if assert.NoError(t, err) {
				assert.False(t, bucketList.More)
				assert.Equal(t, 0, len(bucketList.Items))
			}
		}
	})
}

func TestNotExistingBucket(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, masterDB satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		project := &console.Project{
			Name: "TestProject",
		}
		project, err := masterDB.Console().Projects().Insert(ctx, project)
		require.NoError(t, err)

		bucketsDB := masterDB.MetainfoBuckets()
		_, err = bucketsDB.Get(ctx, project.ID, "")
		assert.True(t, storj.ErrNoBucket.Has(err))

		err = bucketsDB.Delete(ctx, project.ID, "")
		assert.True(t, storj.ErrNoBucket.Has(err))

		_, err = bucketsDB.Get(ctx, project.ID, "not-existing-bucket")
		assert.True(t, storj.ErrBucketNotFound.Has(err))
	})
}

func TestListBuckets(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, masterDB satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		project := &console.Project{
			Name: "TestProject",
		}
		project, err := masterDB.Console().Projects().Insert(ctx, project)
		require.NoError(t, err)

		bucketNames := []string{"a", "aa", "b", "bb", "c"}

		bucketsDB := masterDB.MetainfoBuckets()
		for _, name := range bucketNames {
			id, err := uuid.New()
			require.NoError(t, err)
			bucket := &metainfo.Bucket{ID: *id, Name: name, ProjectID: project.ID}
			err = bucketsDB.Create(ctx, bucket)
			require.NoError(t, err)
		}

		for i, tt := range []struct {
			cursor string
			dir    storj.ListDirection
			limit  int
			more   bool
			result []string
		}{
			// after
			{cursor: "", dir: storj.After, limit: 0, more: false, result: []string{"a", "aa", "b", "bb", "c"}},
			{cursor: "`", dir: storj.After, limit: 0, more: false, result: []string{"a", "aa", "b", "bb", "c"}},
			{cursor: "b", dir: storj.After, limit: 0, more: false, result: []string{"bb", "c"}},
			{cursor: "c", dir: storj.After, limit: 0, more: false, result: []string{}},
			{cursor: "ca", dir: storj.After, limit: 0, more: false, result: []string{}},
			{cursor: "", dir: storj.After, limit: 1, more: true, result: []string{"a"}},
			{cursor: "`", dir: storj.After, limit: 1, more: true, result: []string{"a"}},
			{cursor: "aa", dir: storj.After, limit: 1, more: true, result: []string{"b"}},
			{cursor: "c", dir: storj.After, limit: 1, more: false, result: []string{}},
			{cursor: "ca", dir: storj.After, limit: 1, more: false, result: []string{}},
			{cursor: "", dir: storj.After, limit: 2, more: true, result: []string{"a", "aa"}},
			{cursor: "`", dir: storj.After, limit: 2, more: true, result: []string{"a", "aa"}},
			{cursor: "aa", dir: storj.After, limit: 2, more: true, result: []string{"b", "bb"}},
			{cursor: "bb", dir: storj.After, limit: 2, more: false, result: []string{"c"}},
			{cursor: "c", dir: storj.After, limit: 2, more: false, result: []string{}},
			{cursor: "ca", dir: storj.After, limit: 2, more: false, result: []string{}},
			// forward
			{cursor: "", dir: storj.Forward, limit: 0, more: false, result: []string{"a", "aa", "b", "bb", "c"}},
			{cursor: "`", dir: storj.Forward, limit: 0, more: false, result: []string{"a", "aa", "b", "bb", "c"}},
			{cursor: "b", dir: storj.Forward, limit: 0, more: false, result: []string{"b", "bb", "c"}},
			{cursor: "c", dir: storj.Forward, limit: 0, more: false, result: []string{"c"}},
			{cursor: "ca", dir: storj.Forward, limit: 0, more: false, result: []string{}},
			{cursor: "", dir: storj.Forward, limit: 1, more: true, result: []string{"a"}},
			{cursor: "`", dir: storj.Forward, limit: 1, more: true, result: []string{"a"}},
			{cursor: "aa", dir: storj.Forward, limit: 1, more: true, result: []string{"aa"}},
			{cursor: "c", dir: storj.Forward, limit: 1, more: false, result: []string{"c"}},
			{cursor: "ca", dir: storj.Forward, limit: 1, more: false, result: []string{}},
			{cursor: "", dir: storj.Forward, limit: 2, more: true, result: []string{"a", "aa"}},
			{cursor: "`", dir: storj.Forward, limit: 2, more: true, result: []string{"a", "aa"}},
			{cursor: "aa", dir: storj.Forward, limit: 2, more: true, result: []string{"aa", "b"}},
			{cursor: "bb", dir: storj.Forward, limit: 2, more: false, result: []string{"bb", "c"}},
			{cursor: "c", dir: storj.Forward, limit: 2, more: false, result: []string{"c"}},
			{cursor: "ca", dir: storj.Forward, limit: 2, more: false, result: []string{}},
			{cursor: "", dir: storj.Backward, limit: 0, more: false, result: []string{"a", "aa", "b", "bb", "c"}},
			{cursor: "`", dir: storj.Backward, limit: 0, more: false, result: []string{}},
			{cursor: "b", dir: storj.Backward, limit: 0, more: false, result: []string{"a", "aa", "b"}},
			{cursor: "c", dir: storj.Backward, limit: 0, more: false, result: []string{"a", "aa", "b", "bb", "c"}},
			{cursor: "ca", dir: storj.Backward, limit: 0, more: false, result: []string{"a", "aa", "b", "bb", "c"}},
			// backward
			{cursor: "", dir: storj.Backward, limit: 1, more: true, result: []string{"c"}},
			{cursor: "`", dir: storj.Backward, limit: 1, more: false, result: []string{}},
			{cursor: "aa", dir: storj.Backward, limit: 1, more: true, result: []string{"aa"}},
			{cursor: "c", dir: storj.Backward, limit: 1, more: true, result: []string{"c"}},
			{cursor: "ca", dir: storj.Backward, limit: 1, more: true, result: []string{"c"}},
			{cursor: "", dir: storj.Backward, limit: 2, more: true, result: []string{"bb", "c"}},
			{cursor: "`", dir: storj.Backward, limit: 2, more: false, result: []string{}},
			{cursor: "aa", dir: storj.Backward, limit: 2, more: false, result: []string{"a", "aa"}},
			{cursor: "bb", dir: storj.Backward, limit: 2, more: true, result: []string{"b", "bb"}},
			{cursor: "c", dir: storj.Backward, limit: 2, more: true, result: []string{"bb", "c"}},
			{cursor: "ca", dir: storj.Backward, limit: 2, more: true, result: []string{"bb", "c"}},
			// before
			{cursor: "", dir: storj.Before, limit: 0, more: false, result: []string{"a", "aa", "b", "bb", "c"}},
			{cursor: "`", dir: storj.Before, limit: 0, more: false, result: []string{}},
			{cursor: "b", dir: storj.Before, limit: 0, more: false, result: []string{"a", "aa"}},
			{cursor: "c", dir: storj.Before, limit: 0, more: false, result: []string{"a", "aa", "b", "bb"}},
			{cursor: "ca", dir: storj.Before, limit: 0, more: false, result: []string{"a", "aa", "b", "bb", "c"}},
			{cursor: "", dir: storj.Before, limit: 1, more: true, result: []string{"c"}},
			{cursor: "`", dir: storj.Before, limit: 1, more: false, result: []string{}},
			{cursor: "aa", dir: storj.Before, limit: 1, more: false, result: []string{"a"}},
			{cursor: "c", dir: storj.Before, limit: 1, more: true, result: []string{"bb"}},
			{cursor: "ca", dir: storj.Before, limit: 1, more: true, result: []string{"c"}},
			{cursor: "", dir: storj.Before, limit: 2, more: true, result: []string{"bb", "c"}},
			{cursor: "`", dir: storj.Before, limit: 2, more: false, result: []string{}},
			{cursor: "aa", dir: storj.Before, limit: 2, more: false, result: []string{"a"}},
			{cursor: "bb", dir: storj.Before, limit: 2, more: true, result: []string{"aa", "b"}},
			{cursor: "c", dir: storj.Before, limit: 2, more: true, result: []string{"b", "bb"}},
			{cursor: "ca", dir: storj.Before, limit: 2, more: true, result: []string{"bb", "c"}},
		} {
			errTag := fmt.Sprintf("%d. %+v", i, tt)

			bucketList, err := bucketsDB.List(ctx, project.ID, metainfo.BucketListOptions{
				Cursor:    tt.cursor,
				Direction: tt.dir,
				Limit:     tt.limit,
			})

			if assert.NoError(t, err, errTag) {
				assert.Equal(t, tt.more, bucketList.More, errTag)
				assert.Equal(t, tt.result, getBucketNames(bucketList), errTag)
			}
		}
	})
}

func getBucketNames(bucketList metainfo.BucketList) []string {
	names := make([]string, len(bucketList.Items))

	for i, item := range bucketList.Items {
		names[i] = item.Name
	}

	return names
}
