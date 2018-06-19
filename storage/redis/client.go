// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package redis

import (
	"time"

	"github.com/go-redis/redis"
	"github.com/zeebo/errs"
	"storj.io/storj/storage"
)

var (
	// Error is a redis error
	Error = errs.Class("redis error")
)

const (
	defaultNodeExpiration = 61 * time.Minute
)

// redisClient is the entrypoint into Redis
type redisClient struct {
	db  *redis.Client
	TTL time.Duration
}

// NewClient returns a configured Client instance, verifying a sucessful connection to redis
func NewClient(address, password string, db int) (storage.KeyValueStore, error) {
	c := &redisClient{
		db: redis.NewClient(&redis.Options{
			Addr:     address,
			Password: password,
			DB:       db,
		}),
		TTL: defaultNodeExpiration,
	}

	// ping here to verify we are able to connect to redis with the initialized client.
	if err := c.db.Ping().Err(); err != nil {
		return nil, Error.New("ping failed", err)
	}

	return c, nil
}

// Get looks up the provided key from redis returning either an error or the result.
func (c *redisClient) Get(key storage.Key) (storage.Value, error) {
	b, err := c.db.Get(string(key)).Bytes()
	if err != nil {
		return nil, Error.New("get error", err)
	}

	return b, nil
}

// Put adds a value to the provided key in redis, returning an error on failure.
func (c *redisClient) Put(key storage.Key, value storage.Value) error {
	v, err := value.MarshalBinary()

	if err != nil {
		return Error.New("put error", err)
	}

	err = c.db.Set(key.String(), v, c.TTL).Err()
	if err != nil {
		return Error.New("put error", err)
	}

	return nil
}

// List returns either a list of keys for which boltdb has values or an error.
func (c *redisClient) List() (_ storage.Keys, _ error) {
	results, err := c.db.Keys("*").Result()
	if err != nil {
		return nil, Error.New("list error", err)
	}

	keys := make(storage.Keys, len(results))
	for i, k := range results {
		keys[i] = storage.Key(k)
	}

	return keys, nil
}

// Delete deletes a key/value pair from redis, for a given the key
func (c *redisClient) Delete(key storage.Key) error {
	err := c.db.Del(key.String()).Err()
	if err != nil {
		return Error.New("delete error", err)
	}

	return err
}

// Close closes a redis client
func (c *redisClient) Close() error {
	return c.db.Close()
}
