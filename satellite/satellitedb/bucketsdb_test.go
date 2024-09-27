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
		require.False(t, settings.ObjectLockEnabled)
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
		require.True(t, bucket.ObjectLockEnabled)

		settings, err = bucketsDB.GetBucketObjectLockSettings(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		require.True(t, settings.ObjectLockEnabled)

		updateParams.ObjectLockEnabled = false

		_, err = bucketsDB.UpdateBucketObjectLockSettings(ctx, updateParams)
		require.Error(t, err)

		mode := storj.ComplianceMode
		modePtr := &mode
		updateParams.DefaultRetentionMode = &modePtr
		updateParams.ObjectLockEnabled = true

		bucket, err = bucketsDB.UpdateBucketObjectLockSettings(ctx, updateParams)
		require.NoError(t, err)
		require.Equal(t, storj.ComplianceMode, bucket.DefaultRetentionMode)

		settings, err = bucketsDB.GetBucketObjectLockSettings(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		require.Equal(t, mode, settings.DefaultRetentionMode)

		mode = storj.NoRetention

		bucket, err = bucketsDB.UpdateBucketObjectLockSettings(ctx, updateParams)
		require.NoError(t, err)
		require.Equal(t, storj.NoRetention, bucket.DefaultRetentionMode)

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
		require.Nil(t, bucket.DefaultRetentionDays)
		require.NotNil(t, bucket.DefaultRetentionYears)
		require.Equal(t, years, *bucket.DefaultRetentionYears)

		settings, err = bucketsDB.GetBucketObjectLockSettings(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		require.Equal(t, years, settings.DefaultRetentionYears)

		updateParams.DefaultRetentionMode = nil
		updateParams.DefaultRetentionYears = nil

		bucket, err = bucketsDB.UpdateBucketObjectLockSettings(ctx, updateParams)
		require.NoError(t, err)
		require.Nil(t, bucket.DefaultRetentionDays)
		require.NotNil(t, bucket.DefaultRetentionYears)
		require.Equal(t, years, *bucket.DefaultRetentionYears)
		require.Equal(t, storj.NoRetention, bucket.DefaultRetentionMode)

		updateParams.DefaultRetentionYears = &yearsPtr
		yearsPtr = nil
		updateParams.DefaultRetentionDays = &daysPtr

		bucket, err = bucketsDB.UpdateBucketObjectLockSettings(ctx, updateParams)
		require.NoError(t, err)
		require.Nil(t, bucket.DefaultRetentionYears)
		require.NotNil(t, bucket.DefaultRetentionDays)
		require.Equal(t, days, *bucket.DefaultRetentionDays)

		settings, err = bucketsDB.GetBucketObjectLockSettings(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		require.Equal(t, days, settings.DefaultRetentionDays)

		daysPtr = nil

		bucket, err = bucketsDB.UpdateBucketObjectLockSettings(ctx, updateParams)
		require.NoError(t, err)
		require.Nil(t, bucket.DefaultRetentionDays)
		require.Nil(t, bucket.DefaultRetentionYears)

		zeroValue := 0
		*updateParams.DefaultRetentionYears = &zeroValue
		*updateParams.DefaultRetentionDays = &zeroValue

		bucket, err = bucketsDB.UpdateBucketObjectLockSettings(ctx, updateParams)
		require.NoError(t, err)
		require.Nil(t, bucket.DefaultRetentionDays)
		require.Nil(t, bucket.DefaultRetentionYears)

		negativeValue := -1
		*updateParams.DefaultRetentionYears = &negativeValue

		_, err = bucketsDB.UpdateBucketObjectLockSettings(ctx, updateParams)
		require.Error(t, err)
	}, satellitedbtest.WithSpanner())
}
