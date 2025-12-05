// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
	})
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
	})
}

func TestBucketTagging(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		bucketsDB := db.Buckets()
		projectID := testrand.UUID()
		bucketName := testrand.BucketName()

		_, err := db.Console().Projects().Insert(ctx, &console.Project{ID: projectID})
		require.NoError(t, err)

		tags, err := bucketsDB.GetBucketTagging(ctx, []byte(bucketName), projectID)
		require.ErrorIs(t, err, buckets.ErrBucketNotFound.Instance())
		require.Nil(t, tags)

		err = bucketsDB.SetBucketTagging(ctx, []byte(bucketName), projectID, []buckets.Tag{})
		require.ErrorIs(t, err, buckets.ErrBucketNotFound.Instance())

		_, err = bucketsDB.CreateBucket(ctx, buckets.Bucket{
			ID:        testrand.UUID(),
			Name:      bucketName,
			ProjectID: projectID,
		})
		require.NoError(t, err)

		tags, err = bucketsDB.GetBucketTagging(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		require.Empty(t, tags)

		var expectedTags []buckets.Tag
		for i := 0; i < 16; i++ {
			expectedTags = append(expectedTags, buckets.Tag{
				Key:   string(testrand.RandAlphaNumeric(16)),
				Value: string(testrand.RandAlphaNumeric(16)),
			})
		}
		// Ensure that there are no issues encoding/decoding tags with empty keys or values.
		expectedTags = append(expectedTags, buckets.Tag{})

		require.NoError(t, bucketsDB.SetBucketTagging(ctx, []byte(bucketName), projectID, expectedTags))
		tags, err = bucketsDB.GetBucketTagging(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		require.Equal(t, expectedTags, tags)

		require.NoError(t, bucketsDB.SetBucketTagging(ctx, []byte(bucketName), projectID, []buckets.Tag{}))
		tags, err = bucketsDB.GetBucketTagging(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		require.Empty(t, tags)
	})
}

func TestBucketNotificationConfig_UpdateAndGet(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		projectID := testrand.UUID()
		bucketName := testrand.BucketName()

		// Create project and bucket first
		_, err := db.Console().Projects().Insert(ctx, &console.Project{ID: projectID})
		require.NoError(t, err)

		_, err = db.Buckets().CreateBucket(ctx, buckets.Bucket{
			ID:        testrand.UUID(),
			Name:      bucketName,
			ProjectID: projectID,
		})
		require.NoError(t, err)

		// Test 1: Get non-existent configuration should return nil
		config, err := db.Buckets().GetBucketNotificationConfig(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		require.Nil(t, config)

		// Test 2: Insert new configuration
		newConfig := buckets.NotificationConfig{
			ConfigID:     "test-config-1",
			TopicName:    "projects/test-project/topics/test-topic",
			Events:       []string{"s3:ObjectCreated:Put", "s3:ObjectRemoved:Delete"},
			FilterPrefix: []byte("logs/"),
			FilterSuffix: []byte(".txt"),
		}

		err = db.Buckets().UpdateBucketNotificationConfig(ctx, []byte(bucketName), projectID, newConfig)
		require.NoError(t, err)

		// Test 3: Get inserted configuration
		newRetrieved, err := db.Buckets().GetBucketNotificationConfig(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		require.NotNil(t, newRetrieved)
		assert.Equal(t, newConfig.ConfigID, newRetrieved.ConfigID)
		assert.Equal(t, newConfig.TopicName, newRetrieved.TopicName)
		assert.Equal(t, newConfig.Events, newRetrieved.Events)
		assert.Equal(t, newConfig.FilterPrefix, newRetrieved.FilterPrefix)
		assert.Equal(t, newConfig.FilterSuffix, newRetrieved.FilterSuffix)
		assert.WithinDuration(t, time.Now(), newRetrieved.CreatedAt, time.Minute)
		assert.WithinDuration(t, time.Now(), newRetrieved.UpdatedAt, time.Minute)

		// Test 4: Update existing configuration (UPSERT)
		updatedConfig := buckets.NotificationConfig{
			ConfigID:     "test-config-2",
			TopicName:    "projects/test-project/topics/updated-topic",
			Events:       []string{"s3:ObjectCreated:*"},
			FilterPrefix: []byte("data/"),
			FilterSuffix: []byte(".json"),
		}

		err = db.Buckets().UpdateBucketNotificationConfig(ctx, []byte(bucketName), projectID, updatedConfig)
		require.NoError(t, err)

		// Test 5: Verify update
		updatedRetrieved, err := db.Buckets().GetBucketNotificationConfig(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		require.NotNil(t, updatedRetrieved)
		assert.Equal(t, updatedConfig.ConfigID, updatedRetrieved.ConfigID)
		assert.Equal(t, updatedConfig.TopicName, updatedRetrieved.TopicName)
		assert.Equal(t, updatedConfig.Events, updatedRetrieved.Events)
		assert.Equal(t, updatedConfig.FilterPrefix, updatedRetrieved.FilterPrefix)
		assert.Equal(t, updatedConfig.FilterSuffix, updatedRetrieved.FilterSuffix)
		assert.Equal(t, newRetrieved.CreatedAt, updatedRetrieved.CreatedAt)
		assert.WithinDuration(t, time.Now(), updatedRetrieved.UpdatedAt, time.Minute)
		assert.Greater(t, updatedRetrieved.UpdatedAt, updatedRetrieved.CreatedAt)
	})
}

func TestBucketNotificationConfig_AutoGeneratedConfigID(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		projectID := testrand.UUID()
		bucketName := testrand.BucketName()

		// Create project and bucket
		_, err := db.Console().Projects().Insert(ctx, &console.Project{ID: projectID})
		require.NoError(t, err)

		_, err = db.Buckets().CreateBucket(ctx, buckets.Bucket{
			ID:        testrand.UUID(),
			Name:      bucketName,
			ProjectID: projectID,
		})
		require.NoError(t, err)

		// Test 1: Insert configuration without config_id (should auto-generate)
		newConfig := buckets.NotificationConfig{
			ConfigID:  "", // Empty - should be auto-generated by database
			TopicName: "projects/test-project/topics/test-topic",
			Events:    []string{"s3:ObjectCreated:Put"},
		}

		err = db.Buckets().UpdateBucketNotificationConfig(ctx, []byte(bucketName), projectID, newConfig)
		require.NoError(t, err)

		// Verify config_id was auto-generated
		retrieved, err := db.Buckets().GetBucketNotificationConfig(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		require.NotNil(t, retrieved)
		assert.NotEmpty(t, retrieved.ConfigID, "config_id should be auto-generated")

		// Test 2: Update with empty config_id should preserve the existing config_id
		updatedConfig := buckets.NotificationConfig{
			ConfigID:  "", // Empty - should preserve existing config_id
			TopicName: "projects/test-project/topics/updated-topic",
			Events:    []string{"s3:ObjectCreated:*", "s3:ObjectRemoved:*"},
		}

		err = db.Buckets().UpdateBucketNotificationConfig(ctx, []byte(bucketName), projectID, updatedConfig)
		require.NoError(t, err)

		// Verify config_id was preserved and other fields were updated
		updatedRetrieved, err := db.Buckets().GetBucketNotificationConfig(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		require.NotNil(t, updatedRetrieved)
		assert.Equal(t, retrieved.ConfigID, updatedRetrieved.ConfigID, "config_id should be preserved when updating with empty config_id")
		assert.Equal(t, updatedConfig.TopicName, updatedRetrieved.TopicName)
		assert.Equal(t, updatedConfig.Events, updatedRetrieved.Events)
	})
}

func TestBucketNotificationConfig_Delete(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		projectID := testrand.UUID()
		bucketName := testrand.BucketName()

		// Create project and bucket
		_, err := db.Console().Projects().Insert(ctx, &console.Project{ID: projectID})
		require.NoError(t, err)

		_, err = db.Buckets().CreateBucket(ctx, buckets.Bucket{
			ID:        testrand.UUID(),
			Name:      bucketName,
			ProjectID: projectID,
		})
		require.NoError(t, err)

		// Insert configuration
		config := buckets.NotificationConfig{
			ConfigID:  "test-config",
			TopicName: "projects/test-project/topics/test-topic",
			Events:    []string{"s3:ObjectCreated:Put"},
		}

		err = db.Buckets().UpdateBucketNotificationConfig(ctx, []byte(bucketName), projectID, config)
		require.NoError(t, err)

		// Verify it exists
		retrieved, err := db.Buckets().GetBucketNotificationConfig(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		require.NotNil(t, retrieved)
		assert.Equal(t, config.ConfigID, retrieved.ConfigID)

		// Delete configuration
		err = db.Buckets().DeleteBucketNotificationConfig(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)

		// Verify it's gone
		retrieved, err = db.Buckets().GetBucketNotificationConfig(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		require.Nil(t, retrieved)

		// Delete non-existent configuration (should not error)
		err = db.Buckets().DeleteBucketNotificationConfig(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
	})
}

func TestBucketNotificationConfig_CascadeDelete(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		projectID := testrand.UUID()
		bucketName := testrand.BucketName()

		// Create project and bucket
		_, err := db.Console().Projects().Insert(ctx, &console.Project{ID: projectID})
		require.NoError(t, err)

		_, err = db.Buckets().CreateBucket(ctx, buckets.Bucket{
			ID:        testrand.UUID(),
			Name:      bucketName,
			ProjectID: projectID,
		})
		require.NoError(t, err)

		// Insert notification configuration
		config := buckets.NotificationConfig{
			ConfigID:  "test-config",
			TopicName: "projects/test-project/topics/test-topic",
			Events:    []string{"s3:ObjectCreated:Put"},
		}

		err = db.Buckets().UpdateBucketNotificationConfig(ctx, []byte(bucketName), projectID, config)
		require.NoError(t, err)

		// Verify configuration exists
		retrieved, err := db.Buckets().GetBucketNotificationConfig(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		require.NotNil(t, retrieved)
		assert.Equal(t, config.ConfigID, retrieved.ConfigID)

		// Delete the bucket (should cascade delete the notification config)
		err = db.Buckets().DeleteBucket(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)

		// Verify notification configuration was cascade deleted
		retrieved, err = db.Buckets().GetBucketNotificationConfig(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		require.Nil(t, retrieved)
	})
}

func TestBucketNotificationConfig_NullableFilters(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		projectID := testrand.UUID()
		bucketName := testrand.BucketName()

		// Create project and bucket
		_, err := db.Console().Projects().Insert(ctx, &console.Project{ID: projectID})
		require.NoError(t, err)

		_, err = db.Buckets().CreateBucket(ctx, buckets.Bucket{
			ID:        testrand.UUID(),
			Name:      bucketName,
			ProjectID: projectID,
		})
		require.NoError(t, err)

		// Test 1: No filters (nil byte slices)
		config1 := buckets.NotificationConfig{
			ConfigID:     "config-no-filters",
			TopicName:    "projects/test-project/topics/test-topic",
			Events:       []string{"s3:ObjectCreated:Put"},
			FilterPrefix: nil,
			FilterSuffix: nil,
		}

		err = db.Buckets().UpdateBucketNotificationConfig(ctx, []byte(bucketName), projectID, config1)
		require.NoError(t, err)

		retrieved, err := db.Buckets().GetBucketNotificationConfig(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		require.NotNil(t, retrieved)
		assert.Empty(t, retrieved.FilterPrefix)
		assert.Empty(t, retrieved.FilterSuffix)

		// Test 2: Only prefix filter
		config2 := buckets.NotificationConfig{
			ConfigID:     "config-prefix-only",
			TopicName:    "projects/test-project/topics/test-topic",
			Events:       []string{"s3:ObjectCreated:Put"},
			FilterPrefix: []byte("logs/"),
			FilterSuffix: nil,
		}

		err = db.Buckets().UpdateBucketNotificationConfig(ctx, []byte(bucketName), projectID, config2)
		require.NoError(t, err)

		retrieved, err = db.Buckets().GetBucketNotificationConfig(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		require.NotNil(t, retrieved)
		assert.Equal(t, []byte("logs/"), retrieved.FilterPrefix)
		assert.Empty(t, retrieved.FilterSuffix)

		// Test 3: Only suffix filter
		config3 := buckets.NotificationConfig{
			ConfigID:     "config-suffix-only",
			TopicName:    "projects/test-project/topics/test-topic",
			Events:       []string{"s3:ObjectCreated:Put"},
			FilterPrefix: nil,
			FilterSuffix: []byte(".jpg"),
		}

		err = db.Buckets().UpdateBucketNotificationConfig(ctx, []byte(bucketName), projectID, config3)
		require.NoError(t, err)

		retrieved, err = db.Buckets().GetBucketNotificationConfig(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		require.NotNil(t, retrieved)
		assert.Empty(t, retrieved.FilterPrefix)
		assert.Equal(t, []byte(".jpg"), retrieved.FilterSuffix)
	})
}

func TestBucketNotificationConfig_Validation(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		projectID := testrand.UUID()
		bucketName := testrand.BucketName()

		// Create project and bucket
		_, err := db.Console().Projects().Insert(ctx, &console.Project{ID: projectID})
		require.NoError(t, err)

		_, err = db.Buckets().CreateBucket(ctx, buckets.Bucket{
			ID:        testrand.UUID(),
			Name:      bucketName,
			ProjectID: projectID,
		})
		require.NoError(t, err)

		// Test: Empty topic name should error
		t.Run("empty topic name", func(t *testing.T) {
			config := buckets.NotificationConfig{
				ConfigID:  "config-empty-topic",
				TopicName: "",
				Events:    []string{"s3:ObjectCreated:Put"},
			}

			err := db.Buckets().UpdateBucketNotificationConfig(ctx, []byte(bucketName), projectID, config)
			require.Error(t, err)
		})

		// Test: Nil events should error
		t.Run("nil events", func(t *testing.T) {
			config := buckets.NotificationConfig{
				ConfigID:  "config-nil-events",
				TopicName: "projects/test-project/topics/test-topic",
				Events:    nil,
			}

			err := db.Buckets().UpdateBucketNotificationConfig(ctx, []byte(bucketName), projectID, config)
			require.Error(t, err)
		})

		// Test: Empty events should error
		t.Run("empty events", func(t *testing.T) {
			config := buckets.NotificationConfig{
				ConfigID:  "config-empty-events",
				TopicName: "projects/test-project/topics/test-topic",
				Events:    []string{},
			}

			err := db.Buckets().UpdateBucketNotificationConfig(ctx, []byte(bucketName), projectID, config)
			require.Error(t, err)
		})
	})
}

func TestBucketNotificationConfig_EventsSerialization(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		projectID := testrand.UUID()
		bucketName := testrand.BucketName()

		// Create project and bucket
		_, err := db.Console().Projects().Insert(ctx, &console.Project{ID: projectID})
		require.NoError(t, err)

		_, err = db.Buckets().CreateBucket(ctx, buckets.Bucket{
			ID:        testrand.UUID(),
			Name:      bucketName,
			ProjectID: projectID,
		})
		require.NoError(t, err)

		for _, tc := range []struct {
			name   string
			events []string
		}{
			{
				name:   "single event",
				events: []string{"s3:ObjectCreated:Put"},
			},
			{
				name:   "multiple specific events",
				events: []string{"s3:ObjectCreated:Put", "s3:ObjectCreated:Copy", "s3:ObjectRemoved:Delete"},
			},
			{
				name:   "wildcard events",
				events: []string{"s3:ObjectCreated:*", "s3:ObjectRemoved:*"},
			},
			{
				name:   "mixed specific and wildcard",
				events: []string{"s3:ObjectCreated:*", "s3:ObjectRemoved:Delete"},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				config := buckets.NotificationConfig{
					ConfigID:  "config-" + tc.name,
					TopicName: "projects/test-project/topics/test-topic",
					Events:    tc.events,
				}

				err := db.Buckets().UpdateBucketNotificationConfig(ctx, []byte(bucketName), projectID, config)
				require.NoError(t, err)

				retrieved, err := db.Buckets().GetBucketNotificationConfig(ctx, []byte(bucketName), projectID)
				require.NoError(t, err)
				require.NotNil(t, retrieved)
				assert.Equal(t, tc.events, retrieved.Events)
			})
		}
	})
}
