// Copyright (C) 2019 Storj Labs, Inc.
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

	"storj.io/common/debug"
	"storj.io/common/identity"
	"storj.io/common/peertls/extensions"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/version"
	"storj.io/storj/private/lifecycle"
	version_checker "storj.io/storj/private/version/checker"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeevents"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/satellite/repair/repairer"
	"storj.io/storj/satellite/reputation"
)

// Repairer is the repairer process.
//
// architecture: Peer
type Repairer struct {
	Log      *zap.Logger
	Identity *identity.FullIdentity

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

	Overlay    *overlay.Service
	Reputation *reputation.Service
	Orders     struct {
		Service *orders.Service
	}

	Audit struct {
		Reporter audit.Reporter
	}

	EcRepairer      *repairer.ECRepairer
	SegmentRepairer *repairer.SegmentRepairer
	Queue           queue.RepairQueue
	Repairer        *repairer.Service
}

// NewRepairer creates a new repairer peer.
func NewRepairer(log *zap.Logger, full *identity.FullIdentity,
	metabaseDB *metabase.DB,
	revocationDB extensions.RevocationDB,
	repairQueue queue.RepairQueue,
	bucketsDB buckets.DB,
	overlayCache overlay.DB,
	nodeEvents nodeevents.DB,
	reputationdb reputation.DB,
	containmentDB audit.Containment,
	versionInfo version.Info, config *Config, atomicLogLevel *zap.AtomicLevel,
) (*Repairer, error) {
	peer := &Repairer{
		Log:      log,
		Identity: full,

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
		debugConfig.ControlTitle = "Repair"
		peer.Debug.Server = debug.NewServerWithAtomicLevel(log.Named("debug"), peer.Debug.Listener, monkit.Default, debugConfig, atomicLogLevel)
		peer.Servers.Add(lifecycle.Item{
			Name:  "debug",
			Run:   peer.Debug.Server.Run,
			Close: peer.Debug.Server.Close,
		})
	}

	{
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

	{ // setup dialer
		sc := config.Server

		tlsOptions, err := tlsopts.NewOptions(peer.Identity, sc.Config, revocationDB)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Dialer = rpc.NewDefaultDialer(tlsOptions)
		peer.Dialer.DialTimeout = config.Repairer.DialTimeout
	}

	{ // setup overlay
		placement, err := config.Placement.Parse(config.Overlay.Node.CreateDefaultPlacement, nil)
		if err != nil {
			return nil, err
		}

		peer.Overlay, err = overlay.NewService(log.Named("overlay"), overlayCache, nodeEvents, placement, config.Console.ExternalAddress, config.Console.SatelliteName, config.Overlay)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
		peer.Services.Add(lifecycle.Item{
			Name:  "overlay",
			Run:   peer.Overlay.Run,
			Close: peer.Overlay.Close,
		})
	}

	{ // setup reputation
		if config.Reputation.FlushInterval > 0 {
			cachingDB := reputation.NewCachingDB(log.Named("reputation:writecache"), reputationdb, config.Reputation)
			peer.Services.Add(lifecycle.Item{
				Name: "reputation:writecache",
				Run:  cachingDB.Manage,
			})
			reputationdb = cachingDB
		}
		peer.Reputation = reputation.NewService(log.Named("reputation:service"),
			peer.Overlay,
			reputationdb,
			config.Reputation,
		)

		peer.Services.Add(lifecycle.Item{
			Name:  "reputation",
			Close: peer.Reputation.Close,
		})
	}

	{ // setup orders
		placement, err := config.Placement.Parse(config.Overlay.Node.CreateDefaultPlacement, nil)
		if err != nil {
			return nil, err
		}

		peer.Orders.Service, err = orders.NewService(
			log.Named("orders"),
			signing.SignerFromFullIdentity(peer.Identity),
			peer.Overlay,
			// orders service needs DB only for handling
			// PUT and GET actions which are not used by
			// repairer so we can set noop implementation.
			orders.NewNoopDB(),
			placement.CreateFilters,
			config.Orders,
		)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
	}

	{ // setup audit
		peer.Audit.Reporter = audit.NewReporter(
			log.Named("reporter"),
			peer.Reputation,
			peer.Overlay,
			metabaseDB,
			containmentDB,
			config.Audit)
	}

	{ // setup repairer
		placement, err := config.Placement.Parse(config.Overlay.Node.CreateDefaultPlacement, nil)
		if err != nil {
			return nil, err
		}

		peer.EcRepairer = repairer.NewECRepairer(
			peer.Dialer,
			signing.SigneeFromPeerIdentity(peer.Identity.PeerIdentity()),
			config.Repairer.DialTimeout,
			config.Repairer.DownloadTimeout,
			config.Repairer.InMemoryRepair,
			config.Repairer.InMemoryUpload,
			config.Repairer.DownloadLongTail,
		)

		if len(config.Repairer.RepairExcludedCountryCodes) == 0 {
			config.Repairer.RepairExcludedCountryCodes = config.Overlay.RepairExcludedCountryCodes
		}

		peer.SegmentRepairer, err = repairer.NewSegmentRepairer(
			log.Named("segment-repair"),
			metabaseDB,
			peer.Orders.Service,
			peer.Overlay,
			peer.Audit.Reporter,
			peer.EcRepairer,
			placement,
			config.Checker.RepairThresholdOverrides,
			config.Checker.RepairTargetOverrides,
			config.Repairer,
		)
		if err != nil {
			return nil, err
		}
		peer.Services.Add(lifecycle.Item{
			Name:  "segment-repair",
			Run:   peer.SegmentRepairer.Run,
			Close: peer.Repairer.Close,
		})

		peer.Queue = repairQueue
		peer.Repairer = repairer.NewService(log.Named("repairer"), repairQueue, &config.Repairer, peer.SegmentRepairer)

		peer.Services.Add(lifecycle.Item{
			Name:  "repair",
			Run:   peer.Repairer.Run,
			Close: peer.Repairer.Close,
		})
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Repair Worker", peer.Repairer.Loop))

	}

	return peer, nil
}

// Run runs the repair process until it's either closed or it errors.
func (peer *Repairer) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	group, ctx := errgroup.WithContext(ctx)

	pprof.Do(ctx, pprof.Labels("subsystem", "repairer"), func(ctx context.Context) {
		peer.Servers.Run(ctx, group)
		peer.Services.Run(ctx, group)

		pprof.Do(ctx, pprof.Labels("name", "subsystem-wait"), func(ctx context.Context) {
			err = group.Wait()
		})
	})
	return err
}

// Close closes all the resources.
func (peer *Repairer) Close() error {
	return errs.Combine(
		peer.Servers.Close(),
		peer.Services.Close(),
	)
}

// ID returns the peer ID.
func (peer *Repairer) ID() storj.NodeID { return peer.Identity.ID }
