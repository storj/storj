// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package revocation

import (
	"context"

	"storj.io/common/peertls/extensions"
	"storj.io/common/peertls/tlsopts"
	"storj.io/storj/private/kvstore/boltdb"
	"storj.io/storj/private/kvstore/redis"
	"storj.io/storj/shared/dbutil"
)

// OpenDBFromCfg is a convenience method to create a revocation DB
// directly from a config. If the revocation extension option is not set, it
// returns a nil db with no error.
func OpenDBFromCfg(ctx context.Context, cfg tlsopts.Config) (*DB, error) {
	if !cfg.Extensions.Revocation {
		return &DB{}, nil
	}
	return OpenDB(ctx, cfg.RevocationDBURL)
}

// OpenDB returns a new revocation database given the URL.
func OpenDB(ctx context.Context, dbURL string) (*DB, error) {
	driver, source, _, err := dbutil.SplitConnStr(dbURL)
	if err != nil {
		return nil, extensions.ErrRevocationDB.Wrap(err)
	}
	var db *DB
	switch driver {
	case "bolt":
		db, err = openDBBolt(ctx, source)
		if err != nil {
			return nil, extensions.ErrRevocationDB.Wrap(err)
		}
	case "redis":
		db, err = openDBRedis(ctx, dbURL)
		if err != nil {
			return nil, extensions.ErrRevocationDB.Wrap(err)
		}
	default:
		return nil, extensions.ErrRevocationDB.New("database scheme not supported: %s", driver)
	}
	return db, nil
}

// openDBBolt creates a bolt-backed DB.
func openDBBolt(ctx context.Context, path string) (*DB, error) {
	client, err := boltdb.New(path, extensions.RevocationBucket)
	if err != nil {
		return nil, err
	}
	return &DB{
		store: client,
	}, nil
}

// openDBRedis creates a redis-backed DB.
func openDBRedis(ctx context.Context, address string) (*DB, error) {
	client, err := redis.OpenClientFrom(ctx, address)
	if err != nil {
		return nil, err
	}
	return &DB{
		store: client,
	}, nil
}
