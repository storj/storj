// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/errs2"
	"storj.io/storj/internal/version"
	version_checker "storj.io/storj/internal/version/checker"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/rpc"
	"storj.io/storj/pkg/server"
	"storj.io/storj/pkg/signing"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/accounting/live"
	"storj.io/storj/satellite/accounting/rollup"
	"storj.io/storj/satellite/accounting/tally"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleweb"
	"storj.io/storj/satellite/contact"
	"storj.io/storj/satellite/dbcleanup"
	"storj.io/storj/satellite/gc"
	"storj.io/storj/satellite/gracefulexit"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/marketingweb"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/metrics"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/paymentsconfig"
	"storj.io/storj/satellite/payments/stripecoinpayments"
	"storj.io/storj/satellite/repair/checker"
	"storj.io/storj/satellite/repair/irreparable"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/satellite/repair/repairer"
	"storj.io/storj/satellite/rewards"
)

var mon = monkit.Package()

// DB is the master database for the satellite
//
// architecture: Master Database
type DB interface {
	// CreateTables initializes the database
	CreateTables() error
	// Close closes the database
	Close() error

	// CreateSchema sets the schema
	CreateSchema(schema string) error
	// DropSchema drops the schema
	DropSchema(schema string) error

	// PeerIdentities returns a storage for peer identities
	PeerIdentities() overlay.PeerIdentities
	// OverlayCache returns database for caching overlay information
	OverlayCache() overlay.DB
	// Attribution returns database for partner keys information
	Attribution() attribution.DB
	// StoragenodeAccounting returns database for storing information about storagenode use
	StoragenodeAccounting() accounting.StoragenodeAccounting
	// ProjectAccounting returns database for storing information about project data use
	ProjectAccounting() accounting.ProjectAccounting
	// RepairQueue returns queue for segments that need repairing
	RepairQueue() queue.RepairQueue
	// Irreparable returns database for failed repairs
	Irreparable() irreparable.DB
	// Console returns database for satellite console
	Console() console.DB
	//  returns database for marketing admin GUI
	Rewards() rewards.DB
	// Orders returns database for orders
	Orders() orders.DB
	// Containment returns database for containment
	Containment() audit.Containment
	// Buckets returns the database to interact with buckets
	Buckets() metainfo.BucketsDB
	// GracefulExit returns database for graceful exit
	GracefulExit() gracefulexit.DB
	// StripeCustomers returns table for storing stripe customers
	Customers() stripecoinpayments.CustomersDB
	// CoinpaymentsTransactions returns db for storing coinpayments transactions.
	CoinpaymentsTransactions() stripecoinpayments.TransactionsDB
}

// Config is the global config satellite
type Config struct {
	Identity identity.Config
	Server   server.Config

	Contact contact.Config
	Overlay overlay.Config

	Metainfo metainfo.Config
	Orders   orders.Config

	Checker  checker.Config
	Repairer repairer.Config
	Audit    audit.Config

	GarbageCollection gc.Config

	DBCleanup dbcleanup.Config

	Tally          tally.Config
	Rollup         rollup.Config
	LiveAccounting live.Config

	Mail    mailservice.Config
	Console consoleweb.Config

	Marketing marketingweb.Config

	Version version_checker.Config

	GracefulExit gracefulexit.Config

	Metrics metrics.Config
}

// Peer is the satellite
//
// architecture: Peer
type Peer struct {
	// core dependencies
	Log      *zap.Logger
	Identity *identity.FullIdentity
	DB       DB

	Dialer rpc.Dialer

	Version *version_checker.Service

	// services and endpoints
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
		Service *orders.Service
	}

	Repair struct {
		Checker  *checker.Checker
		Repairer *repairer.Service
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
		Tally        *tally.Service
		Rollup       *rollup.Service
		ProjectUsage *accounting.ProjectUsage
	}

	LiveAccounting struct {
		Cache accounting.Cache
	}

	Payments struct {
		Accounts payments.Accounts
		Clearing payments.Clearing
	}

	GracefulExit struct {
		Chore *gracefulexit.Chore
	}

	Metrics struct {
		Chore *metrics.Chore
	}
}

// New creates a new satellite
func New(log *zap.Logger, full *identity.FullIdentity, db DB, pointerDB metainfo.PointerDB, revocationDB extensions.RevocationDB, liveAccounting accounting.Cache, versionInfo version.Info, config *Config) (*Peer, error) {
	peer := &Peer{
		Log:      log,
		Identity: full,
		DB:       db,
	}

	var err error

	{ // setup version control
		if !versionInfo.IsZero() {
			peer.Log.Sugar().Debugf("Binary Version: %s with CommitHash %s, built at %s as Release %v",
				versionInfo.Version.String(), versionInfo.CommitHash, versionInfo.Timestamp.String(), versionInfo.Release)
		}
		peer.Version = version_checker.NewService(log.Named("version"), config.Version, versionInfo, "Satellite")
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

	{ // setup overlay
		log.Debug("Starting overlay")

		peer.Overlay.DB = overlay.NewCombinedCache(peer.DB.OverlayCache())
		peer.Overlay.Service = overlay.NewService(peer.Log.Named("overlay"), peer.Overlay.DB, config.Overlay)
	}

	{ // setup live accounting
		log.Debug("Setting up live accounting")
		peer.LiveAccounting.Cache = liveAccounting
	}

	{ // setup accounting project usage
		log.Debug("Setting up accounting project usage")
		peer.Accounting.ProjectUsage = accounting.NewProjectUsage(
			peer.DB.ProjectAccounting(),
			peer.LiveAccounting.Cache,
			config.Rollup.MaxAlphaUsage,
		)
	}

	{ // setup orders
		log.Debug("Setting up orders")
		peer.Orders.Service = orders.NewService(
			peer.Log.Named("orders:service"),
			signing.SignerFromFullIdentity(peer.Identity),
			peer.Overlay.Service,
			peer.DB.Orders(),
			config.Orders.Expiration,
			&pb.NodeAddress{
				Transport: pb.NodeTransport_TCP_TLS_GRPC,
				Address:   config.Contact.ExternalAddress,
			},
			config.Repairer.MaxExcessRateOptimalThreshold,
		)
	}

	{ // setup metainfo
		log.Debug("Setting up metainfo")

		peer.Metainfo.Database = pointerDB // for logging: storelogger.New(peer.Log.Named("pdb"), db)
		peer.Metainfo.Service = metainfo.NewService(peer.Log.Named("metainfo:service"),
			peer.Metainfo.Database,
			peer.DB.Buckets(),
		)
		peer.Metainfo.Loop = metainfo.NewLoop(config.Metainfo.Loop, peer.Metainfo.Database)
	}

	{ // setup datarepair
		log.Debug("Setting up datarepair")
		// TODO: simplify argument list somehow
		peer.Repair.Checker = checker.NewChecker(
			peer.Log.Named("checker"),
			peer.DB.RepairQueue(),
			peer.DB.Irreparable(),
			peer.Metainfo.Service,
			peer.Metainfo.Loop,
			peer.Overlay.Service,
			config.Checker)

		segmentRepairer := repairer.NewSegmentRepairer(
			log.Named("repairer"),
			peer.Metainfo.Service,
			peer.Orders.Service,
			peer.Overlay.Service,
			peer.Dialer,
			config.Repairer.Timeout,
			config.Repairer.MaxExcessRateOptimalThreshold,
			config.Checker.RepairOverride,
			config.Repairer.DownloadTimeout,
			signing.SigneeFromPeerIdentity(peer.Identity.PeerIdentity()),
		)

		peer.Repair.Repairer = repairer.NewService(
			peer.Log.Named("repairer"),
			peer.DB.RepairQueue(),
			&config.Repairer,
			segmentRepairer,
		)
	}

	{ // setup audit
		log.Debug("Setting up audits")
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

		peer.Audit.Worker, err = audit.NewWorker(peer.Log.Named("audit worker"),
			peer.Audit.Queue,
			peer.Audit.Verifier,
			peer.Audit.Reporter,
			config,
		)

		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Audit.Chore = audit.NewChore(peer.Log.Named("audit chore"),
			peer.Audit.Queue,
			peer.Metainfo.Loop,
			config,
		)
	}

	{ // setup garbage collection
		log.Debug("Setting up garbage collection")

		peer.GarbageCollection.Service = gc.NewService(
			peer.Log.Named("garbage collection"),
			config.GarbageCollection,
			peer.Dialer,
			peer.Overlay.DB,
			peer.Metainfo.Loop,
		)
	}

	{ // setup db cleanup
		log.Debug("Setting up db cleanup")
		peer.DBCleanup.Chore = dbcleanup.NewChore(peer.Log.Named("dbcleanup"), peer.DB.Orders(), config.DBCleanup)
	}

	{ // setup accounting
		log.Debug("Setting up accounting")
		peer.Accounting.Tally = tally.New(peer.Log.Named("tally"), peer.DB.StoragenodeAccounting(), peer.DB.ProjectAccounting(), peer.LiveAccounting.Cache, peer.Metainfo.Loop, config.Tally.Interval)
		peer.Accounting.Rollup = rollup.New(peer.Log.Named("rollup"), peer.DB.StoragenodeAccounting(), config.Rollup.Interval, config.Rollup.DeleteTallies)
	}

	{ // setup payments
		config := paymentsconfig.Config{}

		service := stripecoinpayments.NewService(
			peer.Log.Named("stripecoinpayments service"),
			config.StripeCoinPayments,
			peer.DB.Customers(),
			peer.DB.CoinpaymentsTransactions())

		peer.Payments.Accounts = service.Accounts()
		peer.Payments.Clearing = stripecoinpayments.NewChore(
			peer.Log.Named("stripecoinpayments clearing loop"),
			service,
			config.StripeCoinPayments.TransactionUpdateInterval,
			config.StripeCoinPayments.AccountBalanceUpdateInterval)
	}

	{ // setup graceful exit
		if config.GracefulExit.Enabled {
			log.Debug("Setting up graceful exit")
			peer.GracefulExit.Chore = gracefulexit.NewChore(peer.Log.Named("graceful exit chore"), peer.DB.GracefulExit(), peer.Overlay.DB, peer.Metainfo.Loop, config.GracefulExit)
		}
	}

	{ // setup metrics service
		peer.Metrics.Chore = metrics.NewChore(
			peer.Log.Named("metrics"),
			config.Metrics,
			peer.Metainfo.Loop,
		)
	}

	return peer, nil
}

// Run runs satellite until it's either closed or it errors.
func (peer *Peer) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	group, ctx := errgroup.WithContext(ctx)

	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Metainfo.Loop.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Version.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Repair.Checker.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Repair.Repairer.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.DBCleanup.Chore.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Accounting.Tally.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Accounting.Rollup.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Audit.Worker.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Audit.Chore.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.GarbageCollection.Service.Run(ctx))
	})
	if peer.GracefulExit.Chore != nil {
		group.Go(func() error {
			return errs2.IgnoreCanceled(peer.GracefulExit.Chore.Run(ctx))
		})
	}
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Metrics.Chore.Run(ctx))
	})

	return group.Wait()
}

// Close closes all the resources.
func (peer *Peer) Close() error {
	var errlist errs.Group

	// TODO: ensure that Close can be called on nil-s that way this code won't need the checks.

	// close servers, to avoid new connections to closing subsystems
	if peer.Metrics.Chore != nil {
		errlist.Add(peer.Metrics.Chore.Close())
	}

	if peer.GracefulExit.Chore != nil {
		errlist.Add(peer.GracefulExit.Chore.Close())
	}

	// close services in reverse initialization order

	if peer.Audit.Chore != nil {
		errlist.Add(peer.Audit.Chore.Close())
	}
	if peer.Audit.Worker != nil {
		errlist.Add(peer.Audit.Worker.Close())
	}

	if peer.Accounting.Rollup != nil {
		errlist.Add(peer.Accounting.Rollup.Close())
	}
	if peer.Accounting.Tally != nil {
		errlist.Add(peer.Accounting.Tally.Close())
	}

	if peer.DBCleanup.Chore != nil {
		errlist.Add(peer.DBCleanup.Chore.Close())
	}
	if peer.Repair.Repairer != nil {
		errlist.Add(peer.Repair.Repairer.Close())
	}
	if peer.Repair.Checker != nil {
		errlist.Add(peer.Repair.Checker.Close())
	}

	if peer.Overlay.Service != nil {
		errlist.Add(peer.Overlay.Service.Close())
	}
	if peer.Metainfo.Loop != nil {
		errlist.Add(peer.Metainfo.Loop.Close())
	}

	return errlist.Err()
}

// ID returns the peer ID.
func (peer *Peer) ID() storj.NodeID { return peer.Identity.ID }
