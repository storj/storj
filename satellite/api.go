// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"

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
	"storj.io/storj/private/post"
	"storj.io/storj/private/post/oauth2"
	"storj.io/storj/private/server"
	"storj.io/storj/private/version/checker"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/accountmanagementapikeys"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/console/consoleweb"
	"storj.io/storj/satellite/contact"
	"storj.io/storj/satellite/gracefulexit"
	"storj.io/storj/satellite/inspector"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/mailservice/simulate"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/metainfo/piecedeletion"
	"storj.io/storj/satellite/nodestats"
	"storj.io/storj/satellite/oidc"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/paymentsconfig"
	"storj.io/storj/satellite/payments/stripecoinpayments"
	"storj.io/storj/satellite/reputation"
	"storj.io/storj/satellite/rewards"
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
		Metabase      *metabase.DB
		PieceDeletion *piecedeletion.Service
		Endpoint      *metainfo.Endpoint
	}

	Inspector struct {
		Endpoint *inspector.Endpoint
	}

	Accounting struct {
		ProjectUsage *accounting.Service
	}

	LiveAccounting struct {
		Cache accounting.Cache
	}

	ProjectLimits struct {
		Cache *accounting.ProjectLimitCache
	}

	Mail struct {
		Service *mailservice.Service
	}

	Payments struct {
		Accounts   payments.Accounts
		Conversion *stripecoinpayments.ConversionService
		Service    *stripecoinpayments.Service
		Stripe     stripecoinpayments.StripeClient
	}

	AccountManagementAPIKeys struct {
		Service *accountmanagementapikeys.Service
	}

	Console struct {
		Listener net.Listener
		Service  *console.Service
		Endpoint *consoleweb.Server
	}

	Marketing struct {
		PartnersService *rewards.PartnersService
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
		if config.Debug.Address != "" {
			peer.Debug.Listener, err = net.Listen("tcp", config.Debug.Address)
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

	{ // setup overlay
		peer.Overlay.DB = peer.DB.OverlayCache()

		peer.Overlay.Service, err = overlay.NewService(peer.Log.Named("overlay"), peer.Overlay.DB, config.Overlay)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
		peer.Services.Add(lifecycle.Item{
			Name:  "overlay",
			Close: peer.Overlay.Service.Close,
		})
	}

	{ // setup reputation
		peer.Reputation.Service = reputation.NewService(peer.Log.Named("reputation"), peer.Overlay.DB, peer.DB.Reputation(), config.Reputation)

		peer.Services.Add(lifecycle.Item{
			Name:  "reputation",
			Close: peer.Reputation.Service.Close,
		})
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
					Address: peer.Addr(),
				},
			},
			Type:    pb.NodeType_SATELLITE,
			Version: *pbVersion,
		}
		peer.Contact.Service = contact.NewService(peer.Log.Named("contact:service"), self, peer.Overlay.Service, peer.DB.PeerIdentities(), peer.Dialer, config.Contact)
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

	{ // setup project limits
		peer.ProjectLimits.Cache = accounting.NewProjectLimitCache(peer.DB.ProjectAccounting(),
			config.Console.Config.UsageLimits.Storage.Free,
			config.Console.Config.UsageLimits.Bandwidth.Free,
			config.Console.Config.UsageLimits.Segment.Free,
			config.ProjectLimit,
		)
	}

	{ // setup accounting project usage
		peer.Accounting.ProjectUsage = accounting.NewService(
			peer.DB.ProjectAccounting(),
			peer.LiveAccounting.Cache,
			peer.ProjectLimits.Cache,
			*metabaseDB,
			config.LiveAccounting.BandwidthCacheTTL,
			config.LiveAccounting.AsOfSystemInterval,
		)
	}

	{ // setup oidc
		peer.OIDC.Service = oidc.NewService(db.OIDC())
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
			peer.Buckets.Service,
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

	{ // setup marketing partners service
		peer.Marketing.PartnersService = rewards.NewPartnersService(
			peer.Log.Named("partners"),
			rewards.DefaultPartnersDB,
		)
	}

	{ // setup analytics service
		peer.Analytics.Service = analytics.NewService(peer.Log.Named("analytics:service"), config.Analytics, config.Console.SatelliteName)

		peer.Services.Add(lifecycle.Item{
			Name:  "analytics:service",
			Run:   peer.Analytics.Service.Run,
			Close: peer.Analytics.Service.Close,
		})
	}

	{ // setup metainfo
		peer.Metainfo.Metabase = metabaseDB

		peer.Metainfo.PieceDeletion, err = piecedeletion.NewService(
			peer.Log.Named("metainfo:piecedeletion"),
			peer.Dialer,
			peer.Overlay.Service,
			config.Metainfo.PieceDeletion,
		)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
		peer.Services.Add(lifecycle.Item{
			Name:  "metainfo:piecedeletion",
			Run:   peer.Metainfo.PieceDeletion.Run,
			Close: peer.Metainfo.PieceDeletion.Close,
		})

		peer.Metainfo.Endpoint, err = metainfo.NewEndpoint(
			peer.Log.Named("metainfo:endpoint"),
			peer.Buckets.Service,
			peer.Metainfo.Metabase,
			peer.Metainfo.PieceDeletion,
			peer.Orders.Service,
			peer.Overlay.Service,
			peer.DB.Attribution(),
			peer.Marketing.PartnersService,
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

	{ // setup inspector
		peer.Inspector.Endpoint = inspector.NewEndpoint(
			peer.Log.Named("inspector"),
			peer.Overlay.Service,
			peer.Metainfo.Metabase,
		)
		if err := internalpb.DRPCRegisterHealthInspector(peer.Server.PrivateDRPC(), peer.Inspector.Endpoint); err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
	}

	{ // setup mailservice
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
			sender = simulate.NewDefaultLinkClicker()
		}

		peer.Mail.Service, err = mailservice.New(
			peer.Log.Named("mail:service"),
			sender,
			mailConfig.TemplatePath,
		)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Services.Add(lifecycle.Item{
			Name:  "mail:service",
			Close: peer.Mail.Service.Close,
		})
	}

	{ // setup payments
		pc := config.Payments

		var stripeClient stripecoinpayments.StripeClient
		switch pc.Provider {
		default:
			stripeClient = stripecoinpayments.NewStripeMock(
				peer.ID(),
				peer.DB.StripeCoinPayments().Customers(),
				peer.DB.Console().Users(),
			)
		case "stripecoinpayments":
			stripeClient = stripecoinpayments.NewStripeClient(log, pc.StripeCoinPayments)
		}

		peer.Payments.Service, err = stripecoinpayments.NewService(
			peer.Log.Named("payments.stripe:service"),
			stripeClient,
			pc.StripeCoinPayments,
			peer.DB.StripeCoinPayments(),
			peer.DB.Console().Projects(),
			peer.DB.ProjectAccounting(),
			pc.StorageTBPrice,
			pc.EgressTBPrice,
			pc.SegmentPrice,
			pc.BonusRate)

		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Payments.Stripe = stripeClient
		peer.Payments.Accounts = peer.Payments.Service.Accounts()
		peer.Payments.Conversion = stripecoinpayments.NewConversionService(
			peer.Log.Named("payments.stripe:version"),
			peer.Payments.Service,
			pc.StripeCoinPayments.ConversionRatesCycleInterval)

		peer.Services.Add(lifecycle.Item{
			Name:  "payments.stripe:version",
			Run:   peer.Payments.Conversion.Run,
			Close: peer.Payments.Conversion.Close,
		})
	}

	{ // setup account management api keys
		peer.AccountManagementAPIKeys.Service = accountmanagementapikeys.NewService(peer.DB.OIDC().OAuthTokens(), config.AccountManagementAPIKeys)
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

		peer.Console.Service, err = console.NewService(
			peer.Log.Named("console:service"),
			&consoleauth.Hmac{Secret: []byte(consoleConfig.AuthTokenSecret)},
			peer.DB.Console(),
			peer.AccountManagementAPIKeys.Service,
			peer.DB.ProjectAccounting(),
			peer.Accounting.ProjectUsage,
			peer.Buckets.Service,
			peer.Marketing.PartnersService,
			peer.Payments.Accounts,
			peer.Analytics.Service,
			consoleConfig.Config,
		)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		pricing := paymentsconfig.PricingValues{
			StorageTBPrice: config.Payments.StorageTBPrice,
			EgressTBPrice:  config.Payments.EgressTBPrice,
			SegmentPrice:   config.Payments.SegmentPrice,
		}

		peer.Console.Endpoint = consoleweb.NewServer(
			peer.Log.Named("console:endpoint"),
			consoleConfig,
			peer.Console.Service,
			peer.OIDC.Service,
			peer.Mail.Service,
			peer.Marketing.PartnersService,
			peer.Analytics.Service,
			peer.Console.Listener,
			config.Payments.StripeCoinPayments.StripePublicKey,
			pricing,
			peer.URL(),
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
				peer.DB.GracefulExit(),
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

// Run runs satellite until it's either closed or it errors.
func (peer *API) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	group, ctx := errgroup.WithContext(ctx)

	peer.Servers.Run(ctx, group)
	peer.Services.Run(ctx, group)

	return group.Wait()
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
