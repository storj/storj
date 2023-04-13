// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package buckets_test

import (
	"sort"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/macaroon"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/metabase"
)

func newTestBucket(name string, projectID uuid.UUID) buckets.Bucket {
	return buckets.Bucket{
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
			BlockSize:   9 * 10,
		},
		Placement: storj.EU,
	}
}

func TestBasicBucketOperations(t *testing.T) {
	testplanet.Run(t, testplanet.Config{SatelliteCount: 1}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		db := sat.DB
		consoleDB := db.Console()

		project, err := consoleDB.Projects().Insert(ctx, &console.Project{Name: "testproject1"})
		require.NoError(t, err)

		bucketsDB := sat.API.Buckets.Service
		expectedBucket := newTestBucket("testbucket", project.ID)

		count, err := bucketsDB.CountBuckets(ctx, project.ID)
		require.NoError(t, err)
		require.Equal(t, 0, count)

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
		require.Equal(t, expectedBucket.Placement, bucket.Placement)

		// GetMinimalBucket
		minimalBucket, err := bucketsDB.GetMinimalBucket(ctx, []byte("testbucket"), project.ID)
		require.NoError(t, err)
		require.Equal(t, []byte("testbucket"), minimalBucket.Name)
		require.False(t, minimalBucket.CreatedAt.IsZero())

		_, err = bucketsDB.GetMinimalBucket(ctx, []byte("not-existing-bucket"), project.ID)
		require.True(t, buckets.ErrBucketNotFound.Has(err), err)

		// GetBucketPlacement
		placement, err := bucketsDB.GetBucketPlacement(ctx, []byte("testbucket"), project.ID)
		require.NoError(t, err)
		require.Equal(t, expectedBucket.Placement, placement)

		_, err = bucketsDB.GetBucketPlacement(ctx, []byte("not-existing-bucket"), project.ID)
		require.True(t, buckets.ErrBucketNotFound.Has(err), err)

		// CountBuckets
		count, err = bucketsDB.CountBuckets(ctx, project.ID)
		require.NoError(t, err)
		require.Equal(t, 1, count)
		_, err = bucketsDB.CreateBucket(ctx, newTestBucket("testbucket2", project.ID))
		require.NoError(t, err)
		count, err = bucketsDB.CountBuckets(ctx, project.ID)
		require.NoError(t, err)
		require.Equal(t, 2, count)

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
	testplanet.Run(t, testplanet.Config{SatelliteCount: 1}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		db := sat.DB
		consoleDB := db.Console()

		project, err := consoleDB.Projects().Insert(ctx, &console.Project{Name: "testproject1"})
		require.NoError(t, err)

		bucketsDB := sat.API.Buckets.Service

		allowedBuckets := macaroon.AllowedBuckets{
			Buckets: map[string]struct{}{},
		}
		{ // setup some test buckets
			var testBucketNames = []string{"aaa", "bbb", "mmm", "qqq", "zzz",
				"test.bucket", "123", "0test", "999", "test-bucket.thing",
			}
			for _, bucket := range testBucketNames {
				testBucket := newTestBucket(bucket, project.ID)
				_, err := bucketsDB.CreateBucket(ctx, testBucket)
				allowedBuckets.Buckets[bucket] = struct{}{}
				if err != nil {
					require.NoError(t, err)
				}
			}
		}

		for _, tt := range testCases {
			tt := tt // avoid scopelint error
			t.Run(tt.name, func(t *testing.T) {

				listOpts := buckets.ListOptions{
					Cursor:    tt.cursor,
					Direction: storj.Forward,
					Limit:     tt.limit,
				}
				bucketList, err := bucketsDB.ListBuckets(ctx, project.ID, listOpts, allowedBuckets)
				require.NoError(t, err)
				require.Equal(t, tt.expectedItems, len(bucketList.Items))
				require.Equal(t, tt.expectedMore, bucketList.More)
			})
		}
	})
}

func TestListBucketsNotAllowed(t *testing.T) {
	testCases := []struct {
		name           string
		cursor         string
		limit          int
		expectedItems  int
		expectedMore   bool
		allowAll       bool
		allowedBuckets map[string]struct{}
		expectedNames  []string
	}{
		{"empty string cursor, 2 allowed", "", 10, 1, false, false, map[string]struct{}{"aaa": {}, "ddd": {}}, []string{"aaa"}},
		{"empty string cursor, more", "", 2, 2, true, false, map[string]struct{}{"aaa": {}, "bbb": {}, "zzz": {}}, []string{"aaa", "bbb"}},
		{"empty string cursor, 3 allowed", "", 4, 3, false, false, map[string]struct{}{"aaa": {}, "bbb": {}, "zzz": {}}, []string{"aaa", "bbb", "zzz"}},
		{"last bucket cursor", "zzz", 2, 1, false, false, map[string]struct{}{"zzz": {}}, []string{"zzz"}},
		{"last bucket cursor, allow all", "zzz", 2, 1, false, true, map[string]struct{}{"zzz": {}}, []string{"zzz"}},
		{"empty string cursor, allow all, more", "", 5, 5, true, true, map[string]struct{}{"": {}}, []string{"123", "0test", "999", "aaa", "bbb"}},
	}
	testplanet.Run(t, testplanet.Config{SatelliteCount: 1}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		db := sat.DB
		consoleDB := db.Console()

		project, err := consoleDB.Projects().Insert(ctx, &console.Project{Name: "testproject1"})
		require.NoError(t, err)

		bucketsDB := sat.API.Buckets.Service

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
			listOpts := buckets.ListOptions{
				Cursor:    tt.cursor,
				Direction: storj.Forward,
				Limit:     tt.limit,
			}
			t.Run(tt.name, func(t *testing.T) {
				allowed := macaroon.AllowedBuckets{
					Buckets: tt.allowedBuckets,
					All:     tt.allowAll,
				}
				bucketList, err := bucketsDB.ListBuckets(ctx,
					project.ID,
					listOpts,
					allowed,
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

func TestBatchBuckets(t *testing.T) {
	testplanet.Run(t, testplanet.Config{SatelliteCount: 1}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		db := sat.DB
		consoleDB := db.Console()

		var testBucketNames = []string{"aaa", "bbb", "mmm", "qqq", "zzz",
			"test.bucket", "123", "0test", "999", "test-bucket.thing",
		}

		bucketsService := sat.API.Buckets.Service
		var expectedBucketLocations []metabase.BucketLocation

		for i := 1; i < 4; i++ {
			project, err := consoleDB.Projects().Insert(ctx, &console.Project{Name: "testproject" + strconv.Itoa(i)})
			require.NoError(t, err)

			for _, bucket := range testBucketNames {
				testBucket := newTestBucket(bucket, project.ID)
				_, err := bucketsService.CreateBucket(ctx, testBucket)
				require.NoError(t, err)
				expectedBucketLocations = append(expectedBucketLocations, metabase.BucketLocation{
					ProjectID:  project.ID,
					BucketName: bucket,
				})
			}
		}

		sortBucketLocations(expectedBucketLocations)

		testLimits := []int{1, 3, 30, 1000, len(expectedBucketLocations)}

		for _, testLimit := range testLimits {
			more, err := db.Buckets().IterateBucketLocations(ctx, uuid.UUID{}, "", testLimit, func(bucketLocations []metabase.BucketLocation) (err error) {
				if testLimit > len(expectedBucketLocations) {
					testLimit = len(expectedBucketLocations)
				}

				expectedResult := expectedBucketLocations[:testLimit]
				require.Equal(t, expectedResult, bucketLocations)
				return nil
			})
			require.NoError(t, err)
			if testLimit < len(expectedBucketLocations) {
				require.True(t, more)
			} else {
				require.False(t, more)
			}
		}
	})
}

func sortBucketLocations(locations []metabase.BucketLocation) {
	sort.Slice(locations, func(i, j int) bool {
		if locations[i].ProjectID == locations[j].ProjectID {
			return locations[i].BucketName < locations[j].BucketName
		}
		return locations[i].ProjectID.Less(locations[j].ProjectID)
	})
}
