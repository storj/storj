// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package eventing_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/eventing"
	"storj.io/storj/satellite/eventing/eventingconfig"
)

// CountingBucketsDB is a mock that counts how many times GetBucketNotificationConfig is called.
type CountingBucketsDB struct {
	buckets.DB
	config    *buckets.NotificationConfig
	callCount int
}

func (db *CountingBucketsDB) GetBucketNotificationConfig(ctx context.Context, bucketName []byte, projectID uuid.UUID) (*buckets.NotificationConfig, error) {
	db.callCount++
	return db.config, nil
}

func testCacheConfig(ttl time.Duration) eventingconfig.Config {
	return eventingconfig.Config{
		Cache: eventingconfig.CacheConfig{
			TTL:      ttl,
			Capacity: 100,
		},
	}
}

func TestConfigCache_Get(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	projectID := testrand.UUID()
	bucketName := "test-bucket"

	expectedConfig := &buckets.NotificationConfig{
		TopicName:    "projects/test/topics/test-topic",
		Events:       []string{"s3:ObjectCreated:*"},
		FilterPrefix: []byte("prefix/"),
		FilterSuffix: []byte(".txt"),
	}

	t.Run("cache miss - fetch from DB and cache", func(t *testing.T) {
		mockDB := &CountingBucketsDB{config: expectedConfig}
		cache, err := eventing.NewConfigCache(mockDB, testCacheConfig(time.Hour))
		require.NoError(t, err)

		// First call - should hit DB.
		config, err := cache.GetBucketNotificationConfig(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		require.Equal(t, expectedConfig, config)
		require.Equal(t, 1, mockDB.callCount, "first call should query database")

		// Second call - should use cache.
		config2, err := cache.GetBucketNotificationConfig(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		require.Equal(t, expectedConfig, config2)
		require.Equal(t, 1, mockDB.callCount, "second call should use cache, not query database")
	})

	t.Run("cache nil config", func(t *testing.T) {
		nilConfigDB := &CountingBucketsDB{config: nil}
		cache, err := eventing.NewConfigCache(nilConfigDB, testCacheConfig(time.Hour))
		require.NoError(t, err)

		nilBucketName := "bucket-no-config"

		// First call - should hit DB.
		config, err := cache.GetBucketNotificationConfig(ctx, []byte(nilBucketName), projectID)
		require.NoError(t, err)
		require.Nil(t, config)
		require.Equal(t, 1, nilConfigDB.callCount, "first call should query database")

		// Second call - should return nil from negative cache.
		config2, err := cache.GetBucketNotificationConfig(ctx, []byte(nilBucketName), projectID)
		require.NoError(t, err)
		require.Nil(t, config2)
		require.Equal(t, 1, nilConfigDB.callCount, "second call should use negative cache, not query database")
	})
}

func TestConfigCache_TTL(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	projectID := testrand.UUID()
	bucketName := "test-bucket"

	t.Run("positive entry expires", func(t *testing.T) {
		mockDB := &CountingBucketsDB{config: &buckets.NotificationConfig{TopicName: "topic1"}}
		cache, err := eventing.NewConfigCache(mockDB, testCacheConfig(50*time.Millisecond))
		require.NoError(t, err)

		// Populate cache.
		_, err = cache.GetBucketNotificationConfig(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		assert.Equal(t, 1, mockDB.callCount)

		// Change DB — cache should still serve old value.
		mockDB.config = &buckets.NotificationConfig{TopicName: "topic2"}
		result, err := cache.GetBucketNotificationConfig(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		assert.Equal(t, "topic1", result.TopicName)
		assert.Equal(t, 1, mockDB.callCount, "should still use cache")

		// Wait for TTL to expire.
		time.Sleep(100 * time.Millisecond)

		result, err = cache.GetBucketNotificationConfig(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		assert.Equal(t, "topic2", result.TopicName)
		assert.Equal(t, 2, mockDB.callCount, "should re-query after TTL expires")
	})

	t.Run("negative entry expires", func(t *testing.T) {
		mockDB := &CountingBucketsDB{config: nil}
		cache, err := eventing.NewConfigCache(mockDB, testCacheConfig(50*time.Millisecond))
		require.NoError(t, err)

		// Populate cache.
		result, err := cache.GetBucketNotificationConfig(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		require.Nil(t, result)
		assert.Equal(t, 1, mockDB.callCount)

		// Change DB — cache should still return nil.
		mockDB.config = &buckets.NotificationConfig{TopicName: "topic1"}
		result, err = cache.GetBucketNotificationConfig(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		require.Nil(t, result)
		assert.Equal(t, 1, mockDB.callCount, "should use cache")

		// Wait for TTL to expire.
		time.Sleep(100 * time.Millisecond)

		result, err = cache.GetBucketNotificationConfig(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "topic1", result.TopicName)
		assert.Equal(t, 2, mockDB.callCount, "should re-query after TTL expires")
	})
}

func TestConfigCache_CacheKeyIsolation(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	projectID1 := uuid.UUID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10}
	projectID2 := uuid.UUID{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20}

	config1 := &buckets.NotificationConfig{TopicName: "topic1"}
	config2 := &buckets.NotificationConfig{TopicName: "topic2"}

	sharedDB := &CountingBucketsDB{}
	cache, err := eventing.NewConfigCache(sharedDB, testCacheConfig(time.Hour))
	require.NoError(t, err)

	// Cache config for project1/bucket1.
	sharedDB.config = config1
	result1, err := cache.GetBucketNotificationConfig(ctx, []byte("bucket1"), projectID1)
	require.NoError(t, err)
	assert.Equal(t, "topic1", result1.TopicName)

	// Cache config for project2/bucket1 (different project, same bucket name).
	sharedDB.config = config2
	result2, err := cache.GetBucketNotificationConfig(ctx, []byte("bucket1"), projectID2)
	require.NoError(t, err)
	assert.Equal(t, "topic2", result2.TopicName)

	// Change DB to prove reads below are served from cache.
	sharedDB.config = &buckets.NotificationConfig{TopicName: "wrong"}

	cached1, err := cache.GetBucketNotificationConfig(ctx, []byte("bucket1"), projectID1)
	require.NoError(t, err)
	assert.Equal(t, "topic1", cached1.TopicName)

	cached2, err := cache.GetBucketNotificationConfig(ctx, []byte("bucket1"), projectID2)
	require.NoError(t, err)
	assert.Equal(t, "topic2", cached2.TopicName)
}

func TestConfigCache_InvalidConfig(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	mockDB := &CountingBucketsDB{}

	t.Run("zero TTL", func(t *testing.T) {
		_, err := eventing.NewConfigCache(mockDB, testCacheConfig(0))
		require.ErrorContains(t, err, "TTL must be positive")
	})

	t.Run("zero Capacity", func(t *testing.T) {
		_, err := eventing.NewConfigCache(mockDB, eventingconfig.Config{
			Cache: eventingconfig.CacheConfig{
				TTL:      time.Hour,
				Capacity: 0,
			},
		})
		require.ErrorContains(t, err, "Capacity must be positive")
	})
}
