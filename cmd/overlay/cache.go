// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"go.uber.org/zap"

	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/statdb"
	"storj.io/storj/pkg/utils"
	"storj.io/storj/storage"
	"storj.io/storj/storage/boltdb"
	"storj.io/storj/storage/redis"
	"storj.io/storj/storage/storelogger"
)

type cacheConfig struct {
	NodesPath   string `help:"the path to a JSON file containing an object with IP keys and nodeID values"`
	DatabaseURL string `help:"the database connection string to use"`
}

func (c cacheConfig) open() (*overlay.Cache, error) {
	driver, source, err := utils.SplitDBURL(c.DatabaseURL)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	var db storage.KeyValueStore
	var sdb *statdb.StatDB

	switch driver {
	case "bolt":
		db, err = boltdb.New(source, overlay.OverlayBucket)
		if err != nil {
			return nil, Error.New("invalid overlay cache database: %s", err)
		}
		zap.S().Info("Starting overlay cache with BoltDB")
	case "redis":
		db, err = redis.NewClientFrom(c.DatabaseURL)
		if err != nil {
			return nil, Error.New("invalid overlay cache database: %s", err)
		}
		zap.S().Info("Starting overlay cache with Redis")
	default:
		return nil, Error.New("database scheme not supported: %s", driver)
	}

	// add logger
	db = storelogger.New(zap.L(), db)
	sdb, err = statdb.NewStatDB("postgres", source, zap.L()) //todo:  unhardcode this
	if err != nil {
		return nil, Error.New("statdb error: %s", err)
	}

	return overlay.NewOverlayCache(db, nil, sdb), nil
}
