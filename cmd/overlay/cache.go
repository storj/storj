// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"net/url"
	"strconv"

	"go.uber.org/zap"

	"storj.io/storj/pkg/overlay"
)

type cacheConfig struct {
	NodesPath   string `help:"the path to a JSON file containing an object with IP keys and nodeID values"`
	DatabaseURL string `help:"the database connection string to use"`
}

func (c cacheConfig) open() (*overlay.Cache, error) {
	dburl, err := url.Parse(c.DatabaseURL)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	var cache *overlay.Cache
	switch dburl.Scheme {
	case "bolt":
		cache, err = overlay.NewBoltOverlayCache(dburl.Path, nil)
		if err != nil {
			return nil, err
		}
		zap.S().Info("Starting overlay cache with BoltDB")
	case "redis":
		db, err := strconv.Atoi(dburl.Query().Get("db"))
		if err != nil {
			return nil, Error.New("invalid db: %s", err)
		}
		cache, err = overlay.NewRedisOverlayCache(dburl.Host, overlay.GetUserPassword(dburl), db, nil)
		if err != nil {
			return nil, err
		}
		zap.S().Info("Starting overlay cache with Redis")
	default:
		return nil, Error.New("database scheme not supported: %s", dburl.Scheme)
	}

	return cache, nil
}
