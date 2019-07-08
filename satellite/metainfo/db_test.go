// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/require"

	"storj.io/storj/satellite/console"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metainfo"
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

func TestListBuckets(t *testing.T) {
	testCases := []struct {
		name          string
		cursor        string
		direction     storj.ListDirection
		limit         int
		expectedItems int
		expectedMore  bool
	}{
		{"empty string cursor", "", storj.Forward, 10, 10, false},
		{"last bucket cursor", "zzz", storj.Forward, 2, 1, false},
		{"non matching cursor", "ccc", storj.Forward, 10, 5, false},
		{"first bucket cursor", "0test", storj.Forward, 10, 10, false},
		{"empty string cursor, more", "", storj.Forward, 5, 5, true},
		{"non matching cursor, more", "ccc", storj.Forward, 3, 3, true},
		{"first bucket cursor, more", "0test", storj.Forward, 5, 5, true},
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
					fmt.Println(err)
				}
			}
		}
		setup(ctx, bucketsDB, project.ID)

		for _, tt := range testCases {
			tt := tt // avoid scopelint error
			t.Run(tt.name, func(t *testing.T) {
				bucketList, err := bucketsDB.ListBuckets(ctx, project.ID, storj.BucketListOptions{
					Cursor:    tt.cursor,
					Direction: tt.direction,
					Limit:     tt.limit,
				})
				require.NoError(t, err)
				require.Equal(t, tt.expectedItems, len(bucketList.Items))
				require.Equal(t, tt.expectedMore, bucketList.More)
			})
		}
	})
}
