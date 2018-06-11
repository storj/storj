// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package redis

import (
	"time"

	"github.com/go-redis/redis"
	"storj.io/storj/pkg/overlay"
)

// Client is the entrypoint into Redis
type redisClient struct {
	DB *redis.Client
}

// NewClient returns a configured Client instance, verifying a sucessful connection to redis
func NewClient(address, password string, db int) (overlay.Client, error) {
	c := &redisClient{
		DB: redis.NewClient(&redis.Options{
			Addr:     address,
			Password: password,
			DB:       db,
		}),
	}

	// ping here to verify we are able to connect to the redis instacne with the initialized client.
	if err := c.DB.Ping().Err(); err != nil {
		return nil, err
	}

	return c, nil
}

// Get looks up the provided key from the redis cache returning either an error or the result.
func (c *redisClient) Get(key string) ([]byte, error) {
	return c.DB.Get(key).Bytes()
}

// Set adds a value to the provided key in the Redis cache, returning an error on failure.

func (c *redisClient) Set(key string, value []byte, ttl time.Duration) error {
	return c.DB.Set(key, value, ttl).Err()
}

// Ping returns an error if pinging the underlying redis server failed
func (c *redisClient) Ping() error {
	return c.DB.Ping().Err()
}
