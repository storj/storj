package boltdb

import (
	"go.uber.org/zap"
	"storj.io/storj/storage"
)

// NewClient instantiates a new BoltDB client given db file path, and a bucket name
func NewClient(log *zap.Logger, path, bucket string) (storage.KeyValueStore, error) {
	client, err := New(path, bucket)
	db := storage.NewLogger(log, client)
	return db, err
}
