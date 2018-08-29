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
	fileMode     = 0600
	maxKeyLookup = 100
)

var (
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
		_ = db.Close()
		return nil, err
	}

	return &Client{
		db:     db,
		Path:   path,
		Bucket: []byte(bucket),
	}, nil
}

// Put adds a value to the provided key in boltdb, returning an error on failure.
func (c *Client) Put(key storage.Key, value storage.Value) error {
	if key == nil {
		return Error.New("invalid key")
	}

	return c.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(c.Bucket)
		return b.Put(key, value)
	})
}

// Get looks up the provided key from boltdb returning either an error or the result.
func (c *Client) Get(pathKey storage.Key) (storage.Value, error) {
	var pointerBytes []byte
	err := c.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(c.Bucket)
		v := b.Get(pathKey)
		if len(v) == 0 {
			return storage.ErrKeyNotFound.New(pathKey.String())
		}

		pointerBytes = v
		return nil
	})

	if err != nil {
		// TODO: log
		return nil, err
	}

	return pointerBytes, nil
}

// List returns either a list of keys for which boltdb has values or an error.
func (c *Client) List(startingKey storage.Key, limit storage.Limit) (storage.Keys, error) {
	return c.listHelper(false, startingKey, limit)
}

// ReverseList returns either a list of keys for which boltdb has values or an error.
// Starts from startingKey and iterates backwards
func (c *Client) ReverseList(startingKey storage.Key, limit storage.Limit) (storage.Keys, error) {
	return c.listHelper(true, startingKey, limit)
}

func (c *Client) listHelper(reverseList bool, startingKey storage.Key, limit storage.Limit) (storage.Keys, error) {
	var paths storage.Keys
	err := c.db.Update(func(tx *bolt.Tx) error {
		cur := tx.Bucket(c.Bucket).Cursor()
		var k []byte
		start := firstOrLast(reverseList, cur)
		iterate := prevOrNext(reverseList, cur)
		if startingKey == nil {
			k, _ = start()
		} else {
			k, _ = cur.Seek(startingKey)
		}
		for ; k != nil; k, _ = iterate() {
			paths = append(paths, k)
			if limit > 0 && int(limit) == len(paths) {
				break
			}
		}
		return nil
	})
	return paths, err
}

func firstOrLast(reverseList bool, cur *bolt.Cursor) func() ([]byte, []byte) {
	if reverseList {
		return cur.Last
	}
	return cur.First
}

func prevOrNext(reverseList bool, cur *bolt.Cursor) func() ([]byte, []byte) {
	if reverseList {
		return cur.Prev
	}
	return cur.Next
}

//ListV2 is the new definition and will replace `List` definition
func (c *Client) ListV2(opts storage.ListOptions) (storage.Items, storage.More, error) {
	//TODO write the implementation
	panic("to do")
}

// Delete deletes a key/value pair from boltdb, for a given the key
func (c *Client) Delete(pathKey storage.Key) error {
	return c.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(c.Bucket).Delete(pathKey)
	})
}

// Close closes a BoltDB client
func (c *Client) Close() error {
	return c.db.Close()
}

// GetAll finds all values for the provided keys up to 100 keys
// if more keys are provided than the maximum an error will be returned.
func (c *Client) GetAll(keys storage.Keys) (storage.Values, error) {
	lk := len(keys)
	if lk > maxKeyLookup {
		return nil, Error.New(fmt.Sprintf("requested %d keys, maximum is %d", lk, maxKeyLookup))
	}

	vals := make(storage.Values, lk)
	for i, v := range keys {
		val, err := c.Get(v)
		if err != nil {
			return nil, err
		}

		vals[i] = val
	}
	return vals, nil
}

func (store *Client) Iterate(prefix, after storage.Key, delimiter byte) storage.Iterator {
	var items []storage.ListItem

	err := store.db.View(func(tx *bolt.Tx) error {
		cursor := tx.Bucket(store.Bucket).Cursor()

		if prefix == nil {
			var key, value []byte
			var dirPrefix []byte
			var isPrefix bool = false

			// position to the first item
			if after == nil {
				key, value = cursor.First()
			} else {
				key, value = cursor.Seek([]byte(after))
				if bytes.Equal(key, after) {
					key, value = cursor.Next()
				}
			}

			for key != nil {
				if p := bytes.IndexByte(key, delimiter); p >= 0 {
					key = key[:p+1]
					dirPrefix = append(dirPrefix, key...) // copy
					value = nil
					isPrefix = true
				} else {
					dirPrefix = nil
					isPrefix = false
				}

				items = append(items, storage.ListItem{
					Key:      storage.CloneKey(storage.Key(key)),
					Value:    storage.CloneValue(storage.Value(value)),
					IsPrefix: isPrefix,
				})

				// next item
				key, value = cursor.Next()
				if isPrefix {
					for bytes.HasPrefix(key, dirPrefix) && key != nil {
						key, value = cursor.Next()
					}
				}
			}

			return nil
		}

		var key, value []byte
		var dirPrefix []byte
		var isPrefix bool = false

		// position to the first item
		if after == nil || after.Less(prefix) {
			key, value = cursor.Seek([]byte(prefix))
		} else {
			key, value = cursor.Seek([]byte(after))
			if bytes.Equal(key, after) {
				key, value = cursor.Next()
			}
		}

		for key != nil && bytes.HasPrefix(key, prefix) {
			if p := bytes.IndexByte(key[len(prefix):], delimiter); p >= 0 {
				key = key[:len(prefix)+p+1]
				dirPrefix = append(dirPrefix, key...) // copy
				value = nil
				isPrefix = true
			} else {
				dirPrefix = nil
				isPrefix = false
			}

			items = append(items, storage.ListItem{
				Key:      storage.CloneKey(storage.Key(key)),
				Value:    storage.CloneValue(storage.Value(value)),
				IsPrefix: isPrefix,
			})

			// next item
			key, value = cursor.Next()
			if isPrefix {
				for bytes.HasPrefix(key, dirPrefix) && key != nil {
					key, value = cursor.Next()
				}
			}
		}

		return nil
	})

	return &staticIterator{
		items: items,
		err:   err,
	}
}

type staticIterator struct {
	err   error
	items []storage.ListItem
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
