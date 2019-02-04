// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/kademlia"
	pstore "storj.io/storj/pkg/piecestore"
	"storj.io/storj/pkg/piecestore/psserver/psdb"
	"storj.io/storj/storage"
	"storj.io/storj/storage/boltdb"
	"storj.io/storj/storage/teststore"
	"storj.io/storj/storagenode"
)

var _ storagenode.DB = (*DB)(nil)

// Config configures storage node database
type Config struct {
	// TODO: figure out better names
	Storage  string
	Info     string
	Kademlia string
}

// DB contains access to different database tables
type DB struct {
	storage  *pstore.Storage
	psdb     *psdb.DB
	kdb, ndb storage.KeyValueStore
}

// New creates a new master database for storage node
func New(config Config) (*DB, error) {
	storage := pstore.NewStorage(config.Storage)

	psdb, err := psdb.Open(config.Info)
	if err != nil {
		return nil, err
	}

	dbs, err := boltdb.NewShared(config.Kademlia, kademlia.KademliaBucket, kademlia.NodeBucket)
	if err != nil {
		return nil, err
	}

	return &DB{
		storage: storage,
		psdb:    psdb,
		kdb:     dbs[0],
		ndb:     dbs[1],
	}, nil
}

// NewInMemory creates new inmemory master database for storage node
// TODO: still stores data on disk
func NewInMemory(storageDir string) (*DB, error) {
	storage := pstore.NewStorage(storageDir)

	psdb, err := psdb.OpenInMemory()
	if err != nil {
		return nil, err
	}

	return &DB{
		storage: storage,
		psdb:    psdb,
		kdb:     teststore.New(),
		ndb:     teststore.New(),
	}, nil
}

// CreateTables creates any necessary tables.
func (db *DB) CreateTables() error {
	return nil
}

// Close closes any resources.
func (db *DB) Close() error {
	return errs.Combine(
		db.psdb.Close(),
		db.kdb.Close(),
		db.ndb.Close(),
		db.storage.Close(),
	)
}

// Storage returns piecestore location
func (db *DB) Storage() *pstore.Storage {
	return db.storage
}

// PSDB returns piecestore database
func (db *DB) PSDB() *psdb.DB {
	return db.psdb
}

// RoutingTable returns kademlia routing table
func (db *DB) RoutingTable() (kdb, ndb storage.KeyValueStore) {
	return db.kdb, db.ndb
}
