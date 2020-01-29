// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/peertls/extensions"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/storj/private/lifecycle"
	"storj.io/storj/private/version"
	version_checker "storj.io/storj/private/version/checker"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/accounting/reportedrollup"
	"storj.io/storj/satellite/accounting/rollup"
	"storj.io/storj/satellite/accounting/tally"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/contact"
	"storj.io/storj/satellite/dbcleanup"
	"storj.io/storj/satellite/downtime"
	"storj.io/storj/satellite/gc"
	"storj.io/storj/satellite/gracefulexit"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/metrics"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/mockpayments"
	"storj.io/storj/satellite/payments/stripecoinpayments"
	"storj.io/storj/satellite/repair/checker"
)

// Core is the satellite core process that runs chores
//
// architecture: Peer
type Core struct {
	// core dependencies
	Log      *zap.Logger
	Identity *identity.FullIdentity
	DB       DB

	Servers  *lifecycle.Group
	Services *lifecycle.Group

	Dialer rpc.Dialer

	Version *version_checker.Service

	// services and endpoints
	Contact struct {
		Service *contact.Service
	}

	Overlay struct {
		DB      overlay.DB
		Service *overlay.Service
	}

	Metainfo struct {
		Database metainfo.PointerDB // TODO: move into pointerDB
		Service  *metainfo.Service
		Loop     *metainfo.Loop
	}

	Orders struct {
		DB      orders.DB
		Service *orders.Service
		Chore   *orders.Chore
	}

	Repair struct {
		Checker *checker.Checker
	}
	Audit struct {
		Queue    *audit.Queue
		Worker   *audit.Worker
		Chore    *audit.Chore
		Verifier *audit.Verifier
		Reporter *audit.Reporter
	}

	GarbageCollection struct {
		Service *gc.Service
	}

	DBCleanup struct {
		Chore *dbcleanup.Chore
	}

	Accounting struct {
		Tally               *tally.Service
		Rollup              *rollup.Service
		ProjectUsage        *accounting.Service
		ReportedRollupChore *reportedrollup.Chore
	}

	LiveAccounting struct {
		Cache accounting.Cache
	}

	Payments struct {
		Accounts payments.Accounts
		Chore    *stripecoinpayments.Chore
	}

	GracefulExit struct {
		Chore *gracefulexit.Chore
	}

	Metrics struct {
		Chore *metrics.Chore
	}

	DowntimeTracking struct {
		DetectionChore  *downtime.DetectionChore
		EstimationChore *downtime.EstimationChore
		Service         *downtime.Service
	}
}

// New creates a new satellite
func New(log *zap.Logger, full *identity.FullIdentity, db DB,
	pointerDB metainfo.PointerDB, revocationDB extensions.RevocationDB, liveAccounting accounting.Cache,
	rollupsWriteCache *orders.RollupsWriteCache,
	versionInfo version.Info, config *Config) (*Core, error) {
	peer := &Core{
		Log:      log,
		Identity: full,
		DB:       db,

		Servers:  lifecycle.NewGroup(log.Named("servers")),
		Services: lifecycle.NewGroup(log.Named("services")),
	}

	var err error

	{ // setup version control
		if !versionInfo.IsZero() {
			peer.Log.Sugar().Debugf("Binary Version: %s with CommitHash %s, built at %s as Release %v",
				versionInfo.Version.String(), versionInfo.CommitHash, versionInfo.Timestamp.String(), versionInfo.Release)
		}
		peer.Version = version_checker.NewService(log.Named("version"), config.Version, versionInfo, "Satellite")

		peer.Services.Add(lifecycle.Item{
			Name: "version",
			Run:  peer.Version.Run,
		})
	}

	{ // setup listener and server
		log.Debug("Starting listener and server")
		sc := config.Server

		tlsOptions, err := tlsopts.NewOptions(peer.Identity, sc.Config, revocationDB)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Dialer = rpc.NewDefaultDialer(tlsOptions)
	}

	{ // setup contact service
		pbVersion, err := versionInfo.Proto()
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		self := &overlay.NodeDossier{
			Node: pb.Node{
				Id: peer.ID(),
				Address: &pb.NodeAddress{
					Address: config.Contact.ExternalAddress,
				},
			},
			Type:    pb.NodeType_SATELLITE,
			Version: *pbVersion,
		}
		peer.Contact.Service = contact.NewService(peer.Log.Named("contact:service"), self, peer.Overlay.Service, peer.DB.PeerIdentities(), peer.Dialer)
		peer.Services.Add(lifecycle.Item{
			Name:  "contact:service",
			Close: peer.Contact.Service.Close,
		})
	}

	{ // setup overlay
		peer.Overlay.DB = overlay.NewCombinedCache(peer.DB.OverlayCache())
		peer.Overlay.Service = overlay.NewService(peer.Log.Named("overlay"), peer.Overlay.DB, config.Overlay)
		peer.Services.Add(lifecycle.Item{
			Name:  "overlay",
			Close: peer.Overlay.Service.Close,
		})
	}

	{ // setup live accounting
		peer.LiveAccounting.Cache = liveAccounting
	}

	{ // setup accounting project usage
		peer.Accounting.ProjectUsage = accounting.NewService(
			peer.DB.ProjectAccounting(),
			peer.LiveAccounting.Cache,
			config.Rollup.MaxAlphaUsage,
		)
	}

	{ // setup orders
		peer.Orders.DB = rollupsWriteCache
		peer.Orders.Chore = orders.NewChore(log.Named("orders:chore"), rollupsWriteCache, config.Orders)
		peer.Services.Add(lifecycle.Item{
			Name:  "overlay",
			Run:   peer.Orders.Chore.Run,
			Close: peer.Orders.Chore.Close,
		})
		peer.Orders.Service = orders.NewService(
			peer.Log.Named("orders:service"),
			signing.SignerFromFullIdentity(peer.Identity),
			peer.Overlay.Service,
			peer.Orders.DB,
			config.Orders.Expiration,
			&pb.NodeAddress{
				Transport: pb.NodeTransport_TCP_TLS_GRPC,
				Address:   config.Contact.ExternalAddress,
			},
			config.Repairer.MaxExcessRateOptimalThreshold,
			config.Orders.NodeStatusLogging,
		)
	}

	{ // setup metainfo
		peer.Metainfo.Database = pointerDB // for logging: storelogger.New(peer.Log.Named("pdb"), db)
		peer.Metainfo.Service = metainfo.NewService(peer.Log.Named("metainfo:service"),
			peer.Metainfo.Database,
			peer.DB.Buckets(),
		)
		peer.Metainfo.Loop = metainfo.NewLoop(config.Metainfo.Loop, peer.Metainfo.Database)
		peer.Services.Add(lifecycle.Item{
			Name:  "metainfo:loop",
			Run:   peer.Metainfo.Loop.Run,
			Close: peer.Metainfo.Loop.Close,
		})
	}

	{ // setup datarepair
		// TODO: simplify argument list somehow
		peer.Repair.Checker = checker.NewChecker(
			peer.Log.Named("repair:checker"),
			peer.DB.RepairQueue(),
			peer.DB.Irreparable(),
			peer.Metainfo.Service,
			peer.Metainfo.Loop,
			peer.Overlay.Service,
			config.Checker)
		peer.Services.Add(lifecycle.Item{
			Name:  "repair:checker",
			Run:   peer.Repair.Checker.Run,
			Close: peer.Repair.Checker.Close,
		})
	}

	{ // setup audit
		config := config.Audit

		peer.Audit.Queue = &audit.Queue{}

		peer.Audit.Verifier = audit.NewVerifier(log.Named("audit:verifier"),
			peer.Metainfo.Service,
			peer.Dialer,
			peer.Overlay.Service,
			peer.DB.Containment(),
			peer.Orders.Service,
			peer.Identity,
			config.MinBytesPerSecond,
			config.MinDownloadTimeout,
		)

		peer.Audit.Reporter = audit.NewReporter(log.Named("audit:reporter"),
			peer.Overlay.Service,
			peer.DB.Containment(),
			config.MaxRetriesStatDB,
			int32(config.MaxReverifyCount),
		)

		peer.Audit.Worker, err = audit.NewWorker(peer.Log.Named("audit:worker"),
			peer.Audit.Queue,
			peer.Audit.Verifier,
			peer.Audit.Reporter,
			config,
		)
		peer.Services.Add(lifecycle.Item{
			Name:  "audit:worker",
			Run:   peer.Audit.Worker.Run,
			Close: peer.Audit.Worker.Close,
		})

		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Audit.Chore = audit.NewChore(peer.Log.Named("audit:chore"),
			peer.Audit.Queue,
			peer.Metainfo.Loop,
			config,
		)
		peer.Services.Add(lifecycle.Item{
			Name:  "audit:chore",
			Run:   peer.Audit.Chore.Run,
			Close: peer.Audit.Chore.Close,
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
	}

	{ // setup db cleanup
		peer.DBCleanup.Chore = dbcleanup.NewChore(peer.Log.Named("dbcleanup"), peer.DB.Orders(), config.DBCleanup)
		peer.Services.Add(lifecycle.Item{
			Name:  "dbcleanup",
			Run:   peer.DBCleanup.Chore.Run,
			Close: peer.DBCleanup.Chore.Close,
		})
	}

	{ // setup accounting
		peer.Accounting.Tally = tally.New(peer.Log.Named("accounting:tally"), peer.DB.StoragenodeAccounting(), peer.DB.ProjectAccounting(), peer.LiveAccounting.Cache, peer.Metainfo.Loop, config.Tally.Interval)
		peer.Services.Add(lifecycle.Item{
			Name:  "accounting:tally",
			Run:   peer.Accounting.Tally.Run,
			Close: peer.Accounting.Tally.Close,
		})

		peer.Accounting.Rollup = rollup.New(peer.Log.Named("accounting:rollup"), peer.DB.StoragenodeAccounting(), config.Rollup.Interval, config.Rollup.DeleteTallies)
		peer.Services.Add(lifecycle.Item{
			Name:  "accounting:rollup",
			Run:   peer.Accounting.Rollup.Run,
			Close: peer.Accounting.Rollup.Close,
		})

		peer.Accounting.ReportedRollupChore = reportedrollup.NewChore(peer.Log.Named("accounting:reported-rollup"), peer.DB.Orders(), config.ReportedRollup)
		peer.Services.Add(lifecycle.Item{
			Name:  "accounting:reported-rollup",
			Run:   peer.Accounting.ReportedRollupChore.Run,
			Close: peer.Accounting.ReportedRollupChore.Close,
		})
	}

	// TODO: remove in future, should be in API
	{ // setup payments
		pc := config.Payments

		switch pc.Provider {
		default:
			peer.Payments.Accounts = mockpayments.Accounts()
		case "stripecoinpayments":
			service := stripecoinpayments.NewService(
				peer.Log.Named("payments.stripe:service"),
				pc.StripeCoinPayments,
				peer.DB.StripeCoinPayments(),
				peer.DB.Console().Projects(),
				peer.DB.ProjectAccounting(),
				pc.PerObjectPrice,
				pc.EgressPrice,
				pc.TbhPrice)

			peer.Payments.Accounts = service.Accounts()

			peer.Payments.Chore = stripecoinpayments.NewChore(
				peer.Log.Named("payments.stripe:clearing"),
				service,
				pc.StripeCoinPayments.TransactionUpdateInterval,
				pc.StripeCoinPayments.AccountBalanceUpdateInterval,
			)
			peer.Services.Add(lifecycle.Item{
				Name: "payments.stripe:service",
				Run:  peer.Payments.Chore.Run,
			})
		}
	}

	{ // setup graceful exit
		if config.GracefulExit.Enabled {
			peer.GracefulExit.Chore = gracefulexit.NewChore(peer.Log.Named("gracefulexit"), peer.DB.GracefulExit(), peer.Overlay.DB, peer.Metainfo.Loop, config.GracefulExit)
			peer.Services.Add(lifecycle.Item{
				Name:  "gracefulexit",
				Run:   peer.GracefulExit.Chore.Run,
				Close: peer.GracefulExit.Chore.Close,
			})
		} else {
			peer.Log.Named("gracefulexit").Info("disabled")
		}
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
	}

	{ // setup downtime tracking
		peer.DowntimeTracking.Service = downtime.NewService(peer.Log.Named("downtime"), peer.Overlay.Service, peer.Contact.Service)

		peer.DowntimeTracking.DetectionChore = downtime.NewDetectionChore(
			peer.Log.Named("downtime:detection"),
			config.Downtime,
			peer.Overlay.Service,
			peer.DowntimeTracking.Service,
			peer.DB.DowntimeTracking(),
		)
		peer.Services.Add(lifecycle.Item{
			Name:  "downtime:detection",
			Run:   peer.DowntimeTracking.DetectionChore.Run,
			Close: peer.DowntimeTracking.DetectionChore.Close,
		})

		peer.DowntimeTracking.EstimationChore = downtime.NewEstimationChore(
			peer.Log.Named("downtime:estimation"),
			config.Downtime,
			peer.Overlay.Service,
			peer.DowntimeTracking.Service,
			peer.DB.DowntimeTracking(),
		)
		peer.Services.Add(lifecycle.Item{
			Name:  "downtime:estimation",
			Run:   peer.DowntimeTracking.EstimationChore.Run,
			Close: peer.DowntimeTracking.EstimationChore.Close,
		})
	}

	return peer, nil
}

// Run runs satellite until it's either closed or it errors.
func (peer *Core) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	group, ctx := errgroup.WithContext(ctx)

	peer.Servers.Run(ctx, group)
	peer.Services.Run(ctx, group)

	return group.Wait()
}

// Close closes all the resources.
func (peer *Core) Close() error {
	return errs.Combine(
		peer.Servers.Close(),
		peer.Services.Close(),
	)
}

// ID returns the peer ID.
func (peer *Core) ID() storj.NodeID { return peer.Identity.ID }
