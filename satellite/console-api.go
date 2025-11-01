// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"runtime/pprof"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/debug"
	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/peertls/extensions"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/version"
	"storj.io/storj/private/healthcheck"
	"storj.io/storj/private/lifecycle"
	"storj.io/storj/private/server"
	"storj.io/storj/private/version/checker"
	"storj.io/storj/satellite/abtesting"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/console/consoleauth/csrf"
	"storj.io/storj/satellite/console/consoleauth/sso"
	"storj.io/storj/satellite/console/consoleservice"
	"storj.io/storj/satellite/console/consoleweb"
	"storj.io/storj/satellite/console/restapikeys"
	"storj.io/storj/satellite/console/restkeys"
	"storj.io/storj/satellite/console/userinfo"
	"storj.io/storj/satellite/console/valdi"
	"storj.io/storj/satellite/console/valdi/valdiclient"
	"storj.io/storj/satellite/emission"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/kms"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/mailservice/hubspotmails"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/oidc"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/storjscan"
	"storj.io/storj/satellite/payments/stripe"
)

// ConsoleAPI is the satellite console API process.
//
// architecture: Peer
type ConsoleAPI struct {
	Log      *zap.Logger
	Identity *identity.FullIdentity
	DB       DB

	Servers  *lifecycle.Group
	Services *lifecycle.Group

	Dialer          rpc.Dialer
	Server          *server.Server
	ExternalAddress string

	Version struct {
		Chore   *checker.Chore
		Service *checker.Service
	}

	Debug struct {
		Listener net.Listener
		Server   *debug.Server
	}

	Overlay struct {
		DB      overlay.DB
		Service *overlay.Service
	}

	Orders struct {
		DB       orders.DB
		Endpoint *orders.Endpoint
		Service  *orders.Service
		Chore    *orders.Chore
	}

	Userinfo struct {
		Endpoint *userinfo.Endpoint
	}

	Accounting struct {
		ProjectUsage *accounting.Service
	}

	LiveAccounting struct {
		Cache accounting.Cache
	}

	Mail struct {
		Service        *mailservice.Service
		HubspotService *hubspotmails.Service
	}

	Payments struct {
		Accounts       payments.Accounts
		DepositWallets payments.DepositWallets

		StorjscanService *storjscan.Service
		StorjscanClient  *storjscan.Client

		StripeService *stripe.Service
		StripeClient  stripe.Client
	}

	Console struct {
		Listener       net.Listener
		Service        *console.Service
		ConsoleService *consoleservice.Service // this is a duplicate of Service, but should replace it in the future.
		RestKeys       restapikeys.Service
		Endpoint       *consoleweb.Server
		AuthTokens     *consoleauth.Service
	}

	Entitlements struct {
		Service *entitlements.Service
	}

	Valdi struct {
		Service *valdi.Service
		Client  *valdiclient.Client
	}

	OIDC struct {
		Service *oidc.Service
	}

	Analytics struct {
		Service *analytics.Service
	}

	ABTesting struct {
		Service *abtesting.Service
	}

	Buckets struct {
		Service *buckets.Service
	}

	KeyManagement struct {
		Service *kms.Service
	}

	SSO struct {
		Service *sso.Service
	}

	CSRF struct {
		Service *csrf.Service
	}

	HealthCheck struct {
		Server *healthcheck.Server
	}
}

// NewConsoleAPI creates a new satellite console API process.
func NewConsoleAPI(log *zap.Logger, full *identity.FullIdentity, db DB,
	metabaseDB *metabase.DB, revocationDB extensions.RevocationDB,
	liveAccounting accounting.Cache, rollupsWriteCache *orders.RollupsWriteCache,
	config *Config, versionInfo version.Info, atomicLogLevel *zap.AtomicLevel) (*ConsoleAPI, error) {
	peer := &ConsoleAPI{
		Log:             log,
		Identity:        full,
		DB:              db,
		ExternalAddress: config.Contact.ExternalAddress,

		Servers:  lifecycle.NewGroup(log.Named("servers")),
		Services: lifecycle.NewGroup(log.Named("services")),
	}

	{ // setup buckets service
		peer.Buckets.Service = buckets.NewService(db.Buckets(), metabaseDB, db.Attribution())
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
		debugConfig.ControlTitle = "API"
		peer.Debug.Server = debug.NewServerWithAtomicLevel(log.Named("debug"), peer.Debug.Listener, monkit.Default, debugConfig, atomicLogLevel)
		peer.Servers.Add(lifecycle.Item{
			Name:  "debug",
			Run:   peer.Debug.Server.Run,
			Close: peer.Debug.Server.Close,
		})
	}

	var err error

	{
		peer.Log.Info("Version info",
			zap.Stringer("Version", versionInfo.Version.Version),
			zap.String("Commit Hash", versionInfo.CommitHash),
			zap.Stringer("Build Timestamp", versionInfo.Timestamp),
			zap.Bool("Release Build", versionInfo.Release),
		)

		peer.Version.Service = checker.NewService(log.Named("version"), config.Version, versionInfo, "Satellite")
		peer.Version.Chore = checker.NewChore(peer.Version.Service, config.Version.CheckInterval)

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

		peer.Server, err = server.New(log.Named("server"), tlsOptions, sc)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		if peer.ExternalAddress == "" {
			// not ideal, but better than nothing
			peer.ExternalAddress = peer.Server.Addr().String()
		}

		peer.Servers.Add(lifecycle.Item{
			Name: "server",
			Run: func(ctx context.Context) error {
				// Don't change the format of this comment, it is used to figure out the node id.
				peer.Log.Info(fmt.Sprintf("Node %s started", peer.Identity.ID))
				peer.Log.Info("Public server started on " + peer.Addr())
				peer.Log.Info("Private server started on " + peer.PrivateAddr())
				return peer.Server.Run(ctx)
			},
			Close: peer.Server.Close,
		})
	}

	{ // setup live accounting
		peer.LiveAccounting.Cache = liveAccounting
	}

	{ // setup accounting project usage
		peer.Accounting.ProjectUsage = accounting.NewService(
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

	{ // setup oidc
		peer.OIDC.Service = oidc.NewService(db.OIDC())
	}

	placement, err := config.Placement.Parse(config.Overlay.Node.CreateDefaultPlacement, nodeselection.NewPlacementConfigEnvironment(nil, nil))
	if err != nil {
		return nil, err
	}

	{ // setup orders
		peer.Orders.DB = rollupsWriteCache
		peer.Orders.Chore = orders.NewChore(log.Named("orders:chore"), rollupsWriteCache, config.Orders)
		peer.Services.Add(lifecycle.Item{
			Name:  "orders:chore",
			Run:   peer.Orders.Chore.Run,
			Close: peer.Orders.Chore.Close,
		})
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Orders Chore", peer.Orders.Chore.Loop))
		var err error
		peer.Orders.Service, err = orders.NewService(
			peer.Log.Named("orders:service"),
			signing.SignerFromFullIdentity(peer.Identity),
			peer.Overlay.Service,
			peer.Orders.DB,
			placement.CreateFilters,
			config.Orders,
		)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		satelliteSignee := signing.SigneeFromPeerIdentity(peer.Identity.PeerIdentity())
		peer.Orders.Endpoint = orders.NewEndpoint(
			peer.Log.Named("orders:endpoint"),
			satelliteSignee,
			peer.Orders.DB,
			peer.DB.NodeAPIVersion(),
			peer.Orders.Service,
			config.Orders,
			peer.Overlay.Service,
		)

		if err := pb.DRPCRegisterOrders(peer.Server.DRPC(), peer.Orders.Endpoint); err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
	}

	{ // setup analytics service
		peer.Analytics.Service = analytics.NewService(peer.Log.Named("analytics:service"), config.Analytics, config.Console.SatelliteName, config.Console.ExternalAddress)

		peer.Services.Add(lifecycle.Item{
			Name:  "analytics:service",
			Run:   peer.Analytics.Service.Run,
			Close: peer.Analytics.Service.Close,
		})
	}

	{ // setup legacy and hubspot mail services
		peer.Mail.Service, err = setupMailService(peer.Log, config.Mail)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Services.Add(lifecycle.Item{
			Name:  "mail:service",
			Close: peer.Mail.Service.Close,
		})

		peer.Mail.HubspotService = hubspotmails.NewService(peer.Log.Named("mail:hubspotservice"), peer.Analytics.Service, config.HubspotMails)

		peer.Services.Add(lifecycle.Item{
			Name:  "hubspotmails:service",
			Close: peer.Mail.HubspotService.Close,
		})
	}

	{ // setup AB test service
		peer.ABTesting.Service = abtesting.NewService(peer.Log.Named("abtesting:service"), config.Console.ABTesting)

		peer.Services.Add(lifecycle.Item{
			Name: "abtesting:service",
		})
	}

	{ // setup kms
		if len(config.KeyManagement.KeyInfos.Values) > 0 {
			peer.KeyManagement.Service = kms.NewService(config.KeyManagement)

			peer.Services.Add(lifecycle.Item{
				Name: "kms:service",
				Run:  peer.KeyManagement.Service.Initialize,
			})
		}
	}

	{ // setup userinfo.
		if config.Userinfo.Enabled {
			peer.Userinfo.Endpoint, err = userinfo.NewEndpoint(
				peer.Log.Named("userinfo:endpoint"),
				peer.DB.Console().Users(),
				peer.DB.Console().APIKeys(),
				peer.DB.Console().Projects(),
				config.Userinfo,
			)
			if err != nil {
				return nil, errs.Combine(err, peer.Close())
			}

			if err := pb.DRPCRegisterUserInfo(peer.Server.DRPC(), peer.Userinfo.Endpoint); err != nil {
				return nil, errs.Combine(err, peer.Close())
			}

			peer.Services.Add(lifecycle.Item{
				Name:  "userinfo:endpoint",
				Close: peer.Userinfo.Endpoint.Close,
			})
		} else {
			peer.Log.Named("userinfo:endpoint").Info("disabled")
		}
	}

	{ // setup entitlements
		peer.Entitlements.Service = entitlements.NewService(
			peer.Log.Named("entitlements:service"),
			db.Console().Entitlements(),
		)
	}

	emissionService := emission.NewService(config.Emission)

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

		minimumChargeDate, err := pc.MinimumCharge.GetEffectiveDate()
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Payments.StripeService, err = stripe.NewService(
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
				Emission:     emissionService,
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

		peer.Payments.StripeClient = stripeClient
		peer.Payments.Accounts = peer.Payments.StripeService.Accounts()

		peer.Payments.StorjscanClient = storjscan.NewClient(
			pc.Storjscan.Endpoint,
			pc.Storjscan.Auth.Identifier,
			pc.Storjscan.Auth.Secret)

		peer.Payments.StorjscanService = storjscan.NewService(log.Named("storjscan-service"),
			peer.DB.Wallets(),
			peer.DB.StorjscanPayments(),
			peer.Payments.StorjscanClient,
			pc.Storjscan.Confirmations,
			pc.BonusRate)

		peer.Payments.DepositWallets = peer.Payments.StorjscanService
	}

	{ // setup console
		consoleConfig := config.Console
		consoleConfig.SsoEnabled = config.SSO.Enabled
		peer.Console.Listener, err = net.Listen("tcp", consoleConfig.Address)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
		if consoleConfig.AuthTokenSecret == "" {
			return nil, errs.New("Auth token secret required")
		}

		signer := &consoleauth.Hmac{Secret: []byte(consoleConfig.AuthTokenSecret)}
		peer.Console.AuthTokens = consoleauth.NewService(config.ConsoleAuth, signer)

		externalAddress := consoleConfig.ExternalAddress
		if externalAddress == "" {
			externalAddress = "http://" + peer.Console.Listener.Addr().String()
		}

		if config.SSO.Enabled {
			// setup sso
			peer.SSO.Service = sso.NewService(
				externalAddress,
				peer.Console.AuthTokens,
				config.SSO,
			)

			peer.Services.Add(lifecycle.Item{
				Name: "sso:service",
				Run:  peer.SSO.Service.Initialize,
			})
		}

		accountFreezeService := console.NewAccountFreezeService(
			db.Console(),
			peer.Analytics.Service,
			consoleConfig.AccountFreeze,
		)

		if config.Console.CloudGpusEnabled {
			peer.Valdi.Client, err = valdiclient.New(peer.Log.Named("valdi:client"), http.DefaultClient, config.Valdi.Config)
			if err != nil {
				return nil, errs.Combine(err, peer.Close())
			}

			peer.Valdi.Service, err = valdi.NewService(peer.Log.Named("valdi:service"), config.Valdi, peer.Valdi.Client)
			if err != nil {
				return nil, errs.Combine(err, peer.Close())
			}
		}

		minimumChargeDate, err := config.Payments.MinimumCharge.GetEffectiveDate()
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		if consoleConfig.NewPricingStartDate != "" {
			_, err = time.Parse("2006-01-02", consoleConfig.NewPricingStartDate)
			if err != nil {
				return nil, errs.Combine(err, peer.Close())
			}
		}

		consoleConfig.Config.SupportURL = consoleConfig.GeneralRequestURL
		consoleConfig.Config.LoginURL, err = url.JoinPath(consoleConfig.ExternalAddress, "login")
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		consoleConfig.SkuEnabled = config.Payments.StripeCoinPayments.SkuEnabled

		productModels, err := config.Payments.Products.ToModels()
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		for _, model := range productModels {
			if model.PriceSummary != "" {
				consoleConfig.ProductPriceSummaries = append(consoleConfig.ProductPriceSummaries, model.PriceSummary)
			}
		}

		peer.Console.Service, err = console.NewService(
			peer.Log.Named("console:service"),
			peer.DB.Console(),
			peer.DB.Console().RestApiKeys(),
			restkeys.NewService(peer.DB.OIDC().OAuthTokens(), config.Console.RestAPIKeys.DefaultExpiration),
			peer.DB.ProjectAccounting(),
			peer.Accounting.ProjectUsage,
			peer.Buckets.Service,
			peer.DB.Attribution(),
			peer.Payments.Accounts,
			peer.Payments.DepositWallets,
			peer.DB.Billing(),
			peer.Analytics.Service,
			peer.Console.AuthTokens,
			peer.Mail.Service,
			peer.Mail.HubspotService,
			accountFreezeService,
			emissionService,
			peer.KeyManagement.Service,
			peer.SSO.Service,
			externalAddress,
			consoleConfig.SatelliteName,
			config.Metainfo.ProjectLimits.MaxBuckets,
			config.SSO.Enabled,
			placement,
			console.ObjectLockAndVersioningConfig{
				ObjectLockEnabled:              config.Metainfo.ObjectLockEnabled,
				UseBucketLevelObjectVersioning: config.Metainfo.UseBucketLevelObjectVersioning,
			},
			peer.Valdi.Service,
			config.Payments.MinimumCharge.Amount,
			minimumChargeDate,
			config.Payments.PackagePlans.Packages,
			config.Entitlements,
			peer.Entitlements.Service,
			config.Payments.PlacementPriceOverrides.ToMap(),
			productModels,
			consoleConfig.Config,
		)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Console.ConsoleService, err = consoleservice.NewService(
			peer.Log.Named("console:service"),
			consoleservice.ServiceDependencies{
				ConsoleDB:            peer.DB.Console(),
				AccountFreezeService: accountFreezeService,
			},
			consoleConfig.Config,
		)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Console.RestKeys = peer.Console.Service
		peer.CSRF.Service = csrf.NewService(signer)

		prices, err := config.Payments.UsagePrice.ToModel()
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		consoleConfig.EntitlementsEnabled = config.Entitlements.Enabled

		peer.Console.Endpoint = consoleweb.NewServer(
			peer.Log.Named("console:endpoint"),
			consoleConfig,
			peer.Console.Service,
			peer.Console.ConsoleService,
			peer.OIDC.Service,
			peer.Mail.Service,
			peer.Mail.HubspotService,
			peer.Analytics.Service,
			peer.ABTesting.Service,
			accountFreezeService,
			peer.SSO.Service,
			peer.CSRF.Service,
			peer.Console.Listener,
			config.Payments.StripeCoinPayments.StripePublicKey,
			config.Payments.Storjscan.Confirmations, peer.URL(), console.ObjectLockAndVersioningConfig{
				ObjectLockEnabled:              config.Metainfo.ObjectLockEnabled,
				UseBucketLevelObjectVersioning: config.Metainfo.UseBucketLevelObjectVersioning,
			},
			config.Analytics,
			config.Payments.MinimumCharge,
			prices,
			config.Entitlements.Enabled,
		)

		peer.Servers.Add(lifecycle.Item{
			Name:  "console:endpoint",
			Run:   peer.Console.Endpoint.Run,
			Close: peer.Console.Endpoint.Close,
		})
	}

	{ // setup health check
		if config.HealthCheck.Enabled {
			listener, err := net.Listen("tcp", config.HealthCheck.Address)
			if err != nil {
				return nil, errs.Combine(err, peer.Close())
			}

			srv := healthcheck.NewServer(peer.Log.Named("healthcheck:server"), listener, peer.Payments.StripeService)
			peer.HealthCheck.Server = srv

			peer.Servers.Add(lifecycle.Item{
				Name:  "healthcheck",
				Run:   srv.Run,
				Close: srv.Close,
			})
		}
	}

	return peer, nil
}

// Run runs satellite until it's either closed or it errors.
func (peer *ConsoleAPI) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	group, ctx := errgroup.WithContext(ctx)

	pprof.Do(ctx, pprof.Labels("subsystem", "api"), func(ctx context.Context) {
		peer.Servers.Run(ctx, group)
		peer.Services.Run(ctx, group)

		pprof.Do(ctx, pprof.Labels("name", "subsystem-wait"), func(ctx context.Context) {
			err = group.Wait()
		})
	})
	return err
}

// Close closes all the resources.
func (peer *ConsoleAPI) Close() error {
	return errs.Combine(
		peer.Servers.Close(),
		peer.Services.Close(),
	)
}

// ID returns the peer ID.
func (peer *ConsoleAPI) ID() storj.NodeID { return peer.Identity.ID }

// Addr returns the public address.
func (peer *ConsoleAPI) Addr() string {
	return peer.ExternalAddress
}

// URL returns the storj.NodeURL.
func (peer *ConsoleAPI) URL() storj.NodeURL {
	return storj.NodeURL{ID: peer.ID(), Address: peer.Addr()}
}

// PrivateAddr returns the private address.
func (peer *ConsoleAPI) PrivateAddr() string { return peer.Server.PrivateAddr().String() }
