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
