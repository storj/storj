package boltdb

import (
	"time"

	"github.com/boltdb/bolt"
)

var defaultTimeout = 1 * time.Second

// Client is the storage interface for the Bolt database
type Client struct {
	DB          *bolt.DB
	UsersBucket *bolt.Bucket
}

// New instantiates a new BoltDB client
func New() (*Client, error) {
	db, err := bolt.Open("my.db", 0600, &bolt.Options{Timeout: defaultTimeout})
	if err != nil {
		return nil, err
	}

	b := &bolt.Bucket{}
	err = db.Update(func(tx *bolt.Tx) error {
		b, err = tx.CreateBucketIfNotExists([]byte("users"))
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &Client{
		DB: db,
	}, nil
}
