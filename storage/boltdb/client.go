// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package boltdb

import (
	"time"

	"github.com/boltdb/bolt"
)

var (
	defaultTimeout = 1 * time.Second

	ErrDbOpen = Error.New("error boltdb failed to open")
	ErrInitDb = Error.New("error instantiating boltdb")
)

// Client is the storage interface for the Bolt database
type Client struct {
	db   *bolt.DB
	Path string
}

// New instantiates a new BoltDB client
func New(path string) (*Client, error) {
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: defaultTimeout})
	if err != nil {
		return nil, ErrDbOpen
	}

	return &Client{
		db:   db,
		Path: path,
	}, nil
}

func (c *Client) Close() error {
	return c.db.Close()
}
