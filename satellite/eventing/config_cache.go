// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package eventing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/eventing/eventingconfig"
)

// ConfigCache provides caching for bucket notification configurations.
// It wraps buckets.DB and caches results in Redis.
type ConfigCache struct {
	log    *zap.Logger
	db     buckets.DB
	client *redis.Client
	ttl    time.Duration
}

// NewConfigCache creates a new bucket notification cache using the Redis connection URL.
// Returns nil if Redis address is not configured (cache disabled).
func NewConfigCache(log *zap.Logger, db buckets.DB, cfg eventingconfig.Config) (*ConfigCache, error) {
	if cfg.Cache.Address == "" {
		log.Info("Bucket eventing config cache disabled - no Redis address configured")
		return nil, nil
	}

	if cfg.Cache.TTL <= 0 {
		return nil, errs.New("TTL must be positive")
	}

	opts, err := redis.ParseURL(cfg.Cache.Address)
	if err != nil {
		return nil, errs.New("invalid Redis URL: %w", err)
	}

	client := redis.NewClient(opts)

	log.Info("Bucket eventing cache config enabled",
		zap.String("address", cfg.Cache.Address),
		zap.Duration("ttl", cfg.Cache.TTL),
	)

	return &ConfigCache{
		log:    log,
		db:     db,
		client: client,
		ttl:    cfg.Cache.TTL,
	}, nil
}

// Ping checks if the Redis connection is alive.
func (c *ConfigCache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Close closes the Redis client connection.
func (c *ConfigCache) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}

// GetBucketNotificationConfig retrieves a bucket notification configuration from cache or database.
// It caches the result (including nil configurations) to prevent repeated DB queries.
func (c *ConfigCache) GetBucketNotificationConfig(ctx context.Context, bucketName []byte, projectID uuid.UUID) (*buckets.NotificationConfig, error) {
	cacheKey := c.createCacheKey(projectID, string(bucketName))

	// Try to get from cache
	val, err := c.client.Get(ctx, cacheKey).Bytes()
	if err == nil {
		// Cache hit - unmarshal the configuration
		if len(val) == 0 {
			// Empty value means no configuration exists
			return nil, nil
		}

		var config buckets.NotificationConfig
		if err := json.Unmarshal(val, &config); err != nil {
			// Failed to unmarshal - corrupted cache data, log and fall through to database
			c.log.Warn("Failed to unmarshal cached notification config, falling back to database",
				zap.Stringer("project_id", projectID),
				zap.ByteString("bucket_name", bucketName),
				zap.Error(err))
			// Continue to database fallback below
		} else {
			return &config, nil
		}
	}

	if err != nil && !errors.Is(err, redis.Nil) {
		// Redis error (not a cache miss) - log and fall through to database
		// We don't want to fail the request just because Redis is down
		c.log.Warn("Redis get failed, falling back to database",
			zap.Stringer("project_id", projectID),
			zap.ByteString("bucket_name", bucketName),
			zap.Error(err))
		// Continue to database fallback below
	}

	// Cache miss - query database
	config, err := c.db.GetBucketNotificationConfig(ctx, bucketName, projectID)
	if err != nil {
		return nil, err
	}

	// Store in cache
	if err := c.set(ctx, cacheKey, config); err != nil {
		// Log error but don't fail the request - we got the data from DB
		c.log.Warn("Failed to cache notification config",
			zap.Stringer("project_id", projectID),
			zap.ByteString("bucket_name", bucketName),
			zap.Error(err))
	}

	return config, nil
}

// Invalidate removes a bucket notification configuration from the cache.
// This should be called after updating or deleting a configuration.
func (c *ConfigCache) Invalidate(ctx context.Context, projectID uuid.UUID, bucketName string) error {
	cacheKey := c.createCacheKey(projectID, bucketName)
	err := c.client.Del(ctx, cacheKey).Err()
	if err != nil {
		return errs.New("Redis del failed: %w", err)
	}

	return nil
}

// set stores a notification configuration in the cache.
// It stores nil configurations as empty values to prevent repeated DB queries.
func (c *ConfigCache) set(ctx context.Context, cacheKey string, config *buckets.NotificationConfig) error {
	var val []byte
	if config != nil {
		var err error
		val, err = json.Marshal(config)
		if err != nil {
			return errs.New("failed to marshal notification config: %w", err)
		}
	}
	// Empty val (len == 0) will cache "no configuration exists"

	err := c.client.Set(ctx, cacheKey, val, c.ttl).Err()
	if err != nil {
		return errs.New("Redis set failed: %w", err)
	}
	return nil
}

// createCacheKey generates the Redis cache key for a bucket notification configuration.
// Format: bucket-eventing:{project_id_hex}:{bucket_name}
func (c *ConfigCache) createCacheKey(projectID uuid.UUID, bucketName string) string {
	return fmt.Sprintf("bucket-eventing:%x:%s", projectID, bucketName)
}
