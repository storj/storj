// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"
	"errors"
	"math/rand"
	"net"
	"runtime/pprof"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/debug"
	"storj.io/storj/private/lifecycle"
	"storj.io/storj/satellite/accounting/nodetally"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/durability"
	"storj.io/storj/satellite/gc/piecetracker"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/metrics"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair/checker"
	"storj.io/storj/satellite/repair/queue"
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
		Queue    queue.RepairQueue
		Observer *checker.Observer
	}

	Accounting struct {
		NodeTallyObserver *nodetally.Observer
	}

	PieceTracker struct {
		Observer *piecetracker.Observer
	}

	DurabilityReport struct {
		Observer []*durability.Report
	}

	RangedLoop struct {
		Service *rangedloop.Service
	}
}

// NewRangedLoop creates a new satellite ranged loop process.
func NewRangedLoop(log *zap.Logger, db DB, metabaseDB *metabase.DB, repairQueue queue.RepairQueue, config *Config, atomicLogLevel *zap.AtomicLevel) (_ *RangedLoop, err error) {
	peer := &RangedLoop{
		Log: log,
		DB:  db,

		Servers:  lifecycle.NewGroup(log.Named("servers")),
		Services: lifecycle.NewGroup(log.Named("services")),
	}

	{ // setup debug
		var err error
		if config.Debug.Addr != "" {
			peer.Debug.Listener, err = net.Listen("tcp", config.Debug.Addr)
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
		var nodeSet audit.AuditedNodes
		if config.Audit.NodeFilter != "" {
			filter, err := nodeselection.FilterFromString(config.Audit.NodeFilter, nil)
			if err != nil {
				return nil, err
			}
			nodeSet = audit.NewFilteredNodes(filter, db.OverlayCache(), metabaseDB)
		}
		peer.Audit.Observer = audit.NewObserver(log.Named("audit"), nodeSet, db.VerifyQueue(), config.Audit)
	}

	{ // setup metrics observer
		peer.Metrics.Observer = metrics.NewObserver()
	}

	{ // setup node tally observer
		peer.Accounting.NodeTallyObserver = nodetally.NewObserver(
			log.Named("accounting:nodetally"),
			db.StoragenodeAccounting(),
			metabaseDB, config.NodeTally)
	}

	{ // setup piece tracker observer
		peer.PieceTracker.Observer = piecetracker.NewObserver(
			log.Named("piecetracker"),
			metabaseDB,
			peer.DB.OverlayCache(),
			config.PieceTracker,
		)
	}

	{ // setup overlay
		placement, err := config.Placement.Parse(config.Overlay.Node.CreateDefaultPlacement, nil)
		if err != nil {
			return nil, err
		}

		peer.Overlay.Service, err = overlay.NewService(peer.Log.Named("overlay"), peer.DB.OverlayCache(), peer.DB.NodeEvents(), placement, config.Console.ExternalAddress, config.Console.SatelliteName, config.Overlay)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
		peer.Services.Add(lifecycle.Item{
			Name:  "overlay",
			Run:   peer.Overlay.Service.Run,
			Close: peer.Overlay.Service.Close,
		})
	}

	{ // setup
		classes, err := config.Durability.CreateNodeClassifiers()
		if err != nil {
			return nil, err
		}

		for class, f := range classes {
			cache := checker.NewReliabilityCache(peer.Overlay.Service, config.Checker.ReliabilityCacheStaleness, config.Checker.OnlineWindow)
			peer.DurabilityReport.Observer = append(peer.DurabilityReport.Observer, durability.NewDurability(db.OverlayCache(), metabaseDB, cache, class, f, config.RangedLoop.AsOfSystemInterval))
		}
	}

	{ // setup repair
		placement, err := config.Placement.Parse(config.Overlay.Node.CreateDefaultPlacement, nil)
		if err != nil {
			return nil, err
		}

		if len(config.Checker.RepairExcludedCountryCodes) == 0 {
			config.Checker.RepairExcludedCountryCodes = config.Overlay.RepairExcludedCountryCodes
		}

		peer.Repair.Queue = repairQueue

		reliabilityCache := checker.NewReliabilityCache(peer.Overlay.Service, config.Checker.ReliabilityCacheStaleness, config.Checker.OnlineWindow)
		var health checker.Health
		switch config.Checker.HealthScore {
		case "probability":
			health = checker.NewProbabilityHealth(config.Checker.NodeFailureRate, reliabilityCache)
		case "normalized":
			health = checker.NewNormalizedHealth()
		default:
			panic("invalid health score: " + config.Checker.HealthScore)
		}

		peer.Repair.Observer = checker.NewObserver(
			peer.Log.Named("repair:checker"),
			peer.Repair.Queue,
			peer.Overlay.Service,
			placement,
			config.Checker,
			health,
		)
	}

	{ // setup ranged loop
		rand := rand.New(rand.NewSource(time.Now().UnixNano()))

		observers := []rangedloop.Observer{
			rangedloop.NewLiveCountObserver(metabaseDB, config.RangedLoop.SuspiciousProcessedRatio, config.RangedLoop.AsOfSystemInterval),
			peer.Metrics.Observer,
		}

		if config.Audit.UseRangedLoop {
			observers = append(observers, peer.Audit.Observer)
		}

		if config.Tally.UseRangedLoop {
			observers = append(observers, peer.Accounting.NodeTallyObserver)
		}

		if config.Repairer.UseRangedLoop {
			observers = append(observers, peer.Repair.Observer)
		}

		if config.PieceTracker.UseRangedLoop {
			observers = append(observers, peer.PieceTracker.Observer)
		}

		if config.DurabilityReport.Enabled {
			sequenceObservers := []rangedloop.Observer{}
			for _, observer := range peer.DurabilityReport.Observer {
				sequenceObservers = append(sequenceObservers, observer)
			}

			// shuffle observers list to be sure that each observer will be executed first from time to time
			rand.Shuffle(len(sequenceObservers), func(i, j int) {
				sequenceObservers[i], sequenceObservers[j] = sequenceObservers[j], sequenceObservers[i]
			})
			observers = append(observers, rangedloop.NewSequenceObserver(sequenceObservers...))
		}

		segments := rangedloop.NewMetabaseRangeSplitter(log.Named("rangedloop-metabase-range-splitter"), metabaseDB, config.RangedLoop)
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
