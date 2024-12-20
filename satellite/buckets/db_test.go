// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package buckets_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/macaroon"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func newTestBucket(name string, projectID uuid.UUID) buckets.Bucket {
	return buckets.Bucket{
		ID:        testrand.UUID(),
		Name:      name,
		ProjectID: projectID,
		Placement: storj.EU,
		ObjectLock: buckets.ObjectLockSettings{
			Enabled: true,
		},
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
		require.Equal(t, expectedBucket.Placement, bucket.Placement)
		require.Equal(t, expectedBucket.ObjectLock, bucket.ObjectLock)

		// GetMinimalBucket
		minimalBucket, err := bucketsDB.GetMinimalBucket(ctx, []byte("testbucket"), project.ID)
		require.NoError(t, err)
		require.Equal(t, []byte("testbucket"), minimalBucket.Name)
		require.False(t, minimalBucket.CreatedAt.IsZero())

		_, err = bucketsDB.GetMinimalBucket(ctx, []byte("not-existing-bucket"), project.ID)
		require.True(t, buckets.ErrBucketNotFound.Has(err), err)

		// GetBucketVersioningState
		versioningState, err := bucketsDB.GetBucketVersioningState(ctx, []byte("testbucket"), project.ID)
		require.NoError(t, err)
		require.Equal(t, buckets.VersioningUnsupported, versioningState)

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

		expectedBucket2 := newTestBucket("testbucket2", project.ID)
		expectedBucket2.ObjectLock.Enabled = false
		_, err = bucketsDB.CreateBucket(ctx, expectedBucket2)
		require.NoError(t, err)

		count, err = bucketsDB.CountBuckets(ctx, project.ID)
		require.NoError(t, err)
		require.Equal(t, 2, count)

		// GetBucketObjectLockEnabled
		enabled, err := bucketsDB.GetBucketObjectLockEnabled(ctx, []byte("testbucket"), project.ID)
		require.NoError(t, err)
		require.True(t, enabled)

		enabled, err = bucketsDB.GetBucketObjectLockEnabled(ctx, []byte("testbucket2"), project.ID)
		require.NoError(t, err)
		require.False(t, enabled)

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
					Direction: buckets.DirectionForward,
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
				Direction: buckets.DirectionForward,
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

func TestIterateBucketLocations_ProjectsWithMutipleBuckets(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		var expectedBucketLocations []metabase.BucketLocation

		var testBucketNames = []string{"aaa", "bbb", "mmm", "qqq", "zzz",
			"test.bucket", "123", "0test", "999", "test-bucket.thing",
		}

		for i := 1; i < 4; i++ {
			project, err := db.Console().Projects().Insert(ctx, &console.Project{
				ID:   testrand.UUID(),
				Name: "testproject" + strconv.Itoa(i),
			})
			require.NoError(t, err)

			for _, bucketName := range testBucketNames {
				expectedBucketLocations = append(expectedBucketLocations, metabase.BucketLocation{
					ProjectID:  project.ID,
					BucketName: metabase.BucketName(bucketName),
				})

				_, err = db.Buckets().CreateBucket(ctx, buckets.Bucket{
					ID:        testrand.UUID(),
					ProjectID: project.ID,
					Name:      bucketName,
				})
				require.NoError(t, err)
			}
		}

		for _, pageSize := range []int{1, 3, 30, 1000, len(expectedBucketLocations)} {
			bucketLocations := []metabase.BucketLocation{}

			err := db.Buckets().IterateBucketLocations(ctx, pageSize, func(bl []metabase.BucketLocation) error {
				bucketLocations = append(bucketLocations, bl...)
				return nil
			})
			require.NoError(t, err)

			require.ElementsMatch(t, expectedBucketLocations, bucketLocations)
		}
	})
}

func TestIterateBucketLocations_MultipleProjectsWithSingleBucket(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		expectedBucketLocations := []metabase.BucketLocation{}

		for i := 1; i < 16; i++ {
			location := metabase.BucketLocation{
				ProjectID:  testrand.UUID(),
				BucketName: metabase.BucketName("bucket" + strconv.Itoa(i)),
			}
			expectedBucketLocations = append(expectedBucketLocations, location)

			project, err := db.Console().Projects().Insert(ctx, &console.Project{
				ID:   location.ProjectID,
				Name: "test",
			})
			require.NoError(t, err)

			_, err = db.Buckets().CreateBucket(ctx, buckets.Bucket{
				ID:        testrand.UUID(),
				ProjectID: project.ID,
				Name:      string(location.BucketName),
			})
			require.NoError(t, err)
		}

		for pageSize := 1; pageSize < len(expectedBucketLocations)+1; pageSize++ {
			bucketLocations := []metabase.BucketLocation{}

			err := db.Buckets().IterateBucketLocations(ctx, pageSize, func(bl []metabase.BucketLocation) error {
				bucketLocations = append(bucketLocations, bl...)
				return nil
			})
			require.NoError(t, err)

			require.ElementsMatch(t, expectedBucketLocations, bucketLocations)
		}
	})
}

func TestEnableSuspendBucketVersioning(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		db := planet.Satellites[0].API.DB.Buckets()
		projectID := planet.Uplinks[0].Projects[0].ID

		requireBucketVersioning := func(name string, versioning buckets.Versioning) {
			bucket, err := db.GetBucket(ctx, []byte(name), projectID)
			require.NoError(t, err)
			require.Equal(t, versioning, bucket.Versioning)
		}

		bucketName := testrand.BucketName()
		_, err := db.CreateBucket(ctx, buckets.Bucket{
			Name:       bucketName,
			ProjectID:  projectID,
			Versioning: buckets.Unversioned,
		})
		require.NoError(t, err)

		// verify suspend unversioned bucket fails
		err = db.SuspendBucketVersioning(ctx, []byte(bucketName), projectID)
		require.True(t, buckets.ErrConflict.Has(err))
		requireBucketVersioning(bucketName, buckets.Unversioned)

		// verify enable unversioned bucket succeeds
		err = db.EnableBucketVersioning(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		requireBucketVersioning(bucketName, buckets.VersioningEnabled)

		// verify suspend enabled bucket succeeds
		err = db.SuspendBucketVersioning(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		requireBucketVersioning(bucketName, buckets.VersioningSuspended)

		// verify re-enable suspended bucket succeeds
		err = db.EnableBucketVersioning(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		requireBucketVersioning(bucketName, buckets.VersioningEnabled)

		// verify suspend bucket with Object Lock enabled fails
		lockBucketName := testrand.BucketName()
		_, err = db.CreateBucket(ctx, buckets.Bucket{
			Name:       lockBucketName,
			ProjectID:  projectID,
			Versioning: buckets.Unversioned,
			ObjectLock: buckets.ObjectLockSettings{
				Enabled: true,
			},
		})
		require.NoError(t, err)

		err = db.SuspendBucketVersioning(ctx, []byte(lockBucketName), projectID)
		require.True(t, buckets.ErrConflict.Has(err))
		requireBucketVersioning(lockBucketName, buckets.Unversioned)
	})
}
