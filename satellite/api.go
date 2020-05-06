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

	monkit "github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/pb/pbgrpc"
	"storj.io/common/peertls/extensions"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/private/debug"
	"storj.io/private/version"
	"storj.io/storj/pkg/auth/grpcauth"
	"storj.io/storj/pkg/server"
	"storj.io/storj/private/lifecycle"
	"storj.io/storj/private/post"
	"storj.io/storj/private/post/oauth2"
	"storj.io/storj/private/version/checker"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/console/consoleweb"
	"storj.io/storj/satellite/contact"
	"storj.io/storj/satellite/gracefulexit"
	"storj.io/storj/satellite/heldamount"
	"storj.io/storj/satellite/inspector"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/mailservice/simulate"
	"storj.io/storj/satellite/marketingweb"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/metainfo/piecedeletion"
	"storj.io/storj/satellite/nodestats"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/mockpayments"
	"storj.io/storj/satellite/payments/stripecoinpayments"
	"storj.io/storj/satellite/referrals"
	"storj.io/storj/satellite/repair/irreparable"
	"storj.io/storj/satellite/rewards"
	"storj.io/storj/satellite/vouchers"
)

// API is the satellite API process
//
// architecture: Peer
type API struct {
	Log      *zap.Logger
	Identity *identity.FullIdentity
	DB       DB

	Servers  *lifecycle.Group
	Services *lifecycle.Group

	Dialer rpc.Dialer
	Server *server.Server

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
		DB        overlay.DB
		Service   *overlay.Service
		Inspector *overlay.Inspector
	}

	Vouchers struct {
		Endpoint *vouchers.Endpoint
	}

	Orders struct {
		DB       orders.DB
		Endpoint *orders.Endpoint
		Service  *orders.Service
		Chore    *orders.Chore
	}

	Metainfo struct {
		Database      metainfo.PointerDB
		Service       *metainfo.Service
		PieceDeletion *piecedeletion.Service
		Endpoint2     *metainfo.Endpoint
	}

	Inspector struct {
		Endpoint *inspector.Endpoint
	}

	Repair struct {
		Inspector *irreparable.Inspector
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
		Accounts  payments.Accounts
		Inspector *stripecoinpayments.Endpoint
		Version   *stripecoinpayments.VersionService
	}

	Referrals struct {
		Service *referrals.Service
	}

	Console struct {
		Listener net.Listener
		Service  *console.Service
		Endpoint *consoleweb.Server
	}

	Marketing struct {
		PartnersService *rewards.PartnersService

		Listener net.Listener
		Endpoint *marketingweb.Server
	}

	NodeStats struct {
		Endpoint *nodestats.Endpoint
	}

	HeldAmount struct {
		Endpoint *heldamount.Endpoint
		Service  *heldamount.Service
		DB       heldamount.DB
	}

	GracefulExit struct {
		Endpoint *gracefulexit.Endpoint
	}
}

// NewAPI creates a new satellite API process
func NewAPI(log *zap.Logger, full *identity.FullIdentity, db DB,
	pointerDB metainfo.PointerDB, revocationDB extensions.RevocationDB, liveAccounting accounting.Cache, rollupsWriteCache *orders.RollupsWriteCache,
	config *Config, versionInfo version.Info) (*API, error) {
	peer := &API{
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
				err = nil
			}
		}
		debugConfig := config.Debug
		debugConfig.ControlTitle = "API"
		peer.Debug.Server = debug.NewServer(log.Named("debug"), peer.Debug.Listener, monkit.Default, debugConfig)
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

		apiKeyInterceptor := grpcauth.NewAPIKeyInterceptor()
		var loggingInterceptor grpc.UnaryServerInterceptor
		if sc.DebugLogTraffic {
			loggingInterceptor = server.UnaryMessageLoggingInterceptor(log)
		}

		peer.Server, err = server.New(log.Named("server"), tlsOptions, sc.Address, sc.PrivateAddress, apiKeyInterceptor, loggingInterceptor)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
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

		peer.Overlay.Service = overlay.NewService(peer.Log.Named("overlay"), peer.Overlay.DB, config.Overlay)
		peer.Services.Add(lifecycle.Item{
			Name:  "overlay",
			Close: peer.Overlay.Service.Close,
		})

		peer.Overlay.Inspector = overlay.NewInspector(peer.Overlay.Service)
		pbgrpc.RegisterOverlayInspectorServer(peer.Server.PrivateGRPC(), peer.Overlay.Inspector)
		if err := pb.DRPCRegisterOverlayInspector(peer.Server.PrivateDRPC(), peer.Overlay.Inspector); err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
	}

	{ // setup contact service
		c := config.Contact
		if c.ExternalAddress == "" {
			c.ExternalAddress = peer.Addr()
		}

		pbVersion, err := versionInfo.Proto()
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		self := &overlay.NodeDossier{
			Node: pb.Node{
				Id: peer.ID(),
				Address: &pb.NodeAddress{
					Address: c.ExternalAddress,
				},
			},
			Type:    pb.NodeType_SATELLITE,
			Version: *pbVersion,
		}
		peer.Contact.Service = contact.NewService(peer.Log.Named("contact:service"), self, peer.Overlay.Service, peer.DB.PeerIdentities(), peer.Dialer, config.Contact.Timeout)
		peer.Contact.Endpoint = contact.NewEndpoint(peer.Log.Named("contact:endpoint"), peer.Contact.Service)
		pbgrpc.RegisterNodeServer(peer.Server.GRPC(), peer.Contact.Endpoint)
		if err := pb.DRPCRegisterNode(peer.Server.DRPC(), peer.Contact.Endpoint); err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Services.Add(lifecycle.Item{
			Name:  "contact:service",
			Close: peer.Contact.Service.Close,
		})
	}

	{ // setup vouchers
		pbgrpc.RegisterVouchersServer(peer.Server.GRPC(), peer.Vouchers.Endpoint)
		if err := pb.DRPCRegisterVouchers(peer.Server.DRPC(), peer.Vouchers.Endpoint); err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
	}

	{ // setup live accounting
		peer.LiveAccounting.Cache = liveAccounting
	}

	{ // setup accounting project usage
		peer.Accounting.ProjectUsage = accounting.NewService(
			peer.DB.ProjectAccounting(),
			peer.LiveAccounting.Cache,
			config.Rollup.MaxAlphaUsage,
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

		satelliteSignee := signing.SigneeFromPeerIdentity(peer.Identity.PeerIdentity())
		peer.Orders.Endpoint = orders.NewEndpoint(
			peer.Log.Named("orders:endpoint"),
			satelliteSignee,
			peer.Orders.DB,
			config.Orders.SettlementBatchSize,
		)
		peer.Orders.Service = orders.NewService(
			peer.Log.Named("orders:service"),
			signing.SignerFromFullIdentity(peer.Identity),
			peer.Overlay.Service,
			peer.Orders.DB,
			config.Orders.Expiration,
			&pb.NodeAddress{
				Transport: pb.NodeTransport_TCP_TLS_GRPC,
				Address:   config.Contact.ExternalAddress,
			},
			config.Repairer.MaxExcessRateOptimalThreshold,
			config.Orders.NodeStatusLogging,
		)
		pbgrpc.RegisterOrdersServer(peer.Server.GRPC(), peer.Orders.Endpoint)
		if err := pb.DRPCRegisterOrders(peer.Server.DRPC(), peer.Orders.Endpoint.DRPC()); err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
	}

	{ // setup marketing portal
		peer.Marketing.PartnersService = rewards.NewPartnersService(
			peer.Log.Named("partners"),
			rewards.DefaultPartnersDB,
			[]string{
				"https://us-central-1.tardigrade.io/",
				"https://asia-east-1.tardigrade.io/",
				"https://europe-west-1.tardigrade.io/",
			},
		)

		peer.Marketing.Listener, err = net.Listen("tcp", config.Marketing.Address)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Marketing.Endpoint, err = marketingweb.NewServer(
			peer.Log.Named("marketing:endpoint"),
			config.Marketing,
			peer.DB.Rewards(),
			peer.Marketing.PartnersService,
			peer.Marketing.Listener,
		)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Servers.Add(lifecycle.Item{
			Name:  "marketing:endpoint",
			Run:   peer.Marketing.Endpoint.Run,
			Close: peer.Marketing.Endpoint.Close,
		})
	}

	{ // setup metainfo
		peer.Metainfo.Database = pointerDB
		peer.Metainfo.Service = metainfo.NewService(peer.Log.Named("metainfo:service"),
			peer.Metainfo.Database,
			peer.DB.Buckets(),
		)

		peer.Metainfo.PieceDeletion, err = piecedeletion.NewService(
			peer.Log.Named("metainfo:piecedeletion"),
			peer.Dialer,
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

		peer.Metainfo.Endpoint2, err = metainfo.NewEndpoint(
			peer.Log.Named("metainfo:endpoint"),
			peer.Metainfo.Service,
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
			config.Metainfo,
		)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		pbgrpc.RegisterMetainfoServer(peer.Server.GRPC(), peer.Metainfo.Endpoint2)
		if err := pb.DRPCRegisterMetainfo(peer.Server.DRPC(), peer.Metainfo.Endpoint2); err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Services.Add(lifecycle.Item{
			Name:  "metainfo:endpoint",
			Close: peer.Metainfo.Endpoint2.Close,
		})
	}

	{ // setup datarepair
		peer.Repair.Inspector = irreparable.NewInspector(peer.DB.Irreparable())
		pbgrpc.RegisterIrreparableInspectorServer(peer.Server.PrivateGRPC(), peer.Repair.Inspector)
		if err := pb.DRPCRegisterIrreparableInspector(peer.Server.PrivateDRPC(), peer.Repair.Inspector); err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
	}

	{ // setup inspector
		peer.Inspector.Endpoint = inspector.NewEndpoint(
			peer.Log.Named("inspector"),
			peer.Overlay.Service,
			peer.Metainfo.Service,
		)
		pbgrpc.RegisterHealthInspectorServer(peer.Server.PrivateGRPC(), peer.Inspector.Endpoint)
		if err := pb.DRPCRegisterHealthInspector(peer.Server.PrivateDRPC(), peer.Inspector.Endpoint); err != nil {
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

		peer.Services.Add(lifecycle.Item{
			Name:  "mail:service",
			Close: peer.Mail.Service.Close,
		})
	}

	{ // setup payments
		pc := config.Payments

		switch pc.Provider {
		default:
			peer.Payments.Accounts = mockpayments.Accounts()
		case "stripecoinpayments":
			service, err := stripecoinpayments.NewService(
				peer.Log.Named("payments.stripe:service"),
				pc.StripeCoinPayments,
				peer.DB.StripeCoinPayments(),
				peer.DB.Console().Projects(),
				peer.DB.ProjectAccounting(),
				pc.StorageTBPrice,
				pc.EgressTBPrice,
				pc.ObjectPrice,
				pc.BonusRate,
				pc.CouponValue,
				pc.CouponDuration,
				pc.CouponProjectLimit,
				pc.MinCoinPayment)

			if err != nil {
				return nil, errs.Combine(err, peer.Close())
			}

			peer.Payments.Accounts = service.Accounts()
			peer.Payments.Inspector = stripecoinpayments.NewEndpoint(service)

			peer.Payments.Version = stripecoinpayments.NewVersionService(
				peer.Log.Named("payments.stripe:version"),
				service,
				pc.StripeCoinPayments.ConversionRatesCycleInterval)

			pbgrpc.RegisterPaymentsServer(peer.Server.PrivateGRPC(), peer.Payments.Inspector)
			if err := pb.DRPCRegisterPayments(peer.Server.PrivateDRPC(), peer.Payments.Inspector); err != nil {
				return nil, errs.Combine(err, peer.Close())
			}

			peer.Services.Add(lifecycle.Item{
				Name:  "payments.stripe:version",
				Run:   peer.Payments.Version.Run,
				Close: peer.Payments.Version.Close,
			})
		}
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

		peer.Referrals.Service = referrals.NewService(
			peer.Log.Named("referrals:service"),
			signing.SignerFromFullIdentity(peer.Identity),
			config.Referrals,
			peer.Dialer,
			peer.DB.Console().Users(),
			consoleConfig.PasswordCost,
		)

		peer.Console.Service, err = console.NewService(
			peer.Log.Named("console:service"),
			&consoleauth.Hmac{Secret: []byte(consoleConfig.AuthTokenSecret)},
			peer.DB.Console(),
			peer.DB.ProjectAccounting(),
			peer.Accounting.ProjectUsage,
			peer.DB.Rewards(),
			peer.Marketing.PartnersService,
			peer.Payments.Accounts,
			consoleConfig.Config,
			config.Payments.MinCoinPayment,
		)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Console.Endpoint = consoleweb.NewServer(
			peer.Log.Named("console:endpoint"),
			consoleConfig,
			peer.Console.Service,
			peer.Mail.Service,
			peer.Referrals.Service,
			peer.Console.Listener,
			config.Payments.StripeCoinPayments.StripePublicKey,
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
			peer.DB.StoragenodeAccounting(),
			config.Payments,
		)
		pbgrpc.RegisterNodeStatsServer(peer.Server.GRPC(), peer.NodeStats.Endpoint)
		if err := pb.DRPCRegisterNodeStats(peer.Server.DRPC(), peer.NodeStats.Endpoint); err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
	}

	{ // setup heldamount endpoint
		peer.HeldAmount.DB = peer.DB.HeldAmount()
		peer.HeldAmount.Service = heldamount.NewService(
			peer.Log.Named("heldamount:service"),
			peer.HeldAmount.DB)
		peer.HeldAmount.Endpoint = heldamount.NewEndpoint(
			peer.Log.Named("heldamount:endpoint"),
			peer.DB.StoragenodeAccounting(),
			peer.Overlay.DB,
			peer.HeldAmount.Service)
		pbgrpc.RegisterHeldAmountServer(peer.Server.GRPC(), peer.HeldAmount.Endpoint)
		if err := pb.DRPCRegisterHeldAmount(peer.Server.DRPC(), peer.HeldAmount.Endpoint); err != nil {
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
				peer.Metainfo.Service,
				peer.Orders.Service,
				peer.DB.PeerIdentities(),
				config.GracefulExit)

			pbgrpc.RegisterSatelliteGracefulExitServer(peer.Server.GRPC(), peer.GracefulExit.Endpoint)
			if err := pb.DRPCRegisterSatelliteGracefulExit(peer.Server.DRPC(), peer.GracefulExit.Endpoint.DRPC()); err != nil {
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

// Local returns the peer local node info.
func (peer *API) Local() overlay.NodeDossier { return peer.Contact.Service.Local() }

// Addr returns the public address.
func (peer *API) Addr() string { return peer.Server.Addr().String() }

// URL returns the storj.NodeURL.
func (peer *API) URL() storj.NodeURL {
	return storj.NodeURL{ID: peer.ID(), Address: peer.Addr()}
}

// PrivateAddr returns the private address.
func (peer *API) PrivateAddr() string { return peer.Server.PrivateAddr().String() }
