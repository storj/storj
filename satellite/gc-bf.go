// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"
	"errors"
	"net"
	"runtime/pprof"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/identity"
	"storj.io/common/peertls/extensions"
	"storj.io/common/storj"
	"storj.io/private/debug"
	"storj.io/private/version"
	"storj.io/storj/private/lifecycle"
	version_checker "storj.io/storj/private/version/checker"
	"storj.io/storj/satellite/gc/bloomfilter"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/segmentloop"
	"storj.io/storj/satellite/overlay"
)

// GarbageCollectionBF is the satellite garbage collection process which collects bloom filters.
//
// architecture: Peer
type GarbageCollectionBF struct {
	Log      *zap.Logger
	Identity *identity.FullIdentity
	DB       DB

	Servers  *lifecycle.Group
	Services *lifecycle.Group

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
		SegmentLoop *segmentloop.Service
	}

	GarbageCollection struct {
		Service *bloomfilter.Service
	}
}

// NewGarbageCollectionBF creates a new satellite garbage collection peer which collects storage nodes bloom filters.
func NewGarbageCollectionBF(log *zap.Logger, full *identity.FullIdentity, db DB,
	metabaseDB *metabase.DB, revocationDB extensions.RevocationDB,
	versionInfo version.Info, config *Config, atomicLogLevel *zap.AtomicLevel) (*GarbageCollectionBF, error) {
	peer := &GarbageCollectionBF{
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
			}
		}
		debugConfig := config.Debug
		debugConfig.ControlTitle = "GC-BloomFilter"
		peer.Debug.Server = debug.NewServerWithAtomicLevel(log.Named("debug"), peer.Debug.Listener, monkit.Default, debugConfig, atomicLogLevel)
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

	{ // setup overlay
		peer.Overlay.DB = peer.DB.OverlayCache()
	}

	{ // setup metainfo
		peer.Metainfo.SegmentLoop = segmentloop.New(
			log.Named("segmentloop"),
			config.Metainfo.SegmentLoop,
			metabaseDB,
		)
		peer.Services.Add(lifecycle.Item{
			Name:  "metainfo:segmentloop",
			Run:   peer.Metainfo.SegmentLoop.Run,
			Close: peer.Metainfo.SegmentLoop.Close,
		})
	}

	{ // setup garbage collection bloom filters
		peer.GarbageCollection.Service = bloomfilter.NewService(
			peer.Log.Named("garbage-collection-bf"),
			config.GarbageCollectionBF,
			peer.Overlay.DB,
			peer.Metainfo.SegmentLoop,
		)
		peer.Services.Add(lifecycle.Item{
			Name: "garbage-collection-bf",
			Run:  peer.GarbageCollection.Service.Run,
		})
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Garbage Collection Bloom Filters", peer.GarbageCollection.Service.Loop))
	}

	return peer, nil
}

// Run runs satellite garbage collection until it's either closed or it errors.
func (peer *GarbageCollectionBF) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	group, ctx := errgroup.WithContext(ctx)

	pprof.Do(ctx, pprof.Labels("subsystem", "gc-bloomfilter"), func(ctx context.Context) {
		peer.Servers.Run(ctx, group)
		peer.Services.Run(ctx, group)

		pprof.Do(ctx, pprof.Labels("name", "subsystem-wait"), func(ctx context.Context) {
			err = group.Wait()
		})
	})
	return err
}

// Close closes all the resources.
func (peer *GarbageCollectionBF) Close() error {
	return errs.Combine(
		peer.Servers.Close(),
		peer.Services.Close(),
	)
}

// ID returns the peer ID.
func (peer *GarbageCollectionBF) ID() storj.NodeID { return peer.Identity.ID }
