// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"testing"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func newTestBucket(name string, projectID uuid.UUID) storj.Bucket {
	return storj.Bucket{
		ID:                  testrand.UUID(),
		Name:                name,
		ProjectID:           projectID,
		PathCipher:          storj.EncAESGCM,
		DefaultSegmentsSize: 65536,
		DefaultRedundancyScheme: storj.RedundancyScheme{
			Algorithm:      storj.ReedSolomon,
			ShareSize:      9,
			RequiredShares: 10,
			RepairShares:   11,
			OptimalShares:  12,
			TotalShares:    13,
		},
		DefaultEncryptionParameters: storj.EncryptionParameters{
			CipherSuite: storj.EncAESGCM,
			BlockSize:   32,
		},
	}
}

func TestBasicBucketOperations(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()
		consoleDB := db.Console()
		project, err := consoleDB.Projects().Insert(ctx, &console.Project{Name: "testproject1"})
		require.NoError(t, err)

		bucketsDB := db.Buckets()
		expectedBucket := newTestBucket("testbucket", project.ID)

		// CreateBucket
		_, err = bucketsDB.CreateBucket(ctx, expectedBucket)
		require.NoError(t, err)

		// GetBucket
		bucket, err := bucketsDB.GetBucket(ctx, []byte("testbucket"), project.ID)
		require.NoError(t, err)
		require.Equal(t, expectedBucket.ID, bucket.ID)
		require.Equal(t, expectedBucket.Name, bucket.Name)
		require.Equal(t, expectedBucket.ProjectID, bucket.ProjectID)
		require.Equal(t, expectedBucket.PathCipher, bucket.PathCipher)
		require.Equal(t, expectedBucket.DefaultSegmentsSize, bucket.DefaultSegmentsSize)
		require.Equal(t, expectedBucket.DefaultRedundancyScheme, bucket.DefaultRedundancyScheme)
		require.Equal(t, expectedBucket.DefaultEncryptionParameters, bucket.DefaultEncryptionParameters)

		// DeleteBucket
		err = bucketsDB.DeleteBucket(ctx, []byte("testbucket"), project.ID)
		require.NoError(t, err)
	})
}

func TestListBucketsAllAllowed(t *testing.T) {
	testCases := []struct {
		name          string
		cursor        string
		limit         int
		expectedItems int
		expectedMore  bool
	}{
		{"empty string cursor", "", 10, 10, false},
		{"last bucket cursor", "zzz", 2, 1, false},
		{"non matching cursor", "ccc", 10, 5, false},
		{"first bucket cursor", "0test", 10, 10, false},
		{"empty string cursor, more", "", 5, 5, true},
		{"non matching cursor, more", "ccc", 3, 3, true},
		{"first bucket cursor, more", "0test", 5, 5, true},
	}
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()
		consoleDB := db.Console()
		project, err := consoleDB.Projects().Insert(ctx, &console.Project{Name: "testproject1"})
		require.NoError(t, err)

		bucketsDB := db.Buckets()

		var allowedPaths = map[string]struct{}{}
		{ // setup some test buckets
			var testBucketNames = []string{"aaa", "bbb", "mmm", "qqq", "zzz",
				"test.bucket", "123", "0test", "999", "test-bucket.thing",
			}
			for _, bucket := range testBucketNames {
				testBucket := newTestBucket(bucket, project.ID)
				_, err := bucketsDB.CreateBucket(ctx, testBucket)
				allowedPaths[bucket] = struct{}{}
				if err != nil {
					require.NoError(t, err)
				}
			}
		}

		for _, tt := range testCases {
			tt := tt // avoid scopelint error
			t.Run(tt.name, func(t *testing.T) {
				bucketList, err := bucketsDB.ListBuckets(ctx, project.ID, storj.BucketListOptions{
					Cursor:    tt.cursor,
					Direction: storj.Forward,
					Limit:     tt.limit,
				},
					allowedPaths,
				)
				require.NoError(t, err)
				require.Equal(t, tt.expectedItems, len(bucketList.Items))
				require.Equal(t, tt.expectedMore, bucketList.More)
			})
		}
	})
}

func TestListBucketsNotAllowed(t *testing.T) {
	testCases := []struct {
		name          string
		cursor        string
		limit         int
		expectedItems int
		expectedMore  bool
		allowedPaths  map[string]struct{}
		expectedNames []string
	}{
		{"empty string cursor, 2 allowed", "", 10, 1, false, map[string]struct{}{"aaa": {}, "ddd": {}}, []string{"aaa"}},
		{"empty string cursor, more", "", 2, 2, true, map[string]struct{}{"aaa": {}, "bbb": {}, "zzz": {}}, []string{"aaa", "bbb"}},
		{"empty string cursor, 3 allowed", "", 4, 3, false, map[string]struct{}{"aaa": {}, "bbb": {}, "zzz": {}}, []string{"aaa", "bbb", "zzz"}},
		{"last bucket cursor", "zzz", 2, 1, false, map[string]struct{}{"zzz": {}}, []string{"zzz"}},
	}
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()
		consoleDB := db.Console()
		project, err := consoleDB.Projects().Insert(ctx, &console.Project{Name: "testproject1"})
		require.NoError(t, err)

		bucketsDB := db.Buckets()

		{ // setup some test buckets
			var testBucketNames = []string{"aaa", "bbb", "mmm", "qqq", "zzz",
				"test.bucket", "123", "0test", "999", "test-bucket.thing",
			}
			for _, bucket := range testBucketNames {
				testBucket := newTestBucket(bucket, project.ID)
				_, err := bucketsDB.CreateBucket(ctx, testBucket)
				if err != nil {
					require.NoError(t, err)
				}
			}
		}

		for _, tt := range testCases {
			tt := tt // avoid scopelint error
			listOpts := storj.BucketListOptions{
				Cursor:    tt.cursor,
				Direction: storj.Forward,
				Limit:     tt.limit,
			}
			t.Run(tt.name, func(t *testing.T) {
				bucketList, err := bucketsDB.ListBuckets(ctx, project.ID,
					listOpts,
					tt.allowedPaths,
				)
				require.NoError(t, err)
				require.Equal(t, tt.expectedItems, len(bucketList.Items))
				require.Equal(t, tt.expectedMore, bucketList.More)
				for _, actualItem := range bucketList.Items {
					require.Contains(t, tt.expectedNames, actualItem.Name)

				}
			})
		}
	})
}
