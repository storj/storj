// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package boltdb

import (
	"bytes"
	"fmt"
	"time"

	"github.com/boltdb/bolt"

	"storj.io/storj/storage"
)

// Client is the entrypoint into a bolt data store
type Client struct {
	db     *bolt.DB
	Path   string
	Bucket []byte
}

const (
	// fileMode sets permissions so owner can read and write
	fileMode       = 0600
	maxKeyLookup   = 100
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
		// TODO: don't hide error here
		_ = db.Close()
		return nil, err
	}

	return &Client{
		db:     db,
		Path:   path,
		Bucket: []byte(bucket),
	}, nil
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
func (client *Client) List(first storage.Key, limit storage.Limit) (storage.Keys, error) {
	return storage.ListKeys(client, first, limit)
}

// ReverseList returns either a list of keys for which boltdb has values or an error.
// Starts from first and iterates backwards
func (client *Client) ReverseList(first storage.Key, limit storage.Limit) (storage.Keys, error) {
	return storage.ReverseListKeys(client, first, limit)
}

// Close closes a BoltDB client
func (client *Client) Close() error {
	return client.db.Close()
}

// GetAll finds all values for the provided keys up to 100 keys
// if more keys are provided than the maximum an error will be returned.
func (client *Client) GetAll(keys storage.Keys) (storage.Values, error) {
	lk := len(keys)
	if lk > maxKeyLookup {
		return nil, Error.New(fmt.Sprintf("requested %d keys, maximum is %d", lk, maxKeyLookup))
	}

	vals := make(storage.Values, 0, lk)
	err := client.view(func(bucket *bolt.Bucket) error {
		for _, key := range keys {
			val := bucket.Get([]byte(key))
			vals = append(vals, storage.CloneValue(storage.Value(val)))
		}
		return nil
	})

	return vals, err
}

// Iterate iterates over collapsed items with prefix starting from first or the next key
func (client *Client) Iterate(prefix, first storage.Key, delimiter byte, fn func(storage.Iterator) error) error {
	return client.iterate(prefix, first, false, delimiter, fn)
}

// IterateAll iterates over all items with prefix starting from first or the next key
func (client *Client) IterateAll(prefix, first storage.Key, fn func(storage.Iterator) error) error {
	return client.iterate(prefix, first, true, '/', fn)
}

// IterateReverse iterates over collapsed items with prefix starting from first or the prev key
func (client *Client) IterateReverse(prefix, first storage.Key, delimiter byte, fn func(storage.Iterator) error) error {
	return client.iterateReverse(prefix, first, false, delimiter, fn)
}

// IterateReverseAll iterates over all items with prefix starting from first or the prev key
func (client *Client) IterateReverseAll(prefix, first storage.Key, fn func(storage.Iterator) error) error {
	return client.iterateReverse(prefix, first, true, '/', fn)
}

func (client *Client) iterate(prefix, first storage.Key, recurse bool, delimiter byte, fn func(storage.Iterator) error) error {
	return client.view(func(bucket *bolt.Bucket) error {
		cursor := bucket.Cursor()

		start := true
		lastPrefix := []byte{}
		wasPrefix := false

		return fn(storage.IteratorFunc(func(item *storage.ListItem) bool {
			var key, value []byte
			if start {
				if first == nil || first.Less(prefix) {
					key, value = cursor.Seek([]byte(prefix))
				} else {
					key, value = cursor.Seek([]byte(first))
				}
				start = false
			} else {
				key, value = cursor.Next()
			}

			if !recurse {
				// when non-recursive skip all items that have the same prefix
				if wasPrefix && bytes.HasPrefix(key, lastPrefix) {
					key, value = cursor.Seek(storage.AfterPrefix(lastPrefix))
					wasPrefix = false
				}
			}

			if key == nil || !bytes.HasPrefix(key, prefix) {
				return false
			}

			if !recurse {
				// check whether the entry is a proper prefix
				if p := bytes.IndexByte(key[len(prefix):], delimiter); p >= 0 {
					key = key[:len(prefix)+p+1]
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

func (client *Client) iterateReverse(prefix, first storage.Key, recurse bool, delimiter byte, fn func(storage.Iterator) error) error {
	return client.view(func(bucket *bolt.Bucket) error {
		cursor := bucket.Cursor()

		start := true
		lastPrefix := []byte{}
		wasPrefix := false

		return fn(storage.IteratorFunc(func(item *storage.ListItem) bool {
			var key, value []byte
			if start {
				start = false
				if prefix == nil {
					// there's no prefix
					if first == nil {
						// and no first item, so start from the end
						key, value = cursor.Last()
					} else {
						// theres a first item, so try to position on that or one before that
						key, value = cursor.Seek(first)
						if !bytes.Equal(key, first) {
							key, value = cursor.Prev()
						}
					}
				} else {
					// there's a prefix
					if first == nil || storage.AfterPrefix(prefix).Less(first) {
						// there's no first, or it's after our prefix
						// storage.AfterPrefix("axxx/") is the next item after prefixes
						// so we position to the item before
						nextkey := storage.AfterPrefix(prefix)
						_, _ = cursor.Seek(nextkey)
						key, value = cursor.Prev()
					} else {
						// otherwise try to position on first or one before that
						key, value = cursor.Seek(first)
						if !bytes.Equal(key, first) {
							key, value = cursor.Prev()
						}
					}
				}
			} else {
				key, value = cursor.Prev()
			}

			if !recurse {
				// when non-recursive skip all items that have the same prefix
				if wasPrefix && bytes.HasPrefix(key, lastPrefix) {
					_, _ = cursor.Seek(lastPrefix)
					key, value = cursor.Prev()
					wasPrefix = false
				}
			}

			if key == nil || !bytes.HasPrefix(key, prefix) {
				return false
			}

			if !recurse {
				// check whether the entry is a proper prefix
				if p := bytes.IndexByte(key[len(prefix):], delimiter); p >= 0 {
					key = key[:len(prefix)+p+1]
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
