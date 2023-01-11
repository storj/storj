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

	"storj.io/private/debug"
	"storj.io/storj/private/lifecycle"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/gc/bloomfilter"
	"storj.io/storj/satellite/gracefulexit"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/metrics"
)

// RangedLoop is the satellite ranged loop process.
//
// architecture: Peer
type RangedLoop struct {
	Log *zap.Logger
	DB  DB

	Servers  *lifecycle.Group
	Services *lifecycle.Group

	Audit struct {
		Observer rangedloop.Observer
	}

	Debug struct {
		Listener net.Listener
		Server   *debug.Server
	}

	Metrics struct {
		Observer rangedloop.Observer
	}

	GracefulExit struct {
		Observer rangedloop.Observer
	}

	GarbageCollectionBF struct {
		Observer rangedloop.Observer
	}

	RangedLoop struct {
		Service *rangedloop.Service
	}
}

// NewRangedLoop creates a new satellite ranged loop process.
func NewRangedLoop(log *zap.Logger, db DB, metabaseDB *metabase.DB, config *Config, atomicLogLevel *zap.AtomicLevel) (*RangedLoop, error) {
	peer := &RangedLoop{
		Log: log,
		DB:  db,

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
		debugConfig.ControlTitle = "RangedLoop"
		peer.Debug.Server = debug.NewServerWithAtomicLevel(log.Named("debug"), peer.Debug.Listener, monkit.Default, debugConfig, atomicLogLevel)
		peer.Servers.Add(lifecycle.Item{
			Name:  "debug",
			Run:   peer.Debug.Server.Run,
			Close: peer.Debug.Server.Close,
		})
	}

	{ // setup audit observer
		peer.Audit.Observer = audit.NewObserver(log.Named("audit"), db.VerifyQueue(), config.Audit)
	}

	{ // setup metrics observer
		peer.Metrics.Observer = metrics.NewObserver()
	}

	{ // setup gracefulexit
		peer.GracefulExit.Observer = gracefulexit.NewObserver(
			peer.Log.Named("gracefulexit:observer"),
			peer.DB.GracefulExit(),
			peer.DB.OverlayCache(),
			config.GracefulExit,
		)
	}

	{ // setup garbage collection bloom filter observer
		peer.GarbageCollectionBF.Observer = bloomfilter.NewObserver(log.Named("gc-bf"), config.GarbageCollectionBF, db.OverlayCache())
	}

	{ // setup ranged loop
		observers := []rangedloop.Observer{
			rangedloop.NewLiveCountObserver(),
		}

		if config.Audit.UseRangedLoop {
			observers = append(observers, peer.Audit.Observer)
		}

		if config.Metrics.UseRangedLoop {
			observers = append(observers, peer.Metrics.Observer)
		}

		if config.GracefulExit.Enabled && config.GracefulExit.UseRangedLoop {
			observers = append(observers, peer.GracefulExit.Observer)
		}

		if config.GarbageCollectionBF.Enabled && config.GarbageCollectionBF.UseRangedLoop {
			observers = append(observers, peer.GarbageCollectionBF.Observer)
		}

		segments := rangedloop.NewMetabaseRangeSplitter(metabaseDB, config.RangedLoop.AsOfSystemInterval, config.RangedLoop.BatchSize)
		peer.RangedLoop.Service = rangedloop.NewService(log.Named("rangedloop"), config.RangedLoop, &segments, observers)

		peer.Services.Add(lifecycle.Item{
			Name: "rangeloop",
			Run:  peer.RangedLoop.Service.Run,
		})
	}

	return peer, nil
}

// Run runs satellite ranged loop until it's either closed or it errors.
func (peer *RangedLoop) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	group, ctx := errgroup.WithContext(ctx)

	pprof.Do(ctx, pprof.Labels("subsystem", "rangedloop"), func(ctx context.Context) {
		peer.Servers.Run(ctx, group)
		peer.Services.Run(ctx, group)

		pprof.Do(ctx, pprof.Labels("name", "subsystem-wait"), func(ctx context.Context) {
			err = group.Wait()
		})
	})
	return err
}

// Close closes all the resources.
func (peer *RangedLoop) Close() error {
	return errs.Combine(
		peer.Servers.Close(),
		peer.Services.Close(),
	)
}
