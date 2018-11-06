// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"net/url"
	"strconv"
	"context"

	"go.uber.org/zap"

	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/statdb/sdbclient"
)

type cacheConfig struct {
	NodesPath   string `help:"the path to a JSON file containing an object with IP keys and nodeID values"`
	DatabaseURL string `help:"the database connection string to use"`
	StatDBPort  string `help:"the statdb connection port to use" default:":7778"`
	StatDBKey   string `help:"the statdb api key to use"`
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

	var cache *overlay.Cache
	switch dburl.Scheme {
	case "bolt":
		cache, err = overlay.NewBoltOverlayCache(dburl.Path, nil, statdb)
		if err != nil {
			return nil, err
		}
		zap.S().Info("Starting overlay cache with BoltDB")
	case "redis":
		db, err := strconv.Atoi(dburl.Query().Get("db"))
		if err != nil {
			return nil, Error.New("invalid db: %s", err)
		}
		cache, err = overlay.NewRedisOverlayCache(dburl.Host, overlay.GetUserPassword(dburl), db, nil, statdb)
		if err != nil {
			return nil, err
		}
		zap.S().Info("Starting overlay cache with Redis")
	default:
		return nil, Error.New("database scheme not supported: %s", dburl.Scheme)
	}

	return cache, nil
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
