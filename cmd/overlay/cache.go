// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"net/url"
	"strconv"

	"go.uber.org/zap"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/provider"
)

type cacheConfig struct {
	NodesPath   string `help:"the path to a JSON file containing an object with IP keys and nodeID values" default:"$CONFDIR/nodes.json"`
	DatabaseURL string `help:"the database connection string to use" default:"bolt://$CONFDIR/overlay.db"`
}
type cacheFunc func(*overlay.Cache) error
type cacheInjector struct {
	cacheConfig
	c cacheFunc
}

func (c cacheInjector) Run(ctx context.Context, server *provider.Provider) (err error) {
	defer mon.Task()(&ctx)(&err)

	kad := kademlia.LoadFromContext(ctx)
	if kad == nil {
		return Error.New("programmer error: kademlia responsibility unstarted")
	}

	dburl, err := url.Parse(c.DatabaseURL)
	if err != nil {
		return Error.Wrap(err)
	}

	var cache *overlay.Cache
	switch dburl.Scheme {
	case "bolt":
		cache, err = overlay.NewBoltOverlayCache(dburl.Path, kad)
		if err != nil {
			return err
		}
		zap.S().Info("Starting overlay cache with BoltDB")
	case "redis":
		db, err := strconv.Atoi(dburl.Query().Get("db"))
		if err != nil {
			return Error.New("invalid db: %s", err)
		}
		cache, err = overlay.NewRedisOverlayCache(dburl.Host, overlay.UrlPwd(dburl), db, kad)
		if err != nil {
			return err
		}
		zap.S().Info("Starting overlay cache with Redis")
	default:
		return Error.New("database scheme not supported: %s", dburl.Scheme)
	}

	return c.c(cache)
}
