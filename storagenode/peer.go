// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenode

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

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
	"storj.io/storj/pkg/server"
	"storj.io/storj/private/lifecycle"
	"storj.io/storj/private/version/checker"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/storage"
	"storj.io/storj/storage/filestore"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/collector"
	"storj.io/storj/storagenode/console"
	"storj.io/storj/storagenode/console/consoleassets"
	"storj.io/storj/storagenode/console/consoleserver"
	"storj.io/storj/storagenode/contact"
	"storj.io/storj/storagenode/gracefulexit"
	"storj.io/storj/storagenode/heldamount"
	"storj.io/storj/storagenode/inspector"
	"storj.io/storj/storagenode/monitor"
	"storj.io/storj/storagenode/nodestats"
	"storj.io/storj/storagenode/notifications"
	"storj.io/storj/storagenode/orders"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/piecestore"
	"storj.io/storj/storagenode/preflight"
	"storj.io/storj/storagenode/pricing"
	"storj.io/storj/storagenode/reputation"
	"storj.io/storj/storagenode/retain"
	"storj.io/storj/storagenode/satellites"
	"storj.io/storj/storagenode/storagenodedb"
	"storj.io/storj/storagenode/storageusage"
	"storj.io/storj/storagenode/trust"
	version2 "storj.io/storj/storagenode/version"
)

var (
	mon = monkit.Package()
)

// DB is the master database for Storage Node
//
// architecture: Master Database
type DB interface {
	// MigrateToLatest initializes the database
	MigrateToLatest(ctx context.Context) error
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
	HeldAmount() heldamount.DB
	Pricing() pricing.DB

	Preflight(ctx context.Context) error
}

// Config is all the configuration parameters for a Storage Node.
type Config struct {
	Identity identity.Config

	Server server.Config
	Debug  debug.Config

	Preflight preflight.Config
	Contact   contact.Config
	Operator  OperatorConfig

	// TODO: flatten storage config and only keep the new one
	Storage   piecestore.OldConfig
	Storage2  piecestore.Config
	Collector collector.Config

	Filestore filestore.Config

	Pieces pieces.Config

	Retain retain.Config

	Nodestats nodestats.Config

	Console consoleserver.Config

	Version checker.Config

	Bandwidth bandwidth.Config

	GracefulExit gracefulexit.Config
}

// DatabaseConfig returns the storagenodedb.Config that should be used with this Config.
func (config *Config) DatabaseConfig() storagenodedb.Config {
	dbdir := config.Storage2.DatabaseDir
	if dbdir == "" {
		dbdir = config.Storage.Path
	}
	return storagenodedb.Config{
		Storage:   config.Storage.Path,
		Info:      filepath.Join(dbdir, "piecestore.db"),
		Info2:     filepath.Join(dbdir, "info.db"),
		Pieces:    config.Storage.Path,
		Filestore: config.Filestore,
	}
}

// Verify verifies whether configuration is consistent and acceptable.
func (config *Config) Verify(log *zap.Logger) error {
	err := config.Operator.Verify(log)
	if err != nil {
		return err
	}

	if config.Contact.ExternalAddress != "" {
		err := isAddressValid(config.Contact.ExternalAddress)
		if err != nil {
			return errs.New("invalid contact.external-address: %v", err)
		}
	}

	if config.Server.Address != "" {
		err := isAddressValid(config.Server.Address)
		if err != nil {
			return errs.New("invalid server.address: %v", err)
		}
	}

	return nil
}

func isAddressValid(addrstring string) error {
	addr, port, err := net.SplitHostPort(addrstring)
	if err != nil || port == "" {
		return errs.New("split host-port %q failed: %+v", addrstring, err)
	}
	if addr == "" {
		return nil
	}
	resolvedhosts, err := net.LookupHost(addr)
	if err != nil || len(resolvedhosts) == 0 {
		return errs.New("lookup %q failed: %+v", addr, err)
	}

	return nil
}

// Peer is the representation of a Storage Node.
//
// architecture: Peer
type Peer struct {
	// core dependencies
	Log      *zap.Logger
	Identity *identity.FullIdentity
	DB       DB

	Servers  *lifecycle.Group
	Services *lifecycle.Group

	Dialer rpc.Dialer

	Server *server.Server

	Version struct {
		Chore   *version2.Chore
		Service *checker.Service
	}

	Debug struct {
		Listener net.Listener
		Server   *debug.Server
	}

	// services and endpoints
	// TODO: similar grouping to satellite.Core

	Preflight struct {
		LocalTime *preflight.LocalTime
	}

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
		PieceDeleter  *pieces.Deleter
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

	Heldamount struct {
		Service *heldamount.Service
	}

	Bandwidth *bandwidth.Service
}

// New creates a new Storage Node.
func New(log *zap.Logger, full *identity.FullIdentity, db DB, revocationDB extensions.RevocationDB, config Config, versionInfo version.Info) (*Peer, error) {
	peer := &Peer{
		Log:      log,
		Identity: full,
		DB:       db,

		Servers:  lifecycle.NewGroup(log.Named("servers")),
		Services: lifecycle.NewGroup(log.Named("services")),
	}

	{ // setup notification service.
		peer.Notifications.Service = notifications.NewService(peer.Log, peer.DB.Notifications())
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
		debugConfig.ControlTitle = "Storage Node"
		peer.Debug.Server = debug.NewServer(log.Named("debug"), peer.Debug.Listener, monkit.Default, debugConfig)
		peer.Servers.Add(lifecycle.Item{
			Name:  "debug",
			Run:   peer.Debug.Server.Run,
			Close: peer.Debug.Server.Close,
		})
	}

	var err error

	{
		if !versionInfo.IsZero() {
			peer.Log.Debug("Version info",
				zap.Stringer("Version", versionInfo.Version.Version),
				zap.String("Commit Hash", versionInfo.CommitHash),
				zap.Stringer("Build Timestamp", versionInfo.Timestamp),
				zap.Bool("Release Build", versionInfo.Release),
			)
		}

		peer.Version.Service = checker.NewService(log.Named("version"), config.Version, versionInfo, "Storagenode")
		versionCheckInterval := 24 * time.Hour
		peer.Version.Chore = version2.NewChore(peer.Version.Service, peer.Notifications.Service, peer.Identity.ID, versionCheckInterval)
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

		peer.Server, err = server.New(log.Named("server"), tlsOptions, sc.Address, sc.PrivateAddress, nil)
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

	{ // setup trust pool
		peer.Storage2.Trust, err = trust.NewPool(log.Named("trust"), trust.Dialer(peer.Dialer), config.Storage2.Trust)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
		peer.Services.Add(lifecycle.Item{
			Name: "trust",
			Run:  peer.Storage2.Trust.Run,
		})
	}

	{
		peer.Preflight.LocalTime = preflight.NewLocalTime(peer.Log.Named("preflight:localtime"), config.Preflight, peer.Storage2.Trust, peer.Dialer)
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
		peer.Contact.Service = contact.NewService(peer.Log.Named("contact:service"), peer.Dialer, self, peer.Storage2.Trust)

		peer.Contact.Chore = contact.NewChore(peer.Log.Named("contact:chore"), config.Contact.Interval, peer.Contact.Service)
		peer.Services.Add(lifecycle.Item{
			Name:  "contact:chore",
			Run:   peer.Contact.Chore.Run,
			Close: peer.Contact.Chore.Close,
		})

		peer.Contact.Endpoint = contact.NewEndpoint(peer.Log.Named("contact:endpoint"), peer.Contact.PingStats)
		pbgrpc.RegisterContactServer(peer.Server.GRPC(), peer.Contact.Endpoint)
		if err := pb.DRPCRegisterContact(peer.Server.DRPC(), peer.Contact.Endpoint); err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

	}

	{ // setup storage
		peer.Storage2.BlobsCache = pieces.NewBlobsUsageCache(peer.Log.Named("blobscache"), peer.DB.Pieces())

		peer.Storage2.Store = pieces.NewStore(peer.Log.Named("pieces"),
			peer.Storage2.BlobsCache,
			peer.DB.V0PieceInfo(),
			peer.DB.PieceExpirationDB(),
			peer.DB.PieceSpaceUsedDB(),
			config.Pieces,
		)

		peer.Storage2.PieceDeleter = pieces.NewDeleter(log.Named("piecedeleter"), peer.Storage2.Store, config.Storage2.DeleteWorkers, config.Storage2.DeleteQueueSize)
		peer.Services.Add(lifecycle.Item{
			Name:  "PieceDeleter",
			Run:   peer.Storage2.PieceDeleter.Run,
			Close: peer.Storage2.PieceDeleter.Close,
		})

		peer.Storage2.TrashChore = pieces.NewTrashChore(
			log.Named("pieces:trash"),
			24*time.Hour,   // choreInterval: how often to run the chore
			7*24*time.Hour, // trashExpiryInterval: when items in the trash should be deleted
			peer.Storage2.Trust,
			peer.Storage2.Store,
		)
		peer.Services.Add(lifecycle.Item{
			Name:  "pieces:trash",
			Run:   peer.Storage2.TrashChore.Run,
			Close: peer.Storage2.TrashChore.Close,
		})

		peer.Storage2.CacheService = pieces.NewService(
			log.Named("piecestore:cache"),
			peer.Storage2.BlobsCache,
			peer.Storage2.Store,
			config.Storage2.CacheSyncInterval,
		)
		peer.Services.Add(lifecycle.Item{
			Name:  "piecestore:cache",
			Run:   peer.Storage2.CacheService.Run,
			Close: peer.Storage2.CacheService.Close,
		})
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Piecestore Cache", peer.Storage2.CacheService.Loop))

		peer.Storage2.Monitor = monitor.NewService(
			log.Named("piecestore:monitor"),
			peer.Storage2.Store,
			peer.Contact.Service,
			peer.DB.Bandwidth(),
			config.Storage.AllocatedDiskSpace.Int64(),
			//TODO use config.Storage.Monitor.Interval, but for some reason is not set
			config.Storage.KBucketRefreshInterval,
			peer.Contact.Chore.Trigger,
			config.Storage2.Monitor,
		)
		peer.Services.Add(lifecycle.Item{
			Name:  "piecestore:monitor",
			Run:   peer.Storage2.Monitor.Run,
			Close: peer.Storage2.Monitor.Close,
		})
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Piecestore Monitor", peer.Storage2.Monitor.Loop))

		peer.Storage2.RetainService = retain.NewService(
			peer.Log.Named("retain"),
			peer.Storage2.Store,
			config.Retain,
		)
		peer.Services.Add(lifecycle.Item{
			Name:  "retain",
			Run:   peer.Storage2.RetainService.Run,
			Close: peer.Storage2.RetainService.Close,
		})

		peer.Storage2.Endpoint, err = piecestore.NewEndpoint(
			peer.Log.Named("piecestore"),
			signing.SignerFromFullIdentity(peer.Identity),
			peer.Storage2.Trust,
			peer.Storage2.Monitor,
			peer.Storage2.RetainService,
			peer.Contact.PingStats,
			peer.Storage2.Store,
			peer.Storage2.PieceDeleter,
			peer.DB.Orders(),
			peer.DB.Bandwidth(),
			peer.DB.UsedSerials(),
			config.Storage2,
		)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		pbgrpc.RegisterPiecestoreServer(peer.Server.GRPC(), peer.Storage2.Endpoint)
		if err := pb.DRPCRegisterPiecestore(peer.Server.DRPC(), peer.Storage2.Endpoint.DRPC()); err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

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
		peer.Services.Add(lifecycle.Item{
			Name:  "orders",
			Run:   peer.Storage2.Orders.Run,
			Close: peer.Storage2.Orders.Close,
		})
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Orders Sender", peer.Storage2.Orders.Sender))
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Orders Cleanup", peer.Storage2.Orders.Cleanup))
	}

	{ // setup heldamount service.
		peer.Heldamount.Service = heldamount.NewService(
			peer.Log.Named("heldamount:service"),
			peer.DB.HeldAmount(),
			peer.Dialer,
			peer.Storage2.Trust,
		)
	}

	{ // setup node stats service
		peer.NodeStats.Service = nodestats.NewService(
			peer.Log.Named("nodestats:service"),
			peer.Dialer,
			peer.Storage2.Trust,
		)

		peer.NodeStats.Cache = nodestats.NewCache(
			peer.Log.Named("nodestats:cache"),
			config.Nodestats,
			nodestats.CacheStorage{
				Reputation:   peer.DB.Reputation(),
				StorageUsage: peer.DB.StorageUsage(),
				HeldAmount:   peer.DB.HeldAmount(),
				Pricing:      peer.DB.Pricing(),
				Satellites:   peer.DB.Satellites(),
			},
			peer.NodeStats.Service,
			peer.Heldamount.Service,
			peer.Storage2.Trust,
		)
		peer.Services.Add(lifecycle.Item{
			Name:  "nodestats:cache",
			Run:   peer.NodeStats.Cache.Run,
			Close: peer.NodeStats.Cache.Close,
		})
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Node Stats Cache Reputation", peer.NodeStats.Cache.Reputation))
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Node Stats Cache Storage", peer.NodeStats.Cache.Storage))
	}

	{ // setup storage node operator dashboard
		peer.Console.Service, err = console.NewService(
			peer.Log.Named("console:service"),
			peer.DB.Bandwidth(),
			peer.Storage2.Store,
			peer.Version.Service,
			config.Storage.AllocatedDiskSpace,
			config.Operator.Wallet,
			versionInfo,
			peer.Storage2.Trust,
			peer.DB.Reputation(),
			peer.DB.StorageUsage(),
			peer.DB.Pricing(),
			peer.DB.Satellites(),
			peer.Contact.PingStats,
			peer.Contact.Service,
		)
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
			peer.Heldamount.Service,
			peer.Console.Listener,
		)
		peer.Services.Add(lifecycle.Item{
			Name:  "console:endpoint",
			Run:   peer.Console.Endpoint.Run,
			Close: peer.Console.Endpoint.Close,
		})
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
		pbgrpc.RegisterPieceStoreInspectorServer(peer.Server.PrivateGRPC(), peer.Storage2.Inspector)
		if err := pb.DRPCRegisterPieceStoreInspector(peer.Server.PrivateDRPC(), peer.Storage2.Inspector); err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
	}

	{ // setup graceful exit service
		peer.GracefulExit.Endpoint = gracefulexit.NewEndpoint(
			peer.Log.Named("gracefulexit:endpoint"),
			peer.Storage2.Trust,
			peer.DB.Satellites(),
			peer.Storage2.BlobsCache,
		)
		pbgrpc.RegisterNodeGracefulExitServer(peer.Server.PrivateGRPC(), peer.GracefulExit.Endpoint)
		if err := pb.DRPCRegisterNodeGracefulExit(peer.Server.PrivateDRPC(), peer.GracefulExit.Endpoint); err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.GracefulExit.Chore = gracefulexit.NewChore(
			peer.Log.Named("gracefulexit:chore"),
			config.GracefulExit,
			peer.Storage2.Store,
			peer.Storage2.Trust,
			peer.Dialer,
			peer.DB.Satellites(),
		)
		peer.Services.Add(lifecycle.Item{
			Name:  "gracefulexit:chore",
			Run:   peer.GracefulExit.Chore.Run,
			Close: peer.GracefulExit.Chore.Close,
		})
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Graceful Exit", peer.GracefulExit.Chore.Loop))
	}

	peer.Collector = collector.NewService(peer.Log.Named("collector"), peer.Storage2.Store, peer.DB.UsedSerials(), config.Collector)
	peer.Services.Add(lifecycle.Item{
		Name:  "collector",
		Run:   peer.Collector.Run,
		Close: peer.Collector.Close,
	})
	peer.Debug.Server.Panel.Add(
		debug.Cycle("Collector", peer.Collector.Loop))

	peer.Bandwidth = bandwidth.NewService(peer.Log.Named("bandwidth"), peer.DB.Bandwidth(), config.Bandwidth)
	peer.Services.Add(lifecycle.Item{
		Name:  "bandwidth",
		Run:   peer.Bandwidth.Run,
		Close: peer.Bandwidth.Close,
	})
	peer.Debug.Server.Panel.Add(
		debug.Cycle("Bandwidth", peer.Bandwidth.Loop))

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

	if err := peer.Preflight.LocalTime.Check(ctx); err != nil {
		peer.Log.Fatal("Failed preflight check.", zap.Error(err))
		return err
	}

	group, ctx := errgroup.WithContext(ctx)

	peer.Servers.Run(ctx, group)
	peer.Services.Run(ctx, group)

	return group.Wait()
}

// Close closes all the resources.
func (peer *Peer) Close() error {
	return errs.Combine(
		peer.Servers.Close(),
		peer.Services.Close(),
	)
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
