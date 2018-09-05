// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package boltdb

import (
	"bytes"
	"fmt"
	"time"

	"storj.io/storj/pkg/utils"
	"storj.io/storj/storage"

	"github.com/boltdb/bolt"
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
		if closeErr := db.Close(); closeErr != nil {
			return nil, utils.CombineErrors(err, closeErr)
		}
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

// Iterate iterates over items based on opts
func (client *Client) Iterate(opts storage.IterateOptions, fn func(storage.Iterator) error) error {
	if opts.Reverse {
		return client.iterateReverse(opts, fn)
	}
	return client.iterate(opts, fn)
}

func (client *Client) iterate(opts storage.IterateOptions, fn func(storage.Iterator) error) error {
	return client.view(func(bucket *bolt.Bucket) error {
		cursor := bucket.Cursor()

		start := true
		lastPrefix := []byte{}
		wasPrefix := false

		return fn(storage.IteratorFunc(func(item *storage.ListItem) bool {
			var key, value []byte
			if start {
				if opts.First == nil || opts.First.Less(opts.Prefix) {
					key, value = cursor.Seek([]byte(opts.Prefix))
				} else {
					key, value = cursor.Seek([]byte(opts.First))
				}
				start = false
			} else {
				key, value = cursor.Next()
			}

			if !opts.Recurse {
				// when non-recursive skip all items that have the same prefix
				if wasPrefix && bytes.HasPrefix(key, lastPrefix) {
					key, value = cursor.Seek(storage.AfterPrefix(lastPrefix))
					wasPrefix = false
				}
			}

			if key == nil || !bytes.HasPrefix(key, opts.Prefix) {
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

func (client *Client) iterateReverse(opts storage.IterateOptions, fn func(storage.Iterator) error) error {
	return client.view(func(bucket *bolt.Bucket) error {
		cursor := bucket.Cursor()

		start := true
		lastPrefix := []byte{}
		wasPrefix := false

		return fn(storage.IteratorFunc(func(item *storage.ListItem) bool {
			var key, value []byte
			if start {
				start = false
				if opts.Prefix == nil {
					// there's no prefix
					if opts.First == nil {
						// and no first item, so start from the end
						key, value = cursor.Last()
					} else {
						// theres a first item, so try to position on that or one before that
						key, value = cursor.Seek(opts.First)
						if !bytes.Equal(key, opts.First) {
							key, value = cursor.Prev()
						}
					}
				} else {
					// there's a prefix
					if opts.First == nil || storage.AfterPrefix(opts.Prefix).Less(opts.First) {
						// there's no first, or it's after our prefix
						// storage.AfterPrefix("axxx/") is the next item after prefixes
						// so we position to the item before
						nextkey := storage.AfterPrefix(opts.Prefix)
						_, _ = cursor.Seek(nextkey)
						key, value = cursor.Prev()
					} else {
						// otherwise try to position on first or one before that
						key, value = cursor.Seek(opts.First)
						if !bytes.Equal(key, opts.First) {
							key, value = cursor.Prev()
						}
					}
				}
			} else {
				key, value = cursor.Prev()
			}

			if !opts.Recurse {
				// when non-recursive skip all items that have the same prefix
				if wasPrefix && bytes.HasPrefix(key, lastPrefix) {
					_, _ = cursor.Seek(lastPrefix)
					key, value = cursor.Prev()
					wasPrefix = false
				}
			}

			if key == nil || !bytes.HasPrefix(key, opts.Prefix) {
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
