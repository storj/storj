// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"
	"net"
	"net/mail"
	"net/smtp"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/rpc"
	"storj.io/storj/pkg/signing"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/private/errs2"
	"storj.io/storj/private/post"
	"storj.io/storj/private/post/oauth2"
	"storj.io/storj/private/version"
	version_checker "storj.io/storj/private/version/checker"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/accounting/rollup"
	"storj.io/storj/satellite/accounting/tally"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/dbcleanup"
	"storj.io/storj/satellite/gc"
	"storj.io/storj/satellite/gracefulexit"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/mailservice/simulate"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/metrics"
	"storj.io/storj/satellite/notification"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/mockpayments"
	"storj.io/storj/satellite/payments/stripecoinpayments"
	"storj.io/storj/satellite/repair/checker"
	"storj.io/storj/satellite/repair/repairer"
)

// Core is the satellite core process that runs chores
//
// architecture: Peer
type Core struct {
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

	Mail struct {
		Service *mailservice.Service
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
		ProjectUsage *accounting.Service
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

	Notification struct {
		Service *notification.Service
	}
}

// New creates a new satellite
func New(log *zap.Logger, full *identity.FullIdentity, db DB, pointerDB metainfo.PointerDB, revocationDB extensions.RevocationDB, liveAccounting accounting.Cache, versionInfo version.Info, config *Config) (*Core, error) {
	peer := &Core{
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
		peer.Version = version_checker.NewService(
			log.Named("version"),
			config.Version,
			versionInfo,
			"Satellite",
		)
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

	{ // setup mailservice
		log.Debug("Satellite API Process setting up mail service")
		// TODO(yar): test multiple satellites using same OAUTH credentials
		mailConfig := config.Mail

		// validate from mail address
		from, err := mail.ParseAddress(mailConfig.From)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		// validate smtp server address
		host, _, err := net.SplitHostPort(mailConfig.SMTPServerAddress)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		var sender mailservice.Sender
		switch mailConfig.AuthType {
		case "oauth2":
			creds := oauth2.Credentials{
				ClientID:     mailConfig.ClientID,
				ClientSecret: mailConfig.ClientSecret,
				TokenURI:     mailConfig.TokenURI,
			}
			token, err := oauth2.RefreshToken(context.TODO(), creds, mailConfig.RefreshToken)
			if err != nil {
				return nil, errs.Combine(err, peer.Close())
			}

			sender = &post.SMTPSender{
				From: *from,
				Auth: &oauth2.Auth{
					UserEmail: from.Address,
					Storage:   oauth2.NewTokenStore(creds, *token),
				},
				ServerAddress: mailConfig.SMTPServerAddress,
			}
		case "plain":
			sender = &post.SMTPSender{
				From:          *from,
				Auth:          smtp.PlainAuth("", mailConfig.Login, mailConfig.Password, host),
				ServerAddress: mailConfig.SMTPServerAddress,
			}
		case "login":
			sender = &post.SMTPSender{
				From: *from,
				Auth: post.LoginAuth{
					Username: mailConfig.Login,
					Password: mailConfig.Password,
				},
				ServerAddress: mailConfig.SMTPServerAddress,
			}
		default:
			sender = &simulate.LinkClicker{}
		}

		peer.Mail.Service, err = mailservice.New(
			peer.Log.Named("mail:service"),
			sender,
			mailConfig.TemplatePath,
		)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
	}

	{ // setup notification
		log.Debug("Satellite API Process setting up notification endpoint")
		peer.Notification.Service = notification.NewService(
			peer.Log.Named("notification:service"),
			config.Notification,
			peer.Dialer,
			peer.Overlay.Service,
			peer.Mail.Service,
		)
	}

	{ // setup accounting project usage
		log.Debug("Setting up accounting project usage")
		peer.Accounting.ProjectUsage = accounting.NewService(
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
		peer.Metainfo.Service = metainfo.NewService(
			peer.Log.Named("metainfo:service"),
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

		peer.Audit.Reporter = audit.NewReporter(peer.Log.Named("audit:reporter"),
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
		peer.Accounting.Tally = tally.New(
			peer.Log.Named("tally"),
			peer.DB.StoragenodeAccounting(),
			peer.DB.ProjectAccounting(),
			peer.LiveAccounting.Cache,
			peer.Metainfo.Loop,
			config.Tally.Interval,
		)
		peer.Accounting.Rollup = rollup.New(
			peer.Log.Named("rollup"),
			peer.DB.StoragenodeAccounting(),
			config.Rollup.Interval,
			config.Rollup.DeleteTallies,
		)
	}

	// TODO: remove in future, should be in API
	{ // setup payments
		log.Debug("Setting up payments")
		pc := config.Payments

		switch pc.Provider {
		default:
			peer.Payments.Accounts = mockpayments.Accounts()
		case "stripecoinpayments":
			service := stripecoinpayments.NewService(
				peer.Log.Named("stripecoinpayments service"),
				pc.StripeCoinPayments,
				peer.DB.StripeCoinPayments(),
				peer.DB.Console().Projects(),
				peer.DB.ProjectAccounting(),
				pc.PerObjectPrice,
				pc.EgressPrice,
				pc.TbhPrice)

			peer.Payments.Accounts = service.Accounts()

			peer.Payments.Chore = stripecoinpayments.NewChore(
				peer.Log.Named("stripecoinpayments clearing loop"),
				service,
				pc.StripeCoinPayments.TransactionUpdateInterval,
				pc.StripeCoinPayments.AccountBalanceUpdateInterval,
				// TODO: uncomment when coupons will be finished.
				//pc.StripeCoinPayments.CouponUsageCycleInterval,
			)
		}
	}

	{ // setup graceful exit
		if config.GracefulExit.Enabled {
			log.Debug("Setting up graceful exit")
			peer.GracefulExit.Chore = gracefulexit.NewChore(
				peer.Log.Named("graceful exit chore"),
				peer.DB.GracefulExit(),
				peer.Overlay.DB,
				peer.Metainfo.Loop,
				config.GracefulExit,
			)
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
func (peer *Core) Run(ctx context.Context) (err error) {
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
	if peer.Payments.Chore != nil {
		group.Go(func() error {
			return errs2.IgnoreCanceled(peer.Payments.Chore.Run(ctx))
		})
	}

	return group.Wait()
}

// Close closes all the resources.
func (peer *Core) Close() error {
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
func (peer *Core) ID() storj.NodeID { return peer.Identity.ID }
