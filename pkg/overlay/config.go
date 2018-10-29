// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"net/url"
	"strconv"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/utils"
)

var (
	mon = monkit.Package()
	// Error represents an overlay error
	Error = errs.Class("overlay error")
)

// Config is a configuration struct for everything you need to start the
// Overlay cache responsibility.
type Config struct {
	DatabaseURL     string        `help:"the database connection string to use" default:"bolt://$CONFDIR/overlay.db"`
	RefreshInterval time.Duration `help:"the interval at which the cache refreshes itself in seconds" default:"30s"`
}

// CtxKey used for assigning cache
type CtxKey int

const (
	ctxKeyOverlay CtxKey = iota
	ctxKeyOverlayServer
)

// Run implements the provider.Responsibility interface. Run assumes a
// Kademlia responsibility has been started before this one.
func (c Config) Run(ctx context.Context, server *provider.Provider) (
	err error) {
	defer mon.Task()(&ctx)(&err)

	kad := kademlia.LoadFromContext(ctx)
	if kad == nil {
		return Error.New("programmer error: kademlia responsibility unstarted")
	}

	dburl, err := utils.ParseURL(c.DatabaseURL)
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
		cache, err = NewRedisOverlayCache(dburl.Host, GetUserPassword(dburl), db, kad)
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

	ticker := time.NewTicker(c.RefreshInterval)
	defer ticker.Stop()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		for {
			select {
			case <-ticker.C:
				err := cache.Refresh(ctx)
				if err != nil {
					zap.S().Error("Error with cache refresh: ", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	srv := &Server{
		dht:   kad,
		cache: cache,

		// TODO(jt): do something else
		logger:  zap.L(),
		metrics: monkit.Default,
	}
	pb.RegisterOverlayServer(server.GRPC(), srv)
	ctx = context.WithValue(ctx, ctxKeyOverlay, cache)
	ctx = context.WithValue(ctx, ctxKeyOverlayServer, srv)
	return server.Run(ctx)
}

// LoadFromContext gives access to the cache from the context, or returns nil
func LoadFromContext(ctx context.Context) *Cache {
	if v, ok := ctx.Value(ctxKeyOverlay).(*Cache); ok {
		return v
	}
	return nil
}

// LoadServerFromContext gives access to the overlay server from the context, or returns nil
func LoadServerFromContext(ctx context.Context) *Server {
	if v, ok := ctx.Value(ctxKeyOverlayServer).(*Server); ok {
		return v
	}
	return nil
}

// GetUserPassword extracts password from scheme://user:password@hostname
func GetUserPassword(u *url.URL) string {
	if u == nil || u.User == nil {
		return ""
	}
	if pw, ok := u.User.Password(); ok {
		return pw
	}
	return ""
}
