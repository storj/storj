// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime/pprof"
	"strings"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/debug"
	"storj.io/common/identity"
	"storj.io/common/nodetag"
	"storj.io/common/pb"
	"storj.io/common/peertls/extensions"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/version"
	"storj.io/storj/private/lifecycle"
	"storj.io/storj/private/server"
	"storj.io/storj/private/version/checker"
	"storj.io/storj/satellite/abtesting"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/console/consoleweb"
	"storj.io/storj/satellite/console/restkeys"
	"storj.io/storj/satellite/console/userinfo"
	"storj.io/storj/satellite/contact"
	"storj.io/storj/satellite/emission"
	"storj.io/storj/satellite/gracefulexit"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/nodestats"
	"storj.io/storj/satellite/oidc"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/storjscan"
	"storj.io/storj/satellite/payments/stripe"
	"storj.io/storj/satellite/reputation"
	"storj.io/storj/satellite/snopayouts"
)

// API is the satellite API process.
//
// architecture: Peer
type API struct {
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

	Contact struct {
		Service  *contact.Service
		Endpoint *contact.Endpoint
	}

	Overlay struct {
		DB      overlay.DB
		Service *overlay.Service
	}

	Reputation struct {
		Service *reputation.Service
	}

	Orders struct {
		DB       orders.DB
		Endpoint *orders.Endpoint
		Service  *orders.Service
		Chore    *orders.Chore
	}

	Metainfo struct {
		Metabase *metabase.DB
		Endpoint *metainfo.Endpoint
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
		Service *mailservice.Service
	}

	Payments struct {
		Accounts       payments.Accounts
		DepositWallets payments.DepositWallets

		StorjscanService *storjscan.Service
		StorjscanClient  *storjscan.Client

		StripeService *stripe.Service
		StripeClient  stripe.Client
	}

	REST struct {
		Keys *restkeys.Service
	}

	Console struct {
		Listener   net.Listener
		Service    *console.Service
		Endpoint   *consoleweb.Server
		AuthTokens *consoleauth.Service
	}

	NodeStats struct {
		Endpoint *nodestats.Endpoint
	}

	OIDC struct {
		Service *oidc.Service
	}

	SNOPayouts struct {
		Endpoint *snopayouts.Endpoint
		Service  *snopayouts.Service
		DB       snopayouts.DB
	}

	GracefulExit struct {
		Endpoint *gracefulexit.Endpoint
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
}

// NewAPI creates a new satellite API process.
func NewAPI(log *zap.Logger, full *identity.FullIdentity, db DB,
	metabaseDB *metabase.DB, revocationDB extensions.RevocationDB,
	liveAccounting accounting.Cache, rollupsWriteCache *orders.RollupsWriteCache,
	config *Config, versionInfo version.Info, atomicLogLevel *zap.AtomicLevel) (*API, error) {
	peer := &API{
		Log:             log,
		Identity:        full,
		DB:              db,
		ExternalAddress: config.Contact.ExternalAddress,

		Servers:  lifecycle.NewGroup(log.Named("servers")),
		Services: lifecycle.NewGroup(log.Named("services")),
	}

	{ // setup buckets service
		peer.Buckets.Service = buckets.NewService(db.Buckets(), metabaseDB)
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
				peer.Log.Info(fmt.Sprintf("Public server started on %s", peer.Addr()))
				peer.Log.Info(fmt.Sprintf("Private server started on %s", peer.PrivateAddr()))
				return peer.Server.Run(ctx)
			},
			Close: peer.Server.Close,
		})
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

	placements, err := config.Placement.Parse(config.Overlay.Node.CreateDefaultPlacement)
	if err != nil {
		return nil, err
	}

	{ // setup overlay
		peer.Overlay.DB = peer.DB.OverlayCache()

		peer.Overlay.Service, err = overlay.NewService(peer.Log.Named("overlay"), peer.Overlay.DB, peer.DB.NodeEvents(), placements, config.Console.ExternalAddress, config.Console.SatelliteName, config.Overlay)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
		peer.Services.Add(lifecycle.Item{
			Name:  "overlay",
			Run:   peer.Overlay.Service.Run,
			Close: peer.Overlay.Service.Close,
		})
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
		peer.Reputation.Service = reputation.NewService(peer.Log.Named("reputation"), peer.Overlay.Service, reputationDB, config.Reputation)
		peer.Services.Add(lifecycle.Item{
			Name:  "reputation",
			Close: peer.Reputation.Service.Close,
		})
	}

	{ // setup contact service
		authority, err := loadAuthorities(full.PeerIdentity(), config.TagAuthorities)
		if err != nil {
			return nil, err
		}

		peer.Contact.Service = contact.NewService(peer.Log.Named("contact:service"), peer.Overlay.Service, peer.DB.PeerIdentities(), peer.Dialer, authority, config.Contact)
		peer.Contact.Endpoint = contact.NewEndpoint(peer.Log.Named("contact:endpoint"), peer.Contact.Service)
		if err := pb.DRPCRegisterNode(peer.Server.DRPC(), peer.Contact.Endpoint); err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Services.Add(lifecycle.Item{
			Name:  "contact:service",
			Close: peer.Contact.Service.Close,
		})
	}

	{ // setup live accounting
		peer.LiveAccounting.Cache = liveAccounting
	}

	{ // setup accounting project usage
		peer.Accounting.ProjectUsage = accounting.NewService(
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

	placement, err := config.Placement.Parse(config.Overlay.Node.CreateDefaultPlacement)
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
			config.Orders.OrdersSemaphoreSize,
			peer.Orders.Service,
		)

		if err := pb.DRPCRegisterOrders(peer.Server.DRPC(), peer.Orders.Endpoint); err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
	}

	{ // setup analytics service
		peer.Analytics.Service = analytics.NewService(peer.Log.Named("analytics:service"), config.Analytics, config.Console.SatelliteName)

		peer.Services.Add(lifecycle.Item{
			Name:  "analytics:service",
			Run:   peer.Analytics.Service.Run,
			Close: peer.Analytics.Service.Close,
		})
	}

	{ // setup AB test service
		peer.ABTesting.Service = abtesting.NewService(peer.Log.Named("abtesting:service"), config.Console.ABTesting)

		peer.Services.Add(lifecycle.Item{
			Name: "abtesting:service",
		})
	}

	{ // setup metainfo
		peer.Metainfo.Metabase = metabaseDB

		peer.Metainfo.Endpoint, err = metainfo.NewEndpoint(
			peer.Log.Named("metainfo:endpoint"),
			peer.Buckets.Service,
			peer.Metainfo.Metabase,
			peer.Orders.Service,
			peer.Overlay.Service,
			peer.DB.Attribution(),
			peer.DB.PeerIdentities(),
			peer.DB.Console().APIKeys(),
			peer.Accounting.ProjectUsage,
			peer.DB.Console().Projects(),
			signing.SignerFromFullIdentity(peer.Identity),
			peer.DB.Revocation(),
			config.Metainfo,
		)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		if err := pb.DRPCRegisterMetainfo(peer.Server.DRPC(), peer.Metainfo.Endpoint); err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Services.Add(lifecycle.Item{
			Name:  "metainfo:endpoint",
			Close: peer.Metainfo.Endpoint.Close,
		})
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

		peer.Payments.StripeService, err = stripe.NewService(
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
			pc.BonusRate,
			peer.Analytics.Service,
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
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Payments.DepositWallets = peer.Payments.StorjscanService
	}

	{ // setup account management api keys
		peer.REST.Keys = restkeys.NewService(peer.DB.OIDC().OAuthTokens(), config.RESTKeys)
	}

	{ // setup console
		consoleConfig := config.Console
		peer.Console.Listener, err = net.Listen("tcp", consoleConfig.Address)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
		if consoleConfig.AuthTokenSecret == "" {
			return nil, errs.New("Auth token secret required")
		}

		peer.Console.AuthTokens = consoleauth.NewService(config.ConsoleAuth, &consoleauth.Hmac{Secret: []byte(consoleConfig.AuthTokenSecret)})

		externalAddress := consoleConfig.ExternalAddress
		if externalAddress == "" {
			externalAddress = "http://" + peer.Console.Listener.Addr().String()
		}

		accountFreezeService := console.NewAccountFreezeService(
			db.Console(),
			peer.Analytics.Service,
			consoleConfig.AccountFreeze,
		)

		emissionService := emission.NewService(config.Emission)

		peer.Console.Service, err = console.NewService(
			peer.Log.Named("console:service"),
			peer.DB.Console(),
			peer.REST.Keys,
			peer.DB.ProjectAccounting(),
			peer.Accounting.ProjectUsage,
			peer.Buckets.Service,
			peer.Payments.Accounts,
			peer.Payments.DepositWallets,
			peer.DB.Billing(),
			peer.Analytics.Service,
			peer.Console.AuthTokens,
			peer.Mail.Service,
			accountFreezeService,
			emissionService,
			externalAddress,
			consoleConfig.SatelliteName,
			config.Metainfo.ProjectLimits.MaxBuckets,
			placement,
			consoleConfig.Config,
		)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Console.Endpoint = consoleweb.NewServer(
			peer.Log.Named("console:endpoint"),
			consoleConfig,
			peer.Console.Service,
			peer.OIDC.Service,
			peer.Mail.Service,
			peer.Analytics.Service,
			peer.ABTesting.Service,
			accountFreezeService,
			peer.Console.Listener,
			config.Payments.StripeCoinPayments.StripePublicKey,
			config.Payments.Storjscan.Confirmations,
			peer.URL(),
			config.Payments.PackagePlans,
		)

		peer.Servers.Add(lifecycle.Item{
			Name:  "console:endpoint",
			Run:   peer.Console.Endpoint.Run,
			Close: peer.Console.Endpoint.Close,
		})
	}

	{ // setup node stats endpoint
		peer.NodeStats.Endpoint = nodestats.NewEndpoint(
			peer.Log.Named("nodestats:endpoint"),
			peer.Overlay.DB,
			peer.Reputation.Service,
			peer.DB.StoragenodeAccounting(),
			config.Payments,
			config.Compensation,
		)
		if err := pb.DRPCRegisterNodeStats(peer.Server.DRPC(), peer.NodeStats.Endpoint); err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
	}

	{ // setup SnoPayout endpoint
		peer.SNOPayouts.DB = peer.DB.SNOPayouts()
		peer.SNOPayouts.Service = snopayouts.NewService(
			peer.Log.Named("payouts:service"),
			peer.SNOPayouts.DB)
		peer.SNOPayouts.Endpoint = snopayouts.NewEndpoint(
			peer.Log.Named("payouts:endpoint"),
			peer.DB.StoragenodeAccounting(),
			peer.Overlay.DB,
			peer.SNOPayouts.Service)
		if err := pb.DRPCRegisterHeldAmount(peer.Server.DRPC(), peer.SNOPayouts.Endpoint); err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
	}

	{ // setup graceful exit
		if config.GracefulExit.Enabled {
			peer.GracefulExit.Endpoint = gracefulexit.NewEndpoint(
				peer.Log.Named("gracefulexit:endpoint"),
				signing.SignerFromFullIdentity(peer.Identity),
				peer.Overlay.DB,
				peer.Overlay.Service,
				peer.Reputation.Service,
				peer.Metainfo.Metabase,
				peer.Orders.Service,
				peer.DB.PeerIdentities(),
				config.GracefulExit)

			if err := pb.DRPCRegisterSatelliteGracefulExit(peer.Server.DRPC(), peer.GracefulExit.Endpoint); err != nil {
				return nil, errs.Combine(err, peer.Close())
			}
		} else {
			peer.Log.Named("gracefulexit").Info("disabled")
		}
	}

	return peer, nil
}

func loadAuthorities(peerIdentity *identity.PeerIdentity, authorityLocations string) (nodetag.Authority, error) {
	var authority nodetag.Authority
	authority = append(authority, signing.SigneeFromPeerIdentity(peerIdentity))
	for _, cert := range strings.Split(authorityLocations, ",") {
		cert = strings.TrimSpace(cert)
		if cert == "" {
			continue
		}
		cert = strings.TrimSpace(cert)
		raw, err := os.ReadFile(cert)
		if err != nil {
			return nil, errs.New("Couldn't load identity for node tag authority from %s: %v", cert, err)
		}
		pi, err := identity.PeerIdentityFromPEM(raw)
		if err != nil {
			return nil, errs.New("Node tag authority file  %s couldn't be loaded as peer identity: %v", cert, err)
		}
		authority = append(authority, signing.SigneeFromPeerIdentity(pi))
	}
	return authority, nil
}

// Run runs satellite until it's either closed or it errors.
func (peer *API) Run(ctx context.Context) (err error) {
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
func (peer *API) Close() error {
	return errs.Combine(
		peer.Servers.Close(),
		peer.Services.Close(),
	)
}

// ID returns the peer ID.
func (peer *API) ID() storj.NodeID { return peer.Identity.ID }

// Addr returns the public address.
func (peer *API) Addr() string {
	return peer.ExternalAddress
}

// URL returns the storj.NodeURL.
func (peer *API) URL() storj.NodeURL {
	return storj.NodeURL{ID: peer.ID(), Address: peer.Addr()}
}

// PrivateAddr returns the private address.
func (peer *API) PrivateAddr() string { return peer.Server.PrivateAddr().String() }
