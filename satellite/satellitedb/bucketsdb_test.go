// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestUpdateBucketObjectLockSettings(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		bucketName := testrand.BucketName()
		projectID := testrand.UUID()

		_, err := db.Console().Projects().Insert(ctx, &console.Project{ID: projectID})
		require.NoError(t, err)

		bucketsDB := db.Buckets()

		_, err = bucketsDB.CreateBucket(ctx, buckets.Bucket{
			ID:        testrand.UUID(),
			Name:      bucketName,
			ProjectID: projectID,
		})
		require.NoError(t, err)

		settings, err := bucketsDB.GetBucketObjectLockSettings(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		require.False(t, settings.Enabled)
		require.Equal(t, storj.NoRetention, settings.DefaultRetentionMode)
		require.Zero(t, settings.DefaultRetentionDays)
		require.Zero(t, settings.DefaultRetentionYears)

		updateParams := buckets.UpdateBucketObjectLockParams{
			ProjectID:         projectID,
			Name:              bucketName,
			ObjectLockEnabled: true,
		}

		bucket, err := bucketsDB.UpdateBucketObjectLockSettings(ctx, updateParams)
		require.NoError(t, err)
		require.True(t, bucket.ObjectLock.Enabled)

		settings, err = bucketsDB.GetBucketObjectLockSettings(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		require.True(t, settings.Enabled)

		updateParams.ObjectLockEnabled = false

		_, err = bucketsDB.UpdateBucketObjectLockSettings(ctx, updateParams)
		require.Error(t, err)

		mode := storj.ComplianceMode
		modePtr := &mode
		updateParams.DefaultRetentionMode = &modePtr
		updateParams.ObjectLockEnabled = true

		bucket, err = bucketsDB.UpdateBucketObjectLockSettings(ctx, updateParams)
		require.NoError(t, err)
		require.Equal(t, storj.ComplianceMode, bucket.ObjectLock.DefaultRetentionMode)

		settings, err = bucketsDB.GetBucketObjectLockSettings(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		require.Equal(t, mode, settings.DefaultRetentionMode)

		mode = storj.NoRetention

		bucket, err = bucketsDB.UpdateBucketObjectLockSettings(ctx, updateParams)
		require.NoError(t, err)
		require.Equal(t, storj.NoRetention, bucket.ObjectLock.DefaultRetentionMode)

		settings, err = bucketsDB.GetBucketObjectLockSettings(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		require.Equal(t, mode, settings.DefaultRetentionMode)

		days := 10
		daysPtr := &days
		years := 5
		yearsPtr := &years
		updateParams.DefaultRetentionDays = &daysPtr
		updateParams.DefaultRetentionYears = &yearsPtr

		_, err = bucketsDB.UpdateBucketObjectLockSettings(ctx, updateParams)
		require.Error(t, err)

		updateParams.DefaultRetentionDays = nil

		bucket, err = bucketsDB.UpdateBucketObjectLockSettings(ctx, updateParams)
		require.NoError(t, err)
		require.Equal(t, years, bucket.ObjectLock.DefaultRetentionYears)

		settings, err = bucketsDB.GetBucketObjectLockSettings(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		require.Equal(t, years, settings.DefaultRetentionYears)

		updateParams.DefaultRetentionMode = nil
		updateParams.DefaultRetentionYears = nil

		bucket, err = bucketsDB.UpdateBucketObjectLockSettings(ctx, updateParams)
		require.NoError(t, err)
		require.Equal(t, years, bucket.ObjectLock.DefaultRetentionYears)
		require.Equal(t, storj.NoRetention, bucket.ObjectLock.DefaultRetentionMode)

		updateParams.DefaultRetentionYears = &yearsPtr
		yearsPtr = nil
		updateParams.DefaultRetentionDays = &daysPtr

		bucket, err = bucketsDB.UpdateBucketObjectLockSettings(ctx, updateParams)
		require.NoError(t, err)
		require.Equal(t, days, bucket.ObjectLock.DefaultRetentionDays)

		settings, err = bucketsDB.GetBucketObjectLockSettings(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		require.Equal(t, days, settings.DefaultRetentionDays)

		daysPtr = nil

		bucket, err = bucketsDB.UpdateBucketObjectLockSettings(ctx, updateParams)
		require.NoError(t, err)
		require.Zero(t, bucket.ObjectLock.DefaultRetentionDays)
		require.Zero(t, bucket.ObjectLock.DefaultRetentionYears)

		zeroValue := 0
		*updateParams.DefaultRetentionYears = &zeroValue
		*updateParams.DefaultRetentionDays = &zeroValue

		bucket, err = bucketsDB.UpdateBucketObjectLockSettings(ctx, updateParams)
		require.NoError(t, err)
		require.Zero(t, bucket.ObjectLock.DefaultRetentionDays)
		require.Zero(t, bucket.ObjectLock.DefaultRetentionYears)

		negativeValue := -1
		*updateParams.DefaultRetentionYears = &negativeValue

		_, err = bucketsDB.UpdateBucketObjectLockSettings(ctx, updateParams)
		require.Error(t, err)
	}, satellitedbtest.WithSpanner())
}

func TestCreateBucketWithObjectLock(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		bucketsDB := db.Buckets()
		projectID := testrand.UUID()

		_, err := db.Console().Projects().Insert(ctx, &console.Project{ID: projectID})
		require.NoError(t, err)

		requireLock := func(t *testing.T, bucketName string, lockSettings buckets.ObjectLockSettings) {
			bucket, err := bucketsDB.GetBucket(ctx, []byte(bucketName), projectID)
			require.NoError(t, err)
			require.Equal(t, lockSettings, bucket.ObjectLock)
		}

		requireNotExists := func(t *testing.T, bucketName string) {
			exists, err := bucketsDB.HasBucket(ctx, []byte(bucketName), projectID)
			require.NoError(t, err)
			require.False(t, exists)
		}

		t.Run("Success", func(t *testing.T) {
			createReq := buckets.Bucket{
				ID:        testrand.UUID(),
				Name:      testrand.BucketName(),
				ProjectID: projectID,
				ObjectLock: buckets.ObjectLockSettings{
					Enabled: true,
				},
			}

			// Ensure that Object Lock can be enabled.
			bucket, err := bucketsDB.CreateBucket(ctx, createReq)
			require.NoError(t, err)
			require.Equal(t, createReq.ObjectLock, bucket.ObjectLock)

			requireLock(t, createReq.Name, createReq.ObjectLock)

			// Ensure that there are no issues expressing the default retention duration in days
			// or specifying the default retention mode as Compliance.
			createReq.Name = testrand.BucketName()
			createReq.ObjectLock.DefaultRetentionMode = storj.ComplianceMode
			createReq.ObjectLock.DefaultRetentionDays = 3

			bucket, err = bucketsDB.CreateBucket(ctx, createReq)
			require.NoError(t, err)
			require.Equal(t, createReq.ObjectLock, bucket.ObjectLock)

			requireLock(t, createReq.Name, createReq.ObjectLock)

			// Ensure that there are no issues expressing the default retention duration in years
			// or specifying the default retention mode as Governance.
			createReq.Name = testrand.BucketName()
			createReq.ObjectLock.DefaultRetentionMode = storj.GovernanceMode
			createReq.ObjectLock.DefaultRetentionDays = 5

			bucket, err = bucketsDB.CreateBucket(ctx, createReq)
			require.NoError(t, err)
			require.Equal(t, createReq.ObjectLock, bucket.ObjectLock)

			requireLock(t, createReq.Name, createReq.ObjectLock)
		})

		t.Run("Object Lock not enabled", func(t *testing.T) {
			bucketName := testrand.BucketName()
			bucket, err := bucketsDB.CreateBucket(ctx, buckets.Bucket{
				ID:        testrand.UUID(),
				Name:      bucketName,
				ProjectID: projectID,
				ObjectLock: buckets.ObjectLockSettings{
					Enabled:              false,
					DefaultRetentionMode: storj.ComplianceMode,
					DefaultRetentionDays: 3,
				},
			})
			require.Error(t, err)
			require.True(t, buckets.ErrBucket.Has(err))
			require.Empty(t, bucket)

			exists, err := bucketsDB.HasBucket(ctx, []byte(bucketName), projectID)
			require.NoError(t, err)
			require.False(t, exists)
		})

		t.Run("Missing retention mode", func(t *testing.T) {
			bucketName := testrand.BucketName()
			bucket, err := bucketsDB.CreateBucket(ctx, buckets.Bucket{
				ID:        testrand.UUID(),
				Name:      bucketName,
				ProjectID: projectID,
				ObjectLock: buckets.ObjectLockSettings{
					Enabled:              true,
					DefaultRetentionDays: 3,
				},
			})
			require.Error(t, err)
			require.True(t, buckets.ErrBucket.Has(err))
			require.Empty(t, bucket)
			requireNotExists(t, bucketName)
		})

		for _, tt := range []struct {
			name  string
			days  int
			years int
		}{
			{
				name:  "Default retention days and years specified",
				days:  3,
				years: 5,
			}, {
				name:  "Default retention days and years missing",
				days:  0,
				years: 0,
			}, {
				name: "Negative default retention days",
				days: -1,
			}, {
				name:  "Negative default retention years",
				years: -1,
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				bucketName := testrand.BucketName()
				bucket, err := bucketsDB.CreateBucket(ctx, buckets.Bucket{
					ID:        testrand.UUID(),
					Name:      bucketName,
					ProjectID: projectID,
					ObjectLock: buckets.ObjectLockSettings{
						Enabled:               true,
						DefaultRetentionMode:  storj.ComplianceMode,
						DefaultRetentionDays:  tt.days,
						DefaultRetentionYears: tt.years,
					},
				})
				require.Error(t, err)
				require.True(t, buckets.ErrBucket.Has(err))
				require.Empty(t, bucket)
				requireNotExists(t, bucketName)
			})
		}
	}, satellitedbtest.WithSpanner())
}
