// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package boltdb

import (
	"time"

	"github.com/boltdb/bolt"
	"go.uber.org/zap"
)

var (
	defaultTimeout = 1 * time.Second
)

const (
	// fileMode sets permissions so owner can read and write
	fileMode = 0600
)

// Client is the storage interface for the Bolt database
type Client struct {
	logger *zap.Logger
	db     *bolt.DB
	Path   string
	Bucket []byte
}

// New instantiates a new BoltDB client
func New(logger *zap.Logger, path string) (*Client, error) {
	db, err := bolt.Open(path, fileMode, &bolt.Options{Timeout: defaultTimeout})
	if err != nil {
		return nil, err
	}

	return &Client{
		logger: logger,
		db:     db,
		Path:   path,
	}, nil
}

// Close closes a BoltDB client
func (c *Client) Close() error {
	return c.db.Close()
}

// Get looks up the provided key from the Bolt bucket `c.Bucket` returning either an error or the result.
func (c *Client) Get(key string) ([]byte, error) {
	var value []byte
	err := c.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(c.Bucket)
		value = b.Get([]byte(key))
		return nil
	})

	return value, err
}

// Set adds a value to the provided key in the Bolt bucket `c.Bucket`, returning an error on failure.
func (c *Client) Set(key string, value []byte, ttl time.Duration) error {
  err := c.db.Update(func(tx *bolt.Tx) error {
    b := tx.Bucket(c.Bucket)
    err := b.Put([]byte(key), value)
    return err
  })

  return err
}
