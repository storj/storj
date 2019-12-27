// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package revocation

import (
	"storj.io/common/peertls/extensions"
	"storj.io/common/peertls/tlsopts"
	"storj.io/storj/private/dbutil"
	"storj.io/storj/storage/boltdb"
	"storj.io/storj/storage/redis"
)

// NewDBFromCfg is a convenience method to create a revocation DB
// directly from a config. If the revocation extension option is not set, it
// returns a nil db with no error.
func NewDBFromCfg(cfg tlsopts.Config) (*DB, error) {
	if !cfg.Extensions.Revocation {
		return &DB{}, nil
	}
	return NewDB(cfg.RevocationDBURL)
}

// NewDB returns a new revocation database given the URL
func NewDB(dbURL string) (*DB, error) {
	driver, source, _, err := dbutil.SplitConnStr(dbURL)
	if err != nil {
		return nil, extensions.ErrRevocationDB.Wrap(err)
	}
	var db *DB
	switch driver {
	case "bolt":
		db, err = newDBBolt(source)
		if err != nil {
			return nil, extensions.ErrRevocationDB.Wrap(err)
		}
	case "redis":
		db, err = newDBRedis(dbURL)
		if err != nil {
			return nil, extensions.ErrRevocationDB.Wrap(err)
		}
	default:
		return nil, extensions.ErrRevocationDB.New("database scheme not supported: %s", driver)
	}
	return db, nil
}

// newDBBolt creates a bolt-backed DB
func newDBBolt(path string) (*DB, error) {
	client, err := boltdb.New(path, extensions.RevocationBucket)
	if err != nil {
		return nil, err
	}
	return &DB{
		store: client,
	}, nil
}

// newDBRedis creates a redis-backed DB.
func newDBRedis(address string) (*DB, error) {
	client, err := redis.NewClientFrom(address)
	if err != nil {
		return nil, err
	}
	return &DB{
		store: client,
	}, nil
}
