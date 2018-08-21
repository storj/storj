// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package redis

import (
	"fmt"
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
	maxKeyLookup          = 100
)

// Client is the entrypoint into Redis
type Client struct {
	db  *redis.Client
	TTL time.Duration
}

// NewClient returns a configured Client instance, verifying a sucessful connection to redis
func NewClient(address, password string, db int) (*Client, error) {
	c := &Client{
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
func (c *Client) Get(key storage.Key) (storage.Value, error) {
	b, err := c.db.Get(string(key)).Bytes()

	if len(b) == 0 {
		return nil, storage.ErrKeyNotFound.New(key.String())
	}

	if err != nil {
		if err.Error() == "redis: nil" {
			return nil, nil
		}

		// TODO: log
		return nil, Error.New("get error", err)
	}

	return b, nil
}

// Put adds a value to the provided key in redis, returning an error on failure.
func (c *Client) Put(key storage.Key, value storage.Value) error {
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
func (c *Client) List(startingKey storage.Key, limit storage.Limit) (storage.Keys, error) {
	var noOrderKeys []string
	if startingKey != nil {
		_, cursor, err := c.db.Scan(0, fmt.Sprintf("%s", startingKey), int64(limit)).Result()
		if err != nil {
			return nil, Error.New("list error with starting key", err)
		}
		keys, _, err := c.db.Scan(cursor, "", int64(limit)).Result()
		if err != nil {
			return nil, Error.New("list error with starting key", err)
		}
		noOrderKeys = keys
	} else if startingKey == nil {
		keys, _, err := c.db.Scan(0, "", int64(limit)).Result()
		if err != nil {
			return nil, Error.New("list error without starting key", err)
		}
		noOrderKeys = keys
	}

	listKeys := make(storage.Keys, len(noOrderKeys))
	for i, k := range noOrderKeys {
		listKeys[i] = storage.Key(k)
	}

	return listKeys, nil
}

// ReverseList returns either a list of keys for which redis has values or an error.
// Starts from startingKey and iterates backwards
func (c *Client) ReverseList(startingKey storage.Key, limit storage.Limit) (storage.Keys, error) {
	//TODO
	return storage.Keys{}, nil
}

// Delete deletes a key/value pair from redis, for a given the key
func (c *Client) Delete(key storage.Key) error {
	err := c.db.Del(key.String()).Err()
	if err != nil {
		return Error.New("delete error", err)
	}

	return err
}

// Close closes a redis client
func (c *Client) Close() error {
	return c.db.Close()
}

// GetAll is the bulk method for gets from the redis data store
// The maximum keys returned will be 100. If more than that is requested an
// error will be returned
func (c *Client) GetAll(keys storage.Keys) (storage.Values, error) {
	lk := len(keys)
	if lk > maxKeyLookup {
		return nil, Error.New(fmt.Sprintf("requested %d keys, maximum is %d", lk, maxKeyLookup))
	}

	ks := make([]string, lk)
	for i, v := range keys {
		ks[i] = v.String()
	}

	vs, err := c.db.MGet(ks...).Result()
	if err != nil {
		return []storage.Value{}, err
	}

	values := []storage.Value{}
	for _, v := range vs {
		values = append(values, storage.Value([]byte(v.(string))))
	}
	return values, nil
}
