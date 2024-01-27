// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package redis

import (
	"bytes"
	"context"
	"errors"
	"net/url"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/storj/private/kvstore"
)

var (
	// Error is a redis error.
	Error = errs.Class("redis")

	mon = monkit.Package()
)

// TODO(coyle): this should be set to 61 * time.Minute after we implement Ping and Refresh on Overlay.
// This disables the TTL since the Set command only includes a TTL if it is greater than 0.
const defaultNodeExpiration = 0 * time.Minute

// Client is the entrypoint into Redis.
type Client struct {
	db  *redis.Client
	TTL time.Duration
}

// OpenClient returns a configured Client instance, verifying a successful connection to redis.
func OpenClient(ctx context.Context, address, password string, db int) (*Client, error) {
	client := &Client{
		db: redis.NewClient(&redis.Options{
			Addr:     address,
			Password: password,
			DB:       db,
		}),
		TTL: defaultNodeExpiration,
	}

	// ping here to verify we are able to connect to redis with the initialized client.
	if err := client.db.Ping(ctx).Err(); err != nil {
		return nil, Error.New("ping failed: %v", err)
	}

	return client, nil
}

// OpenClientFrom returns a configured Client instance from a redis address, verifying a successful connection to redis.
func OpenClientFrom(ctx context.Context, address string) (*Client, error) {
	redisurl, err := url.Parse(address)
	if err != nil {
		return nil, err
	}

	if redisurl.Scheme != "redis" {
		return nil, Error.New("not a redis:// formatted address")
	}

	q := redisurl.Query()

	db, err := strconv.Atoi(q.Get("db"))
	if err != nil {
		return nil, err
	}

	return OpenClient(ctx, redisurl.Host, q.Get("password"), db)
}

// Get looks up the provided key from redis returning either an error or the result.
func (client *Client) Get(ctx context.Context, key kvstore.Key) (_ kvstore.Value, err error) {
	defer mon.Task()(&ctx)(&err)
	if key.IsZero() {
		return nil, kvstore.ErrEmptyKey.New("")
	}
	return get(ctx, client.db, key)
}

// Put adds a value to the provided key in redis, returning an error on failure.
func (client *Client) Put(ctx context.Context, key kvstore.Key, value kvstore.Value) (err error) {
	defer mon.Task()(&ctx)(&err)
	if key.IsZero() {
		return kvstore.ErrEmptyKey.New("")
	}
	return put(ctx, client.db, key, value, client.TTL)
}

// IncrBy increments the value stored in key by the specified value.
func (client *Client) IncrBy(ctx context.Context, key kvstore.Key, value int64) (err error) {
	defer mon.Task()(&ctx)(&err)
	if key.IsZero() {
		return kvstore.ErrEmptyKey.New("")
	}
	_, err = client.db.IncrBy(ctx, key.String(), value).Result()
	return err
}

// Eval evaluates a Lua 5.1 script on Redis Server.
// This arguments can be accessed by Lua using the KEYS global variable
// in the form of a one-based array (so KEYS[1], KEYS[2], ...).
func (client *Client) Eval(ctx context.Context, script string, keys []string) (err error) {
	return eval(ctx, client.db, script, keys)
}

// Delete deletes a key/value pair from redis, for a given the key.
func (client *Client) Delete(ctx context.Context, key kvstore.Key) (err error) {
	defer mon.Task()(&ctx)(&err)
	if key.IsZero() {
		return kvstore.ErrEmptyKey.New("")
	}
	return delete(ctx, client.db, key)
}

// FlushDB deletes all keys in the currently selected DB.
func (client *Client) FlushDB(ctx context.Context) error {
	_, err := client.db.FlushDB(ctx).Result()
	return err
}

// Close closes a redis client.
func (client *Client) Close() error {
	return client.db.Close()
}

// Range iterates over all items in unspecified order.
func (client *Client) Range(ctx context.Context, fn func(context.Context, kvstore.Key, kvstore.Value) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	it := client.db.Scan(ctx, 0, "", 0).Iterator()

	var lastKey string
	var lastOk bool
	for it.Next(ctx) {
		key := it.Val()
		// redis may return duplicates
		if lastOk && key == lastKey {
			continue
		}
		lastKey, lastOk = key, true

		value, err := get(ctx, client.db, kvstore.Key(key))
		if err != nil {
			return Error.Wrap(err)
		}

		if err := fn(ctx, kvstore.Key(key), value); err != nil {
			return err
		}
	}

	return Error.Wrap(it.Err())
}

// CompareAndSwap atomically compares and swaps oldValue with newValue.
func (client *Client) CompareAndSwap(ctx context.Context, key kvstore.Key, oldValue, newValue kvstore.Value) (err error) {
	defer mon.Task()(&ctx)(&err)
	if key.IsZero() {
		return kvstore.ErrEmptyKey.New("")
	}

	txf := func(tx *redis.Tx) error {
		value, err := get(ctx, tx, key)
		if kvstore.ErrKeyNotFound.Has(err) {
			if oldValue != nil {
				return kvstore.ErrKeyNotFound.New("%q", key)
			}

			if newValue == nil {
				return nil
			}

			// runs only if the watched keys remain unchanged
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				return put(ctx, pipe, key, newValue, client.TTL)
			})
			return err
		}
		if err != nil {
			return err
		}

		if !bytes.Equal(value, oldValue) {
			return kvstore.ErrValueChanged.New("%q", key)
		}

		// runs only if the watched keys remain unchanged
		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			if newValue == nil {
				return delete(ctx, pipe, key)
			}
			return put(ctx, pipe, key, newValue, client.TTL)
		})

		return err
	}

	err = client.db.Watch(ctx, txf, key.String())
	if errors.Is(err, redis.TxFailedErr) {
		return kvstore.ErrValueChanged.New("%q", key)
	}
	return Error.Wrap(err)
}

func get(ctx context.Context, cmdable redis.Cmdable, key kvstore.Key) (_ kvstore.Value, err error) {
	defer mon.Task()(&ctx)(&err)
	value, err := cmdable.Get(ctx, string(key)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, kvstore.ErrKeyNotFound.New("%q", key)
	}
	if err != nil && !errors.Is(err, redis.TxFailedErr) {
		return nil, Error.New("get error: %v", err)
	}
	return value, errs.Wrap(err)
}

func put(ctx context.Context, cmdable redis.Cmdable, key kvstore.Key, value kvstore.Value, ttl time.Duration) (err error) {
	defer mon.Task()(&ctx)(&err)
	err = cmdable.Set(ctx, key.String(), []byte(value), ttl).Err()
	if err != nil && !errors.Is(err, redis.TxFailedErr) {
		return Error.New("put error: %v", err)
	}
	return errs.Wrap(err)
}

func delete(ctx context.Context, cmdable redis.Cmdable, key kvstore.Key) (err error) {
	defer mon.Task()(&ctx)(&err)
	err = cmdable.Del(ctx, key.String()).Err()
	if err != nil && !errors.Is(err, redis.TxFailedErr) {
		return Error.New("delete error: %v", err)
	}
	return errs.Wrap(err)
}

func eval(ctx context.Context, cmdable redis.Cmdable, script string, keys []string) (err error) {
	defer mon.Task()(&ctx)(&err)
	err = cmdable.Eval(ctx, script, keys, nil).Err()
	if err != nil && !errors.Is(err, redis.TxFailedErr) {
		return Error.New("eval error: %v", err)
	}
	return errs.Wrap(err)
}
