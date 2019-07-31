// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"path/filepath"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/errs2"
	"storj.io/storj/internal/post"
	"storj.io/storj/internal/post/oauth2"
	"storj.io/storj/internal/version"
	"storj.io/storj/pkg/auth/grpcauth"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/server"
	"storj.io/storj/pkg/signing"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/accounting/live"
	"storj.io/storj/satellite/accounting/rollup"
	"storj.io/storj/satellite/accounting/tally"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/certdb"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/console/consoleweb"
	"storj.io/storj/satellite/discovery"
	"storj.io/storj/satellite/gc"
	"storj.io/storj/satellite/inspector"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/mailservice/simulate"
	"storj.io/storj/satellite/marketingweb"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/nodestats"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/localpayments"
	"storj.io/storj/satellite/payments/stripepayments"
	"storj.io/storj/satellite/repair/checker"
	"storj.io/storj/satellite/repair/irreparable"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/satellite/repair/repairer"
	"storj.io/storj/satellite/rewards"
	"storj.io/storj/satellite/vouchers"
	"storj.io/storj/storage"
	"storj.io/storj/storage/boltdb"
)

var mon = monkit.Package()

// DB is the master database for the satellite
type DB interface {
	// CreateTables initializes the database
	CreateTables() error
	// Close closes the database
	Close() error

	// CreateSchema sets the schema
	CreateSchema(schema string) error
	// DropSchema drops the schema
	DropSchema(schema string) error

	// CertDB returns database for storing uplink's public key & ID
	CertDB() certdb.DB
	// OverlayCache returns database for caching overlay information
	OverlayCache() overlay.DB
	// Attribution returns database for partner keys information
	Attribution() attribution.DB
	// StoragenodeAccounting returns database for storing information about storagenode use
	StoragenodeAccounting() accounting.StoragenodeAccounting
	// ProjectAccounting returns database for storing information about project data use
	ProjectAccounting() accounting.ProjectAccounting
	// RepairQueue returns queue for segments that need repairing
	RepairQueue() queue.RepairQueue
	// Irreparable returns database for failed repairs
	Irreparable() irreparable.DB
	// Console returns database for satellite console
	Console() console.DB
	//  returns database for marketing admin GUI
	Rewards() rewards.DB
	// Orders returns database for orders
	Orders() orders.DB
	// Containment returns database for containment
	Containment() audit.Containment
	// Buckets returns the database to interact with buckets
	Buckets() metainfo.BucketsDB
}

// Config is the global config satellite
type Config struct {
	Identity identity.Config
	Server   server.Config

	Kademlia  kademlia.Config
	Overlay   overlay.Config
	Discovery discovery.Config

	Metainfo metainfo.Config
	Orders   orders.Config

	Checker  checker.Config
	Repairer repairer.Config
	Audit    audit.Config

	GarbageCollection gc.Config

	Tally          tally.Config
	Rollup         rollup.Config
	LiveAccounting live.Config

	Mail    mailservice.Config
	Console consoleweb.Config

	Marketing marketingweb.Config
	Vouchers  vouchers.Config

	Version version.Config
}

// Peer is the satellite
type Peer struct {
	// core dependencies
	Log      *zap.Logger
	Identity *identity.FullIdentity
	DB       DB

	Transport transport.Client

	Server *server.Server

	Version *version.Service

	// services and endpoints
	Kademlia struct {
		kdb, ndb, adb storage.KeyValueStore // TODO: move these into DB

		RoutingTable *kademlia.RoutingTable
		Service      *kademlia.Kademlia
		Endpoint     *kademlia.Endpoint
		Inspector    *kademlia.Inspector
	}

	Overlay struct {
		Service   *overlay.Cache
		Inspector *overlay.Inspector
	}

	Discovery struct {
		Service *discovery.Discovery
	}

	Metainfo struct {
		Database  storage.KeyValueStore // TODO: move into pointerDB
		Service   *metainfo.Service
		Endpoint2 *metainfo.Endpoint
		Loop      *metainfo.Loop
	}

	Inspector struct {
		Endpoint *inspector.Endpoint
	}

	Orders struct {
		Endpoint *orders.Endpoint
		Service  *orders.Service
	}

	Repair struct {
		Checker   *checker.Checker
		Repairer  *repairer.Service
		Inspector *irreparable.Inspector
	}
	Audit struct {
		Service *audit.Service
	}

	GarbageCollection struct {
		Service *gc.Service
	}

	Accounting struct {
		Tally        *tally.Service
		Rollup       *rollup.Service
		ProjectUsage *accounting.ProjectUsage
	}

	LiveAccounting struct {
		Service live.Service
	}

	Mail struct {
		Service *mailservice.Service
	}

	Vouchers struct {
		Endpoint *vouchers.Endpoint
	}

	Console struct {
		Listener net.Listener
		Service  *console.Service
		Endpoint *consoleweb.Server
	}

	Marketing struct {
		Listener net.Listener
		Endpoint *marketingweb.Server
	}

	NodeStats struct {
		Endpoint *nodestats.Endpoint
	}
}

// New creates a new satellite
func New(log *zap.Logger, full *identity.FullIdentity, db DB, config *Config, versionInfo version.Info) (*Peer, error) {
	peer := &Peer{
		Log:      log,
		Identity: full,
		DB:       db,
	}

	var err error

	{
		test := version.Info{}
		if test != versionInfo {
			peer.Log.Sugar().Debugf("Binary Version: %s with CommitHash %s, built at %s as Release %v",
				versionInfo.Version.String(), versionInfo.CommitHash, versionInfo.Timestamp.String(), versionInfo.Release)
		}
		peer.Version = version.NewService(log.Named("version"), config.Version, versionInfo, "Satellite")
	}

	{ // setup listener and server
		log.Debug("Starting listener and server")
		sc := config.Server
		options, err := tlsopts.NewOptions(peer.Identity, sc.Config)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Transport = transport.NewClient(options)

		unaryInterceptor := grpcauth.NewAPIKeyInterceptor()
		if sc.DebugLogTraffic {
			unaryInterceptor = server.CombineInterceptors(unaryInterceptor, server.UnaryMessageLoggingInterceptor(log))
		}
		peer.Server, err = server.New(log.Named("server"), options, sc.Address, sc.PrivateAddress, unaryInterceptor)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
	}

	{ // setup overlay
		log.Debug("Starting overlay")

		peer.Overlay.Service = overlay.NewCache(peer.Log.Named("overlay"), peer.DB.OverlayCache(), config.Overlay)
		peer.Transport = peer.Transport.WithObservers(peer.Overlay.Service)

		peer.Overlay.Inspector = overlay.NewInspector(peer.Overlay.Service)
		pb.RegisterOverlayInspectorServer(peer.Server.PrivateGRPC(), peer.Overlay.Inspector)
	}

	{ // setup kademlia
		log.Debug("Setting up Kademlia")
		config := config.Kademlia
		// TODO: move this setup logic into kademlia package
		if config.ExternalAddress == "" {
			config.ExternalAddress = peer.Addr()
		}

		pbVersion, err := versionInfo.Proto()
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		self := &overlay.NodeDossier{
			Node: pb.Node{
				Id: peer.ID(),
				Address: &pb.NodeAddress{
					Address: config.ExternalAddress,
				},
			},
			Type: pb.NodeType_SATELLITE,
			Operator: pb.NodeOperator{
				Email:  config.Operator.Email,
				Wallet: config.Operator.Wallet,
			},
			Version: *pbVersion,
		}

		{ // setup routing table
			// TODO: clean this up, should be part of database
			log.Debug("Setting up routing table")
			bucketIdentifier := peer.ID().String()[:5] // need a way to differentiate between nodes if running more than one simultaneously
			dbpath := filepath.Join(config.DBPath, fmt.Sprintf("kademlia_%s.db", bucketIdentifier))

			if err := os.MkdirAll(config.DBPath, 0777); err != nil && !os.IsExist(err) {
				return nil, err
			}

			dbs, err := boltdb.NewShared(dbpath, kademlia.KademliaBucket, kademlia.NodeBucket, kademlia.AntechamberBucket)
			if err != nil {
				return nil, errs.Combine(err, peer.Close())
			}
			peer.Kademlia.kdb, peer.Kademlia.ndb, peer.Kademlia.adb = dbs[0], dbs[1], dbs[2]

			peer.Kademlia.RoutingTable, err = kademlia.NewRoutingTable(peer.Log.Named("routing"), self, peer.Kademlia.kdb, peer.Kademlia.ndb, peer.Kademlia.adb, &config.RoutingTableConfig)
			if err != nil {
				return nil, errs.Combine(err, peer.Close())
			}

			peer.Transport = peer.Transport.WithObservers(peer.Kademlia.RoutingTable)
		}

		peer.Kademlia.Service, err = kademlia.NewService(peer.Log.Named("kademlia"), peer.Transport, peer.Kademlia.RoutingTable, config)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Kademlia.Endpoint = kademlia.NewEndpoint(peer.Log.Named("kademlia:endpoint"), peer.Kademlia.Service, peer.Kademlia.RoutingTable)
		pb.RegisterNodesServer(peer.Server.GRPC(), peer.Kademlia.Endpoint)

		peer.Kademlia.Inspector = kademlia.NewInspector(peer.Kademlia.Service, peer.Identity)
		pb.RegisterKadInspectorServer(peer.Server.PrivateGRPC(), peer.Kademlia.Inspector)
	}

	{ // setup discovery
		log.Debug("Setting up discovery")
		config := config.Discovery
		peer.Discovery.Service = discovery.New(peer.Log.Named("discovery"), peer.Overlay.Service, peer.Kademlia.Service, config)
	}

	{ // setup vouchers
		log.Debug("Setting up vouchers")
		peer.Vouchers.Endpoint = vouchers.NewEndpoint(
			peer.Log.Named("vouchers"),
			signing.SignerFromFullIdentity(peer.Identity),
			peer.Overlay.Service,
			config.Vouchers.Expiration,
		)
		pb.RegisterVouchersServer(peer.Server.GRPC(), peer.Vouchers.Endpoint)
	}

	{ // setup live accounting
		log.Debug("Setting up live accounting")
		config := config.LiveAccounting
		liveAccountingService, err := live.New(peer.Log.Named("live-accounting"), config)
		if err != nil {
			return nil, err
		}
		peer.LiveAccounting.Service = liveAccountingService
	}

	{ // setup accounting project usage
		log.Debug("Setting up accounting project usage")
		peer.Accounting.ProjectUsage = accounting.NewProjectUsage(
			peer.DB.ProjectAccounting(),
			peer.LiveAccounting.Service,
			config.Rollup.MaxAlphaUsage,
		)
	}

	{ // setup orders
		log.Debug("Setting up orders")
		satelliteSignee := signing.SigneeFromPeerIdentity(peer.Identity.PeerIdentity())
		peer.Orders.Endpoint = orders.NewEndpoint(
			peer.Log.Named("orders:endpoint"),
			satelliteSignee,
			peer.DB.CertDB(),
			peer.DB.Orders(),
		)
		peer.Orders.Service = orders.NewService(
			peer.Log.Named("orders:service"),
			signing.SignerFromFullIdentity(peer.Identity),
			peer.Overlay.Service,
			peer.DB.Orders(),
			config.Orders.Expiration,
			&pb.NodeAddress{
				Transport: pb.NodeTransport_TCP_TLS_GRPC,
				Address:   config.Kademlia.ExternalAddress,
			},
			config.Repairer.MaxExcessRateOptimalThreshold,
		)
		pb.RegisterOrdersServer(peer.Server.GRPC(), peer.Orders.Endpoint)
	}

	{ // setup metainfo
		log.Debug("Setting up metainfo")
		db, err := metainfo.NewStore(peer.Log.Named("metainfo:store"), config.Metainfo.DatabaseURL)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Metainfo.Database = db // for logging: storelogger.New(peer.Log.Named("pdb"), db)
		peer.Metainfo.Service = metainfo.NewService(peer.Log.Named("metainfo:service"),
			peer.Metainfo.Database,
			peer.DB.Buckets(),
		)
		peer.Metainfo.Loop = metainfo.NewLoop(config.Metainfo.Loop, peer.Metainfo.Service)

		peer.Metainfo.Endpoint2 = metainfo.NewEndpoint(
			peer.Log.Named("metainfo:endpoint"),
			peer.Metainfo.Service,
			peer.Orders.Service,
			peer.Overlay.Service,
			peer.DB.Attribution(),
			peer.DB.Containment(),
			peer.DB.Console().APIKeys(),
			peer.Accounting.ProjectUsage,
			config.Metainfo.RS,
			signing.SignerFromFullIdentity(peer.Identity),
		)

		pb.RegisterMetainfoServer(peer.Server.GRPC(), peer.Metainfo.Endpoint2)
	}

	{ // setup datarepair
		log.Debug("Setting up datarepair")
		// TODO: simplify argument list somehow
		peer.Repair.Checker = checker.NewChecker(
			peer.Metainfo.Service,
			peer.DB.RepairQueue(),
			peer.Overlay.Service, peer.DB.Irreparable(),
			0, peer.Log.Named("checker"),
			config.Checker)

		peer.Repair.Repairer = repairer.NewService(
			peer.Log.Named("repairer"),
			peer.DB.RepairQueue(),
			&config.Repairer,
			config.Repairer.Interval,
			config.Repairer.MaxRepair,
			peer.Transport,
			peer.Metainfo.Service,
			peer.Orders.Service,
			peer.Overlay.Service,
		)

		peer.Repair.Inspector = irreparable.NewInspector(peer.DB.Irreparable())
		pb.RegisterIrreparableInspectorServer(peer.Server.PrivateGRPC(), peer.Repair.Inspector)
	}

	{ // setup audit
		log.Debug("Setting up audits")
		config := config.Audit

		peer.Audit.Service, err = audit.NewService(peer.Log.Named("audit"),
			config,
			peer.Metainfo.Service,
			peer.Orders.Service,
			peer.Transport,
			peer.Overlay.Service,
			peer.DB.Containment(),
			peer.Identity,
		)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
	}

	{ // setup garbage collection
		log.Debug("Setting up garbage collection")

		peer.GarbageCollection.Service = gc.NewService(
			peer.Log.Named("garbage collection"),
			config.GarbageCollection,
			peer.Transport,
			peer.DB.OverlayCache(),
			peer.Metainfo.Loop,
		)
	}

	{ // setup accounting
		log.Debug("Setting up accounting")
		peer.Accounting.Tally = tally.New(peer.Log.Named("tally"), peer.DB.StoragenodeAccounting(), peer.DB.ProjectAccounting(), peer.LiveAccounting.Service, peer.Metainfo.Service, peer.Overlay.Service, 0, config.Tally.Interval)
		peer.Accounting.Rollup = rollup.New(peer.Log.Named("rollup"), peer.DB.StoragenodeAccounting(), config.Rollup.Interval, config.Rollup.DeleteTallies)
	}

	{ // setup inspector
		log.Debug("Setting up inspector")
		peer.Inspector.Endpoint = inspector.NewEndpoint(
			peer.Log.Named("inspector"),
			peer.Overlay.Service,
			peer.Metainfo.Service,
		)

		pb.RegisterHealthInspectorServer(peer.Server.PrivateGRPC(), peer.Inspector.Endpoint)
	}

	{ // setup mailservice
		log.Debug("Setting up mail service")
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

	{ // setup console
		log.Debug("Setting up console")
		consoleConfig := config.Console

		peer.Console.Listener, err = net.Listen("tcp", consoleConfig.Address)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		if consoleConfig.AuthTokenSecret == "" {
			return nil, errs.New("Auth token secret required")
		}

		// TODO: change mock implementation to using mock stripe backend
		var pmService payments.Service
		if consoleConfig.StripeKey != "" {
			pmService = stripepayments.NewService(peer.Log.Named("stripe:service"), consoleConfig.StripeKey)
		} else {
			pmService = localpayments.NewService(nil)
		}

		peer.Console.Service, err = console.NewService(
			peer.Log.Named("console:service"),
			&consoleauth.Hmac{Secret: []byte(consoleConfig.AuthTokenSecret)},
			peer.DB.Console(),
			peer.DB.Rewards(),
			pmService,
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
			peer.Console.Listener,
		)
	}

	{ // setup marketing portal
		log.Debug("Setting up marketing server")
		marketingConfig := config.Marketing

		peer.Marketing.Listener, err = net.Listen("tcp", marketingConfig.Address)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Marketing.Endpoint, err = marketingweb.NewServer(
			peer.Log.Named("marketing:endpoint"),
			marketingConfig,
			peer.DB.Rewards(),
			peer.Marketing.Listener,
		)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
	}

	{ // setup node stats endpoint
		log.Debug("Setting up node stats endpoint")

		peer.NodeStats.Endpoint = nodestats.NewEndpoint(
			peer.Log.Named("nodestats:endpoint"),
			peer.DB.OverlayCache(),
			peer.DB.StoragenodeAccounting())

		pb.RegisterNodeStatsServer(peer.Server.GRPC(), peer.NodeStats.Endpoint)
	}

	return peer, nil
}

// Run runs satellite until it's either closed or it errors.
func (peer *Peer) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	group, ctx := errgroup.WithContext(ctx)

	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Version.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Kademlia.Service.Bootstrap(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Kademlia.Service.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Discovery.Service.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Repair.Checker.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Metainfo.Loop.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Repair.Repairer.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Accounting.Tally.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Accounting.Rollup.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Audit.Service.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.GarbageCollection.Service.Run(ctx))
	})
	group.Go(func() error {
		// TODO: move the message into Server instead
		// Don't change the format of this comment, it is used to figure out the node id.
		peer.Log.Sugar().Infof("Node %s started", peer.Identity.ID)
		peer.Log.Sugar().Infof("Public server started on %s", peer.Addr())
		peer.Log.Sugar().Infof("Private server started on %s", peer.PrivateAddr())
		return errs2.IgnoreCanceled(peer.Server.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Console.Endpoint.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Marketing.Endpoint.Run(ctx))
	})

	return group.Wait()
}

// Close closes all the resources.
func (peer *Peer) Close() error {
	var errlist errs.Group

	// TODO: ensure that Close can be called on nil-s that way this code won't need the checks.

	// close servers, to avoid new connections to closing subsystems
	if peer.Server != nil {
		errlist.Add(peer.Server.Close())
	}

	if peer.Console.Endpoint != nil {
		errlist.Add(peer.Console.Endpoint.Close())
	} else if peer.Console.Listener != nil {
		errlist.Add(peer.Console.Listener.Close())
	}

	if peer.Mail.Service != nil {
		errlist.Add(peer.Mail.Service.Close())
	}

	if peer.Marketing.Endpoint != nil {
		errlist.Add(peer.Marketing.Endpoint.Close())
	} else if peer.Marketing.Listener != nil {
		errlist.Add(peer.Marketing.Listener.Close())
	}

	// close services in reverse initialization order
	if peer.Repair.Repairer != nil {
		errlist.Add(peer.Repair.Repairer.Close())
	}
	if peer.Repair.Checker != nil {
		errlist.Add(peer.Repair.Checker.Close())
	}

	if peer.Metainfo.Database != nil {
		errlist.Add(peer.Metainfo.Database.Close())
	}

	if peer.Discovery.Service != nil {
		errlist.Add(peer.Discovery.Service.Close())
	}

	// TODO: add kademlia.Endpoint for consistency
	if peer.Kademlia.Service != nil {
		errlist.Add(peer.Kademlia.Service.Close())
	}
	if peer.Kademlia.RoutingTable != nil {
		errlist.Add(peer.Kademlia.RoutingTable.Close())
	}

	if peer.Overlay.Service != nil {
		errlist.Add(peer.Overlay.Service.Close())
	}

	if peer.Kademlia.ndb != nil || peer.Kademlia.kdb != nil || peer.Kademlia.adb != nil {
		errlist.Add(peer.Kademlia.kdb.Close())
		errlist.Add(peer.Kademlia.ndb.Close())
		errlist.Add(peer.Kademlia.adb.Close())
	}

	return errlist.Err()
}

// ID returns the peer ID.
func (peer *Peer) ID() storj.NodeID { return peer.Identity.ID }

// Local returns the peer local node info.
func (peer *Peer) Local() overlay.NodeDossier { return peer.Kademlia.RoutingTable.Local() }

// Addr returns the public address.
func (peer *Peer) Addr() string { return peer.Server.Addr().String() }

// URL returns the storj.NodeURL.
func (peer *Peer) URL() storj.NodeURL { return storj.NodeURL{ID: peer.ID(), Address: peer.Addr()} }

// PrivateAddr returns the private address.
func (peer *Peer) PrivateAddr() string { return peer.Server.PrivateAddr().String() }
