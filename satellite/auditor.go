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
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeevents"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/reputation"
)

// Auditor is the auditor process.
//
// architecture: Peer
type Auditor struct {
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

	Mail       *mailservice.Service
	Overlay    *overlay.Service
	Reputation *reputation.Service
	Orders     struct {
		Service *orders.Service
	}

	Audit struct {
		Verifier       *audit.Verifier
		Reverifier     *audit.Reverifier
		VerifyQueue    audit.VerifyQueue
		ReverifyQueue  audit.ReverifyQueue
		Reporter       audit.Reporter
		Worker         *audit.Worker
		ReverifyWorker *audit.ReverifyWorker
	}
}

// NewAuditor creates a new auditor peer.
func NewAuditor(log *zap.Logger, full *identity.FullIdentity,
	metabaseDB *metabase.DB,
	revocationDB extensions.RevocationDB,
	verifyQueue audit.VerifyQueue,
	reverifyQueue audit.ReverifyQueue,
	overlayCache overlay.DB,
	nodeEvents nodeevents.DB,
	reputationdb reputation.DB,
	containmentDB audit.Containment,
	versionInfo version.Info, config *Config, atomicLogLevel *zap.AtomicLevel,
) (*Auditor, error) {
	peer := &Auditor{
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
		debugConfig.ControlTitle = "Audit"
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

	{ // setup dialer
		sc := config.Server

		tlsOptions, err := tlsopts.NewOptions(peer.Identity, sc.Config, revocationDB)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Dialer = rpc.NewDefaultDialer(tlsOptions)
	}

	placement, err := config.Placement.Parse(config.Overlay.Node.CreateDefaultPlacement, nil)
	if err != nil {
		return nil, err
	}

	{ // setup overlay
		var err error
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
			// auditor so we can set noop implementation.
			orders.NewNoopDB(),
			placement.CreateFilters,
			config.Orders,
		)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
	}

	{ // setup audit
		// force tcp for now because audit is very sensitive to how errors
		// are returned, and adding quic can cause problems
		dialer := peer.Dialer
		//lint:ignore SA1019 deprecated is fine here.
		//nolint:staticcheck // deprecated is fine here.
		dialer.Connector = rpc.NewDefaultTCPConnector(nil)

		peer.Audit.VerifyQueue = verifyQueue
		peer.Audit.ReverifyQueue = reverifyQueue

		peer.Audit.Verifier = audit.NewVerifier(log.Named("audit:verifier"),
			metabaseDB,
			dialer,
			peer.Overlay,
			containmentDB,
			peer.Orders.Service,
			peer.Identity,
			config.Audit.MinBytesPerSecond,
			config.Audit.MinDownloadTimeout)
		peer.Audit.Reverifier = audit.NewReverifier(log.Named("audit:reverifier"),
			peer.Audit.Verifier,
			reverifyQueue,
			config.Audit)

		peer.Audit.Reporter = audit.NewReporter(
			log.Named("reporter"),
			peer.Reputation,
			peer.Overlay,
			metabaseDB,
			containmentDB,
			config.Audit)

		peer.Audit.Worker = audit.NewWorker(log.Named("audit:verify-worker"),
			verifyQueue,
			peer.Audit.Verifier,
			reverifyQueue,
			peer.Audit.Reporter,
			config.Audit)
		peer.Services.Add(lifecycle.Item{
			Name:  "audit:verify-worker",
			Run:   peer.Audit.Worker.Run,
			Close: peer.Audit.Worker.Close,
		})
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Audit Verify Worker", peer.Audit.Worker.Loop))

		peer.Audit.ReverifyWorker = audit.NewReverifyWorker(peer.Log.Named("audit:reverify-worker"),
			reverifyQueue,
			peer.Audit.Reverifier,
			peer.Audit.Reporter,
			config.Audit)
		peer.Services.Add(lifecycle.Item{
			Name:  "audit:reverify-worker",
			Run:   peer.Audit.ReverifyWorker.Run,
			Close: peer.Audit.ReverifyWorker.Close,
		})
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Audit Reverify Worker", peer.Audit.ReverifyWorker.Loop))
	}

	return peer, nil
}

// Run runs the auditor process until it's either closed or it errors.
func (peer *Auditor) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	group, ctx := errgroup.WithContext(ctx)

	pprof.Do(ctx, pprof.Labels("subsystem", "auditor"), func(ctx context.Context) {
		peer.Servers.Run(ctx, group)
		peer.Services.Run(ctx, group)

		pprof.Do(ctx, pprof.Labels("name", "subsystem-wait"), func(ctx context.Context) {
			err = group.Wait()
		})
	})
	return err
}

// Close closes all the resources.
func (peer *Auditor) Close() error {
	return errs.Combine(
		peer.Servers.Close(),
		peer.Services.Close(),
	)
}

// ID returns the peer ID.
func (peer *Auditor) ID() storj.NodeID { return peer.Identity.ID }
