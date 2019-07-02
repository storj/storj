// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package bootstrapdb

import (
	"github.com/zeebo/errs"

	"storj.io/storj/bootstrap"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/storage"
	"storj.io/storj/storage/boltdb"
	"storj.io/storj/storage/teststore"
)

var _ bootstrap.DB = (*DB)(nil)

// Config configures storage node database
type Config struct {
	Kademlia string
}

// DB contains access to different database tables
type DB struct {
	kdb, ndb, adb storage.KeyValueStore
}

// New creates a new master database for storage node
func New(config Config) (*DB, error) {
	dbs, err := boltdb.NewShared(config.Kademlia, kademlia.KademliaBucket, kademlia.NodeBucket, kademlia.AntechamberBucket)
	if err != nil {
		return nil, err
	}

	return &DB{
		kdb: dbs[0],
		ndb: dbs[1],
		adb: dbs[2],
	}, nil
}

// NewInMemory creates new inmemory master database for storage node
// TODO: still stores data on disk
func NewInMemory(storageDir string) (*DB, error) {
	return &DB{
		kdb: teststore.New(),
		ndb: teststore.New(),
		adb: teststore.New(),
	}, nil
}

// CreateTables initializes the database
func (db *DB) CreateTables() error { return nil }

// Close closes any resources.
func (db *DB) Close() error {
	return errs.Combine(
		db.kdb.Close(),
		db.ndb.Close(),
		db.adb.Close(),
	)
}

// RoutingTable returns kademlia routing table
func (db *DB) RoutingTable() (kdb, ndb, adb storage.KeyValueStore) {
	return db.kdb, db.ndb, db.adb
}
