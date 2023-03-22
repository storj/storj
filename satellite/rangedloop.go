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
	"storj.io/storj/satellite/accounting/nodetally"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/gc/bloomfilter"
	"storj.io/storj/satellite/gracefulexit"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/metrics"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair/checker"
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

	Overlay struct {
		Service *overlay.Service
	}

	Repair struct {
		Observer rangedloop.Observer
	}

	GracefulExit struct {
		Observer rangedloop.Observer
	}

	GarbageCollectionBF struct {
		Observer rangedloop.Observer
	}

	Accounting struct {
		NodeTallyObserver *nodetally.RangedLoopObserver
	}

	RangedLoop struct {
		Service *rangedloop.Service
	}
}

// NewRangedLoop creates a new satellite ranged loop process.
func NewRangedLoop(log *zap.Logger, db DB, metabaseDB *metabase.DB, config *Config, atomicLogLevel *zap.AtomicLevel) (_ *RangedLoop, err error) {
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

	{ // setup node tally observer
		peer.Accounting.NodeTallyObserver = nodetally.NewRangedLoopObserver(
			log.Named("accounting:nodetally"),
			db.StoragenodeAccounting(),
			metabaseDB)
	}

	{ // setup overlay
		peer.Overlay.Service, err = overlay.NewService(peer.Log.Named("overlay"), peer.DB.OverlayCache(), peer.DB.NodeEvents(), config.Console.ExternalAddress, config.Console.SatelliteName, config.Overlay)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
		peer.Services.Add(lifecycle.Item{
			Name:  "overlay",
			Run:   peer.Overlay.Service.Run,
			Close: peer.Overlay.Service.Close,
		})
	}

	{ // setup repair
		peer.Repair.Observer = checker.NewRangedLoopObserver(
			peer.Log.Named("repair:checker"),
			peer.DB.RepairQueue(),
			peer.Overlay.Service,
			config.Checker,
		)
	}

	{ // setup garbage collection bloom filter observer
		peer.GarbageCollectionBF.Observer = bloomfilter.NewObserver(log.Named("gc-bf"), config.GarbageCollectionBF, db.OverlayCache())
	}

	{ // setup ranged loop
		observers := []rangedloop.Observer{
			rangedloop.NewLiveCountObserver(metabaseDB, config.RangedLoop.SuspiciousProcessedRatio, config.RangedLoop.AsOfSystemInterval),
		}

		if config.Audit.UseRangedLoop {
			observers = append(observers, peer.Audit.Observer)
		}

		if config.Metrics.UseRangedLoop {
			observers = append(observers, peer.Metrics.Observer)
		}

		if config.Tally.UseRangedLoop {
			observers = append(observers, peer.Accounting.NodeTallyObserver)
		}

		if config.GracefulExit.Enabled && config.GracefulExit.UseRangedLoop {
			observers = append(observers, peer.GracefulExit.Observer)
		}

		if config.GarbageCollectionBF.Enabled && config.GarbageCollectionBF.UseRangedLoop {
			observers = append(observers, peer.GarbageCollectionBF.Observer)
		}

		if config.Repairer.UseRangedLoop {
			observers = append(observers, peer.Repair.Observer)
		}

		segments := rangedloop.NewMetabaseRangeSplitter(metabaseDB, config.RangedLoop.AsOfSystemInterval, config.RangedLoop.BatchSize)
		peer.RangedLoop.Service = rangedloop.NewService(log.Named("rangedloop"), config.RangedLoop, segments, observers)

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
