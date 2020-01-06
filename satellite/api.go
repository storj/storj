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

	"storj.io/common/errs2"
	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/peertls/extensions"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/storj/pkg/auth/grpcauth"
	"storj.io/storj/pkg/server"
	"storj.io/storj/private/post"
	"storj.io/storj/private/post/oauth2"
	"storj.io/storj/private/version"
	"storj.io/storj/private/version/checker"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/console/consoleweb"
	"storj.io/storj/satellite/contact"
	"storj.io/storj/satellite/gracefulexit"
	"storj.io/storj/satellite/inspector"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/mailservice/simulate"
	"storj.io/storj/satellite/marketingweb"
	"storj.io/storj/satellite/metainfo"
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

	Dialer  rpc.Dialer
	Server  *server.Server
	Version *checker.Service

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
		Endpoint *orders.Endpoint
		Service  *orders.Service
	}

	Metainfo struct {
		Database  metainfo.PointerDB
		Service   *metainfo.Service
		Endpoint2 *metainfo.Endpoint
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

	GracefulExit struct {
		Endpoint *gracefulexit.Endpoint
	}
}

// NewAPI creates a new satellite API process
func NewAPI(log *zap.Logger, full *identity.FullIdentity, db DB, pointerDB metainfo.PointerDB, revocationDB extensions.RevocationDB, liveAccounting accounting.Cache, config *Config, versionInfo version.Info) (*API, error) {
	peer := &API{
		Log:      log,
		Identity: full,
		DB:       db,
	}

	var err error

	{
		if !versionInfo.IsZero() {
			peer.Log.Sugar().Debugf("Binary Version: %s with CommitHash %s, built at %s as Release %v",
				versionInfo.Version.String(), versionInfo.CommitHash, versionInfo.Timestamp.String(), versionInfo.Release)
		}
		peer.Version = checker.NewService(log.Named("version"), config.Version, versionInfo, "Satellite")
	}

	{ // setup listener and server
		log.Debug("Satellite API Process starting listener and server")
		sc := config.Server

		tlsOptions, err := tlsopts.NewOptions(peer.Identity, sc.Config, revocationDB)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Dialer = rpc.NewDefaultDialer(tlsOptions)

		unaryInterceptor := grpcauth.NewAPIKeyInterceptor()
		if sc.DebugLogTraffic {
			unaryInterceptor = server.CombineInterceptors(unaryInterceptor, server.UnaryMessageLoggingInterceptor(log))
		}
		peer.Server, err = server.New(log.Named("server"), tlsOptions, sc.Address, sc.PrivateAddress, unaryInterceptor)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
	}

	{ // setup overlay
		log.Debug("Satellite API Process starting overlay")
		peer.Overlay.DB = overlay.NewCombinedCache(peer.DB.OverlayCache())
		peer.Overlay.Service = overlay.NewService(peer.Log.Named("overlay"), peer.Overlay.DB, config.Overlay)
		peer.Overlay.Inspector = overlay.NewInspector(peer.Overlay.Service)
		pb.RegisterOverlayInspectorServer(peer.Server.PrivateGRPC(), peer.Overlay.Inspector)
		pb.DRPCRegisterOverlayInspector(peer.Server.PrivateDRPC(), peer.Overlay.Inspector)
	}

	{ // setup contact service
		log.Debug("Satellite API Process setting up contact service")
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
		peer.Contact.Service = contact.NewService(peer.Log.Named("contact:service"), self, peer.Overlay.Service, peer.DB.PeerIdentities(), peer.Dialer)
		peer.Contact.Endpoint = contact.NewEndpoint(peer.Log.Named("contact:endpoint"), peer.Contact.Service)
		pb.RegisterNodeServer(peer.Server.GRPC(), peer.Contact.Endpoint)
		pb.DRPCRegisterNode(peer.Server.DRPC(), peer.Contact.Endpoint)
	}

	{ // setup vouchers
		log.Debug("Satellite API Process setting up vouchers")
		pb.RegisterVouchersServer(peer.Server.GRPC(), peer.Vouchers.Endpoint)
		pb.DRPCRegisterVouchers(peer.Server.DRPC(), peer.Vouchers.Endpoint)
	}

	{ // setup live accounting
		log.Debug("Satellite API Process setting up live accounting")
		peer.LiveAccounting.Cache = liveAccounting
	}

	{ // setup accounting project usage
		log.Debug("Satellite API Process setting up accounting project usage")
		peer.Accounting.ProjectUsage = accounting.NewService(
			peer.DB.ProjectAccounting(),
			peer.LiveAccounting.Cache,
			config.Rollup.MaxAlphaUsage,
		)
	}

	{ // setup orders
		log.Debug("Satellite API Process setting up orders endpoint")
		satelliteSignee := signing.SigneeFromPeerIdentity(peer.Identity.PeerIdentity())
		peer.Orders.Endpoint = orders.NewEndpoint(
			peer.Log.Named("orders:endpoint"),
			satelliteSignee,
			peer.DB.Orders(),
			config.Orders.SettlementBatchSize,
		)
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
		pb.RegisterOrdersServer(peer.Server.GRPC(), peer.Orders.Endpoint)
		pb.DRPCRegisterOrders(peer.Server.DRPC(), peer.Orders.Endpoint.DRPC())
	}

	{ // setup marketing portal
		log.Debug("Satellite API Process setting up marketing server")

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
	}

	{ // setup metainfo
		log.Debug("Satellite API Process setting up metainfo")
		peer.Metainfo.Database = pointerDB
		peer.Metainfo.Service = metainfo.NewService(peer.Log.Named("metainfo:service"),
			peer.Metainfo.Database,
			peer.DB.Buckets(),
		)
		peer.Metainfo.Endpoint2 = metainfo.NewEndpoint(
			peer.Log.Named("metainfo:endpoint"),
			peer.Metainfo.Service,
			peer.Orders.Service,
			peer.Overlay.Service,
			peer.DB.Attribution(),
			peer.Marketing.PartnersService,
			peer.DB.PeerIdentities(),
			peer.Dialer,
			peer.DB.Console().APIKeys(),
			peer.Accounting.ProjectUsage,
			config.Metainfo.RS,
			signing.SignerFromFullIdentity(peer.Identity),
			config.Metainfo.MaxCommitInterval,
		)
		pb.RegisterMetainfoServer(peer.Server.GRPC(), peer.Metainfo.Endpoint2)
		pb.DRPCRegisterMetainfo(peer.Server.DRPC(), peer.Metainfo.Endpoint2)
	}

	{ // setup datarepair
		log.Debug("Satellite API Process setting up datarepair inspector")
		peer.Repair.Inspector = irreparable.NewInspector(peer.DB.Irreparable())
		pb.RegisterIrreparableInspectorServer(peer.Server.PrivateGRPC(), peer.Repair.Inspector)
		pb.DRPCRegisterIrreparableInspector(peer.Server.PrivateDRPC(), peer.Repair.Inspector)
	}

	{ // setup inspector
		log.Debug("Satellite API Process setting up inspector")
		peer.Inspector.Endpoint = inspector.NewEndpoint(
			peer.Log.Named("inspector"),
			peer.Overlay.Service,
			peer.Metainfo.Service,
		)
		pb.RegisterHealthInspectorServer(peer.Server.PrivateGRPC(), peer.Inspector.Endpoint)
		pb.DRPCRegisterHealthInspector(peer.Server.PrivateDRPC(), peer.Inspector.Endpoint)
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

	{ // setup payments
		log.Debug("Satellite API Process setting up payments")
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
			peer.Payments.Inspector = stripecoinpayments.NewEndpoint(service)

			peer.Payments.Version = stripecoinpayments.NewVersionService(
				peer.Log.Named("stripecoinpayments version service"),
				service,
				pc.StripeCoinPayments.ConversionRatesCycleInterval)

			pb.RegisterPaymentsServer(peer.Server.PrivateGRPC(), peer.Payments.Inspector)
			pb.DRPCRegisterPayments(peer.Server.PrivateDRPC(), peer.Payments.Inspector)
		}
	}

	{ // setup console
		log.Debug("Satellite API Process setting up console")
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
			consoleConfig.PasswordCost,
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
	}

	{ // setup node stats endpoint
		log.Debug("Satellite API Process setting up node stats endpoint")
		peer.NodeStats.Endpoint = nodestats.NewEndpoint(
			peer.Log.Named("nodestats:endpoint"),
			peer.Overlay.DB,
			peer.DB.StoragenodeAccounting(),
		)
		pb.RegisterNodeStatsServer(peer.Server.GRPC(), peer.NodeStats.Endpoint)
		pb.DRPCRegisterNodeStats(peer.Server.DRPC(), peer.NodeStats.Endpoint)
	}

	{ // setup graceful exit
		if config.GracefulExit.Enabled {
			log.Debug("Satellite API Process setting up graceful exit endpoint")
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

			pb.RegisterSatelliteGracefulExitServer(peer.Server.GRPC(), peer.GracefulExit.Endpoint)
			pb.DRPCRegisterSatelliteGracefulExit(peer.Server.DRPC(), peer.GracefulExit.Endpoint.DRPC())
		}
	}

	return peer, nil
}

// Run runs satellite until it's either closed or it errors.
func (peer *API) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	group, ctx := errgroup.WithContext(ctx)

	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Version.Run(ctx))
	})
	group.Go(func() error {
		// Don't change the format of this comment, it is used to figure out the node id.
		peer.Log.Sugar().Infof("Node %s started", peer.Identity.ID)
		peer.Log.Sugar().Infof("Public server started on %s", peer.Addr())
		peer.Log.Sugar().Infof("Private server started on %s", peer.PrivateAddr())
		return errs2.IgnoreCanceled(peer.Server.Run(ctx))
	})
	if peer.Payments.Version != nil {
		group.Go(func() error {
			return errs2.IgnoreCanceled(peer.Payments.Version.Run(ctx))
		})
	}
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Console.Endpoint.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Marketing.Endpoint.Run(ctx))
	})

	return group.Wait()
}

// Close closes all the resources.
func (peer *API) Close() error {
	var errlist errs.Group

	// close servers, to avoid new connections to closing subsystems
	if peer.Server != nil {
		errlist.Add(peer.Server.Close())
	}
	if peer.Marketing.Endpoint != nil {
		errlist.Add(peer.Marketing.Endpoint.Close())
	} else if peer.Marketing.Listener != nil {
		errlist.Add(peer.Marketing.Listener.Close())
	}
	if peer.Console.Endpoint != nil {
		errlist.Add(peer.Console.Endpoint.Close())
	} else if peer.Console.Listener != nil {
		errlist.Add(peer.Console.Listener.Close())
	}
	if peer.Mail.Service != nil {
		errlist.Add(peer.Mail.Service.Close())
	}
	if peer.Payments.Version != nil {
		errlist.Add(peer.Payments.Version.Close())
	}
	if peer.Metainfo.Endpoint2 != nil {
		errlist.Add(peer.Metainfo.Endpoint2.Close())
	}
	if peer.Contact.Service != nil {
		errlist.Add(peer.Contact.Service.Close())
	}
	if peer.Overlay.Service != nil {
		errlist.Add(peer.Overlay.Service.Close())
	}
	return errlist.Err()
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
