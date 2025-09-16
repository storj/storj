// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenode

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
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
	"storj.io/common/process"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/common/version"
	"storj.io/storj/private/emptyfs"
	"storj.io/storj/private/lifecycle"
	"storj.io/storj/private/multinodepb"
	"storj.io/storj/private/server"
	"storj.io/storj/private/version/checker"
	"storj.io/storj/storagenode/apikeys"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/blobstore"
	"storj.io/storj/storagenode/blobstore/filestore"
	"storj.io/storj/storagenode/collector"
	"storj.io/storj/storagenode/console"
	"storj.io/storj/storagenode/console/consoleserver"
	"storj.io/storj/storagenode/contact"
	"storj.io/storj/storagenode/forgetsatellite"
	"storj.io/storj/storagenode/gracefulexit"
	"storj.io/storj/storagenode/hashstore"
	"storj.io/storj/storagenode/healthcheck"
	"storj.io/storj/storagenode/inspector"
	"storj.io/storj/storagenode/internalpb"
	"storj.io/storj/storagenode/monitor"
	"storj.io/storj/storagenode/multinode"
	"storj.io/storj/storagenode/nodestats"
	"storj.io/storj/storagenode/notifications"
	"storj.io/storj/storagenode/operator"
	"storj.io/storj/storagenode/orders"
	"storj.io/storj/storagenode/payouts"
	"storj.io/storj/storagenode/payouts/estimatedpayouts"
	"storj.io/storj/storagenode/piecemigrate"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/pieces/lazyfilewalker"
	"storj.io/storj/storagenode/piecestore"
	"storj.io/storj/storagenode/piecestore/signaturecheck"
	"storj.io/storj/storagenode/piecestore/usedserials"
	"storj.io/storj/storagenode/preflight"
	"storj.io/storj/storagenode/pricing"
	"storj.io/storj/storagenode/reputation"
	"storj.io/storj/storagenode/retain"
	"storj.io/storj/storagenode/satellites"
	"storj.io/storj/storagenode/satstore"
	"storj.io/storj/storagenode/storagenodedb"
	"storj.io/storj/storagenode/storageusage"
	"storj.io/storj/storagenode/trust"
	snVersion "storj.io/storj/storagenode/version"
)

var mon = monkit.Package()

// Assets contains either the built admin/back-office/ui or it is nil.
var Assets fs.FS = emptyfs.FS{}

const trashExpiryInterval = 7 * 24 * time.Hour

// DB is the master database for Storage Node.
//
// architecture: Master Database
type DB interface {
	// Config returns the configuration used to initialize the database.
	Config() storagenodedb.Config

	// MigrateToLatest initializes the database
	MigrateToLatest(ctx context.Context) error

	// Close closes the database
	Close() error

	Pieces() blobstore.Blobs

	Orders() orders.DB
	V0PieceInfo() pieces.V0PieceInfoDB
	PieceExpirationDB() pieces.PieceExpirationDB
	PieceSpaceUsedDB() pieces.PieceSpaceUsedDB
	Bandwidth() bandwidth.DB
	Reputation() reputation.DB
	StorageUsage() storageusage.DB
	Satellites() satellites.DB
	Notifications() notifications.DB
	Payout() payouts.DB
	Pricing() pricing.DB
	APIKeys() apikeys.DB
	GCFilewalkerProgress() pieces.GCFilewalkerProgressDB
	UsedSpacePerPrefix() pieces.UsedSpacePerPrefixDB

	Preflight(ctx context.Context) error
}

// Config is all the configuration parameters for a Storage Node.
type Config struct {
	Identity identity.Config

	Server server.Config
	Debug  debug.Config

	Preflight preflight.Config
	Contact   contact.Config
	Operator  operator.Config

	Hashstore hashstore.Config

	// TODO: flatten storage config and only keep the new one
	Storage           piecestore.OldConfig
	Storage2          piecestore.Config
	Storage2Migration piecemigrate.Config
	Collector         collector.Config

	Filestore filestore.Config

	Pieces pieces.Config

	Retain retain.Config

	Nodestats nodestats.Config

	Reputation reputation.Config

	Console consoleserver.Config

	Healthcheck healthcheck.Config

	Version snVersion.Config

	Bandwidth bandwidth.Config

	GracefulExit gracefulexit.Config

	ForgetSatellite forgetsatellite.Config
}

// DatabaseConfig returns the storagenodedb.Config that should be used with this Config.
func (config *Config) DatabaseConfig() storagenodedb.Config {
	dbdir := config.Storage2.DatabaseDir
	if dbdir == "" {
		dbdir = config.Storage.Path
	}
	return storagenodedb.Config{
		Storage:   config.Storage.Path,
		Cache:     config.Pieces.FileStatCache,
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
	resolvedhosts, err := (&net.Resolver{}).LookupHost(context.Background(), addr)
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
	Log         *zap.Logger
	Identity    *identity.FullIdentity
	DB          DB
	UsedSerials *usedserials.Table
	OrdersStore *orders.FileStore

	Servers  *lifecycle.Group
	Services *lifecycle.Group

	Dialer rpc.Dialer

	Server *server.Server

	Version struct {
		Chore   *snVersion.Chore
		Service *checker.Service
	}

	Healthcheck struct {
		Service  *healthcheck.Service
		Endpoint *healthcheck.Endpoint
	}

	Debug struct {
		Listener net.Listener
		Server   *debug.Server
		Endpoint *debug.Endpoint
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
		QUICStats *contact.QUICStats
	}

	Estimation struct {
		Service *estimatedpayouts.Service
	}

	Storage2 struct {
		// TODO: lift things outside of it to organize better
		Trust              *trust.Pool
		SpaceReport        monitor.SpaceReport
		OldPieceBackend    *piecestore.OldPieceBackend
		HashStoreBackend   *piecestore.HashStoreBackend
		MigrationState     *satstore.SatelliteStore
		MigrationChore     *piecemigrate.Chore
		MigratingBackend   *piecestore.MigratingBackend
		PieceBackend       *piecestore.TestingBackend
		Endpoint           *piecestore.Endpoint
		Inspector          *inspector.Endpoint
		Monitor            *monitor.Service
		Orders             *orders.Service
		RestoreTimeManager *retain.RestoreTimeManager
		BloomFilterManager *retain.BloomFilterManager
	}

	StorageOld struct {
		Store          *pieces.Store
		TrashChore     *pieces.TrashChore
		BlobsCache     *pieces.BlobsUsageCache
		CacheService   *pieces.CacheService
		RetainService  *retain.Service
		FileWalker     *pieces.FileWalker
		LazyFileWalker *lazyfilewalker.Supervisor
		Collector      *collector.Service
	}

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
		Service      *gracefulexit.Service
		Endpoint     *gracefulexit.Endpoint
		Chore        *gracefulexit.Chore
		BlobsCleaner *gracefulexit.BlobsCleaner
	}

	ForgetSatellite struct {
		Endpoint *forgetsatellite.Endpoint
		Chore    *forgetsatellite.Chore
		Cleaner  *forgetsatellite.Cleaner
	}

	Notifications struct {
		Service *notifications.Service
	}

	Payout struct {
		Service  *payouts.Service
		Endpoint *payouts.Endpoint
	}

	Bandwidth struct {
		Service *bandwidth.Service
		Cache   *bandwidth.Cache
	}

	Reputation struct {
		Service *reputation.Service
		Chore   *reputation.Chore
	}

	Multinode struct {
		Storage   *multinode.StorageEndpoint
		Bandwidth *multinode.BandwidthEndpoint
		Node      *multinode.NodeEndpoint
		Payout    *multinode.PayoutEndpoint
	}
}

// New creates a new Storage Node.
func New(log *zap.Logger, full *identity.FullIdentity, db DB, revocationDB extensions.RevocationDB, config Config, versionInfo version.Info, atomicLogLevel *zap.AtomicLevel) (*Peer, error) {
	peer := &Peer{
		Log:      log,
		Identity: full,
		DB:       db,

		Servers:  lifecycle.NewGroup(process.NamedLog(log, "servers")),
		Services: lifecycle.NewGroup(process.NamedLog(log, "services")),
	}

	initializeDiskMon(log)

	{ // setup notification service.
		peer.Notifications.Service = notifications.NewService(peer.Log, peer.DB.Notifications())
	}

	var err error

	{ // version setup
		if !versionInfo.IsZero() {
			peer.Log.Debug("Version info",
				zap.Stringer("Version", versionInfo.Version.Version),
				zap.String("Commit Hash", versionInfo.CommitHash),
				zap.Stringer("Build Timestamp", versionInfo.Timestamp),
				zap.Bool("Release Build", versionInfo.Release),
			)
		}

		if !config.Version.RunMode.Disabled() {
			peer.Version.Service = checker.NewService(process.NamedLog(log, "version"), config.Version.Config, versionInfo, "Storagenode")
			versionCheckInterval := 12 * time.Hour
			peer.Version.Chore = snVersion.NewChore(process.NamedLog(log, "version:chore"), peer.Version.Service, peer.Notifications.Service, peer.Identity.ID, versionCheckInterval)
			versionChore := lifecycle.Item{
				Name: "version:chore",
				Run:  peer.Version.Chore.Run,
			}

			if config.Version.RunMode.Once() {
				versionChore.Run = peer.Version.Chore.RunOnce
			}
			peer.Services.Add(versionChore)
		}
	}

	{
		peer.Healthcheck.Service = healthcheck.NewService(peer.DB.Reputation(), config.Healthcheck.Details)
		peer.Healthcheck.Endpoint = healthcheck.NewEndpoint(peer.Healthcheck.Service)
	}

	{ // setup listener and server
		sc := config.Server

		sc.Config.UsePeerCAWhitelist = false
		tlsOptions, err := tlsopts.NewOptions(peer.Identity, sc.Config, revocationDB)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Dialer = rpc.NewDefaultDialer(tlsOptions)

		peer.Server, err = server.New(process.NamedLog(log, "server"), tlsOptions, sc)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		if config.Healthcheck.Enabled {
			peer.Server.AddHTTPFallback(peer.Healthcheck.Endpoint.HandleHTTP)
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

	{ // setup trust pool
		peer.Storage2.Trust, err = trust.NewPool(process.NamedLog(log, "trust"), trust.Dialer(peer.Dialer), config.Storage2.Trust, peer.DB.Satellites())
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
		peer.Services.Add(lifecycle.Item{
			Name: "trust",
			Run:  peer.Storage2.Trust.Run,
		})
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
		debugConfig.ControlTitle = "Storage Node"
		peer.Debug.Server = debug.NewServerWithAtomicLevel(process.NamedLog(log, "debug"), peer.Debug.Listener, monkit.Default, debugConfig, atomicLogLevel)
		peer.Servers.Add(lifecycle.Item{
			Name:  "debug",
			Run:   peer.Debug.Server.Run,
			Close: peer.Debug.Server.Close,
		})

		peer.Debug.Endpoint = debug.NewEndpoint(func(ctx context.Context) error {
			sat, err := identity.PeerIdentityFromContext(ctx)
			if err != nil {
				return err
			}
			if !peer.Storage2.Trust.IsTrusted(ctx, sat.ID) {
				return errs.New("untrusted peer")
			}
			return nil
		})
		if err := pb.DRPCRegisterDebug(peer.Server.DRPC(), peer.Debug.Endpoint); err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
	}

	{
		peer.Preflight.LocalTime = preflight.NewLocalTime(process.NamedLog(peer.Log, "preflight:localtime"), config.Preflight, peer.Storage2.Trust, peer.Dialer)
	}

	logsPath, tablePath := config.Hashstore.Directories(config.Storage.Path)
	metaDir := filepath.Join(logsPath, "meta")

	peer.Storage2.MigrationState = satstore.NewSatelliteStore(metaDir, "migrate")

	{ // setup contact service
		c := config.Contact
		if c.ExternalAddress == "" {
			c.ExternalAddress = peer.Addr()
		}

		pbVersion, err := versionInfo.Proto()
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
		noiseKeyAttestation, err := peer.Server.NoiseKeyAttestation(context.Background())
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
		self := contact.NodeInfo{
			ID:      peer.ID(),
			Address: c.ExternalAddress,
			Operator: pb.NodeOperator{
				Email:          config.Operator.Email,
				Wallet:         config.Operator.Wallet,
				WalletFeatures: config.Operator.WalletFeatures,
			},
			Version:             *pbVersion,
			NoiseKeyAttestation: noiseKeyAttestation,
			DebounceLimit:       peer.Server.DebounceLimit(),
			FastOpen:            peer.Server.FastOpen(),
			HashstoreMemtbl:     config.Hashstore.TableDefaultKind.Kind == hashstore.TableKind_MemTbl,
		}

		self.HashstoreWriteToNew = ReportHashstoreWriteToNew(peer.Log, peer.Storage2.MigrationState)

		peer.Contact.PingStats = new(contact.PingStats)
		peer.Contact.QUICStats = contact.NewQUICStats(peer.Server.IsQUICEnabled())

		tags, err := contact.GetTags(context.Background(), config.Contact, peer.Identity)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Contact.Service = contact.NewService(process.NamedLog(peer.Log, "contact:service"), peer.Dialer, self, peer.Storage2.Trust, peer.Contact.QUICStats, tags)

		peer.Contact.Chore = contact.NewChore(process.NamedLog(peer.Log, "contact:chore"), config.Contact.Interval, config.Contact.CheckInTimeout, peer.Contact.Service)
		peer.Services.Add(lifecycle.Item{
			Name:  "contact:chore",
			Run:   peer.Contact.Chore.Run,
			Close: peer.Contact.Chore.Close,
		})

		peer.Contact.Endpoint = contact.NewEndpoint(process.NamedLog(log, "contact:endpoint"), peer.Storage2.Trust, peer.Contact.PingStats)
		if err := pb.DRPCRegisterContact(peer.Server.DRPC(), peer.Contact.Endpoint); err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
	}

	{ // setup bandwidth service
		peer.Bandwidth.Cache = bandwidth.NewCache(peer.DB.Bandwidth())
		peer.Bandwidth.Service = bandwidth.NewService(process.NamedLog(peer.Log, "bandwidth"), peer.Bandwidth.Cache, config.Bandwidth)
		peer.Services.Add(lifecycle.Item{
			Name:  "bandwidth",
			Run:   peer.Bandwidth.Service.Run,
			Close: peer.Bandwidth.Service.Close,
		})
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Bandwidth", peer.Bandwidth.Service.Loop))
	}

	{ // setup storage
		peer.StorageOld.BlobsCache = pieces.NewBlobsUsageCache(process.NamedLog(log, "blobscache"), peer.DB.Pieces())

		oldBlobStore := peer.DB.Pieces()
		if !config.Storage2.Monitor.DedicatedDisk {
			oldBlobStore = peer.StorageOld.BlobsCache
		}

		peer.StorageOld.FileWalker = pieces.NewFileWalker(process.NamedLog(log, "filewalker"), oldBlobStore, peer.DB.V0PieceInfo(), peer.DB.GCFilewalkerProgress(), peer.DB.UsedSpacePerPrefix())

		if config.Pieces.EnableLazyFilewalker {
			executable, err := os.Executable()
			if err != nil {
				return nil, errs.Combine(err, peer.Close())
			}

			peer.StorageOld.LazyFileWalker = lazyfilewalker.NewSupervisor(process.NamedLog(peer.Log, "lazyfilewalker"), db.Config().LazyFilewalkerConfig(), executable)
		}

		oldPieceExpiration, err := getPieceExpirationStore(process.NamedLog(log, "pieceexpiration"), db.PieceExpirationDB(), config.Storage, config.Storage2, config.Pieces)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.StorageOld.Store = pieces.NewStore(process.NamedLog(peer.Log, "pieces"),
			peer.StorageOld.FileWalker,
			peer.StorageOld.LazyFileWalker,
			oldBlobStore,
			peer.DB.V0PieceInfo(),
			oldPieceExpiration,
			config.Pieces,
		)

		peer.StorageOld.TrashChore = pieces.NewTrashChore(
			process.NamedLog(log, "pieces:trash"),
			config.Pieces.TrashChoreInterval, // choreInterval: how often to run the chore
			trashExpiryInterval,              // trashExpiryInterval: when items in the trash should be deleted
			peer.Storage2.Trust,
			peer.StorageOld.Store,
		)
		peer.Services.Add(lifecycle.Item{
			Name:  "pieces:trash",
			Run:   peer.StorageOld.TrashChore.Run,
			Close: peer.StorageOld.TrashChore.Close,
		})

		peer.Storage2.RestoreTimeManager = retain.NewRestoreTimeManager(metaDir)
		peer.Storage2.BloomFilterManager, err = retain.NewBloomFilterManager(
			metaDir,
			config.Retain.MaxTimeSkew,
		)
		if err != nil {
			peer.Log.Info("error encountered loading bloom filters", zap.Error(err))
		}

		peer.Storage2.HashStoreBackend, err = piecestore.NewHashStoreBackend(
			context.Background(),
			config.Hashstore,
			logsPath,
			tablePath,
			peer.Storage2.BloomFilterManager,
			peer.Storage2.RestoreTimeManager,
			process.NamedLog(peer.Log, "hashstore"),
		)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
		peer.Services.Add(lifecycle.Item{
			Name:  "hashstore",
			Close: peer.Storage2.HashStoreBackend.Close,
		})
		mon.Chain(peer.Storage2.HashStoreBackend)

		if config.Storage2.Monitor.DedicatedDisk {
			peer.Storage2.SpaceReport = monitor.NewDedicatedDisk(log, config.Storage.Path, config.Storage2.Monitor.MinimumDiskSpace.Int64(), config.Storage2.Monitor.ReservedBytes.Int64())
		} else {
			peer.Storage2.SpaceReport = monitor.NewSharedDisk(log, peer.StorageOld.Store, peer.Storage2.HashStoreBackend, config.Storage2.Monitor.MinimumDiskSpace.Int64(), config.Storage.AllocatedDiskSpace.Int64())

			// enable cache service only when using shared disk
			peer.StorageOld.CacheService = pieces.NewService(
				process.NamedLog(log, "piecestore:cache"),
				peer.StorageOld.BlobsCache,
				peer.StorageOld.Store,
				peer.DB.PieceSpaceUsedDB(),
				config.Storage2.CacheSyncInterval,
				config.Storage2.PieceScanOnStartup,
			)
			peer.Services.Add(lifecycle.Item{
				Name:  "piecestore:cache",
				Run:   peer.StorageOld.CacheService.Run,
				Close: peer.StorageOld.CacheService.Close,
			})
			peer.Debug.Server.Panel.Add(
				debug.Cycle("Piecestore Cache", peer.StorageOld.CacheService.Loop))
		}

		peer.Storage2.Monitor = monitor.NewService(
			process.NamedLog(log, "piecestore:monitor"),
			peer.StorageOld.Store,
			peer.Contact.Service,
			peer.Storage2.SpaceReport,
			config.Storage2.Monitor,
			config.Contact.CheckInTimeout,
		)
		peer.Services.Add(lifecycle.Item{
			Name:  "piecestore:monitor",
			Run:   peer.Storage2.Monitor.Run,
			Close: peer.Storage2.Monitor.Close,
		})
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Piecestore Monitor", peer.Storage2.Monitor.Loop))

		peer.StorageOld.RetainService = retain.NewService(
			process.NamedLog(peer.Log, "retain"),
			peer.StorageOld.Store,
			config.Retain,
		)

		peer.Services.Add(lifecycle.Item{
			Name:  "retain",
			Run:   peer.StorageOld.RetainService.Run,
			Close: peer.StorageOld.RetainService.Close,
		})

		peer.UsedSerials = usedserials.NewTable(config.Storage2.MaxUsedSerialsSize)

		peer.OrdersStore, err = orders.NewFileStore(
			process.NamedLog(peer.Log, "ordersfilestore"),
			config.Storage2.Orders.Path,
			config.Storage2.OrderLimitGracePeriod,
		)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
		peer.Services.Add(lifecycle.Item{
			Name:  "ordersfilestore",
			Close: peer.OrdersStore.Close,
		})

		peer.Storage2.OldPieceBackend = piecestore.NewOldPieceBackend(
			peer.StorageOld.Store,
			peer.StorageOld.TrashChore,
			peer.Storage2.Monitor,
		)

		peer.Storage2.MigrationChore = piecemigrate.NewChore(
			process.NamedLog(peer.Log, "piecemigrate:chore"),
			config.Storage2Migration,
			satstore.NewSatelliteStore(metaDir, "migrate_chore"),
			peer.StorageOld.Store,
			peer.Storage2.HashStoreBackend,
			peer.Contact.Service,
		)
		mon.Chain(peer.Storage2.MigrationChore)

		peer.Services.Add(lifecycle.Item{
			Name:  "piecemigrate:chore",
			Run:   peer.Storage2.MigrationChore.Run,
			Close: peer.Storage2.MigrationChore.Close,
		})
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Piecemigrate Migration Chore", peer.Storage2.MigrationChore.Loop))

		peer.Storage2.MigratingBackend = piecestore.NewMigratingBackend(
			peer.Log,
			peer.Storage2.OldPieceBackend,
			peer.Storage2.HashStoreBackend,
			peer.Storage2.MigrationState,
			peer.Storage2.MigrationChore,
			peer.Contact.Service,
			config.Storage2Migration.SuppressCentralMigration,
		)
		mon.Chain(peer.Storage2.MigratingBackend)

		peer.Storage2.PieceBackend = piecestore.NewTestingBackend(
			peer.Storage2.MigratingBackend,
		)

		peer.Storage2.Endpoint, err = piecestore.NewEndpoint(
			process.NamedLog(peer.Log, "piecestore"),
			peer.Identity,
			peer.Storage2.Trust,
			peer.Storage2.Monitor,
			[]piecestore.QueueRetain{
				peer.StorageOld.RetainService,
				peer.Storage2.BloomFilterManager,
			},
			peer.Contact.PingStats,
			peer.Storage2.PieceBackend,
			peer.OrdersStore,
			peer.Bandwidth.Cache,
			peer.UsedSerials,
			&signaturecheck.Full{},
			config.Storage2,
		)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		if err := pb.DRPCRegisterPiecestore(peer.Server.DRPC(), peer.Storage2.Endpoint); err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
		if err := pb.DRPCRegisterReplaySafePiecestore(peer.Server.ReplaySafeDRPC(), peer.Storage2.Endpoint); err != nil {
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
			process.NamedLog(log, "orders"),
			dialer,
			peer.OrdersStore,
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

	{ // setup payouts.
		peer.Payout.Service, err = payouts.NewService(
			process.NamedLog(peer.Log, "payouts:service"),
			peer.DB.Payout(),
			peer.DB.Reputation(),
			peer.DB.Satellites(),
			peer.Storage2.Trust,
		)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Payout.Endpoint = payouts.NewEndpoint(
			process.NamedLog(peer.Log, "payouts:endpoint"),
			peer.Dialer,
			peer.Storage2.Trust,
		)
	}

	{ // setup reputation service.
		peer.Reputation.Service = reputation.NewService(
			process.NamedLog(peer.Log, "reputation:service"),
			peer.DB.Reputation(),
			peer.Dialer,
			peer.Storage2.Trust,
			peer.Identity.ID,
			peer.Notifications.Service,
		)

		peer.Reputation.Chore = reputation.NewChore(
			process.NamedLog(peer.Log, "reputation:chore"),
			peer.Reputation.Service,
			config.Reputation,
		)

		peer.Services.Add(lifecycle.Item{
			Name:  "reputation:chore",
			Run:   peer.Reputation.Chore.Run,
			Close: peer.Reputation.Chore.Close,
		})

		peer.Debug.Server.Panel.Add(
			debug.Cycle("Reputation Chore", peer.Reputation.Chore.Loop))
	}

	{ // setup node stats service
		peer.NodeStats.Service = nodestats.NewService(
			process.NamedLog(peer.Log, "nodestats:service"),
			peer.Dialer,
			peer.Storage2.Trust,
		)

		peer.NodeStats.Cache = nodestats.NewCache(
			process.NamedLog(peer.Log, "nodestats:cache"),
			config.Nodestats,
			nodestats.CacheStorage{
				StorageUsage: peer.DB.StorageUsage(),
				Payout:       peer.DB.Payout(),
				Pricing:      peer.DB.Pricing(),
			},
			peer.NodeStats.Service,
			peer.Payout.Endpoint,
			peer.Storage2.Trust,
		)
		peer.Services.Add(lifecycle.Item{
			Name:  "nodestats:cache",
			Run:   peer.NodeStats.Cache.Run,
			Close: peer.NodeStats.Cache.Close,
		})
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Node Stats Cache Storage", peer.NodeStats.Cache.Storage))
	}

	{ // setup estimation service
		peer.Estimation.Service = estimatedpayouts.NewService(
			peer.Bandwidth.Cache,
			peer.DB.Reputation(),
			peer.DB.StorageUsage(),
			peer.DB.Pricing(),
			peer.DB.Satellites(),
			peer.Storage2.Trust,
		)
	}

	{ // setup storage node operator dashboard
		_, port, _ := net.SplitHostPort(peer.Addr())
		peer.Console.Service, err = console.NewService(
			process.NamedLog(peer.Log, "console:service"),
			peer.Bandwidth.Cache,
			peer.Version.Service,
			config.Operator.Wallet,
			versionInfo,
			peer.Storage2.Trust,
			peer.DB.Reputation(),
			peer.DB.StorageUsage(),
			peer.DB.Pricing(),
			peer.DB.Satellites(),
			peer.Contact.PingStats,
			peer.Contact.Service,
			peer.Estimation.Service,
			config.Operator.WalletFeatures,
			port,
			peer.Contact.QUICStats,
			peer.Storage2.SpaceReport,
		)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Console.Listener, err = net.Listen("tcp", config.Console.Address)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		assets := Assets
		if config.Console.StaticDir != "" {
			// HACKFIX: Previous setups specify the directory for web/storagenode,
			// instead of the actual built data. This is for backwards compatibility.
			distDir := filepath.Join(config.Console.StaticDir, "dist")
			assets = os.DirFS(distDir)
		}

		peer.Console.Endpoint = consoleserver.NewServer(
			process.NamedLog(peer.Log, "console:endpoint"),
			assets,
			peer.Notifications.Service,
			peer.Console.Service,
			peer.Payout.Service,
			peer.Console.Listener,
		)

		// add console service to peer services
		peer.Services.Add(lifecycle.Item{
			Name:  "console:endpoint",
			Run:   peer.Console.Endpoint.Run,
			Close: peer.Console.Endpoint.Close,
		})
	}

	{ // setup storage inspector
		peer.Storage2.Inspector = inspector.NewEndpoint(
			process.NamedLog(peer.Log, "pieces:inspector"),
			peer.Storage2.SpaceReport,
			peer.Contact.Service,
			peer.Contact.PingStats,
			peer.Bandwidth.Cache,
			config.Storage,
			peer.Console.Listener.Addr(),
			config.Contact.ExternalAddress,
		)
		if err := internalpb.DRPCRegisterPieceStoreInspector(peer.Server.PrivateDRPC(), peer.Storage2.Inspector); err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
	}

	{ // setup graceful exit service
		peer.GracefulExit.Service = gracefulexit.NewService(
			process.NamedLog(peer.Log, "gracefulexit:service"),
			peer.StorageOld.Store, // TODO: update graceful exit to know about hashstore
			peer.Storage2.Trust,
			peer.DB.Satellites(),
			peer.Dialer,
			config.GracefulExit,
		)

		peer.GracefulExit.Endpoint = gracefulexit.NewEndpoint(
			process.NamedLog(peer.Log, "gracefulexit:endpoint"),
			peer.Storage2.Trust,
			peer.DB.Satellites(),
			peer.Dialer,
			peer.StorageOld.BlobsCache, // TODO: update graceful exit to know about hashstore
		)
		if err := internalpb.DRPCRegisterNodeGracefulExit(peer.Server.PrivateDRPC(), peer.GracefulExit.Endpoint); err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.GracefulExit.Chore = gracefulexit.NewChore(
			process.NamedLog(peer.Log, "gracefulexit:chore"),
			peer.GracefulExit.Service,
			peer.Dialer,
			config.GracefulExit,
		)
		peer.GracefulExit.BlobsCleaner = gracefulexit.NewBlobsCleaner(
			process.NamedLog(peer.Log, "gracefulexit:blobscleaner"),
			peer.StorageOld.Store, // TODO: update graceful exit to know about hashstore
			peer.Storage2.Trust,
			peer.DB.Satellites(),
		)
		// Runs once on node start to clean blobs from trash that left after successful GE.
		peer.Services.Add(lifecycle.Item{
			Name: "gracefulexit:blobscleaner",
			Run:  peer.GracefulExit.BlobsCleaner.RemoveBlobs,
		})
		peer.Services.Add(lifecycle.Item{
			Name:  "gracefulexit:chore",
			Run:   peer.GracefulExit.Chore.Run,
			Close: peer.GracefulExit.Chore.Close,
		})
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Graceful Exit", peer.GracefulExit.Chore.Loop))
	}

	{ // setup forget-satellite
		peer.ForgetSatellite.Endpoint = forgetsatellite.NewEndpoint(
			process.NamedLog(peer.Log, "forgetsatellite:endpoint"),
			peer.Storage2.Trust,
			peer.DB.Satellites(),
		)
		if err := internalpb.DRPCRegisterNodeForgetSatellite(peer.Server.PrivateDRPC(), peer.ForgetSatellite.Endpoint); err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.ForgetSatellite.Cleaner = forgetsatellite.NewCleaner(
			process.NamedLog(peer.Log, "forgetsatellite:cleaner"),
			peer.StorageOld.Store,
			peer.Storage2.Trust,
			peer.StorageOld.BlobsCache,
			peer.DB.Satellites(),
			peer.DB.Reputation(),
			peer.DB.V0PieceInfo(),
			peer.Storage2.HashStoreBackend,
		)

		peer.ForgetSatellite.Chore = forgetsatellite.NewChore(
			process.NamedLog(peer.Log, "forgetsatellite:chore"),
			peer.ForgetSatellite.Cleaner,
			config.ForgetSatellite,
		)

		peer.Services.Add(lifecycle.Item{
			Name:  "forgetsatellite:chore",
			Run:   peer.ForgetSatellite.Chore.Run,
			Close: peer.ForgetSatellite.Chore.Close,
		})
		peer.Debug.Server.Panel.Add(
			debug.Cycle("Forget Satellite", peer.ForgetSatellite.Chore.Loop))
	}

	peer.StorageOld.Collector = collector.NewService(
		process.NamedLog(peer.Log, "collector"),
		peer.StorageOld.Store,
		peer.UsedSerials,
		config.Collector)
	peer.Services.Add(lifecycle.Item{
		Name:  "collector",
		Run:   peer.StorageOld.Collector.Run,
		Close: peer.StorageOld.Collector.Close,
	})
	peer.Debug.Server.Panel.Add(
		debug.Cycle("Collector", peer.StorageOld.Collector.Loop))

	{ // setup multinode endpoints
		// TODO: add to peer?
		apiKeys := apikeys.NewService(peer.DB.APIKeys())

		peer.Multinode.Storage = multinode.NewStorageEndpoint(
			process.NamedLog(peer.Log, "multinode:storage-endpoint"),
			apiKeys,
			peer.Storage2.Monitor,
			peer.DB.StorageUsage(),
		)

		peer.Multinode.Bandwidth = multinode.NewBandwidthEndpoint(
			process.NamedLog(peer.Log, "multinode:bandwidth-endpoint"),
			apiKeys,
			peer.Bandwidth.Cache,
		)

		peer.Multinode.Node = multinode.NewNodeEndpoint(
			process.NamedLog(peer.Log, "multinode:node-endpoint"),
			config.Operator,
			apiKeys,
			peer.Version.Service.Info,
			peer.Contact.PingStats,
			peer.DB.Reputation(),
			peer.Storage2.Trust,
		)

		peer.Multinode.Payout = multinode.NewPayoutEndpoint(
			process.NamedLog(peer.Log, "multinode:payout-endpoint"),
			apiKeys,
			peer.DB.Payout(),
			peer.Estimation.Service,
			peer.Payout.Service,
		)

		if err = multinodepb.DRPCRegisterStorage(peer.Server.DRPC(), peer.Multinode.Storage); err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
		if err = multinodepb.DRPCRegisterBandwidth(peer.Server.DRPC(), peer.Multinode.Bandwidth); err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
		if err = multinodepb.DRPCRegisterNode(peer.Server.DRPC(), peer.Multinode.Node); err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
		if err = multinodepb.DRPCRegisterPayout(peer.Server.DRPC(), peer.Multinode.Payout); err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
		if err = multinodepb.DRPCRegisterPayouts(peer.Server.DRPC(), peer.Multinode.Payout); err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
	}

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
		peer.Log.Error("Failed preflight check.", zap.Error(err))
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

// Addr returns the public address.
func (peer *Peer) Addr() string { return peer.Server.Addr().String() }

// URL returns the storj.NodeURL.
func (peer *Peer) URL() storj.NodeURL { return storj.NodeURL{ID: peer.ID(), Address: peer.Addr()} }

// PrivateAddr returns the private address.
func (peer *Peer) PrivateAddr() string { return peer.Server.PrivateAddr().String() }

func getPieceExpirationStore(log *zap.Logger, expDB pieces.PieceExpirationDB, oldCfg piecestore.OldConfig, storeCfg piecestore.Config, cfg pieces.Config) (pieces.PieceExpirationDB, error) {
	if !cfg.EnableFlatExpirationStore {
		return expDB, nil
	}

	flatFileStorePath := cfg.FlatExpirationStorePath
	if abs := filepath.IsAbs(flatFileStorePath); !abs {
		if storeCfg.DatabaseDir != "" {
			flatFileStorePath = filepath.Join(storeCfg.DatabaseDir, flatFileStorePath)
		} else {
			flatFileStorePath = filepath.Join(oldCfg.Path, flatFileStorePath)
		}
	}
	flatExpStore, err := pieces.NewPieceExpirationStore(log, pieces.PieceExpirationConfig{
		DataDir:               flatFileStorePath,
		ConcurrentFileHandles: cfg.FlatExpirationStoreFileHandles,
		MaxBufferTime:         cfg.FlatExpirationStoreMaxBufferTime,
	})

	if err != nil {
		return nil, err
	}

	if !cfg.FlatExpirationIncludeSQLite {
		return flatExpStore, nil
	}
	return pieces.NewCombinedExpirationStore(log, expDB, flatExpStore), nil
}
