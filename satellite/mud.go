// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"net"
	"time"

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
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/accounting/live"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/console/consoleauth/csrf"
	"storj.io/storj/satellite/console/consoleauth/sso"
	"storj.io/storj/satellite/console/consoleservice"
	"storj.io/storj/satellite/console/consoleweb"
	"storj.io/storj/satellite/console/restapikeys"
	"storj.io/storj/satellite/console/restkeys"
	"storj.io/storj/satellite/console/valdi"
	"storj.io/storj/satellite/console/valdi/valdiclient"
	"storj.io/storj/satellite/contact"
	"storj.io/storj/satellite/emission"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/eventing"
	"storj.io/storj/satellite/eventing/eventingconfig"
	"storj.io/storj/satellite/gc/bloomfilter"
	"storj.io/storj/satellite/jobq"
	"storj.io/storj/satellite/kms"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/mailservice/hubspotmails"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/changestream"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/nodeevents"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/oidc"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/billing"
	"storj.io/storj/satellite/payments/paymentsconfig"
	"storj.io/storj/satellite/payments/storjscan"
	"storj.io/storj/satellite/payments/stripe"
	"storj.io/storj/satellite/piecelist"
	"storj.io/storj/satellite/repair/checker"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/satellite/repair/repaircsv"
	"storj.io/storj/satellite/repair/repairer"
	"storj.io/storj/satellite/repair/repairer/manual"
	"storj.io/storj/satellite/reputation"
	srevocation "storj.io/storj/satellite/revocation"
	sndebug "storj.io/storj/shared/debug"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/modular/eventkit"
	"storj.io/storj/shared/modular/profiler"
	"storj.io/storj/shared/modular/tracing"
	"storj.io/storj/shared/mud"
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

	mud.Provide[signing.Signer](ball, signing.SignerFromFullIdentity)
	mud.Provide[storj.NodeURL](ball, func(id storj.NodeID, cfg contact.Config) storj.NodeURL {
		return storj.NodeURL{
			ID:      id,
			Address: cfg.ExternalAddress,
		}
	})

	contact.Module(ball)

	// initialize here due to circular dependencies
	mud.Provide[*consoleweb.Server](ball, CreateServer)
	consoleweb.Module(ball)
	{
		mud.Provide[extensions.RevocationDB](ball, revocation.OpenDBFromCfg)
		mud.Provide[rpc.Dialer](ball, rpc.NewDefaultDialer)
		mud.Provide[*tlsopts.Options](ball, tlsopts.NewOptions)
		config.RegisterConfig[tlsopts.Config](ball, "server")
	}

	{
		overlay.Module(ball)
		mud.View[DB, overlay.DB](ball, DB.OverlayCache)

		// TODO: we must keep it here as it uses consoleweb.Config from overlay package.
		mud.Provide[*overlay.Service](ball, func(log *zap.Logger, db overlay.DB, nodeEvents nodeevents.DB, placements nodeselection.PlacementDefinitions, consoleConfig consoleweb.Config, config overlay.Config) (*overlay.Service, error) {
			return overlay.NewService(log, db, nodeEvents, placements, consoleConfig.ExternalAddress, consoleConfig.SatelliteName, config)
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

	{
		orders.Module(ball)
		mud.View[DB, orders.DB](ball, DB.Orders)
	}
	audit.Module(ball)

	mud.View[DB, nodeevents.DB](ball, DB.NodeEvents)

	piecelist.Module(ball)

	buckets.Module(ball)

	mud.View[DB, buckets.DB](ball, DB.Buckets)
	mud.View[DB, attribution.DB](ball, DB.Attribution)
	mud.View[DB, overlay.PeerIdentities](ball, DB.PeerIdentities)
	mud.View[DB, srevocation.DB](ball, DB.Revocation)
	mud.View[DB, console.DB](ball, DB.Console)
	mud.View[overlay.DB, bloomfilter.Overlay](ball, func(db overlay.DB) bloomfilter.Overlay {
		return db
	})

	mud.Provide[*console.Service](ball, CreateService)
	console.Module(ball)
	// TODO: need to define here due to circular dependencies
	mud.Provide[console.ObjectLockAndVersioningConfig](ball, func(cfg metainfo.Config) console.ObjectLockAndVersioningConfig {
		return console.ObjectLockAndVersioningConfig{
			ObjectLockEnabled:              cfg.ObjectLockEnabled,
			UseBucketLevelObjectVersioning: cfg.UseBucketLevelObjectVersioning,
		}
	})
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

	mud.Provide[*metainfo.MigrationModeFlagExtension](ball, metainfo.NewMigrationModeFlagExtension)
	mud.Provide[eventingconfig.BucketLocationTopicIDMap](ball, func(config eventingconfig.Config) eventingconfig.BucketLocationTopicIDMap {
		return config.Buckets
	})
	mud.Provide[*EndpointRegistration](ball, func(srv *server.Server, metainfoEndpoint *metainfo.Endpoint) (*EndpointRegistration, error) {
		err := pb.DRPCRegisterMetainfo(srv.DRPC(), metainfoEndpoint)
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
		partnerPlacementOverrideMap := pc.PartnersPlacementPriceOverrides.ToMap()
		for _, overrideMap := range partnerPlacementOverrideMap {
			err = paymentsconfig.ValidatePlacementOverrideMap(overrideMap, productPrices, placements)
			if err != nil {
				return stripe.PricingConfig{}, err
			}
		}
		prices, err := pc.UsagePrice.ToModel()
		if err != nil {
			return stripe.PricingConfig{}, err
		}
		return stripe.PricingConfig{
			UsagePrices:         prices,
			UsagePriceOverrides: priceOverrides,
			ProductPriceMap:     productPrices,
			PartnerPlacementMap: partnerPlacementOverrideMap,
			PlacementProductMap: placementOverrideMap,
			PackagePlans:        pc.PackagePlans.Packages,
			BonusRate:           pc.BonusRate,
			MinimumChargeAmount: pc.MinimumCharge.Amount,
			MinimumChargeDate:   minimumChargeDate,
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
	olavc console.ObjectLockAndVersioningConfig,
	ecfg entitlements.Config,
	ssoCfg sso.Config,
	stripeCfg stripe.Config,
	storjscanCfg storjscan.Config,
	pc paymentsconfig.Config) (*consoleweb.Server, error) {

	cwconfig.SsoEnabled = ssoCfg.Enabled
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

	return consoleweb.NewServer(logger, *cwconfig, service, consoleService, oidcService, mailService, hubspotMailService, analytics, abTesting,
		accountFreezeService, ssoService, csrfService, listener, stripePublicKey, storjscanCfg.Confirmations, nodeURL,
		olavc, analyticsConfig, pc.MinimumCharge, prices, ecfg.Enabled), nil
}

// CreateService creates console service.
// TODO: due to circular dependencies, we couldn't put this to console.Module (consoleweb.Config)
func CreateService(log *zap.Logger, store console.DB, restKeys restapikeys.DB, oauthRestKeys restapikeys.Service, projectAccounting accounting.ProjectAccounting,
	projectUsage *accounting.Service, buckets buckets.DB, attributions attribution.DB, accounts payments.Accounts, depositWallets payments.DepositWallets,
	billingDb billing.TransactionsDB, analytics *analytics.Service, tokens *consoleauth.Service, mailService *mailservice.Service, hubspotMailService *hubspotmails.Service,
	accountFreezeService *console.AccountFreezeService, emission *emission.Service, kmsService *kms.Service, ssoService *sso.Service,
	placements nodeselection.PlacementDefinitions, objectLockAndVersioningConfig console.ObjectLockAndVersioningConfig, valdiService *valdi.Service,
	entitlementsService *entitlements.Service, entitlementsConfig entitlements.Config, cw consoleweb.Config, cfg console.Config, mcfg metainfo.Config, ssoCfg sso.Config, pc paymentsconfig.Config) (*console.Service, error) {

	productModels, err := pc.Products.ToModels()
	if err != nil {
		return nil, err
	}

	minimumChargeDate, err := pc.MinimumCharge.GetEffectiveDate()
	if err != nil {
		return nil, err
	}

	return console.NewService(log, store, restKeys, oauthRestKeys, projectAccounting, projectUsage, buckets, attributions, accounts, depositWallets,
		billingDb, analytics, tokens, mailService, hubspotMailService, accountFreezeService, emission, kmsService, ssoService,
		cw.ExternalAddress, cw.SatelliteName, mcfg.ProjectLimits.MaxBuckets, ssoCfg.Enabled, placements, objectLockAndVersioningConfig,
		valdiService, pc.MinimumCharge.Amount, minimumChargeDate, pc.PackagePlans.Packages, entitlementsConfig, entitlementsService,
		pc.PlacementPriceOverrides.ToMap(), productModels, cfg)
}
