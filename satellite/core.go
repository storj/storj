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
	"storj.io/common/pb"
	"storj.io/common/peertls/extensions"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/private/debug"
	"storj.io/private/version"
	"storj.io/storj/private/lifecycle"
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
	"storj.io/storj/satellite/metainfo/expireddeletion"
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

	Version struct {
		Chore   *version_checker.Chore
		Service *version_checker.Service
	}

	Debug struct {
		Listener net.Listener
		Server   *debug.Server
	}

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

	ExpiredDeletion struct {
		Chore *expireddeletion.Chore
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
		debugConfig.ControlTitle = "Core"
		peer.Debug.Server = debug.NewServer(log.Named("debug"), peer.Debug.Listener, monkit.Default, debugConfig)
		peer.Servers.Add(lifecycle.Item{
			Name:  "debug",
			Run:   peer.Debug.Server.Run,
			Close: peer.Debug.Server.Close,
		})
	}

	var err error

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
		peer.Contact.Service = contact.NewService(peer.Log.Named("contact:service"), self, peer.Overlay.Service, peer.DB.PeerIdentities(), peer.Dialer, config.Contact.Timeout)
		peer.Services.Add(lifecycle.Item{
			Name:  "contact:service",
			Close: peer.Contact.Service.Close,
		})
	}

	{ // setup overlay
		peer.Overlay.DB = peer.DB.OverlayCache()
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
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Repair Checker", peer.Repair.Checker.Loop))
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Repair Checker Irreparable", peer.Repair.Checker.IrreparableLoop))
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
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Audit Worker", peer.Audit.Worker.Loop))

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
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Audit Chore", peer.Audit.Chore.Loop))
	}

	{ // setup garbage collection if configured to run with the core
		if config.GarbageCollection.RunInCore {
			peer.GarbageCollection.Service = gc.NewService(
				peer.Log.Named("core-garbage-collection"),
				config.GarbageCollection,
				peer.Dialer,
				peer.Overlay.DB,
				peer.Metainfo.Loop,
			)
			peer.Services.Add(lifecycle.Item{
				Name: "core-garbage-collection",
				Run:  peer.GarbageCollection.Service.Run,
			})
			peer.Debug.Server.Panel.Add(
				debug.Cycle("Core Garbage Collection", peer.GarbageCollection.Service.Loop))
		}
	}

	{ // setup expired segment cleanup
		peer.ExpiredDeletion.Chore = expireddeletion.NewChore(
			peer.Log.Named("core-expired-deletion"),
			config.ExpiredDeletion,
			peer.Metainfo.Service,
			peer.Metainfo.Loop,
		)
		peer.Services.Add(lifecycle.Item{
			Name: "expireddeletion:chore",
			Run:  peer.ExpiredDeletion.Chore.Run,
		})
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Expired Segments Chore", peer.ExpiredDeletion.Chore.Loop))
	}

	{ // setup db cleanup
		peer.DBCleanup.Chore = dbcleanup.NewChore(peer.Log.Named("dbcleanup"), peer.DB.Orders(), config.DBCleanup)
		peer.Services.Add(lifecycle.Item{
			Name:  "dbcleanup",
			Run:   peer.DBCleanup.Chore.Run,
			Close: peer.DBCleanup.Chore.Close,
		})
		peer.Debug.Server.Panel.Add(
			debug.Cycle("DB Cleanup Serials", peer.DBCleanup.Chore.Serials))
	}

	{ // setup accounting
		peer.Accounting.Tally = tally.New(peer.Log.Named("accounting:tally"), peer.DB.StoragenodeAccounting(), peer.DB.ProjectAccounting(), peer.LiveAccounting.Cache, peer.Metainfo.Loop, config.Tally.Interval)
		peer.Services.Add(lifecycle.Item{
			Name:  "accounting:tally",
			Run:   peer.Accounting.Tally.Run,
			Close: peer.Accounting.Tally.Close,
		})
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Accounting Tally", peer.Accounting.Tally.Loop))

		peer.Accounting.Rollup = rollup.New(peer.Log.Named("accounting:rollup"), peer.DB.StoragenodeAccounting(), config.Rollup.Interval, config.Rollup.DeleteTallies)
		peer.Services.Add(lifecycle.Item{
			Name:  "accounting:rollup",
			Run:   peer.Accounting.Rollup.Run,
			Close: peer.Accounting.Rollup.Close,
		})
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Accounting Rollup", peer.Accounting.Rollup.Loop))

		peer.Accounting.ReportedRollupChore = reportedrollup.NewChore(peer.Log.Named("accounting:reported-rollup"), peer.DB.Orders(), config.ReportedRollup)
		peer.Services.Add(lifecycle.Item{
			Name:  "accounting:reported-rollup",
			Run:   peer.Accounting.ReportedRollupChore.Run,
			Close: peer.Accounting.ReportedRollupChore.Close,
		})
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Accounting Reported Rollup", peer.Accounting.ReportedRollupChore.Loop))
	}

	// TODO: remove in future, should be in API
	{ // setup payments
		pc := config.Payments

		switch pc.Provider {
		default:
			peer.Payments.Accounts = mockpayments.Accounts()
		case "stripecoinpayments":
			service, err := stripecoinpayments.NewService(
				peer.Log.Named("payments.stripe:service"),
				pc.StripeCoinPayments,
				peer.DB.StripeCoinPayments(),
				peer.DB.Console().Projects(),
				peer.DB.ProjectAccounting(),
				pc.StorageTBPrice,
				pc.EgressTBPrice,
				pc.ObjectPrice,
				pc.BonusRate,
				pc.CouponValue,
				pc.CouponDuration,
				pc.CouponProjectLimit,
				pc.MinCoinPayment)

			if err != nil {
				return nil, errs.Combine(err, peer.Close())
			}

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
			peer.Debug.Server.Panel.Add(
				debug.Cycle("Payments Stripe Transactions", peer.Payments.Chore.TransactionCycle),
				debug.Cycle("Payments Stripe Account Balance", peer.Payments.Chore.AccountBalanceCycle),
			)
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
			peer.Debug.Server.Panel.Add(
				debug.Cycle("Graceful Exit", peer.GracefulExit.Chore.Loop))
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
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Metrics", peer.Metrics.Chore.Loop))
	}

	{ // setup downtime tracking
		peer.DowntimeTracking.Service = downtime.NewService(peer.Log.Named("downtime"), peer.Overlay.Service, peer.Contact.Service)

		peer.DowntimeTracking.DetectionChore = downtime.NewDetectionChore(
			peer.Log.Named("downtime:detection"),
			config.Downtime,
			peer.Overlay.Service,
			peer.DowntimeTracking.Service,
		)
		peer.Services.Add(lifecycle.Item{
			Name:  "downtime:detection",
			Run:   peer.DowntimeTracking.DetectionChore.Run,
			Close: peer.DowntimeTracking.DetectionChore.Close,
		})
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Downtime Detection", peer.DowntimeTracking.DetectionChore.Loop))

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
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Downtime Estimation", peer.DowntimeTracking.EstimationChore.Loop))
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
