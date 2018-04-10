package boltdb

import (
	"errors"
	"github.com/boltdb/bolt"
	"time"
)

var (
	defaultTimeout = 1 * time.Second

	ErrDbOpen = errors.New("error boltdb failed to open")
	ErrInitDb = errors.New("error instantiating boltdb")
)

// Client is the storage interface for the Bolt database
type Client struct {
	db *bolt.DB
}

// New instantiates a new BoltDB client
func New(path string) (*Client, error) {
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: defaultTimeout})
	if err != nil {
		return nil, ErrDbOpen
	}

	return &Client{
		db: db,
	}, nil
}

func (c *Client) Close() error {
	return c.db.Close()
}
