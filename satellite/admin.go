// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"
	"errors"
	"net"
	"runtime/pprof"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/debug"
	"storj.io/common/identity"
	"storj.io/common/storj"
	"storj.io/common/version"
	"storj.io/storj/private/lifecycle"
	"storj.io/storj/private/version/checker"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/admin"
	backoffice "storj.io/storj/satellite/admin/back-office"
	"storj.io/storj/satellite/admin/back-office/auditlogger"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/restapikeys"
	"storj.io/storj/satellite/console/restkeys"
	"storj.io/storj/satellite/emission"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/stripe"
)

// Admin is the satellite core process that runs chores.
//
// architecture: Peer
type Admin struct {
	// core dependencies
	Log        *zap.Logger
	Identity   *identity.FullIdentity
	DB         DB
	MetabaseDB *metabase.DB

	Servers  *lifecycle.Group
	Services *lifecycle.Group

	Debug struct {
		Listener net.Listener
		Server   *debug.Server
	}

	Version struct {
		Chore   *checker.Chore
		Service *checker.Service
	}

	Analytics struct {
		Service *analytics.Service
	}

	Entitlements struct {
		Service *entitlements.Service
	}

	Payments struct {
		Accounts payments.Accounts
		Service  *stripe.Service
		Stripe   stripe.Client
	}

	Admin struct {
		Listener net.Listener
		Server   *admin.Server
		Service  *backoffice.Service
	}

	Buckets struct {
		Service *buckets.Service
	}

	REST struct {
		Keys restapikeys.Service
	}

	FreezeAccounts struct {
		Service *console.AccountFreezeService
	}

	LiveAccounting struct {
		Cache accounting.Cache
	}

	Accounting struct {
		Service *accounting.Service
	}
}

// NewAdmin creates a new satellite admin peer.
func NewAdmin(log *zap.Logger, full *identity.FullIdentity, db DB, metabaseDB *metabase.DB,
	liveAccounting accounting.Cache, versionInfo version.Info, config *Config, atomicLogLevel *zap.AtomicLevel) (*Admin, error) {
	peer := &Admin{
		Log:        log,
		Identity:   full,
		DB:         db,
		MetabaseDB: metabaseDB,

		Servers:  lifecycle.NewGroup(log.Named("servers")),
		Services: lifecycle.NewGroup(log.Named("services")),
	}

	{
		peer.Buckets.Service = buckets.NewService(db.Buckets(), metabaseDB, db.Attribution())
	}

	{ // setup rest keys
		peer.REST.Keys = console.NewRestKeysService(
			log.Named("restKeyService"),
			db.Console().RestApiKeys(),
			restkeys.NewService(peer.DB.OIDC().OAuthTokens(), config.Console.RestAPIKeys.DefaultExpiration),
			time.Now,
			config.Console.Config,
		)
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
		debugConfig.ControlTitle = "Admin"
		peer.Debug.Server = debug.NewServerWithAtomicLevel(log.Named("debug"), peer.Debug.Listener, monkit.Default, debugConfig, atomicLogLevel)
		peer.Servers.Add(lifecycle.Item{
			Name:  "debug",
			Run:   peer.Debug.Server.Run,
			Close: peer.Debug.Server.Close,
		})
	}

	{
		if !versionInfo.IsZero() {
			peer.Log.Debug("Version info",
				zap.Stringer("Version", versionInfo.Version.Version),
				zap.String("Commit Hash", versionInfo.CommitHash),
				zap.Stringer("Build Timestamp", versionInfo.Timestamp),
				zap.Bool("Release Build", versionInfo.Release),
			)
		}
		peer.Version.Service = checker.NewService(log.Named("version"), config.Version, versionInfo, "Satellite")
		peer.Version.Chore = checker.NewChore(peer.Version.Service, config.Version.CheckInterval)

		peer.Services.Add(lifecycle.Item{
			Name: "version",
			Run:  peer.Version.Chore.Run,
		})
	}

	{ // setup analytics
		peer.Analytics.Service = analytics.NewService(peer.Log.Named("analytics:service"), config.Analytics, config.Console.SatelliteName, config.Console.ExternalAddress)

		peer.Services.Add(lifecycle.Item{
			Name:  "analytics:service",
			Run:   peer.Analytics.Service.Run,
			Close: peer.Analytics.Service.Close,
		})
	}

	{ // setup entitlements
		peer.Entitlements.Service = entitlements.NewService(
			peer.Log.Named("entitlements:service"),
			db.Console().Entitlements(),
		)
	}

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

		productPrices, err := pc.Products.ToModels()
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.FreezeAccounts.Service = console.NewAccountFreezeService(
			db.Console(),
			peer.Analytics.Service,
			config.Console.AccountFreeze,
		)

		minimumChargeDate, err := pc.MinimumCharge.GetEffectiveDate()
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		if config.Console.NewPricingStartDate != "" {
			_, err = time.Parse("2006-01-02", config.Console.NewPricingStartDate)
			if err != nil {
				return nil, errs.Combine(err, peer.Close())
			}
		}

		peer.Payments.Service, err = stripe.NewService(
			peer.Log.Named("payments.stripe:service"),
			stripeClient,
			stripe.ServiceDependencies{
				DB:           peer.DB.StripeCoinPayments(),
				WalletsDB:    peer.DB.Wallets(),
				BillingDB:    peer.DB.Billing(),
				ProjectsDB:   peer.DB.Console().Projects(),
				UsersDB:      peer.DB.Console().Users(),
				UsageDB:      peer.DB.ProjectAccounting(),
				Analytics:    peer.Analytics.Service,
				Emission:     emission.NewService(config.Emission),
				Entitlements: peer.Entitlements.Service,
			},
			stripe.ServiceConfig{
				DeleteAccountEnabled:       config.Console.SelfServeAccountDeleteEnabled,
				DeleteProjectCostThreshold: pc.DeleteProjectCostThreshold,
				EntitlementsEnabled:        config.Entitlements.Enabled,
			},
			pc.StripeCoinPayments,
			stripe.PricingConfig{
				UsagePrices:         prices,
				UsagePriceOverrides: priceOverrides,
				ProductPriceMap:     productPrices,
				PartnerPlacementMap: pc.PartnersPlacementPriceOverrides.ToMap(),
				PlacementProductMap: pc.PlacementPriceOverrides.ToMap(),
				PackagePlans:        pc.PackagePlans.Packages,
				BonusRate:           pc.BonusRate,
				MinimumChargeAmount: pc.MinimumCharge.Amount,
				MinimumChargeDate:   minimumChargeDate,
			},
		)

		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Payments.Stripe = stripeClient
		peer.Payments.Accounts = peer.Payments.Service.Accounts()
	}

	{ // setup live accounting
		peer.LiveAccounting.Cache = liveAccounting
	}

	{ // setup accounting project usage
		peer.Accounting.Service = accounting.NewService(
			log.Named("accounting:projectusage-service"),
			peer.DB.ProjectAccounting(),
			peer.LiveAccounting.Cache,
			*metabaseDB,
			config.LiveAccounting.BandwidthCacheTTL,
			config.Console.Config.UsageLimits.Storage.Free,
			config.Console.Config.UsageLimits.Bandwidth.Free,
			config.Console.Config.UsageLimits.Segment.Free,
			config.LiveAccounting.AsOfSystemInterval,
		)
	}

	{ // setup admin
		var err error
		peer.Admin.Listener, err = net.Listen("tcp", config.Admin.Address)
		if err != nil {
			return nil, err
		}

		placement, err := config.Placement.Parse(config.Overlay.Node.CreateDefaultPlacement, nil)
		if err != nil {
			return nil, err
		}

		adminConfig := config.Admin
		adminConfig.AuthorizationToken = config.Console.AuthToken
		adminConfig.BackOffice.AllowedOauthHost = adminConfig.AllowedOauthHost
		if config.PendingDeleteCleanup.Enabled {
			adminConfig.BackOffice.PendingDeleteUserCleanupEnabled = config.PendingDeleteCleanup.User.Enabled
			adminConfig.BackOffice.PendingDeleteProjectCleanupEnabled = config.PendingDeleteCleanup.Project.Enabled
		}

		productPrices, err := config.Payments.Products.ToModels()
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		logger := auditlogger.New(log.Named("audit-logger"), peer.Analytics.Service, peer.DB.AdminChangeHistory(), config.Admin.BackOffice.AuditLogger)
		if config.Admin.BackOffice.AuditLogger.Enabled {
			peer.Services.Add(lifecycle.Item{
				Name:  "admin-audit-logger",
				Run:   logger.Run,
				Close: logger.Close,
			})
		}

		peer.Admin.Service = backoffice.NewService(
			log.Named("back-office:service"),
			peer.DB.Console(),
			peer.DB.AdminChangeHistory(),
			peer.DB.ProjectAccounting(),
			peer.Accounting.Service,
			backoffice.NewAuthorizer(log.Named("back-office:auth"), adminConfig.BackOffice),
			peer.FreezeAccounts.Service,
			peer.Analytics.Service,
			peer.Buckets.Service,
			peer.Entitlements.Service,
			metabaseDB,
			logger,
			peer.Payments.Accounts,
			peer.REST.Keys,
			placement,
			productPrices,
			config.Metainfo.ProjectLimits.MaxBuckets,
			config.Metainfo.RateLimiter.Rate,
			adminConfig.BackOffice,
			config.Console.Config,
			time.Now,
		)

		peer.Admin.Server = admin.NewServer(
			log.Named("admin"),
			peer.Admin.Listener,
			peer.DB,
			metabaseDB,
			peer.Buckets.Service,
			peer.REST.Keys,
			peer.FreezeAccounts.Service,
			peer.Analytics.Service,
			peer.Payments.Accounts,
			peer.Admin.Service,
			peer.Entitlements.Service,
			placement,
			config.Console,
			config.Entitlements,
			adminConfig,
		)

		peer.Servers.Add(lifecycle.Item{
			Name:  "admin",
			Run:   peer.Admin.Server.Run,
			Close: peer.Admin.Server.Close,
		})
	}

	return peer, nil
}

// Run runs satellite until it's either closed or it errors.
func (peer *Admin) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	group, ctx := errgroup.WithContext(ctx)

	pprof.Do(ctx, pprof.Labels("subsystem", "admin"), func(ctx context.Context) {
		peer.Servers.Run(ctx, group)
		peer.Services.Run(ctx, group)

		pprof.Do(ctx, pprof.Labels("name", "subsystem-wait"), func(ctx context.Context) {
			err = group.Wait()
		})
	})
	return err
}

// Close closes all the resources.
func (peer *Admin) Close() error {
	return errs.Combine(
		peer.Servers.Close(),
		peer.Services.Close(),
	)
}

// ID returns the peer ID.
func (peer *Admin) ID() storj.NodeID { return peer.Identity.ID }
