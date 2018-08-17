// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package boltdb

import (
	"fmt"
	"time"

	"github.com/boltdb/bolt"
	"go.uber.org/zap"

	"storj.io/storj/storage"
)

// Client is the entrypoint into a bolt data store
type Client struct {
	logger *zap.Logger
	db     *bolt.DB
	Path   string
	Bucket []byte
}

const (
	// fileMode sets permissions so owner can read and write
	fileMode = 0600
	// PointerBucket is the string representing the bucket used for `PointerEntries`
	PointerBucket = "pointers"
	// OverlayBucket is the string representing the bucket used for a bolt-backed overlay dht cache
	OverlayBucket = "overlay"
	// KBucket is the string representing the bucket used for the kademlia routing table k-bucket ids
	KBucket = "kbuckets"
	// NodeBucket is the string representing the bucket used for the kademlia routing table node ids
	NodeBucket   = "nodes"
	maxKeyLookup = 100
)

var (
	defaultTimeout = 1 * time.Second
)

// NewClient instantiates a new BoltDB client given a zap logger, db file path, and a bucket name
func NewClient(logger *zap.Logger, path, bucket string) (*Client, error) {
	db, err := bolt.Open(path, fileMode, &bolt.Options{Timeout: defaultTimeout})
	if err != nil {
		return nil, err
	}

	err = db.Update(func(tx *bolt.Tx) error {
		_, err = tx.CreateBucketIfNotExists([]byte(bucket))
		return err
	})
	if err != nil {
		db.Close()
		return nil, err
	}

	return &Client{
		logger: logger,
		db:     db,
		Path:   path,
		Bucket: []byte(bucket),
	}, nil
}

// Put adds a value to the provided key in boltdb, returning an error on failure.
func (c *Client) Put(key storage.Key, value storage.Value) error {
	c.logger.Debug("entering bolt put")
	return c.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(c.Bucket)
		return b.Put(key, value)
	})
}

// Get looks up the provided key from boltdb returning either an error or the result.
func (c *Client) Get(pathKey storage.Key) (storage.Value, error) {
	c.logger.Debug("entering bolt get: " + string(pathKey))
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
	c.logger.Debug("entering bolt list")
	return c.listHelper(false, startingKey, limit)
}

// ReverseList returns either a list of keys for which boltdb has values or an error.
// Starts from startingKey and iterates backwards
func (c *Client) ReverseList(startingKey storage.Key, limit storage.Limit) (storage.Keys, error) {
	c.logger.Debug("entering bolt reverse list")
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
			if limit > 0 && int(limit) == int(len(paths)) {
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

// Delete deletes a key/value pair from boltdb, for a given the key
func (c *Client) Delete(pathKey storage.Key) error {
	c.logger.Debug("entering bolt delete: " + string(pathKey))
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
