// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package boltdb

import (
	"time"

	"github.com/boltdb/bolt"
	"go.uber.org/zap"
	"storj.io/storj/storage"
)

type boltClient struct {
	logger *zap.Logger
	DB     *bolt.DB
	Path   string
	Bucket []byte
}

const (
	// fileMode sets permissions so owner can read and write
	fileMode      = 0600
	PointerBucket = "pointers"
	OverlayBucket = "overlay"
)

var (
	defaultTimeout = 1 * time.Second
)

// NewClient instantiates a new BoltDB client
func NewClient(logger *zap.Logger, path, bucket string) (storage.DB, error) {
	db, err := bolt.Open(path, fileMode, &bolt.Options{Timeout: defaultTimeout})
	if err != nil {
		return nil, err
	}

	return &boltClient{
		logger: logger,
		DB:     db,
		Path:   path,
		Bucket: []byte(bucket),
	}, nil
}

// Put saves the Path and Pointer as a kv entry in the "pointers" bucket
func (c *boltClient) Put(key, value []byte) error {
	c.logger.Debug("entering bolt put")
	return c.DB.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(c.Bucket)
		if err != nil {
			return err
		}

		return b.Put(key, value)
	})
}

// Get retrieves the Pointer value stored at the Path key
func (c *boltClient) Get(pathKey []byte) ([]byte, error) {
	c.logger.Debug("entering bolt get: " + string(pathKey))
	var pointerBytes []byte
	err := c.DB.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(c.Bucket)
		v := b.Get(pathKey)
		if v == nil {
			return Error.New("pointer at %#v not found", string(pathKey))
		}
		pointerBytes = v
		return nil
	})

	return pointerBytes, err
}

// List creates a byte array of all path keys in in the "pointers" bucket
func (c *boltClient) List() ([][]byte, error) {
	c.logger.Debug("entering bolt list")
	var paths [][]byte
	err := c.DB.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(c.Bucket)

		err := b.ForEach(func(key, value []byte) error {
			paths = append(paths, key)
			return nil
		})
		return err
	})

	return paths, err
}

// Delete deletes a kv pair from the "pointers" bucket, given the Path key
func (c *boltClient) Delete(pathKey []byte) error {
	c.logger.Debug("entering bolt delete: " + string(pathKey))
	return c.DB.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(c.Bucket).Delete(pathKey)
	})
}

// Close closes a BoltDB client
func (c *boltClient) Close() error {
	return c.DB.Close()
}
