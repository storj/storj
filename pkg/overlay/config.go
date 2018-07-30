// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"net/url"
	"strconv"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/provider"
	proto "storj.io/storj/protos/overlay"
)

var (
	mon = monkit.Package()
	// Error represents an overlay error
	Error = errs.Class("overlay error")
)

// Config is a configuration struct for everything you need to start the
// Overlay cache responsibility.
type Config struct {
	DatabaseURL string `help:"the database connection string to use" default:"bolt://$CONFDIR/overlay.db"`
}

// Run implements the provider.Responsibility interface. Run assumes a
// Kademlia responsibility has been started before this one.
func (c Config) Run(ctx context.Context, server *provider.Provider) (
	err error) {
	defer mon.Task()(&ctx)(&err)

	kad := kademlia.LoadFromContext(ctx)
	if kad == nil {
		return Error.New("programmer error: kademlia responsibility unstarted")
	}

	dburl, err := url.Parse(c.DatabaseURL)
	if err != nil {
		return Error.Wrap(err)
	}

	var cache *Cache
	switch dburl.Scheme {
	case "bolt":
		cache, err = NewBoltOverlayCache(dburl.Path, kad)
		if err != nil {
			return err
		}
		zap.S().Info("Starting overlay cache with BoltDB")
	case "redis":
		db, err := strconv.Atoi(dburl.Query().Get("db"))
		if err != nil {
			return Error.New("invalid db: %s", err)
		}
		cache, err = NewRedisOverlayCache(dburl.Host, urlPwd(dburl), db, kad)
		if err != nil {
			return err
		}
		zap.S().Info("Starting overlay cache with Redis")
	default:
		return Error.New("database scheme not supported: %s", dburl.Scheme)
	}

	err = cache.Bootstrap(ctx)
	if err != nil {
		return err
	}

	go func() {
		// TODO(jt): should there be a for loop here?
		err := cache.Refresh(ctx)
		if err != nil {
			zap.S().Fatal("cache refreshes stopped", zap.Error(err))
		}
	}()

	proto.RegisterOverlayServer(server.GRPC(), &Server{
		dht:   kad,
		cache: cache,

		// TODO(jt): do something else
		logger:  zap.L(),
		metrics: monkit.Default,
	})

	go func() {
		// TODO(jt): should there be a for loop here?
		// TODO(jt): how is this different from Refresh?
		err := cache.Walk(ctx)
		if err != nil {
			zap.S().Fatal("cache walking stopped", zap.Error(err))
		}
	}()

	return server.Run(ctx)
}

func urlPwd(u *url.URL) string {
	if u.User == nil {
		return ""
	}
	if pw, ok := u.User.Password(); ok {
		return pw
	}
	return ""
}
