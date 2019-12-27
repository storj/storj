// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenode

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/common/errs2"
	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/peertls/extensions"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/storj/pkg/server"
	"storj.io/storj/private/version"
	"storj.io/storj/private/version/checker"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/storage"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/collector"
	"storj.io/storj/storagenode/console"
	"storj.io/storj/storagenode/console/consoleassets"
	"storj.io/storj/storagenode/console/consoleserver"
	"storj.io/storj/storagenode/contact"
	"storj.io/storj/storagenode/gracefulexit"
	"storj.io/storj/storagenode/inspector"
	"storj.io/storj/storagenode/monitor"
	"storj.io/storj/storagenode/nodestats"
	"storj.io/storj/storagenode/notifications"
	"storj.io/storj/storagenode/orders"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/piecestore"
	"storj.io/storj/storagenode/reputation"
	"storj.io/storj/storagenode/retain"
	"storj.io/storj/storagenode/satellites"
	"storj.io/storj/storagenode/storageusage"
	"storj.io/storj/storagenode/trust"
)

var (
	mon = monkit.Package()
)

// DB is the master database for Storage Node
//
// architecture: Master Database
type DB interface {
	// CreateTables initializes the database
	CreateTables(ctx context.Context) error
	// Close closes the database
	Close() error

	Pieces() storage.Blobs

	Orders() orders.DB
	V0PieceInfo() pieces.V0PieceInfoDB
	PieceExpirationDB() pieces.PieceExpirationDB
	PieceSpaceUsedDB() pieces.PieceSpaceUsedDB
	Bandwidth() bandwidth.DB
	UsedSerials() piecestore.UsedSerials
	Reputation() reputation.DB
	StorageUsage() storageusage.DB
	Satellites() satellites.DB
	Notifications() notifications.DB
}

// Config is all the configuration parameters for a Storage Node
type Config struct {
	Identity identity.Config

	Server server.Config

	Contact  contact.Config
	Operator OperatorConfig

	// TODO: flatten storage config and only keep the new one
	Storage   piecestore.OldConfig
	Storage2  piecestore.Config
	Collector collector.Config

	Retain retain.Config

	Nodestats nodestats.Config

	Console consoleserver.Config

	Version checker.Config

	Bandwidth bandwidth.Config

	GracefulExit gracefulexit.Config
}

// Verify verifies whether configuration is consistent and acceptable.
func (config *Config) Verify(log *zap.Logger) error {
	return config.Operator.Verify(log)
}

// Peer is the representation of a Storage Node.
//
// architecture: Peer
type Peer struct {
	// core dependencies
	Log      *zap.Logger
	Identity *identity.FullIdentity
	DB       DB

	Dialer rpc.Dialer

	Server *server.Server

	Version *checker.Service

	// services and endpoints
	// TODO: similar grouping to satellite.Core

	Contact struct {
		Service   *contact.Service
		Chore     *contact.Chore
		Endpoint  *contact.Endpoint
		PingStats *contact.PingStats
	}

	Storage2 struct {
		// TODO: lift things outside of it to organize better
		Trust         *trust.Pool
		Store         *pieces.Store
		TrashChore    *pieces.TrashChore
		BlobsCache    *pieces.BlobsUsageCache
		CacheService  *pieces.CacheService
		RetainService *retain.Service
		Endpoint      *piecestore.Endpoint
		Inspector     *inspector.Endpoint
		Monitor       *monitor.Service
		Orders        *orders.Service
	}

	Collector *collector.Service

	NodeStats struct {
		Service *nodestats.Service
		Cache   *nodestats.Cache
	}

	// Web server with web UI
	Console struct {
		Listener net.Listener
		Service  *console.Service
		Endpoint *consoleserver.Server
	}

	GracefulExit struct {
		Endpoint *gracefulexit.Endpoint
		Chore    *gracefulexit.Chore
	}

	Notifications struct {
		Service *notifications.Service
	}

	Bandwidth *bandwidth.Service
}

// New creates a new Storage Node.
func New(log *zap.Logger, full *identity.FullIdentity, db DB, revocationDB extensions.RevocationDB, config Config, versionInfo version.Info) (*Peer, error) {
	peer := &Peer{
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
		peer.Version = checker.NewService(log.Named("version"), config.Version, versionInfo, "Storagenode")
	}

	{ // setup listener and server
		sc := config.Server

		tlsOptions, err := tlsopts.NewOptions(peer.Identity, sc.Config, revocationDB)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Dialer = rpc.NewDefaultDialer(tlsOptions)

		peer.Server, err = server.New(log.Named("server"), tlsOptions, sc.Address, sc.PrivateAddress, nil)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
	}

	{ // setup trust pool
		peer.Storage2.Trust, err = trust.NewPool(log.Named("trust"), trust.Dialer(peer.Dialer), config.Storage2.Trust)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
	}

	{ // setup notification service.
		peer.Notifications.Service = notifications.NewService(peer.Log, peer.DB.Notifications())
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
					Transport: pb.NodeTransport_TCP_TLS_GRPC,
					Address:   c.ExternalAddress,
				},
			},
			Type: pb.NodeType_STORAGE,
			Operator: pb.NodeOperator{
				Email:  config.Operator.Email,
				Wallet: config.Operator.Wallet,
			},
			Version: *pbVersion,
		}
		peer.Contact.PingStats = new(contact.PingStats)
		peer.Contact.Service = contact.NewService(peer.Log.Named("contact:service"), self)
		peer.Contact.Chore = contact.NewChore(peer.Log.Named("contact:chore"), config.Contact.Interval, peer.Storage2.Trust, peer.Dialer, peer.Contact.Service)
		peer.Contact.Endpoint = contact.NewEndpoint(peer.Log.Named("contact:endpoint"), peer.Contact.PingStats)
		pb.RegisterContactServer(peer.Server.GRPC(), peer.Contact.Endpoint)
		pb.DRPCRegisterContact(peer.Server.DRPC(), peer.Contact.Endpoint)
	}

	{ // setup storage
		peer.Storage2.BlobsCache = pieces.NewBlobsUsageCache(peer.DB.Pieces())

		peer.Storage2.Store = pieces.NewStore(peer.Log.Named("pieces"),
			peer.Storage2.BlobsCache,
			peer.DB.V0PieceInfo(),
			peer.DB.PieceExpirationDB(),
			peer.DB.PieceSpaceUsedDB(),
		)

		peer.Storage2.TrashChore = pieces.NewTrashChore(
			log.Named("pieces:trashchore"),
			24*time.Hour,   // choreInterval: how often to run the chore
			7*24*time.Hour, // trashExpiryInterval: when items in the trash should be deleted
			peer.Storage2.Trust,
			peer.Storage2.Store,
		)

		peer.Storage2.CacheService = pieces.NewService(
			log.Named("piecestore:cacheUpdate"),
			peer.Storage2.BlobsCache,
			peer.Storage2.Store,
			config.Storage2.CacheSyncInterval,
		)

		peer.Storage2.Monitor = monitor.NewService(
			log.Named("piecestore:monitor"),
			peer.Storage2.Store,
			peer.Contact.Service,
			peer.DB.Bandwidth(),
			config.Storage.AllocatedDiskSpace.Int64(),
			config.Storage.AllocatedBandwidth.Int64(),
			//TODO use config.Storage.Monitor.Interval, but for some reason is not set
			config.Storage.KBucketRefreshInterval,
			config.Storage2.Monitor,
		)

		peer.Storage2.RetainService = retain.NewService(
			peer.Log.Named("retain"),
			peer.Storage2.Store,
			config.Retain,
		)

		peer.Storage2.Endpoint, err = piecestore.NewEndpoint(
			peer.Log.Named("piecestore"),
			signing.SignerFromFullIdentity(peer.Identity),
			peer.Storage2.Trust,
			peer.Storage2.Monitor,
			peer.Storage2.RetainService,
			peer.Contact.PingStats,
			peer.Storage2.Store,
			peer.DB.Orders(),
			peer.DB.Bandwidth(),
			peer.DB.UsedSerials(),
			config.Storage2,
		)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
		pb.RegisterPiecestoreServer(peer.Server.GRPC(), peer.Storage2.Endpoint)
		pb.DRPCRegisterPiecestore(peer.Server.DRPC(), peer.Storage2.Endpoint.DRPC())

		// TODO workaround for custom timeout for order sending request (read/write)
		sc := config.Server

		tlsOptions, err := tlsopts.NewOptions(peer.Identity, sc.Config, revocationDB)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		dialer := rpc.NewDefaultDialer(tlsOptions)
		dialer.DialTimeout = config.Storage2.Orders.SenderDialTimeout

		peer.Storage2.Orders = orders.NewService(
			log.Named("orders"),
			dialer,
			peer.DB.Orders(),
			peer.Storage2.Trust,
			config.Storage2.Orders,
		)
	}

	{ // setup node stats service
		peer.NodeStats.Service = nodestats.NewService(
			peer.Log.Named("nodestats:service"),
			peer.Dialer,
			peer.Storage2.Trust)

		peer.NodeStats.Cache = nodestats.NewCache(
			peer.Log.Named("nodestats:cache"),
			config.Nodestats,
			nodestats.CacheStorage{
				Reputation:   peer.DB.Reputation(),
				StorageUsage: peer.DB.StorageUsage(),
			},
			peer.NodeStats.Service,
			peer.Storage2.Trust)
	}

	{ // setup storage node operator dashboard
		peer.Console.Service, err = console.NewService(
			peer.Log.Named("console:service"),
			peer.DB.Bandwidth(),
			peer.Storage2.Store,
			peer.Version,
			config.Storage.AllocatedBandwidth,
			config.Storage.AllocatedDiskSpace,
			config.Operator.Wallet,
			versionInfo,
			peer.Storage2.Trust,
			peer.DB.Reputation(),
			peer.DB.StorageUsage(),
			peer.Contact.PingStats,
			peer.Contact.Service)

		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Console.Listener, err = net.Listen("tcp", config.Console.Address)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		assets := consoleassets.FileSystem
		if config.Console.StaticDir != "" {
			// a specific directory has been configured. use it
			assets = http.Dir(config.Console.StaticDir)
		}

		peer.Console.Endpoint = consoleserver.NewServer(
			peer.Log.Named("console:endpoint"),
			assets,
			peer.Notifications.Service,
			peer.Console.Service,
			peer.Console.Listener,
		)
	}

	{ // setup storage inspector
		peer.Storage2.Inspector = inspector.NewEndpoint(
			peer.Log.Named("pieces:inspector"),
			peer.Storage2.Store,
			peer.Contact.Service,
			peer.Contact.PingStats,
			peer.DB.Bandwidth(),
			config.Storage,
			peer.Console.Listener.Addr(),
			config.Contact.ExternalAddress,
		)
		pb.RegisterPieceStoreInspectorServer(peer.Server.PrivateGRPC(), peer.Storage2.Inspector)
		pb.DRPCRegisterPieceStoreInspector(peer.Server.PrivateDRPC(), peer.Storage2.Inspector)
	}

	{ // setup graceful exit service
		peer.GracefulExit.Endpoint = gracefulexit.NewEndpoint(
			peer.Log.Named("gracefulexit:endpoint"),
			peer.Storage2.Trust,
			peer.DB.Satellites(),
			peer.Storage2.BlobsCache,
		)
		pb.RegisterNodeGracefulExitServer(peer.Server.PrivateGRPC(), peer.GracefulExit.Endpoint)
		pb.DRPCRegisterNodeGracefulExit(peer.Server.PrivateDRPC(), peer.GracefulExit.Endpoint)

		peer.GracefulExit.Chore = gracefulexit.NewChore(
			peer.Log.Named("gracefulexit:chore"),
			config.GracefulExit,
			peer.Storage2.Store,
			peer.Storage2.Trust,
			peer.Dialer,
			peer.DB.Satellites(),
		)
	}

	peer.Collector = collector.NewService(peer.Log.Named("collector"), peer.Storage2.Store, peer.DB.UsedSerials(), config.Collector)

	peer.Bandwidth = bandwidth.NewService(peer.Log.Named("bandwidth"), peer.DB.Bandwidth(), config.Bandwidth)

	return peer, nil
}

// Run runs storage node until it's either closed or it errors.
func (peer *Peer) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	// Refresh the trust pool first. It will be updated periodically via
	// Run() below.
	if err := peer.Storage2.Trust.Refresh(ctx); err != nil {
		return err
	}

	group, ctx := errgroup.WithContext(ctx)

	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Version.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Storage2.Monitor.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Contact.Chore.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Collector.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Storage2.Orders.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Storage2.CacheService.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Storage2.RetainService.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Storage2.TrashChore.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Bandwidth.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Storage2.Trust.Run(ctx))
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
		return errs2.IgnoreCanceled(peer.NodeStats.Cache.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Console.Endpoint.Run(ctx))
	})

	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.GracefulExit.Chore.Run(ctx))
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

	// close services in reverse initialization order
	if peer.GracefulExit.Chore != nil {
		errlist.Add(peer.GracefulExit.Chore.Close())
	}
	if peer.Contact.Chore != nil {
		errlist.Add(peer.Contact.Chore.Close())
	}
	if peer.Bandwidth != nil {
		errlist.Add(peer.Bandwidth.Close())
	}
	if peer.Storage2.TrashChore != nil {
		errlist.Add(peer.Storage2.TrashChore.Close())
	}
	if peer.Storage2.RetainService != nil {
		errlist.Add(peer.Storage2.RetainService.Close())
	}
	if peer.Storage2.Monitor != nil {
		errlist.Add(peer.Storage2.Monitor.Close())
	}
	if peer.Storage2.Orders != nil {
		errlist.Add(peer.Storage2.Orders.Close())
	}
	if peer.Storage2.CacheService != nil {
		errlist.Add(peer.Storage2.CacheService.Close())
	}
	if peer.Collector != nil {
		errlist.Add(peer.Collector.Close())
	}

	if peer.Console.Endpoint != nil {
		errlist.Add(peer.Console.Endpoint.Close())
	} else if peer.Console.Listener != nil {
		errlist.Add(peer.Console.Listener.Close())
	}

	if peer.NodeStats.Cache != nil {
		errlist.Add(peer.NodeStats.Cache.Close())
	}

	return errlist.Err()
}

// ID returns the peer ID.
func (peer *Peer) ID() storj.NodeID { return peer.Identity.ID }

// Local returns the peer local node info.
func (peer *Peer) Local() overlay.NodeDossier { return peer.Contact.Service.Local() }

// Addr returns the public address.
func (peer *Peer) Addr() string { return peer.Server.Addr().String() }

// URL returns the storj.NodeURL.
func (peer *Peer) URL() storj.NodeURL { return storj.NodeURL{ID: peer.ID(), Address: peer.Addr()} }

// PrivateAddr returns the private address.
func (peer *Peer) PrivateAddr() string { return peer.Server.PrivateAddr().String() }
