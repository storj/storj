// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"
	"errors"
	"net"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/identity"
	"storj.io/common/peertls/extensions"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/private/debug"
	"storj.io/private/version"
	"storj.io/storj/private/lifecycle"
	version_checker "storj.io/storj/private/version/checker"
	"storj.io/storj/satellite/gc"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/metrics"
	"storj.io/storj/satellite/overlay"
)

// GarbageCollection is the satellite garbage collection process
//
// architecture: Peer
type GarbageCollection struct {
	Log      *zap.Logger
	Identity *identity.FullIdentity
	DB       DB

	Servers  *lifecycle.Group
	Services *lifecycle.Group

	Dialer rpc.Dialer

	Version struct {
		Chore   *version_checker.Chore
		Service *version_checker.Service
	}

	Debug struct {
		Listener net.Listener
		Server   *debug.Server
	}

	Overlay struct {
		DB overlay.DB
	}

	Metainfo struct {
		Database metainfo.PointerDB
		Loop     *metainfo.Loop
	}

	GarbageCollection struct {
		Service *gc.Service
	}

	Metrics struct {
		Chore *metrics.Chore
	}
}

// NewGarbageCollection creates a new satellite garbage collection process
func NewGarbageCollection(log *zap.Logger, full *identity.FullIdentity, db DB,
	pointerDB metainfo.PointerDB, revocationDB extensions.RevocationDB,
	versionInfo version.Info, config *Config) (*GarbageCollection, error) {
	peer := &GarbageCollection{
		Log:      log,
		Identity: full,
		DB:       db,

		Servers:  lifecycle.NewGroup(log.Named("servers")),
		Services: lifecycle.NewGroup(log.Named("services")),
	}

	{ // setup debug
		var err error
		if config.Debug.Address != "" {
			peer.Debug.Listener, err = net.Listen("tcp", config.Debug.Address)
			if err != nil {
				withoutStack := errors.New(err.Error())
				peer.Log.Debug("failed to start debug endpoints", zap.Error(withoutStack))
				err = nil
			}
		}
		debugConfig := config.Debug
		debugConfig.ControlTitle = "GC"
		peer.Debug.Server = debug.NewServer(log.Named("debug"), peer.Debug.Listener, monkit.Default, debugConfig)
		peer.Servers.Add(lifecycle.Item{
			Name:  "debug",
			Run:   peer.Debug.Server.Run,
			Close: peer.Debug.Server.Close,
		})
	}

	{ // setup version control
		peer.Log.Info("Version info",
			zap.Stringer("Version", versionInfo.Version.Version),
			zap.String("Commit Hash", versionInfo.CommitHash),
			zap.Stringer("Build Timestamp", versionInfo.Timestamp),
			zap.Bool("Release Build", versionInfo.Release),
		)
		peer.Version.Service = version_checker.NewService(log.Named("version"), config.Version, versionInfo, "Satellite")
		peer.Version.Chore = version_checker.NewChore(peer.Version.Service, config.Version.CheckInterval)

		peer.Services.Add(lifecycle.Item{
			Name: "version",
			Run:  peer.Version.Chore.Run,
		})
	}

	{ // setup listener and server
		sc := config.Server

		tlsOptions, err := tlsopts.NewOptions(peer.Identity, sc.Config, revocationDB)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Dialer = rpc.NewDefaultDialer(tlsOptions)
	}

	{ // setup overlay
		peer.Overlay.DB = peer.DB.OverlayCache()
	}

	{ // setup metainfo
		peer.Metainfo.Database = pointerDB

		// Garbage Collection creates its own instance of the metainfo loop here. Since
		// GC runs infrequently, this shouldn'tt add too much extra load on the metainfo db.
		// As long as garbage collection is the only observer joining the metainfo loop, then by default
		// the metainfo loop will only run when the garbage collection joins (which happens every GarbageCollection.Interval)
		peer.Metainfo.Loop = metainfo.NewLoop(config.Metainfo.Loop, peer.Metainfo.Database)
		peer.Services.Add(lifecycle.Item{
			Name:  "metainfo:loop",
			Run:   peer.Metainfo.Loop.Run,
			Close: peer.Metainfo.Loop.Close,
		})
	}

	{ // setup garbage collection
		peer.GarbageCollection.Service = gc.NewService(
			peer.Log.Named("garbage-collection"),
			config.GarbageCollection,
			peer.Dialer,
			peer.Overlay.DB,
			peer.Metainfo.Loop,
		)
		peer.Services.Add(lifecycle.Item{
			Name: "garbage-collection",
			Run:  peer.GarbageCollection.Service.Run,
		})
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Garbage Collection", peer.GarbageCollection.Service.Loop))
	}

	{ // setup metrics service
		peer.Metrics.Chore = metrics.NewChore(
			peer.Log.Named("metrics"),
			config.Metrics,
			peer.Metainfo.Loop,
		)
		peer.Services.Add(lifecycle.Item{
			Name:  "metrics",
			Run:   peer.Metrics.Chore.Run,
			Close: peer.Metrics.Chore.Close,
		})
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Metrics", peer.Metrics.Chore.Loop))
	}

	return peer, nil
}

// Run runs satellite garbage collection until it's either closed or it errors.
func (peer *GarbageCollection) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	group, ctx := errgroup.WithContext(ctx)

	peer.Servers.Run(ctx, group)
	peer.Services.Run(ctx, group)

	return group.Wait()
}

// Close closes all the resources.
func (peer *GarbageCollection) Close() error {
	return errs.Combine(
		peer.Servers.Close(),
		peer.Services.Close(),
	)
}

// ID returns the peer ID.
func (peer *GarbageCollection) ID() storj.NodeID { return peer.Identity.ID }
