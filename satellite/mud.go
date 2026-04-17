// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"net"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/debug"
	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/peertls/extensions"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/storj/private/healthcheck"
	"storj.io/storj/private/revocation"
	"storj.io/storj/private/server"
	"storj.io/storj/satellite/abtesting"
	"storj.io/storj/satellite/accountfreeze"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/accounting/live"
	"storj.io/storj/satellite/accounting/projectbwcleanup"
	"storj.io/storj/satellite/accounting/rollup"
	"storj.io/storj/satellite/accounting/rolluparchive"
	"storj.io/storj/satellite/accounting/tally"
	"storj.io/storj/satellite/admin"
	"storj.io/storj/satellite/admin/changehistory"
	legacyAdmin "storj.io/storj/satellite/admin/legacy"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/balancer"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/compensation"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/console/consoleauth/csrf"
	"storj.io/storj/satellite/console/consoleauth/sso"
	"storj.io/storj/satellite/console/consoleservice"
	"storj.io/storj/satellite/console/consoleweb"
	"storj.io/storj/satellite/console/dbcleanup"
	"storj.io/storj/satellite/console/dbcleanup/pendingdelete"
	"storj.io/storj/satellite/console/emailreminders"
	"storj.io/storj/satellite/console/restapikeys"
	"storj.io/storj/satellite/console/restkeys"
	"storj.io/storj/satellite/console/userinfo"
	"storj.io/storj/satellite/console/valdi"
	"storj.io/storj/satellite/console/valdi/valdiclient"
	"storj.io/storj/satellite/contact"
	"storj.io/storj/satellite/emission"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/eventing"
	"storj.io/storj/satellite/eventing/eventingconfig"
	"storj.io/storj/satellite/gc/bloomfilter"
	"storj.io/storj/satellite/gc/sender"
	"storj.io/storj/satellite/gracefulexit"
	"storj.io/storj/satellite/jobq"
	"storj.io/storj/satellite/kms"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/mailservice/hubspotmails"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/changestream"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/metabase/zombiedeletion"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/metainfo/expireddeletion"
	"storj.io/storj/satellite/nodeapiversion"
	"storj.io/storj/satellite/nodeaudit"
	"storj.io/storj/satellite/nodeevents"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/nodestats"
	"storj.io/storj/satellite/oidc"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/overlay/offlinenodes"
	"storj.io/storj/satellite/overlay/straynodes"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/billing"
	"storj.io/storj/satellite/payments/paymentsconfig"
	"storj.io/storj/satellite/payments/storjscan"
	"storj.io/storj/satellite/payments/stripe"
	"storj.io/storj/satellite/piecelist"
	"storj.io/storj/satellite/projectlimitevents"
	"storj.io/storj/satellite/repair/checker"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/satellite/repair/repaircsv"
	"storj.io/storj/satellite/repair/repairer"
	"storj.io/storj/satellite/repair/repairer/manual"
	"storj.io/storj/satellite/reputation"
	srevocation "storj.io/storj/satellite/revocation"
	"storj.io/storj/satellite/snopayouts"
	"storj.io/storj/satellite/taskqueue"
	"storj.io/storj/satellite/webhook"
	sndebug "storj.io/storj/shared/debug"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/modular/eventkit"
	"storj.io/storj/shared/modular/profiler"
	"storj.io/storj/shared/modular/tracing"
	"storj.io/storj/shared/mud"
	"storj.io/storj/shared/nodetag"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	{
		config.RegisterConfig[debug.Config](ball, "debug")
		sndebug.Module(ball)
	}

	profiler.Module(ball)
	tracing.Module(ball)
	eventkit.Module(ball)

	mud.Provide[*monkit.Registry](ball, func() *monkit.Registry {
		return monkit.Default
	})

	mud.Provide[signing.Signer](ball, signing.SignerFromFullIdentity)
	mud.Provide[storj.NodeURL](ball, func(id storj.NodeID, cfg contact.Config) storj.NodeURL {
		return storj.NodeURL{
			ID:      id,
			Address: cfg.ExternalAddress,
		}
	})

	contact.Module(ball)
	nodetag.Module(ball)
	gracefulexit.Module(ball)

	// initialize here due to circular dependencies
	mud.Provide[*consoleweb.Server](ball, CreateServer)
	consoleweb.Module(ball)
	{
		mud.Provide[extensions.RevocationDB](ball, revocation.OpenDBFromCfg)
		mud.Provide[rpc.Dialer](ball, rpc.NewDefaultPooledDialer)
		mud.Provide[*tlsopts.Options](ball, tlsopts.NewOptions)
		config.RegisterConfig[tlsopts.Config](ball, "server")
	}

	{
		overlay.Module(ball)
		mud.View[DB, overlay.DB](ball, DB.OverlayCache)

		// TODO: we must keep it here as it uses consoleweb.Config from overlay package.
		mud.Provide[*overlay.Service](ball, func(log *zap.Logger, db overlay.DB, nodeEvents nodeevents.DB, placements nodeselection.PlacementDefinitions, consoleConfig consoleweb.Config, config overlay.Config, ncfg nodeevents.Config) (*overlay.Service, error) {
			return overlay.NewService(log, db, nodeEvents, placements, consoleConfig.ExternalAddress, consoleConfig.SatelliteName, config, ncfg)
		})
		mud.Provide[*overlay.UploadNodeCache](ball, func(log *zap.Logger, db overlay.DB, config overlay.Config) (*overlay.UploadNodeCache, error) {
			return overlay.NewUploadNodeCache(log.Named("upload-node-cache"), db, config.NodeSelectionCache.Staleness, config.Node)
		})
	}

	{
		// TODO: fix reversed dependency (nodeselection -> overlay).
		mud.Provide[nodeselection.PlacementDefinitions](ball, func(config nodeselection.PlacementConfig, selectionConfig overlay.NodeSelectionConfig, env nodeselection.PlacementConfigEnvironment) (nodeselection.PlacementDefinitions, error) {
			return config.Placement.Parse(selectionConfig.CreateDefaultPlacement, env)
		})
		nodeselection.Module(ball)
	}
	rangedloop.Module(ball)
	bloomfilter.Module(ball)
	metainfo.Module(ball)
	metabase.Module(ball)
	eventingconfig.Module(ball)
	nodeaudit.Module(ball)
	balancer.Module(ball)

	{
		orders.Module(ball)
		mud.View[DB, orders.DB](ball, DB.Orders)
		mud.View[DB, nodeapiversion.DB](ball, DB.NodeAPIVersion)
	}
	audit.Module(ball)

	mud.View[DB, nodeevents.DB](ball, DB.NodeEvents)
	mud.View[DB, projectlimitevents.DB](ball, DB.ProjectLimitEvents)

	piecelist.Module(ball)

	buckets.Module(ball)

	mud.View[DB, buckets.DB](ball, DB.Buckets)
	mud.View[DB, attribution.DB](ball, DB.Attribution)
	mud.View[DB, accounting.RetentionRemainderDB](ball, DB.RetentionRemainderCharges)
	mud.View[DB, overlay.PeerIdentities](ball, DB.PeerIdentities)
	mud.View[DB, srevocation.DB](ball, DB.Revocation)
	mud.View[DB, console.DB](ball, DB.Console)
	mud.View[overlay.DB, bloomfilter.Overlay](ball, func(db overlay.DB) bloomfilter.Overlay {
		return db
	})

	mud.Provide[*console.Service](ball, CreateService)
	console.Module(ball)
	mud.View[console.Projects, orders.Projects](ball, func(p console.Projects) orders.Projects {
		return p
	})
	// TODO: need to define here due to circular dependencies
	mud.Provide[*console.UpgradeUserObserver](ball, func(consoleDB console.DB, transactionsDB billing.TransactionsDB, cfg consoleweb.Config, freezeService *console.AccountFreezeService, analyticsService *analytics.Service, mailService *mailservice.Service) *console.UpgradeUserObserver {
		return console.NewUpgradeUserObserver(consoleDB, transactionsDB, cfg.UsageLimits, cfg.UserBalanceForUpgrade, cfg.ExternalAddress, freezeService, analyticsService, mailService)
	})

	// TODO: need to define here due to circular dependencies
	mud.Provide[restapikeys.Service](ball, func(log *zap.Logger, db restapikeys.DB, tokens oidc.OAuthTokens, config console.Config) restapikeys.Service {
		return console.NewRestKeysService(log, db, restkeys.NewService(tokens, config.RestAPIKeys.DefaultExpiration), time.Now, config)
	})
	consoleservice.Module(ball)
	consoleauth.Module(ball)
	// need to define here due to circular dependencies
	mud.Provide[consoleauth.Signer](ball, func(configw consoleweb.Config) consoleauth.Signer {
		return &consoleauth.Hmac{Secret: []byte(configw.AuthTokenSecret)}
	})
	sso.Module(ball)
	// TODO: we must keep it here as it uses consoleweb.Config from sso package.
	mud.Provide[*sso.Service](ball, func(consoleConfig consoleweb.Config, tokens *consoleauth.Service, config sso.Config) *sso.Service {
		return sso.NewService(consoleConfig.ExternalAddress, tokens, config)
	})
	csrf.Module(ball)
	valdi.Module(ball)
	valdiclient.Module(ball)
	webhook.Module(ball)
	restkeys.Module(ball)
	mailservice.Module(ball)
	analytics.Module(ball)
	// TODO: we must keep it here as it uses consoleweb.Config from analytics package.
	mud.Provide[*analytics.Service](ball, func(log *zap.Logger, config analytics.Config, consoleConfig consoleweb.Config) *analytics.Service {
		return analytics.NewService(log, config, consoleConfig.SatelliteName, consoleConfig.ExternalAddress)
	})
	abtesting.Module(ball)
	hubspotmails.Module(ball)
	mud.RegisterInterfaceImplementation[metainfo.APIKeys, console.APIKeys](ball)

	// TODO: should be defined here due to circular dependencies (accounting vs live/console config)
	mud.Provide[*accounting.Service](ball, func(log *zap.Logger, projectAccountingDB accounting.ProjectAccounting, liveAccounting accounting.Cache, metabaseDB metabase.DB, cc console.Config, config, lc live.Config) *accounting.Service {
		return accounting.NewService(log, projectAccountingDB, liveAccounting, metabaseDB, lc.BandwidthCacheTTL, cc.UsageLimits.Storage.Free, cc.UsageLimits.Bandwidth.Free, cc.UsageLimits.Segment.Free, lc.AsOfSystemInterval)
	})
	accounting.Module(ball)
	mud.View[DB, accounting.ProjectAccounting](ball, DB.ProjectAccounting)

	live.Module(ball)

	{
		mud.Provide[*server.Server](ball, server.New)
		config.RegisterConfig[server.Config](ball, "server2")
	}

	{
		mud.View[DB, entitlements.DB](ball, func(db DB) entitlements.DB {
			return db.Console().Entitlements()
		})
		mud.Provide[*entitlements.Service](ball, entitlements.NewService)
		config.RegisterConfig[entitlements.Config](ball, "entitlements")
	}

	compensation.Module(ball)
	mud.View[DB, accounting.StoragenodeAccounting](ball, DB.StoragenodeAccounting)
	nodestats.Module(ball)
	userinfo.Module(ball)
	snopayouts.Module(ball)
	mud.View[DB, snopayouts.DB](ball, DB.SNOPayouts)

	mud.Provide[*metainfo.MigrationModeFlagExtension](ball, metainfo.NewMigrationModeFlagExtension)
	mud.Provide[*EndpointRegistration](ball, func(srv *server.Server,
		metainfoEndpoint *metainfo.Endpoint,
		endpoint *contact.Endpoint,
		ne *nodestats.Endpoint,
		ue *userinfo.Endpoint,
		ucfg userinfo.Config,
		se *snopayouts.Endpoint,
		ge *gracefulexit.Endpoint,
		gc gracefulexit.Config,
		oe *orders.Endpoint,
	) (*EndpointRegistration, error) {
		err := pb.DRPCRegisterMetainfo(srv.DRPC(), metainfoEndpoint)
		if err != nil {
			return nil, err
		}

		err = pb.DRPCRegisterOrders(srv.DRPC(), oe)
		if err != nil {
			return nil, err
		}

		err = pb.DRPCRegisterHeldAmount(srv.DRPC(), se)
		if err != nil {
			return nil, err
		}

		if ucfg.Enabled {
			err = pb.DRPCRegisterUserInfo(srv.DRPC(), ue)
			if err != nil {
				return nil, err
			}
		}

		if gc.Enabled {
			err = pb.DRPCRegisterSatelliteGracefulExit(srv.DRPC(), ge)
			if err != nil {
				return nil, err
			}
		}

		err = pb.DRPCRegisterNodeStats(srv.DRPC(), ne)
		if err != nil {
			return nil, err
		}

		err = pb.DRPCRegisterNode(srv.DRPC(), endpoint)
		if err != nil {
			return nil, err
		}
		return &EndpointRegistration{}, nil
	})

	mud.View[DB, audit.ReverifyQueue](ball, DB.ReverifyQueue)
	mud.View[DB, audit.VerifyQueue](ball, DB.VerifyQueue)
	mud.View[DB, audit.WrappedContainment](ball, func(db DB) audit.WrappedContainment {
		return audit.WrappedContainment{
			Containment: db.Containment(),
		}
	})
	mud.View[DB, reputation.DirectDB](ball, func(db DB) reputation.DirectDB {
		return db.Reputation()
	})
	mud.View[*identity.FullIdentity, signing.Signee](ball, func(fullIdentity *identity.FullIdentity) signing.Signee {
		return signing.SigneeFromPeerIdentity(fullIdentity.PeerIdentity())
	})
	checker.Module(ball)
	repairer.Module(ball)
	manual.Module(ball)
	repaircsv.Module(ball)
	reputation.Module(ball)
	jobq.Module(ball)
	taskqueue.Module(ball)
	healthcheck.Module(ball)
	mud.RegisterInterfaceImplementation[queue.RepairQueue, *jobq.RepairJobQueue](ball)
	eventing.Module(ball)
	mud.View[DB, oidc.DB](ball, DB.OIDC)
	oidc.Module(ball)
	mud.View[metabase.Adapter, changestream.Adapter](ball, func(adapter metabase.Adapter) changestream.Adapter {
		csAdapter, ok := adapter.(changestream.Adapter)
		if !ok {
			panic("changestream service requires spanner adapter")
		}
		return csAdapter
	})
	mud.Provide[*mailservice.Service](ball, setupMailService)
	mud.View[DB, stripe.DB](ball, DB.StripeCoinPayments)
	mud.View[DB, storjscan.WalletsDB](ball, DB.Wallets)
	mud.View[DB, billing.TransactionsDB](ball, DB.Billing)
	paymentsconfig.Module(ball)
	mud.Provide[stripe.ServiceConfig](ball, func(cfg console.Config, pc paymentsconfig.Config, ec entitlements.Config) stripe.ServiceConfig {
		return stripe.ServiceConfig{
			DeleteAccountEnabled:       cfg.SelfServeAccountDeleteEnabled,
			DeleteProjectCostThreshold: pc.DeleteProjectCostThreshold,
			EntitlementsEnabled:        ec.Enabled,
		}
	})

	// TODO: due to circular dependencies, we couldn't put these to stripe.Module
	mud.Provide[stripe.PricingConfig](ball, func(pc paymentsconfig.Config, placements nodeselection.PlacementDefinitions) (stripe.PricingConfig, error) {
		minimumChargeDate, err := pc.MinimumCharge.GetEffectiveDate()
		if err != nil {
			return stripe.PricingConfig{}, err
		}
		productPrices, err := pc.Products.ToModels()
		if err != nil {
			return stripe.PricingConfig{}, err
		}
		placementOverrideMap := pc.PlacementPriceOverrides.ToMap()
		err = paymentsconfig.ValidatePlacementOverrideMap(placementOverrideMap, productPrices, placements)
		if err != nil {
			return stripe.PricingConfig{}, err
		}
		priceOverrides, err := pc.UsagePriceOverrides.ToModels()
		if err != nil {
			return stripe.PricingConfig{}, err
		}
		prices, err := pc.UsagePrice.ToModel()
		if err != nil {
			return stripe.PricingConfig{}, err
		}
		return stripe.PricingConfig{
			UsagePrices:         prices,
			UsagePriceOverrides: priceOverrides,
			ProductPriceMap:     productPrices,
			PlacementProductMap: placementOverrideMap,
			PackagePlans:        pc.PackagePlans.Packages,
			BonusRate:           pc.BonusRate,
			MinimumChargeAmount: pc.MinimumCharge.Amount,
			MinimumChargeDate:   minimumChargeDate,
		}, nil
	})
	mud.Provide[accounting.PricingConfig](ball, func(pricingConfig stripe.PricingConfig) (accounting.PricingConfig, error) {
		remainderProductPrices := make(map[int32]accounting.RemainderProductInfo, len(pricingConfig.ProductPriceMap))
		for id, price := range pricingConfig.ProductPriceMap {
			remainderProductPrices[id] = accounting.RemainderProductInfo{
				ProductID:                price.ProductID,
				MinimumRetentionDuration: price.MinimumRetentionDuration,
			}
		}
		return accounting.PricingConfig{
			ProductPrices:       remainderProductPrices,
			PlacementProductMap: pricingConfig.PlacementProductMap,
		}, nil
	})
	stripe.Module(ball)
	emission.Module(ball)
	kms.Module(ball)

	// TODO: remove circular dependency and move it to storjscan.Module
	mud.View[paymentsconfig.Config, storjscan.Config](ball, func(pc paymentsconfig.Config) storjscan.Config {
		return pc.Storjscan
	})
	mud.View[DB, storjscan.PaymentsDB](ball, DB.StorjscanPayments)
	mud.Provide[*storjscan.Service](ball, func(log *zap.Logger, walletsDB storjscan.WalletsDB, paymentsDB storjscan.PaymentsDB, client *storjscan.Client, pc paymentsconfig.Config, cfg storjscan.Config) *storjscan.Service {
		return storjscan.NewService(log, walletsDB, paymentsDB, client, cfg.Confirmations, pc.BonusRate)
	})
	storjscan.Module(ball)
	emailreminders.Module(ball)
	offlinenodes.Module(ball)
	straynodes.Module(ball)

	nodeevents.Module(ball)
	// TODO: remove circular dependencies (overlay.Config vs nodeevents.Config)
	mud.Provide[*nodeevents.Chore](ball, func(log *zap.Logger, db nodeevents.DB, notifier nodeevents.Notifier, config nodeevents.Config, cw consoleweb.Config) *nodeevents.Chore {
		return nodeevents.NewChore(log, db, cw.SatelliteName, notifier, config)
	})

	expireddeletion.Module(ball)
	zombiedeletion.Module(ball)
	tally.Module(ball)
	rollup.Module(ball)
	projectbwcleanup.Module(ball)
	rolluparchive.Module(ball)

	billing.Module(ball)
	// TODO: remove circular dependency between paymentconfig and billing
	mud.Provide[*billing.Chore](ball, func(log *zap.Logger, storjscan *storjscan.Service, transactionsDB billing.TransactionsDB, config billing.Config, pc paymentsconfig.Config, observers billing.ChoreObservers) *billing.Chore {
		return billing.NewChore(log, []billing.PaymentType{storjscan}, transactionsDB, config.Interval, config.DisableLoop, pc.BonusRate, observers)
	})
	// TODO remove circular dependency between console and billing
	mud.Provide[billing.ChoreObservers](ball, func(upgradeUser *console.UpgradeUserObserver, invoice *console.InvoiceTokenPaymentObserver) billing.ChoreObservers {
		return billing.ChoreObservers{
			UpgradeUser: upgradeUser,
			PayInvoices: invoice,
		}
	})

	accountfreeze.Module(ball)
	dbcleanup.Module(ball)
	pendingdelete.Module(ball)
	sender.Module(ball)

	mud.View[DB, changehistory.DB](ball, DB.AdminChangeHistory)
	mud.View[DB, legacyAdmin.DB](ball, func(db DB) legacyAdmin.DB { return db })
	mud.Provide[admin.Defaults](ball, func(cfg metainfo.Config) admin.Defaults {
		return admin.Defaults{
			MaxBuckets: cfg.ProjectLimits.MaxBuckets,
			RateLimit:  int(cfg.RateLimiter.Rate),
		}
	})
	admin.Module(ball)
	mud.Provide[*admin.Server](ball, CreateAdminServer)
}

// EndpointRegistration is a pseudo component to wire server and DRPC endpoints together.
type EndpointRegistration struct{}

// CreateServer creates and configures a console web server with all required dependencies.
func CreateServer(logger *zap.Logger,
	service *console.Service,
	consoleService *consoleservice.Service,
	oidcService *oidc.Service,
	mailService *mailservice.Service,
	hubspotMailService *hubspotmails.Service,
	analytics *analytics.Service,
	abTesting *abtesting.Service,
	accountFreezeService *console.AccountFreezeService,
	ssoService *sso.Service,
	csrfService *csrf.Service,

	nodeURL storj.NodeURL,

	cwconfig *consoleweb.Config,
	analyticsConfig analytics.Config,
	ecfg entitlements.Config,
	ssoCfg sso.Config,
	stripeCfg stripe.Config,
	storjscanCfg storjscan.Config,
	pc paymentsconfig.Config) (*consoleweb.Server, error) {

	listener, err := net.Listen("tcp", cwconfig.Address)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	if cwconfig.AuthTokenSecret == "" {
		return nil, errs.New("Auth token secret required")
	}

	prices, err := pc.UsagePrice.ToModel()
	if err != nil {
		return nil, errs.Wrap(err)
	}

	stripePublicKey := stripeCfg.StripePublicKey

	summaries, err := consoleweb.CreateProductPriceSummaries(pc.Products)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return consoleweb.NewServer(logger, *cwconfig, service, consoleService, oidcService, mailService, hubspotMailService, analytics, abTesting,
		accountFreezeService, ssoService, csrfService, listener, stripePublicKey, storjscanCfg.Confirmations, nodeURL,
		analyticsConfig, pc.MinimumCharge, prices, summaries, ecfg.Enabled, ssoCfg.Enabled), nil
}

// CreateService creates console service.
// TODO: due to circular dependencies, we couldn't put this to console.Module (consoleweb.Config)
func CreateService(log *zap.Logger, store console.DB, restKeys restapikeys.DB, oauthRestKeys restapikeys.Service, projectAccounting accounting.ProjectAccounting,
	projectUsage *accounting.Service, buckets buckets.DB, attributions attribution.DB, accounts payments.Accounts, depositWallets payments.DepositWallets,
	billingDb billing.TransactionsDB, analytics *analytics.Service, tokens *consoleauth.Service, mailService *mailservice.Service, hubspotMailService *hubspotmails.Service,
	accountFreezeService *console.AccountFreezeService, emission *emission.Service, kmsService *kms.Service, ssoService *sso.Service,
	placements nodeselection.PlacementDefinitions, valdiService *valdi.Service, webhookService *webhook.Service,
	entitlementsService *entitlements.Service, entitlementsConfig entitlements.Config, cw consoleweb.Config, cfg console.Config, mcfg metainfo.Config, ssoCfg sso.Config, pc paymentsconfig.Config) (*console.Service, error) {

	productModels, err := pc.Products.ToModels()
	if err != nil {
		return nil, err
	}

	minimumChargeDate, err := pc.MinimumCharge.GetEffectiveDate()
	if err != nil {
		return nil, err
	}

	loginURL, err := cw.LoginURL()
	if err != nil {
		return nil, err
	}
	return console.NewService(log, store, restKeys, oauthRestKeys, projectAccounting, projectUsage, buckets, attributions, accounts, depositWallets,
		billingDb, analytics, tokens, mailService, hubspotMailService, accountFreezeService, emission, kmsService, ssoService,
		cw.ExternalAddress, cw.SatelliteName, cfg.SingleWhiteLabel, mcfg.ProjectLimits.MaxBuckets, ssoCfg.Enabled, placements,
		valdiService, webhookService, pc.MinimumCharge.Amount, minimumChargeDate, pc.PackagePlans.Packages, entitlementsConfig, entitlementsService,
		pc.PlacementPriceOverrides.ToMap(), productModels, cfg, pc.StripeCoinPayments.SkuEnabled, loginURL, cw.SupportURL())
}

// CreateAdminServer creates and configures the admin server with all required dependencies.
func CreateAdminServer(log *zap.Logger,
	db legacyAdmin.DB,
	metabaseDB *metabase.DB,
	buckets *buckets.Service,
	restKeys restapikeys.Service,
	freezeAccounts *console.AccountFreezeService,
	analyticsService *analytics.Service,
	accounts payments.Accounts,
	service *admin.Service,
	entitlements *entitlements.Service,
	placement nodeselection.PlacementDefinitions,
	consoleCfg consoleweb.Config,
	entitlementsCfg entitlements.Config,
	cfg admin.Config,
) (*admin.Server, error) {
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return admin.NewServer(log, listener, db, metabaseDB, buckets, restKeys, freezeAccounts, analyticsService, accounts, service, entitlements, placement, consoleCfg, entitlementsCfg, cfg), nil
}
