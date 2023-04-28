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

	"storj.io/common/identity"
	"storj.io/common/peertls/extensions"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/private/debug"
	"storj.io/private/version"
	"storj.io/storj/private/lifecycle"
	version_checker "storj.io/storj/private/version/checker"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/accounting/projectbwcleanup"
	"storj.io/storj/satellite/accounting/rollup"
	"storj.io/storj/satellite/accounting/rolluparchive"
	"storj.io/storj/satellite/accounting/tally"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/console/dbcleanup"
	"storj.io/storj/satellite/console/emailreminders"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/segmentloop"
	"storj.io/storj/satellite/metabase/zombiedeletion"
	"storj.io/storj/satellite/metainfo/expireddeletion"
	"storj.io/storj/satellite/nodeevents"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/overlay/offlinenodes"
	"storj.io/storj/satellite/overlay/straynodes"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/accountfreeze"
	"storj.io/storj/satellite/payments/billing"
	"storj.io/storj/satellite/payments/storjscan"
	"storj.io/storj/satellite/payments/stripe"
	"storj.io/storj/satellite/repair/checker"
	"storj.io/storj/satellite/reputation"
)

// Core is the satellite core process that runs chores.
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

	Mail struct {
		Service        *mailservice.Service
		EmailReminders *emailreminders.Chore
	}

	Debug struct {
		Listener net.Listener
		Server   *debug.Server
	}

	// services and endpoints
	Overlay struct {
		DB                overlay.DB
		Service           *overlay.Service
		OfflineNodeEmails *offlinenodes.Chore
		DQStrayNodes      *straynodes.Chore
	}

	NodeEvents struct {
		DB       nodeevents.DB
		Notifier nodeevents.Notifier
		Chore    *nodeevents.Chore
	}

	Metainfo struct {
		Metabase    *metabase.DB
		SegmentLoop *segmentloop.Service
	}

	Reputation struct {
		Service *reputation.Service
	}

	Repair struct {
		Checker *checker.Checker
	}

	Audit struct {
		VerifyQueue          audit.VerifyQueue
		ReverifyQueue        audit.ReverifyQueue
		ContainmentSyncChore *audit.ContainmentSyncChore
	}

	ExpiredDeletion struct {
		Chore *expireddeletion.Chore
	}

	ZombieDeletion struct {
		Chore *zombiedeletion.Chore
	}

	Accounting struct {
		Tally                 *tally.Service
		Rollup                *rollup.Service
		RollupArchiveChore    *rolluparchive.Chore
		ProjectBWCleanupChore *projectbwcleanup.Chore
	}

	LiveAccounting struct {
		Cache accounting.Cache
	}

	Payments struct {
		AccountFreeze    *accountfreeze.Chore
		Accounts         payments.Accounts
		BillingChore     *billing.Chore
		StorjscanClient  *storjscan.Client
		StorjscanService *storjscan.Service
		StorjscanChore   *storjscan.Chore
	}

	ConsoleDBCleanup struct {
		Chore *dbcleanup.Chore
	}
}

// New creates a new satellite.
func New(log *zap.Logger, full *identity.FullIdentity, db DB,
	metabaseDB *metabase.DB, revocationDB extensions.RevocationDB,
	liveAccounting accounting.Cache, versionInfo version.Info, config *Config,
	atomicLogLevel *zap.AtomicLevel) (*Core, error) {
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
			}
		}
		debugConfig := config.Debug
		debugConfig.ControlTitle = "Core"
		peer.Debug.Server = debug.NewServerWithAtomicLevel(log.Named("debug"), peer.Debug.Listener, monkit.Default, debugConfig, atomicLogLevel)
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

	{ // setup mailservice
		peer.Mail.Service, err = setupMailService(peer.Log, *config)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Services.Add(lifecycle.Item{
			Name:  "mail:service",
			Close: peer.Mail.Service.Close,
		})
	}

	{ // setup email reminders
		if config.EmailReminders.Enable {
			authTokens := consoleauth.NewService(config.ConsoleAuth, &consoleauth.Hmac{Secret: []byte(config.Console.AuthTokenSecret)})
			if err != nil {
				return nil, errs.Combine(err, peer.Close())
			}

			peer.Mail.EmailReminders = emailreminders.NewChore(
				peer.Log.Named("console:chore"),
				authTokens,
				peer.DB.Console().Users(),
				peer.Mail.Service,
				config.EmailReminders,
				config.Console.ExternalAddress,
			)

			peer.Services.Add(lifecycle.Item{
				Name:  "mail:email-reminders",
				Run:   peer.Mail.EmailReminders.Run,
				Close: peer.Mail.EmailReminders.Close,
			})
		}
	}

	{ // setup overlay
		peer.Overlay.DB = peer.DB.OverlayCache()
		peer.Overlay.Service, err = overlay.NewService(peer.Log.Named("overlay"), peer.Overlay.DB, peer.DB.NodeEvents(), config.Console.ExternalAddress, config.Console.SatelliteName, config.Overlay)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
		peer.Services.Add(lifecycle.Item{
			Name:  "overlay",
			Run:   peer.Overlay.Service.Run,
			Close: peer.Overlay.Service.Close,
		})

		if config.Overlay.SendNodeEmails {
			peer.Overlay.OfflineNodeEmails = offlinenodes.NewChore(log.Named("overlay:offline-node-emails"), peer.Mail.Service, peer.Overlay.Service, config.OfflineNodes)
			peer.Services.Add(lifecycle.Item{
				Name:  "overlay:offline-node-emails",
				Run:   peer.Overlay.OfflineNodeEmails.Run,
				Close: peer.Overlay.OfflineNodeEmails.Close,
			})
			peer.Debug.Server.Panel.Add(
				debug.Cycle("Overlay Offline Node Emails", peer.Overlay.OfflineNodeEmails.Loop))
		}

		if config.StrayNodes.EnableDQ {
			peer.Overlay.DQStrayNodes = straynodes.NewChore(peer.Log.Named("overlay:dq-stray-nodes"), peer.Overlay.Service, config.StrayNodes)
			peer.Services.Add(lifecycle.Item{
				Name:  "overlay:dq-stray-nodes",
				Run:   peer.Overlay.DQStrayNodes.Run,
				Close: peer.Overlay.DQStrayNodes.Close,
			})
			peer.Debug.Server.Panel.Add(
				debug.Cycle("Overlay DQ Stray Nodes", peer.Overlay.DQStrayNodes.Loop))
		}
	}

	{ // setup node events
		if config.Overlay.SendNodeEmails {
			var notifier nodeevents.Notifier
			switch config.NodeEvents.Notifier {
			case "customer.io":
				notifier = nodeevents.NewCustomerioNotifier(
					log.Named("node-events:customer.io-notifier"),
					config.NodeEvents.Customerio,
				)
			default:
				notifier = nodeevents.NewMockNotifier(log.Named("node-events:mock-notifier"))
			}
			peer.NodeEvents.Notifier = notifier
			peer.NodeEvents.DB = peer.DB.NodeEvents()
			peer.NodeEvents.Chore = nodeevents.NewChore(peer.Log.Named("node-events:chore"), peer.NodeEvents.DB, config.Console.SatelliteName, peer.NodeEvents.Notifier, config.NodeEvents)
			peer.Services.Add(lifecycle.Item{
				Name:  "node-events:chore",
				Run:   peer.NodeEvents.Chore.Run,
				Close: peer.NodeEvents.Chore.Close,
			})
		}
	}

	{ // setup live accounting
		peer.LiveAccounting.Cache = liveAccounting
	}

	{ // setup metainfo
		peer.Metainfo.Metabase = metabaseDB

		peer.Metainfo.SegmentLoop = segmentloop.New(
			peer.Log.Named("metainfo:segmentloop"),
			config.Metainfo.SegmentLoop,
			peer.Metainfo.Metabase,
		)
		peer.Services.Add(lifecycle.Item{
			Name:  "metainfo:segmentloop",
			Run:   peer.Metainfo.SegmentLoop.Run,
			Close: peer.Metainfo.SegmentLoop.Close,
		})
	}

	{ // setup data repair
		log := peer.Log.Named("repair:checker")
		if config.Repairer.UseRangedLoop {
			log.Info("using ranged loop")
		} else {
			peer.Repair.Checker = checker.NewChecker(
				log,
				peer.DB.RepairQueue(),
				peer.Metainfo.Metabase,
				peer.Metainfo.SegmentLoop,
				peer.Overlay.Service,
				config.Checker)
			peer.Services.Add(lifecycle.Item{
				Name:  "repair:checker",
				Run:   peer.Repair.Checker.Run,
				Close: peer.Repair.Checker.Close,
			})

			peer.Debug.Server.Panel.Add(
				debug.Cycle("Repair Checker", peer.Repair.Checker.Loop))
		}
	}

	{ // setup reputation
		reputationDB := peer.DB.Reputation()
		if config.Reputation.FlushInterval > 0 {
			cachingDB := reputation.NewCachingDB(log.Named("reputation:writecache"), reputationDB, config.Reputation)
			peer.Services.Add(lifecycle.Item{
				Name: "reputation:writecache",
				Run:  cachingDB.Manage,
			})
			reputationDB = cachingDB
		}
		peer.Reputation.Service = reputation.NewService(log.Named("reputation:service"),
			peer.Overlay.Service,
			reputationDB,
			config.Reputation,
		)
		peer.Services.Add(lifecycle.Item{
			Name:  "reputation",
			Close: peer.Reputation.Service.Close,
		})
	}

	{ // setup audit
		config := config.Audit

		peer.Audit.VerifyQueue = db.VerifyQueue()
		peer.Audit.ReverifyQueue = db.ReverifyQueue()

		peer.Audit.ContainmentSyncChore = audit.NewContainmentSyncChore(peer.Log.Named("audit:containment-sync-chore"),
			peer.Audit.ReverifyQueue,
			peer.Overlay.DB,
			config.ContainmentSyncChoreInterval,
		)
		peer.Services.Add(lifecycle.Item{
			Name: "audit:containment-sync-chore",
			Run:  peer.Audit.ContainmentSyncChore.Run,
		})
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Audit Containment Sync Chore", peer.Audit.ContainmentSyncChore.Loop))
	}

	{ // setup expired segment cleanup
		peer.ExpiredDeletion.Chore = expireddeletion.NewChore(
			peer.Log.Named("core-expired-deletion"),
			config.ExpiredDeletion,
			peer.Metainfo.Metabase,
		)
		peer.Services.Add(lifecycle.Item{
			Name:  "expireddeletion:chore",
			Run:   peer.ExpiredDeletion.Chore.Run,
			Close: peer.ExpiredDeletion.Chore.Close,
		})
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Expired Segments Chore", peer.ExpiredDeletion.Chore.Loop))
	}

	{ // setup zombie objects cleanup
		peer.ZombieDeletion.Chore = zombiedeletion.NewChore(
			peer.Log.Named("core-zombie-deletion"),
			config.ZombieDeletion,
			peer.Metainfo.Metabase,
		)
		peer.Services.Add(lifecycle.Item{
			Name:  "zombiedeletion:chore",
			Run:   peer.ZombieDeletion.Chore.Run,
			Close: peer.ZombieDeletion.Chore.Close,
		})
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Zombie Objects Chore", peer.ZombieDeletion.Chore.Loop))
	}

	{ // setup accounting
		peer.Accounting.Tally = tally.New(peer.Log.Named("accounting:tally"), peer.DB.StoragenodeAccounting(), peer.DB.ProjectAccounting(), peer.LiveAccounting.Cache, peer.Metainfo.Metabase, peer.DB.Buckets(), config.Tally)
		peer.Services.Add(lifecycle.Item{
			Name:  "accounting:tally",
			Run:   peer.Accounting.Tally.Run,
			Close: peer.Accounting.Tally.Close,
		})
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Accounting Tally", peer.Accounting.Tally.Loop))

		// Lets add 1 more day so we catch any off by one errors when deleting tallies
		orderExpirationPlusDay := config.Orders.Expiration + config.Rollup.Interval
		peer.Accounting.Rollup = rollup.New(peer.Log.Named("accounting:rollup"), peer.DB.StoragenodeAccounting(), config.Rollup, orderExpirationPlusDay)
		peer.Services.Add(lifecycle.Item{
			Name:  "accounting:rollup",
			Run:   peer.Accounting.Rollup.Run,
			Close: peer.Accounting.Rollup.Close,
		})
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Accounting Rollup", peer.Accounting.Rollup.Loop))

		peer.Accounting.ProjectBWCleanupChore = projectbwcleanup.NewChore(peer.Log.Named("accounting:chore"), peer.DB.ProjectAccounting(), config.ProjectBWCleanup)
		peer.Services.Add(lifecycle.Item{
			Name:  "accounting:project-bw-rollup",
			Run:   peer.Accounting.ProjectBWCleanupChore.Run,
			Close: peer.Accounting.ProjectBWCleanupChore.Close,
		})
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Accounting Project Bandwidth Rollup", peer.Accounting.ProjectBWCleanupChore.Loop))

		if config.RollupArchive.Enabled {
			peer.Accounting.RollupArchiveChore = rolluparchive.New(peer.Log.Named("accounting:rollup-archive"), peer.DB.StoragenodeAccounting(), peer.DB.ProjectAccounting(), config.RollupArchive)
			peer.Services.Add(lifecycle.Item{
				Name:  "accounting:rollup-archive",
				Run:   peer.Accounting.RollupArchiveChore.Run,
				Close: peer.Accounting.RollupArchiveChore.Close,
			})
			peer.Debug.Server.Panel.Add(
				debug.Cycle("Accounting Rollup Archive", peer.Accounting.RollupArchiveChore.Loop))
		} else {
			peer.Log.Named("rolluparchive").Info("disabled")
		}
	}

	// TODO: remove in future, should be in API
	{ // setup payments
		pc := config.Payments

		var stripeClient stripe.Client
		switch pc.Provider {
		case "": // just new mock, only used in testing binaries
			stripeClient = stripe.NewStripeMock(
				peer.DB.StripeCoinPayments().Customers(),
				peer.DB.Console().Users(),
			)
		case "mock":
			stripeClient = pc.MockProvider
		case "stripecoinpayments":
			stripeClient = stripe.NewStripeClient(log, pc.StripeCoinPayments)
		default:
			return nil, errs.New("invalid stripe coin payments provider %q", pc.Provider)
		}

		prices, err := pc.UsagePrice.ToModel()
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		priceOverrides, err := pc.UsagePriceOverrides.ToModels()
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		service, err := stripe.NewService(
			peer.Log.Named("payments.stripe:service"),
			stripeClient,
			pc.StripeCoinPayments,
			peer.DB.StripeCoinPayments(),
			peer.DB.Wallets(),
			peer.DB.Billing(),
			peer.DB.Console().Projects(),
			peer.DB.Console().Users(),
			peer.DB.ProjectAccounting(),
			prices,
			priceOverrides,
			pc.PackagePlans.Packages,
			pc.BonusRate)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Payments.Accounts = service.Accounts()

		peer.Payments.StorjscanClient = storjscan.NewClient(
			pc.Storjscan.Endpoint,
			pc.Storjscan.Auth.Identifier,
			pc.Storjscan.Auth.Secret)

		peer.Payments.StorjscanService = storjscan.NewService(log.Named("storjscan-service"),
			peer.DB.Wallets(),
			peer.DB.StorjscanPayments(),
			peer.Payments.StorjscanClient)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Payments.StorjscanChore = storjscan.NewChore(
			peer.Log.Named("payments.storjscan:chore"),
			peer.Payments.StorjscanClient,
			peer.DB.StorjscanPayments(),
			config.Payments.Storjscan.Confirmations,
			config.Payments.Storjscan.Interval,
			config.Payments.Storjscan.DisableLoop,
		)
		peer.Services.Add(lifecycle.Item{
			Name: "payments.storjscan:chore",
			Run:  peer.Payments.StorjscanChore.Run,
		})
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Payments Storjscan", peer.Payments.StorjscanChore.TransactionCycle),
		)

		peer.Payments.BillingChore = billing.NewChore(
			peer.Log.Named("payments.billing:chore"),
			[]billing.PaymentType{peer.Payments.StorjscanService},
			peer.DB.Billing(),
			config.Payments.BillingConfig.Interval,
			config.Payments.BillingConfig.DisableLoop,
			config.Payments.BonusRate,
		)
		peer.Services.Add(lifecycle.Item{
			Name:  "billing:chore",
			Run:   peer.Payments.BillingChore.Run,
			Close: peer.Payments.BillingChore.Close,
		})
	}

	{ // setup account freeze
		if config.AccountFreeze.Enabled {
			analyticService := analytics.NewService(peer.Log.Named("analytics:service"), config.Analytics, config.Console.SatelliteName)
			peer.Payments.AccountFreeze = accountfreeze.NewChore(
				peer.Log.Named("payments.accountfreeze:chore"),
				peer.DB.StripeCoinPayments(),
				peer.Payments.Accounts,
				peer.DB.Console().Users(),
				console.NewAccountFreezeService(db.Console().AccountFreezeEvents(), db.Console().Users(), db.Console().Projects(), analyticService),
				analyticService,
				config.AccountFreeze,
			)

			peer.Services.Add(lifecycle.Item{
				Name:  "accountfreeze:chore",
				Run:   peer.Payments.AccountFreeze.Run,
				Close: peer.Payments.AccountFreeze.Close,
			})
		}
	}

	// setup console DB cleanup service
	if config.ConsoleDBCleanup.Enabled {
		peer.ConsoleDBCleanup.Chore = dbcleanup.NewChore(
			peer.Log.Named("console.dbcleanup:chore"),
			peer.DB.Console(),
			config.ConsoleDBCleanup,
		)

		peer.Services.Add(lifecycle.Item{
			Name:  "dbcleanup:chore",
			Run:   peer.ConsoleDBCleanup.Chore.Run,
			Close: peer.ConsoleDBCleanup.Chore.Close,
		})
	}

	return peer, nil
}

// Run runs satellite until it's either closed or it errors.
func (peer *Core) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	group, ctx := errgroup.WithContext(ctx)

	pprof.Do(ctx, pprof.Labels("subsystem", "core"), func(ctx context.Context) {
		peer.Servers.Run(ctx, group)
		peer.Services.Run(ctx, group)

		pprof.Do(ctx, pprof.Labels("name", "subsystem-wait"), func(ctx context.Context) {
			err = group.Wait()
		})
	})
	return err
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
