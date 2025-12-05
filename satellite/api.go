// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime/pprof"
	"strings"
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
	"storj.io/storj/satellite/contact"
	"storj.io/storj/satellite/emission"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/gracefulexit"
	"storj.io/storj/satellite/kms"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/mailservice/hubspotmails"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/nodeselection/tracker"
	"storj.io/storj/satellite/nodestats"
	"storj.io/storj/satellite/oidc"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/paymentsconfig"
	"storj.io/storj/satellite/payments/storjscan"
	"storj.io/storj/satellite/payments/stripe"
	"storj.io/storj/satellite/reputation"
	"storj.io/storj/satellite/snopayouts"
	"storj.io/storj/satellite/trust"
	"storj.io/storj/shared/nodetag"
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

	SuccessTrackers *metainfo.SuccessTrackers
	FailureTracker  metainfo.SuccessTracker
	TrustedUplinks  *trust.TrustedPeersList
	TrackerMonitor  *metainfo.SuccessTrackerMonitor
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
		peer.Buckets.Service = buckets.NewService(db.Buckets(), metabaseDB, db.Attribution())
	}

	var trackerInfo *metainfo.TrackerInfo
	var successTrackerUplinks []storj.NodeID
	{
		successTrackerTrustedUplinks, err := parseNodeIDs(config.Metainfo.SuccessTrackerTrustedUplinks)
		if err != nil {
			log.Warn("Wrong uplink ID for the trusted list of the success trackers", zap.Error(err))
			return nil, err
		}

		successTrackerUplinks, err = parseNodeIDs(config.Metainfo.SuccessTrackerUplinks)
		if err != nil {
			log.Warn("Wrong uplink ID for the list of the success trackers", zap.Error(err))
			return nil, err
		}

		trustedUplinkSlice, err := parseNodeIDs(config.Metainfo.TrustedUplinks)
		if err != nil {
			log.Warn("Wrong uplink ID for the list of the trusted uplinks", zap.Error(err))
			return nil, err
		}

		trustedUplinkSlice = append(trustedUplinkSlice, successTrackerTrustedUplinks...)
		successTrackerUplinks = append(successTrackerUplinks, successTrackerTrustedUplinks...)

		peer.TrackerMonitor, err = metainfo.NewSuccessTrackerMonitor(log, db.OverlayCache(), config.Metainfo)
		if err != nil {
			return nil, err
		}
		newTracker, ok := metainfo.GetNewSuccessTracker(config.Metainfo.SuccessTrackerKind)
		if !ok {
			return nil, errs.New("Unknown success tracker kind %q", config.Metainfo.SuccessTrackerKind)
		}
		peer.SuccessTrackers = metainfo.NewSuccessTrackers(successTrackerUplinks, func(uplink storj.NodeID) metainfo.SuccessTracker {
			tracker := newTracker()
			peer.TrackerMonitor.RegisterTracker(monkit.NewSeriesKey("success_tracker").WithTag("uplink", uplink.String()), tracker)
			return tracker
		})
		monkit.ScopeNamed(mon.Name() + ".success_trackers").Chain(peer.SuccessTrackers)

		peer.FailureTracker = metainfo.NewStochasticPercentSuccessTracker(float32(config.Metainfo.FailureTrackerChanceToSkip))
		peer.TrackerMonitor.RegisterTracker(monkit.NewSeriesKey("failure_tracker"), peer.FailureTracker)

		monkit.ScopeNamed(mon.Name() + ".failure_tracker").Chain(peer.FailureTracker)

		peer.TrustedUplinks = trust.NewTrustedPeerList(trustedUplinkSlice)

		peer.Services.Add(lifecycle.Item{
			Name: "tracker_monitor",
			Run:  peer.TrackerMonitor.Run,
		})

	}

	var prometheusTracker *tracker.PrometheusTracker
	environment := nodeselection.NewPlacementConfigEnvironment(peer.SuccessTrackers, peer.FailureTracker)
	environment.AddPrometheusTracker(func() nodeselection.ScoreNode {
		return prometheusTracker
	})

	placements, err := config.Placement.Parse(config.Overlay.Node.CreateDefaultPlacement, environment)
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

	trackerInfo = metainfo.NewTrackerInfo(peer.SuccessTrackers, peer.FailureTracker, successTrackerUplinks, peer.Overlay.DB)

	nodeSelectionStats := metainfo.NewNodeSelectionStats()

	if config.PrometheusTracker.URL != "" {
		var err error
		prometheusTracker, err = tracker.NewPrometheusTracker(log.Named("prometheus-tracker"), peer.Overlay.DB, config.PrometheusTracker)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
		peer.Services.Add(lifecycle.Item{
			Name: "prometheus-tracker",
			Run:  prometheusTracker.Run,
		})
		environment.AddPrometheusTracker(prometheusTracker)
		trackerInfo = trackerInfo.WithPrometheusTracker(prometheusTracker)
	}

	migrationModeFlag := metainfo.NewMigrationModeFlagExtension(config.Metainfo)

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

		peer.Debug.Server = debug.NewServerWithAtomicLevel(log.Named("debug"), peer.Debug.Listener, monkit.Default,
			debugConfig, atomicLogLevel, migrationModeFlag, trackerInfo, nodeSelectionStats)
		peer.Servers.Add(lifecycle.Item{
			Name:  "debug",
			Run:   peer.Debug.Server.Run,
			Close: peer.Debug.Server.Close,
		})
	}

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
		authority, err := LoadAuthorities(full.PeerIdentity(), config.TagAuthorities)
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
			placements.CreateFilters,
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

	{ // setup entitlements
		peer.Entitlements.Service = entitlements.NewService(
			peer.Log.Named("entitlements:service"),
			db.Console().Entitlements(),
		)
	}

	{ // setup metainfo
		peer.Metainfo.Metabase = metabaseDB
		config.Metainfo.SelfServePlacementSelectEnabled = config.Console.Placement.SelfServeEnabled

		peer.Metainfo.Endpoint, err = metainfo.NewEndpoint(
			peer.Log.Named("metainfo:endpoint"),
			peer.Buckets.Service,
			peer.Metainfo.Metabase,
			peer.Orders.Service,
			peer.Overlay.Service,
			peer.DB.Attribution(),
			peer.DB.PeerIdentities(),
			peer.DB.Console().APIKeys(),
			peer.DB.Console().APIKeyTails(),
			peer.Accounting.ProjectUsage,
			peer.DB.Console().Projects(),
			peer.DB.Console().ProjectMembers(),
			peer.DB.Console().Users(),
			signing.SignerFromFullIdentity(peer.Identity),
			peer.DB.Revocation(),
			peer.SuccessTrackers,
			peer.FailureTracker,
			peer.TrustedUplinks,
			config.Metainfo,
			migrationModeFlag,
			placements,
			config.Console,
			config.Orders,
			nodeSelectionStats,
			config.BucketEventing.Buckets,
			peer.Entitlements.Service,
			config.Entitlements,
		)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		if err := pb.DRPCRegisterMetainfo(peer.Server.DRPC(), peer.Metainfo.Endpoint); err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Services.Add(lifecycle.Item{
			Name:  "metainfo:endpoint",
			Run:   peer.Metainfo.Endpoint.Run,
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

	if !config.DisableConsoleFromSatelliteAPI {
		{ // setup oidc
			peer.OIDC.Service = oidc.NewService(db.OIDC())
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

			placementOverrideMap := pc.PlacementPriceOverrides.ToMap()
			err = paymentsconfig.ValidatePlacementOverrideMap(placementOverrideMap, productPrices, placements)
			if err != nil {
				return nil, errs.Combine(err, peer.Close())
			}

			partnerPlacementOverrideMap := pc.PartnersPlacementPriceOverrides.ToMap()
			for _, overrideMap := range partnerPlacementOverrideMap {
				err = paymentsconfig.ValidatePlacementOverrideMap(overrideMap, productPrices, placements)
				if err != nil {
					return nil, errs.Combine(err, peer.Close())
				}
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
					PartnerPlacementMap: partnerPlacementOverrideMap,
					PlacementProductMap: placementOverrideMap,
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
				placements,
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

			peer.CSRF.Service = csrf.NewService(signer)
			// setup account management api keys
			peer.Console.RestKeys = peer.Console.Service

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
				config.Payments.Storjscan.Confirmations,
				peer.URL(),
				console.ObjectLockAndVersioningConfig{
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
	}

	return peer, nil
}

// LoadAuthorities loads the authorities from the specified locations.
func LoadAuthorities(peerIdentity *identity.PeerIdentity, authorityLocations string) (nodetag.Authority, error) {
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

func parseNodeIDs(nodeIDs []string) ([]storj.NodeID, error) {
	rv := make([]storj.NodeID, 0, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		parsedID, err := storj.NodeIDFromString(nodeID)
		if err != nil {
			return nil, err
		}
		rv = append(rv, parsedID)
	}
	return rv, nil
}
