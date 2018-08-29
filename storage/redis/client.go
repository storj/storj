// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package redis

import (
	"bytes"
	"fmt"
	"sort"
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

// NewClient returns a configured Client instance, verifying a successful connection to redis
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
		return nil, Error.New("ping failed: %v", err)
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
		return nil, Error.New("get error: %v", err)
	}

	return b, nil
}

// Put adds a value to the provided key in redis, returning an error on failure.
func (c *Client) Put(key storage.Key, value storage.Value) error {
	if key == nil {
		return Error.New("invalid key")
	}

	v, err := value.MarshalBinary()

	if err != nil {
		return Error.New("put error: %v", err)
	}

	err = c.db.Set(key.String(), v, c.TTL).Err()
	if err != nil {
		return Error.New("put error: %v", err)
	}

	return nil
}

// List returns either a list of keys for which boltdb has values or an error.
func (c *Client) List(startingKey storage.Key, limit storage.Limit) (storage.Keys, error) {
	var noOrderKeys []string
	if startingKey != nil {
		_, cursor, err := c.db.Scan(0, startingKey.String(), int64(limit)).Result()
		if err != nil {
			return nil, Error.New("list error with starting key: %v", err)
		}
		keys, _, err := c.db.Scan(cursor, "", int64(limit)).Result()
		if err != nil {
			return nil, Error.New("list error with starting key: %v", err)
		}
		noOrderKeys = keys
	} else if startingKey == nil {
		keys, _, err := c.db.Scan(0, "", int64(limit)).Result()
		if err != nil {
			return nil, Error.New("list error without starting key: %v", err)
		}
		noOrderKeys = keys
	}

	listKeys := make(storage.Keys, len(noOrderKeys))
	for i, k := range noOrderKeys {
		listKeys[i] = storage.Key(k)
	}

	return listKeys, nil
}

//ListV2 is the new definition and will replace `List` definition
func (c *Client) ListV2(opts storage.ListOptions) (storage.Items, storage.More, error) {
	//TODO write the implementation
	panic("to do")
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
		return Error.New("delete error: %v", err)
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

func (store *Client) Iterate(prefix, after storage.Key, delimiter byte) storage.Iterator {
	var uncollapsedItems storage.Items
	// match := strings.Replace(string(prefix), "*", "\\*", -1) + "*"
	it := store.db.Scan(0, "", 0).Iterator()
	for it.Next() {
		key := it.Val()
		if prefix != nil && !bytes.HasPrefix([]byte(key), prefix) {
			continue
		}
		if !after.Less(storage.Key(key)) {
			continue
		}

		value, err := store.db.Get(key).Bytes()
		if err != nil {
			return &staticIterator{err: err}
		}

		uncollapsedItems = append(uncollapsedItems, storage.ListItem{
			Key:      storage.Key(key),
			Value:    storage.Value(value),
			IsPrefix: false,
		})
	}

	sort.Sort(uncollapsedItems)

	var items storage.Items

	var dirPrefix []byte
	var isPrefix bool
	for _, item := range uncollapsedItems {
		if isPrefix {
			if bytes.HasPrefix(item.Key, dirPrefix) {
				continue
			}
			isPrefix = false
		}

		if p := bytes.IndexByte(item.Key[len(prefix):], delimiter); p >= 0 {
			dirPrefix = append(dirPrefix[:0], item.Key[:len(prefix)+p+1]...)
			isPrefix = true
			items = append(items, storage.ListItem{
				Key:      storage.CloneKey(storage.Key(dirPrefix)),
				IsPrefix: true,
			})
		} else {
			items = append(items, item)
		}
	}

	return &staticIterator{
		items: items,
	}
}

type staticIterator struct {
	err   error
	items storage.Items
	next  int
}

func (it *staticIterator) Next(item *storage.ListItem) bool {
	if it.next >= len(it.items) {
		return false
	}
	*item = it.items[it.next]
	it.next++
	return true
}

func (it *staticIterator) cleanup() {
	it.items = nil
}

func (it *staticIterator) Close() error {
	it.cleanup()
	return it.err
}
