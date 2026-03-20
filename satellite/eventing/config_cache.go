// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package eventing

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/eventing/eventingconfig"
	"storj.io/storj/shared/lrucache"
)

// ConfigCache provides in-memory caching for bucket notification configurations.
// Configuration changes may take up to the configured TTL to propagate across all pods.
type ConfigCache struct {
	db       buckets.DB
	positive *lrucache.ExpiringLRUOf[buckets.NotificationConfig]
	negative *lrucache.ExpiringLRUOf[struct{}]
}

// NewConfigCache creates a new in-memory bucket notification config cache.
func NewConfigCache(db buckets.DB, cfg eventingconfig.Config) (*ConfigCache, error) {
	if cfg.Cache.TTL <= 0 {
		return nil, errs.New("TTL must be positive")
	}
	if cfg.Cache.Capacity <= 0 {
		return nil, errs.New("Capacity must be positive")
	}

	return &ConfigCache{
		db: db,
		positive: lrucache.NewOf[buckets.NotificationConfig](lrucache.Options{
			Expiration: cfg.Cache.TTL,
			Capacity:   cfg.Cache.Capacity,
			Name:       "bucket-eventing-positive",
		}),
		negative: lrucache.NewOf[struct{}](lrucache.Options{
			Expiration: cfg.Cache.TTL,
			Capacity:   cfg.Cache.Capacity,
			Name:       "bucket-eventing-negative",
		}),
	}, nil
}

// GetBucketNotificationConfig retrieves a bucket notification configuration from
// the in-memory cache, falling back to the database on a cache miss.
func (c *ConfigCache) GetBucketNotificationConfig(ctx context.Context, bucketName []byte, projectID uuid.UUID) (_ *buckets.NotificationConfig, err error) {
	defer mon.Task()(&ctx)(&err)

	key := projectID.String() + "/" + string(bucketName)

	if config, ok := c.positive.GetCached(ctx, key); ok {
		return &config, nil
	}

	if _, ok := c.negative.GetCached(ctx, key); ok {
		return nil, nil
	}

	// Cache miss — query database.
	config, err := c.db.GetBucketNotificationConfig(ctx, bucketName, projectID)
	if err != nil {
		return nil, err
	}

	if config != nil {
		c.positive.Add(ctx, key, *config)
	} else {
		c.negative.Add(ctx, key, struct{}{})
	}

	return config, nil
}
