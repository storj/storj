// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/piecestore/psserver/psdb"
	"storj.io/storj/storage"
	"storj.io/storj/storage/boltdb"
	"storj.io/storj/storage/filestore"
	"storj.io/storj/storage/teststore"
	"storj.io/storj/storagenode"
)

var _ storagenode.DB = (*DB)(nil)

// Config configures storage node database
type Config struct {
	// TODO: figure out better names
	Storage  string
	Info     string
	Info2    string
	Kademlia string

	Pieces string
}

// DB contains access to different database tables
type DB struct {
	log  *zap.Logger
	psdb *psdb.DB

	pieces interface {
		storage.Blobs
		Close() error
	}

	info *infodb

	kdb, ndb storage.KeyValueStore
}

// New creates a new master database for storage node
func New(log *zap.Logger, config Config) (*DB, error) {
	piecesDir, err := filestore.NewDir(config.Pieces)
	if err != nil {
		return nil, err
	}
	pieces := filestore.New(piecesDir)

	infodb, err := newInfo(config.Info2)
	if err != nil {
		return nil, err
	}

	psdb, err := psdb.Open(config.Info)
	if err != nil {
		return nil, err
	}

	dbs, err := boltdb.NewShared(config.Kademlia, kademlia.KademliaBucket, kademlia.NodeBucket)
	if err != nil {
		return nil, err
	}

	return &DB{
		log:  log,
		psdb: psdb,

		pieces: pieces,

		info: infodb,

		kdb: dbs[0],
		ndb: dbs[1],
	}, nil
}

// NewInMemory creates new inmemory master database for storage node
// TODO: still stores data on disk
func NewInMemory(log *zap.Logger, storageDir string) (*DB, error) {
	piecesDir, err := filestore.NewDir(storageDir)
	if err != nil {
		return nil, err
	}
	pieces := filestore.New(piecesDir)

	infodb, err := newInfoInMemory()
	if err != nil {
		return nil, err
	}

	psdb, err := psdb.OpenInMemory()
	if err != nil {
		return nil, err
	}

	return &DB{
		log:  log,
		psdb: psdb,

		pieces: pieces,

		info: infodb,

		kdb: teststore.New(),
		ndb: teststore.New(),
	}, nil
}

// CreateTables creates any necessary tables.
func (db *DB) CreateTables() error {
	migration := db.psdb.Migration()
	return errs.Combine(
		migration.Run(db.log.Named("migration"), db.psdb),
		db.info.CreateTables(db.log.Named("info")),
	)
}

// Close closes any resources.
func (db *DB) Close() error {
	return errs.Combine(
		db.psdb.Close(),
		db.kdb.Close(),
		db.ndb.Close(),

		db.pieces.Close(),
		db.info.Close(),
	)
}

// Pieces returns blob storage for pieces
func (db *DB) Pieces() storage.Blobs {
	return db.pieces
}

// PSDB returns piecestore database
func (db *DB) PSDB() *psdb.DB {
	return db.psdb
}

// RoutingTable returns kademlia routing table
func (db *DB) RoutingTable() (kdb, ndb storage.KeyValueStore) {
	return db.kdb, db.ndb
}
