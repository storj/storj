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
	"storj.io/storj/private/testplanet"
	"storj.io/storj/private/testredis"
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

func TestConfigCache_Get(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// Start miniredis server
	redis, err := testredis.Mini(ctx)
	require.NoError(t, err)
	defer ctx.Check(redis.Close)

	projectID := testrand.UUID()
	bucketName := "test-bucket"

	// Mock buckets DB config
	expectedConfig := &buckets.NotificationConfig{
		TopicName:    "projects/test/topics/test-topic",
		Events:       []string{"s3:ObjectCreated:*"},
		FilterPrefix: []byte("prefix/"),
		FilterSuffix: []byte(".txt"),
	}

	cfg := eventingconfig.Config{
		Cache: eventingconfig.CacheConfig{
			Address: "redis://" + redis.Addr(),
			TTL:     time.Hour,
		},
	}

	t.Run("cache miss - fetch from DB and cache", func(t *testing.T) {
		// Create a mock DB that tracks calls
		mockDB := &CountingBucketsDB{
			config: expectedConfig,
		}

		// Create cache wrapping the mock DB
		cache, err := eventing.NewConfigCache(testplanet.NewLogger(t), mockDB, cfg)
		require.NoError(t, err)
		defer ctx.Check(cache.Close)

		// First call - should hit DB
		config, err := cache.GetBucketNotificationConfig(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		require.Equal(t, expectedConfig, config)
		require.Equal(t, 1, mockDB.callCount, "first call should query database")

		// Second call - should use cache, not hit DB
		config2, err := cache.GetBucketNotificationConfig(ctx, []byte(bucketName), projectID)
		require.NoError(t, err)
		require.Equal(t, expectedConfig, config2)
		require.Equal(t, 1, mockDB.callCount, "second call should use cache, not query database")
	})

	t.Run("cache nil config", func(t *testing.T) {
		nilConfigDB := &CountingBucketsDB{
			config: nil,
		}

		// Create cache wrapping the mock DB
		cache, err := eventing.NewConfigCache(testplanet.NewLogger(t), nilConfigDB, cfg)
		require.NoError(t, err)
		defer ctx.Check(cache.Close)

		// Use different bucket name to avoid cache collision with other tests
		nilBucketName := "bucket-no-config"

		// First call - should hit DB
		config, err := cache.GetBucketNotificationConfig(ctx, []byte(nilBucketName), projectID)
		require.NoError(t, err)
		require.Nil(t, config)
		require.Equal(t, 1, nilConfigDB.callCount, "first call should query database")

		// Second call - should return nil from cache, not hit DB
		config2, err := cache.GetBucketNotificationConfig(ctx, []byte(nilBucketName), projectID)
		require.NoError(t, err)
		require.Nil(t, config2)
		require.Equal(t, 1, nilConfigDB.callCount, "second call should use cache, not query database")
	})
}

func TestConfigCache_Invalidate(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// Start miniredis server
	redis, err := testredis.Mini(ctx)
	require.NoError(t, err)
	defer ctx.Check(redis.Close)

	projectID := testrand.UUID()
	bucketName := "test-bucket"

	config1 := &buckets.NotificationConfig{
		TopicName: "projects/test/topics/topic1",
		Events:    []string{"s3:ObjectCreated:*"},
	}
	mockDB := &CountingBucketsDB{
		config: config1,
	}
	cfg := eventingconfig.Config{
		Cache: eventingconfig.CacheConfig{
			Address: "redis://" + redis.Addr(),
			TTL:     time.Hour,
		},
	}

	// Create cache wrapping the mock DB
	cache, err := eventing.NewConfigCache(testplanet.NewLogger(t), mockDB, cfg)
	require.NoError(t, err)
	defer ctx.Check(cache.Close)

	// Get and cache - should hit DB
	_, err = cache.GetBucketNotificationConfig(ctx, []byte(bucketName), projectID)
	require.NoError(t, err)
	require.Equal(t, 1, mockDB.callCount, "first call should query database")

	// Update the DB config
	config2 := &buckets.NotificationConfig{
		TopicName: "projects/test/topics/topic2",
		Events:    []string{"s3:ObjectRemoved:*"},
	}
	mockDB.config = config2

	// Should still get old config from cache - no DB hit
	cached, err := cache.GetBucketNotificationConfig(ctx, []byte(bucketName), projectID)
	require.NoError(t, err)
	require.Equal(t, "projects/test/topics/topic1", cached.TopicName)
	require.Equal(t, 1, mockDB.callCount, "cached call should not query database")

	// Invalidate cache
	err = cache.Invalidate(ctx, projectID, bucketName)
	require.NoError(t, err)

	// Should now get new config from DB - should hit DB again
	updated, err := cache.GetBucketNotificationConfig(ctx, []byte(bucketName), projectID)
	require.NoError(t, err)
	require.Equal(t, "projects/test/topics/topic2", updated.TopicName)
	require.Equal(t, 2, mockDB.callCount, "call after invalidation should query database")
}

func TestConfigCache_Ping(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	t.Run("valid connection", func(t *testing.T) {
		redis, err := testredis.Mini(ctx)
		require.NoError(t, err)
		defer ctx.Check(redis.Close)

		cache, err := eventing.NewConfigCache(
			testplanet.NewLogger(t),
			&CountingBucketsDB{},
			eventingconfig.Config{
				Cache: eventingconfig.CacheConfig{
					Address: "redis://" + redis.Addr(),
					TTL:     time.Hour,
				},
			})
		require.NoError(t, err)
		defer ctx.Check(cache.Close)

		err = cache.Ping(ctx)
		require.NoError(t, err)
	})
}

func TestConfigCache_NewConfigCache(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	redis, err := testredis.Mini(ctx)
	require.NoError(t, err)
	defer ctx.Check(redis.Close)

	mockDB := &CountingBucketsDB{}

	t.Run("empty Redis address", func(t *testing.T) {
		cache, err := eventing.NewConfigCache(testplanet.NewLogger(t), mockDB, eventingconfig.Config{
			Cache: eventingconfig.CacheConfig{
				Address: "",
				TTL:     time.Hour,
			},
		})
		require.NoError(t, err)
		require.Nil(t, cache)
	})

	t.Run("invalid Redis URL", func(t *testing.T) {
		_, err := eventing.NewConfigCache(testplanet.NewLogger(t), mockDB, eventingconfig.Config{
			Cache: eventingconfig.CacheConfig{
				Address: "not-a-valid-url",
				TTL:     time.Hour,
			},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid Redis URL")
	})

	t.Run("zero TTL", func(t *testing.T) {
		_, err := eventing.NewConfigCache(testplanet.NewLogger(t), mockDB, eventingconfig.Config{
			Cache: eventingconfig.CacheConfig{
				Address: "redis://" + redis.Addr(),
				TTL:     0,
			},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "TTL must be positive")
	})

	t.Run("negative TTL", func(t *testing.T) {
		_, err := eventing.NewConfigCache(testplanet.NewLogger(t), mockDB, eventingconfig.Config{
			Cache: eventingconfig.CacheConfig{
				Address: "redis://" + redis.Addr(),
				TTL:     -1 * time.Hour,
			},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "TTL must be positive")
	})

	t.Run("valid Redis URL", func(t *testing.T) {
		cache, err := eventing.NewConfigCache(testplanet.NewLogger(t), mockDB, eventingconfig.Config{
			Cache: eventingconfig.CacheConfig{
				Address: "redis://" + redis.Addr(),
				TTL:     time.Hour,
			},
		})
		require.NoError(t, err)
		require.NotNil(t, cache)
		require.NoError(t, cache.Close())
	})
}

func TestConfigCache_CacheKey(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	redis, err := testredis.Mini(ctx)
	require.NoError(t, err)
	defer ctx.Check(redis.Close)

	projectID1 := uuid.UUID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10}
	projectID2 := uuid.UUID{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20}

	config1 := &buckets.NotificationConfig{TopicName: "topic1"}
	config2 := &buckets.NotificationConfig{TopicName: "topic2"}

	// Use a shared DB that we can modify to test cache isolation
	sharedDB := &CountingBucketsDB{}

	cacheConfig := eventingconfig.Config{
		Cache: eventingconfig.CacheConfig{
			Address: "redis://" + redis.Addr(),
			TTL:     time.Hour,
		},
	}

	cache, err := eventing.NewConfigCache(testplanet.NewLogger(t), sharedDB, cacheConfig)
	require.NoError(t, err)
	defer ctx.Check(cache.Close)

	// Cache config for project1/bucket1
	sharedDB.config = config1
	result1, err := cache.GetBucketNotificationConfig(ctx, []byte("bucket1"), projectID1)
	require.NoError(t, err)
	assert.Equal(t, "topic1", result1.TopicName)

	// Cache config for project2/bucket1 (different project, same bucket name)
	sharedDB.config = config2
	result2, err := cache.GetBucketNotificationConfig(ctx, []byte("bucket1"), projectID2)
	require.NoError(t, err)
	assert.Equal(t, "topic2", result2.TopicName)

	// Verify cached values are correct by reading from cache (not DB)
	// Change DB to return different value to prove we're reading from cache
	sharedDB.config = &buckets.NotificationConfig{TopicName: "wrong"}

	cached1, err := cache.GetBucketNotificationConfig(ctx, []byte("bucket1"), projectID1)
	require.NoError(t, err)
	assert.Equal(t, "topic1", cached1.TopicName, "should read topic1 from cache")

	cached2, err := cache.GetBucketNotificationConfig(ctx, []byte("bucket1"), projectID2)
	require.NoError(t, err)
	assert.Equal(t, "topic2", cached2.TopicName, "should read topic2 from cache")
}

func TestConfigCache_RedisFailure(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	redis, err := testredis.Mini(ctx)
	require.NoError(t, err)

	projectID := testrand.UUID()
	bucketName := "test-bucket"

	config := &buckets.NotificationConfig{
		TopicName: "projects/test/topics/test",
		Events:    []string{"s3:ObjectCreated:*"},
	}
	mockDB := &CountingBucketsDB{
		config: config,
	}
	cacheConfig := eventingconfig.Config{
		Cache: eventingconfig.CacheConfig{
			Address: "redis://" + redis.Addr(),
			TTL:     time.Hour,
		},
	}

	cache, err := eventing.NewConfigCache(testplanet.NewLogger(t), mockDB, cacheConfig)
	require.NoError(t, err)
	defer ctx.Check(cache.Close)

	// Stop Redis to simulate failure
	require.NoError(t, redis.Close())

	// Should still work by falling back to database
	result, err := cache.GetBucketNotificationConfig(ctx, []byte(bucketName), projectID)
	require.NoError(t, err)
	assert.Equal(t, config, result)
	assert.Equal(t, 1, mockDB.callCount, "should query database when Redis fails")

	// Second call should also hit DB (no working cache)
	result2, err := cache.GetBucketNotificationConfig(ctx, []byte(bucketName), projectID)
	require.NoError(t, err)
	assert.Equal(t, config, result2)
	assert.Equal(t, 2, mockDB.callCount, "should query database again when Redis fails")
}

func TestConfigCache_TTL(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	redis, err := testredis.Mini(ctx)
	require.NoError(t, err)
	defer ctx.Check(redis.Close)

	projectID := testrand.UUID()
	bucketName := "test-bucket"

	config1 := &buckets.NotificationConfig{TopicName: "topic1"}
	mockDB := &CountingBucketsDB{
		config: config1,
	}
	cacheConfig := eventingconfig.Config{
		Cache: eventingconfig.CacheConfig{
			Address: "redis://" + redis.Addr(),
			TTL:     100 * time.Millisecond,
		},
	}

	// Create cache with very short TTL
	cache, err := eventing.NewConfigCache(testplanet.NewLogger(t), mockDB, cacheConfig)
	require.NoError(t, err)
	defer ctx.Check(cache.Close)

	// Cache the config - should hit DB
	_, err = cache.GetBucketNotificationConfig(ctx, []byte(bucketName), projectID)
	require.NoError(t, err)
	assert.Equal(t, 1, mockDB.callCount, "first call should query database")

	// Update the DB config
	config2 := &buckets.NotificationConfig{TopicName: "topic2"}
	mockDB.config = config2

	// Fast-forward time in redis
	redis.FastForward(200 * time.Millisecond)

	// Should get new config from DB after TTL expires - should hit DB again
	result, err := cache.GetBucketNotificationConfig(ctx, []byte(bucketName), projectID)
	require.NoError(t, err)
	assert.Equal(t, "topic2", result.TopicName)
	assert.Equal(t, 2, mockDB.callCount, "call after TTL expiration should query database")
}
