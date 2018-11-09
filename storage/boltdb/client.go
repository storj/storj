// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package boltdb

import (
	"bytes"
	"sync/atomic"
	"time"

	"github.com/boltdb/bolt"

	"storj.io/storj/pkg/utils"
	"storj.io/storj/storage"
)

// Client is the entrypoint into a bolt data store
type Client struct {
	db     *bolt.DB
	Path   string
	Bucket []byte

	referenceCount *int32
}

const (
	// fileMode sets permissions so owner can read and write
	fileMode       = 0600
	defaultTimeout = 1 * time.Second
)

// New instantiates a new BoltDB client given db file path, and a bucket name
func New(path, bucket string) (*Client, error) {
	db, err := bolt.Open(path, fileMode, &bolt.Options{Timeout: defaultTimeout})
	if err != nil {
		return nil, err
	}

	err = db.Update(func(tx *bolt.Tx) error {
		_, err = tx.CreateBucketIfNotExists([]byte(bucket))
		return err
	})

	if err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, utils.CombineErrors(err, closeErr)
		}
		return nil, err
	}

	refCount := new(int32)
	*refCount = 1

	return &Client{
		db:             db,
		referenceCount: refCount,
		Path:           path,
		Bucket:         []byte(bucket),
	}, nil
}

// NewShared instantiates a new BoltDB with multiple buckets
func NewShared(path string, buckets ...string) ([]*Client, error) {
	db, err := bolt.Open(path, fileMode, &bolt.Options{Timeout: defaultTimeout})
	if err != nil {
		return nil, err
	}

	err = db.Update(func(tx *bolt.Tx) error {
		for _, bucket := range buckets {
			_, err := tx.CreateBucketIfNotExists([]byte(bucket))
			if err != nil {
				return err
			}
		}
		return err
	})

	if err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, utils.CombineErrors(err, closeErr)
		}
		return nil, err
	}

	refCount := new(int32)
	*refCount = int32(len(buckets))

	clients := []*Client{}
	for _, bucket := range buckets {
		clients = append(clients, &Client{
			db:             db,
			referenceCount: refCount,
			Path:           path,
			Bucket:         []byte(bucket),
		})
	}

	return clients, nil
}

func (client *Client) update(fn func(*bolt.Bucket) error) error {
	return client.db.Update(func(tx *bolt.Tx) error {
		return fn(tx.Bucket(client.Bucket))
	})
}

func (client *Client) view(fn func(*bolt.Bucket) error) error {
	return client.db.View(func(tx *bolt.Tx) error {
		return fn(tx.Bucket(client.Bucket))
	})
}

// Put adds a value to the provided key in boltdb, returning an error on failure.
func (client *Client) Put(key storage.Key, value storage.Value) error {
	if len(key) == 0 {
		return Error.New("invalid key")
	}
	return client.update(func(bucket *bolt.Bucket) error {
		return bucket.Put(key, value)
	})
}

// Get looks up the provided key from boltdb returning either an error or the result.
func (client *Client) Get(key storage.Key) (storage.Value, error) {
	var value storage.Value
	err := client.view(func(bucket *bolt.Bucket) error {
		data := bucket.Get([]byte(key))
		if len(data) == 0 {
			return storage.ErrKeyNotFound.New(key.String())
		}
		value = storage.CloneValue(storage.Value(data))
		return nil
	})
	return value, err
}

// Delete deletes a key/value pair from boltdb, for a given the key
func (client *Client) Delete(key storage.Key) error {
	return client.update(func(bucket *bolt.Bucket) error {
		return bucket.Delete(key)
	})
}

// List returns either a list of keys for which boltdb has values or an error.
func (client *Client) List(first storage.Key, limit int) (storage.Keys, error) {
	return storage.ListKeys(client, first, limit)
}

// ReverseList returns either a list of keys for which boltdb has values or an error.
// Starts from first and iterates backwards
func (client *Client) ReverseList(first storage.Key, limit int) (storage.Keys, error) {
	return storage.ReverseListKeys(client, first, limit)
}

// Close closes a BoltDB client
func (client *Client) Close() error {
	if atomic.AddInt32(client.referenceCount, -1) == 0 {
		return client.db.Close()
	}
	return nil
}

// GetAll finds all values for the provided keys (up to storage.LookupLimit).
// If more keys are provided than the maximum, an error will be returned.
func (client *Client) GetAll(keys storage.Keys) (storage.Values, error) {
	if len(keys) > storage.LookupLimit {
		return nil, storage.ErrLimitExceeded
	}

	vals := make(storage.Values, 0, len(keys))
	err := client.view(func(bucket *bolt.Bucket) error {
		for _, key := range keys {
			val := bucket.Get([]byte(key))
			if val == nil {
				vals = append(vals, nil)
				continue
			}
			vals = append(vals, storage.CloneValue(storage.Value(val)))
		}
		return nil
	})
	return vals, err
}

// Iterate iterates over items based on opts
func (client *Client) Iterate(opts storage.IterateOptions, fn func(storage.Iterator) error) error {
	return client.view(func(bucket *bolt.Bucket) error {
		var cursor advancer
		if !opts.Reverse {
			cursor = forward{bucket.Cursor()}
		} else {
			cursor = backward{bucket.Cursor()}
		}

		start := true
		lastPrefix := []byte{}
		wasPrefix := false

		return fn(storage.IteratorFunc(func(item *storage.ListItem) bool {
			var key, value []byte
			if start {
				key, value = cursor.PositionToFirst(opts.Prefix, opts.First)
				start = false
			} else {
				key, value = cursor.Advance()
			}

			if !opts.Recurse {
				// when non-recursive skip all items that have the same prefix
				if wasPrefix && bytes.HasPrefix(key, lastPrefix) {
					key, value = cursor.SkipPrefix(lastPrefix)
					wasPrefix = false
				}
			}

			if len(key) == 0 || !bytes.HasPrefix(key, opts.Prefix) {
				return false
			}

			if !opts.Recurse {
				// check whether the entry is a proper prefix
				if p := bytes.IndexByte(key[len(opts.Prefix):], storage.Delimiter); p >= 0 {
					key = key[:len(opts.Prefix)+p+1]
					lastPrefix = append(lastPrefix[:0], key...)

					item.Key = append(item.Key[:0], storage.Key(lastPrefix)...)
					item.Value = item.Value[:0]
					item.IsPrefix = true

					wasPrefix = true
					return true
				}
			}

			item.Key = append(item.Key[:0], storage.Key(key)...)
			item.Value = append(item.Value[:0], storage.Value(value)...)
			item.IsPrefix = false

			return true
		}))
	})
}

type advancer interface {
	PositionToFirst(prefix, first storage.Key) (key, value []byte)
	SkipPrefix(prefix storage.Key) (key, value []byte)
	Advance() (key, value []byte)
}

type forward struct {
	*bolt.Cursor
}

func (cursor forward) PositionToFirst(prefix, first storage.Key) (key, value []byte) {
	if first.IsZero() || first.Less(prefix) {
		return cursor.Seek([]byte(prefix))
	}
	return cursor.Seek([]byte(first))
}

func (cursor forward) SkipPrefix(prefix storage.Key) (key, value []byte) {
	return cursor.Seek(storage.AfterPrefix(prefix))
}

func (cursor forward) Advance() (key, value []byte) {
	return cursor.Next()
}

type backward struct {
	*bolt.Cursor
}

func (cursor backward) PositionToFirst(prefix, first storage.Key) (key, value []byte) {
	if prefix.IsZero() {
		// there's no prefix
		if first.IsZero() {
			// and no first item, so start from the end
			return cursor.Last()
		}
	} else {
		// there's a prefix
		if first.IsZero() || storage.AfterPrefix(prefix).Less(first) {
			// there's no first, or it's after our prefix
			// storage.AfterPrefix("axxx/") is the next item after prefixes
			// so we position to the item before
			nextkey := storage.AfterPrefix(prefix)
			_, _ = cursor.Seek(nextkey)
			return cursor.Prev()
		}
	}

	// otherwise try to position on first or one before that
	key, value = cursor.Seek(first)
	if !bytes.Equal(key, first) {
		key, value = cursor.Prev()
	}
	return key, value
}

func (cursor backward) SkipPrefix(prefix storage.Key) (key, value []byte) {
	_, _ = cursor.Seek(prefix)
	return cursor.Prev()
}

func (cursor backward) Advance() (key, value []byte) {
	return cursor.Prev()
}
