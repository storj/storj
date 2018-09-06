// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package boltdb

import (
	"go.uber.org/zap"

	"storj.io/storj/storage"
	"storj.io/storj/storage/storelogger"
)

// NewClient instantiates a new BoltDB client given db file path, and a bucket name
func NewClient(log *zap.Logger, path, bucket string) (storage.KeyValueStore, error) {
	client, err := New(path, bucket)
	db := storelogger.New(log, client)
	return db, err
}
