// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"net/url"
	"context"

	"go.uber.org/zap"

	"storj.io/storj/pkg/overlay"
	"storj.io/storj/storage"
	"storj.io/storj/storage/boltdb"
	"storj.io/storj/storage/redis"
	"storj.io/storj/storage/storelogger"
	"storj.io/storj/pkg/statdb/sdbclient"
	"storj.io/storj/pkg/provider"
)

type cacheConfig struct {
	NodesPath   string `help:"the path to a JSON file containing an object with IP keys and nodeID values"`
	DatabaseURL string `help:"the database connection string to use"`
	StatDBPort  string `help:"the statdb connection port to use" default:":7778"`
	StatDBKey string `help:"the statdb api key to use"`
}

func (c cacheConfig) open() (*overlay.Cache, error) {
	// TODO(moby) what identity to use for statdb client?
	identity, err := getNewIdentity()
	if err != nil {
		return nil, err
	}
	statdb, err := sdbclient.NewClient(identity, c.StatDBPort, []byte(c.StatDBKey))
	if err != nil {
		return nil, err
	}

	dburl, err := url.Parse(c.DatabaseURL)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	var db storage.KeyValueStore

	switch dburl.Scheme {
	case "bolt":
		db, err = boltdb.New(dburl.Path, overlay.OverlayBucket)
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
		return nil, Error.New("database scheme not supported: %s", dburl.Scheme)
	}

	// add logger
	db = storelogger.New(zap.L(), db)

	return overlay.NewOverlayCache(db, nil, statdb), nil
}


// TODO(moby) this is a temporary function
func getNewIdentity() (*provider.FullIdentity, error) {
	ca, err := provider.NewTestCA(context.Background())
	if err != nil {
		return nil, err
	}
	identity, err := ca.NewIdentity()
	if err != nil {
		return nil, err
	}

	return identity, nil
}