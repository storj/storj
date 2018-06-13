// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package redis

import (
	"time"

	"github.com/go-redis/redis"
	"github.com/zeebo/errs"
	"storj.io/storj/storage"
)

const defaultNodeExpiration = 61 * time.Minute

// Client is the entrypoint into Redis
type redisClient struct {
	DB  *redis.Client
	TTL time.Duration
}

// NewClient returns a configured Client instance, verifying a sucessful connection to redis
func NewClient(address, password string, db int) (storage.KeyValueStore, error) {
	c := &redisClient{
		DB: redis.NewClient(&redis.Options{
			Addr:     address,
			Password: password,
			DB:       db,
		}),
		TTL: defaultNodeExpiration,
	}

	// ping here to verify we are able to connect to the redis instacne with the initialized client.
	if err := c.DB.Ping().Err(); err != nil {
		return nil, err
	}

	return c, nil
}

// Get looks up the provided key from the redis cache returning either an error or the result.
func (c *redisClient) Get(key storage.Key) (storage.Value, error) {
	return c.DB.Get(string(key)).Bytes()
}

// Put adds a value to the provided key in the Redis cache, returning an error on failure.

func (c *redisClient) Put(key storage.Key, value storage.Value) error {
	v, err := value.MarshalBinary()

	if err != nil {
		return err
	}

	return c.DB.Set(key.String(), v, c.TTL).Err()
}

func (c *redisClient) List() (_ storage.Keys, _ error) {
	results, err := c.DB.Keys("*").Result()
	if err != nil {
		return nil, errs.Wrap(err)
	}

	keys := make(storage.Keys, len(results))
	for i, k := range results {
		keys[i] = storage.Key(k)
	}

	return keys, nil
}

func (c *redisClient) Delete(key storage.Key) error {
	return c.DB.Del(key.String()).Err()
}

func (c *redisClient) Close() error {
	return c.DB.Close()
}
