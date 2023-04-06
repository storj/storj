// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package redis

import (
	"bytes"
	"context"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/storj/storage"
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

	lookupLimit int
}

// OpenClient returns a configured Client instance, verifying a successful connection to redis.
func OpenClient(ctx context.Context, address, password string, db int) (*Client, error) {
	client := &Client{
		db: redis.NewClient(&redis.Options{
			Addr:     address,
			Password: password,
			DB:       db,
		}),
		TTL:         defaultNodeExpiration,
		lookupLimit: storage.DefaultLookupLimit,
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

// SetLookupLimit sets the lookup limit.
func (client *Client) SetLookupLimit(v int) { client.lookupLimit = v }

// LookupLimit returns the maximum limit that is allowed.
func (client *Client) LookupLimit() int { return client.lookupLimit }

// Get looks up the provided key from redis returning either an error or the result.
func (client *Client) Get(ctx context.Context, key storage.Key) (_ storage.Value, err error) {
	defer mon.Task()(&ctx)(&err)
	if key.IsZero() {
		return nil, storage.ErrEmptyKey.New("")
	}
	return get(ctx, client.db, key)
}

// Put adds a value to the provided key in redis, returning an error on failure.
func (client *Client) Put(ctx context.Context, key storage.Key, value storage.Value) (err error) {
	defer mon.Task()(&ctx)(&err)
	if key.IsZero() {
		return storage.ErrEmptyKey.New("")
	}
	return put(ctx, client.db, key, value, client.TTL)
}

// IncrBy increments the value stored in key by the specified value.
func (client *Client) IncrBy(ctx context.Context, key storage.Key, value int64) (err error) {
	defer mon.Task()(&ctx)(&err)
	if key.IsZero() {
		return storage.ErrEmptyKey.New("")
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

// List returns either a list of keys for which boltdb has values or an error.
func (client *Client) List(ctx context.Context, first storage.Key, limit int) (_ storage.Keys, err error) {
	defer mon.Task()(&ctx)(&err)
	return storage.ListKeys(ctx, client, first, limit)
}

// Delete deletes a key/value pair from redis, for a given the key.
func (client *Client) Delete(ctx context.Context, key storage.Key) (err error) {
	defer mon.Task()(&ctx)(&err)
	if key.IsZero() {
		return storage.ErrEmptyKey.New("")
	}
	return delete(ctx, client.db, key)
}

// DeleteMultiple deletes keys ignoring missing keys.
func (client *Client) DeleteMultiple(ctx context.Context, keys []storage.Key) (_ storage.Items, err error) {
	defer mon.Task()(&ctx, len(keys))(&err)
	return deleteMultiple(ctx, client.db, keys)
}

// Close closes a redis client.
func (client *Client) Close() error {
	return client.db.Close()
}

// GetAll is the bulk method for gets from the redis data store.
// The maximum keys returned will be LookupLimit. If more than that
// is requested, an error will be returned.
func (client *Client) GetAll(ctx context.Context, keys storage.Keys) (_ storage.Values, err error) {
	defer mon.Task()(&ctx)(&err)
	if len(keys) == 0 {
		return nil, nil
	}
	if len(keys) > client.lookupLimit {
		return nil, storage.ErrLimitExceeded.New("lookup limit exceeded")
	}

	keyStrings := make([]string, len(keys))
	for i, v := range keys {
		keyStrings[i] = v.String()
	}

	results, err := client.db.MGet(ctx, keyStrings...).Result()
	if err != nil {
		return nil, err
	}

	values := []storage.Value{}
	for _, result := range results {
		if result == nil {
			values = append(values, nil)
		} else {
			s, ok := result.(string)
			if !ok {
				return nil, Error.New("invalid result type %T", result)
			}
			values = append(values, storage.Value(s))
		}
	}
	return values, nil
}

// Range iterates over all items in unspecified order.
func (client *Client) Range(ctx context.Context, fn func(context.Context, storage.Key, storage.Value) error) (err error) {
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

		value, err := get(ctx, client.db, storage.Key(key))
		if err != nil {
			return Error.Wrap(err)
		}

		if err := fn(ctx, storage.Key(key), value); err != nil {
			return err
		}
	}

	return Error.Wrap(it.Err())
}

// Iterate iterates over items based on opts.
func (client *Client) Iterate(ctx context.Context, opts storage.IterateOptions, fn func(context.Context, storage.Iterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	if opts.Limit <= 0 || opts.Limit > client.lookupLimit {
		opts.Limit = client.lookupLimit
	}
	return client.IterateWithoutLookupLimit(ctx, opts, fn)
}

// IterateWithoutLookupLimit calls the callback with an iterator over the keys, but doesn't enforce default limit on opts.
func (client *Client) IterateWithoutLookupLimit(ctx context.Context, opts storage.IterateOptions, fn func(context.Context, storage.Iterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	all, err := client.allPrefixedItems(ctx, opts.Prefix, opts.First, nil, opts.Limit)
	if err != nil {
		return err
	}

	if !opts.Recurse {
		all = sortAndCollapse(all, opts.Prefix)
	}

	return fn(ctx, &StaticIterator{
		Items: all,
	})
}

// FlushDB deletes all keys in the currently selected DB.
func (client *Client) FlushDB(ctx context.Context) error {
	_, err := client.db.FlushDB(ctx).Result()
	return err
}

func (client *Client) allPrefixedItems(ctx context.Context, prefix, first, last storage.Key, limit int) (storage.Items, error) {
	var all storage.Items
	seen := map[string]struct{}{}

	match := string(escapeMatch([]byte(prefix))) + "*"
	it := client.db.Scan(ctx, 0, match, 0).Iterator()
	for it.Next(ctx) {
		key := it.Val()
		if !first.IsZero() && storage.Key(key).Less(first) {
			continue
		}
		if !last.IsZero() && last.Less(storage.Key(key)) {
			continue
		}

		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		value, err := client.db.Get(ctx, key).Bytes()
		if err != nil {
			return nil, err
		}

		all = append(all, storage.ListItem{
			Key:      storage.Key(key),
			Value:    storage.Value(value),
			IsPrefix: false,
		})
	}

	sort.Sort(all)

	return all, nil
}

// CompareAndSwap atomically compares and swaps oldValue with newValue.
func (client *Client) CompareAndSwap(ctx context.Context, key storage.Key, oldValue, newValue storage.Value) (err error) {
	defer mon.Task()(&ctx)(&err)
	if key.IsZero() {
		return storage.ErrEmptyKey.New("")
	}

	txf := func(tx *redis.Tx) error {
		value, err := get(ctx, tx, key)
		if storage.ErrKeyNotFound.Has(err) {
			if oldValue != nil {
				return storage.ErrKeyNotFound.New("%q", key)
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
			return storage.ErrValueChanged.New("%q", key)
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
		return storage.ErrValueChanged.New("%q", key)
	}
	return Error.Wrap(err)
}

func get(ctx context.Context, cmdable redis.Cmdable, key storage.Key) (_ storage.Value, err error) {
	defer mon.Task()(&ctx)(&err)
	value, err := cmdable.Get(ctx, string(key)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, storage.ErrKeyNotFound.New("%q", key)
	}
	if err != nil && !errors.Is(err, redis.TxFailedErr) {
		return nil, Error.New("get error: %v", err)
	}
	return value, errs.Wrap(err)
}

func put(ctx context.Context, cmdable redis.Cmdable, key storage.Key, value storage.Value, ttl time.Duration) (err error) {
	defer mon.Task()(&ctx)(&err)
	err = cmdable.Set(ctx, key.String(), []byte(value), ttl).Err()
	if err != nil && !errors.Is(err, redis.TxFailedErr) {
		return Error.New("put error: %v", err)
	}
	return errs.Wrap(err)
}

func delete(ctx context.Context, cmdable redis.Cmdable, key storage.Key) (err error) {
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

func deleteMultiple(ctx context.Context, cmdable redis.Cmdable, keys []storage.Key) (_ storage.Items, err error) {
	defer mon.Task()(&ctx, len(keys))(&err)

	var items storage.Items
	for _, key := range keys {
		value, err := get(ctx, cmdable, key)
		if err != nil {
			if errors.Is(err, redis.Nil) || storage.ErrKeyNotFound.Has(err) {
				continue
			}
			return items, err
		}

		err = delete(ctx, cmdable, key)
		if err != nil {
			if errors.Is(err, redis.Nil) || storage.ErrKeyNotFound.Has(err) {
				continue
			}
			return items, err
		}
		items = append(items, storage.ListItem{
			Key:   key,
			Value: value,
		})
	}

	return items, nil
}
