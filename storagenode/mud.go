// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenode

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/spacemonkeygo/monkit/v3/environment"
	"go.uber.org/zap"

	"storj.io/common/debug"
	"storj.io/common/pb"
	"storj.io/common/peertls/extensions"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/common/version"
	"storj.io/storj/private/revocation"
	"storj.io/storj/private/server"
	"storj.io/storj/private/version/checker"
	sdebug "storj.io/storj/shared/debug"
	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/modular/profiler"
	"storj.io/storj/shared/modular/tracing"
	"storj.io/storj/shared/mud"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/blobstore"
	"storj.io/storj/storagenode/blobstore/filestore"
	"storj.io/storj/storagenode/cleanup"
	"storj.io/storj/storagenode/collector"
	"storj.io/storj/storagenode/console"
	"storj.io/storj/storagenode/console/consoleserver"
	"storj.io/storj/storagenode/contact"
	"storj.io/storj/storagenode/hashstore"
	"storj.io/storj/storagenode/healthcheck"
	"storj.io/storj/storagenode/monitor"
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
	snversion "storj.io/storj/storagenode/version"
)

// RawBlobs is an interface to save the original blob implementation to mud.
// Can be used as an implementation or wrapped by cached.
type RawBlobs interface {
	blobstore.Blobs
}

// Module is a mud Module (definition of the creation of the components).
func Module(ball *mud.Ball) {
	profiler.Module(ball)
	tracing.Module(ball)
	config.RegisterConfig[contact.Config](ball, "contact")
	config.RegisterConfig[server.Config](ball, "server")
	config.RegisterConfig[preflight.Config](ball, "preflight")
	config.RegisterConfig[piecestore.Config](ball, "storage2")
	config.RegisterConfig[piecestore.OldConfig](ball, "storage")
	config.RegisterConfig[piecemigrate.Config](ball, "piecemigrate")
	config.RegisterConfig[debug.Config](ball, "debug")
	config.RegisterConfig[filestore.Config](ball, "filestore")
	config.RegisterConfig[pieces.Config](ball, "pieces")
	config.RegisterConfig[healthcheck.Config](ball, "healthcheck")
	config.RegisterConfig[nodestats.Config](ball, "nodestats")
	config.RegisterConfig[operator.Config](ball, "operator")
	config.RegisterConfig[retain.Config](ball, "retain")
	config.RegisterConfig[bandwidth.Config](ball, "bandwidth")
	config.RegisterConfig[checker.Config](ball, "version")
	config.RegisterConfig[reputation.Config](ball, "reputation")

	mud.View[piecestore.Config, trust.Config](ball, func(c piecestore.Config) trust.Config {
		return c.Trust
	})
	mud.View[piecestore.Config, orders.Config](ball, func(c piecestore.Config) orders.Config {
		return c.Orders
	})

	mud.View[server.Config, *tlsopts.Config](ball, func(s server.Config) *tlsopts.Config {
		return &s.Config
	})
	mud.View[server.Config, tlsopts.Config](ball, func(s server.Config) tlsopts.Config {
		return s.Config
	})
	mud.Provide[version.Info](ball, func() version.Info {
		info := version.Build
		environment.Register(monkit.Default)
		monkit.Default.ScopeNamed("env").Chain(monkit.StatSourceFunc(info.Stats))
		return info
	})
	mud.Provide[extensions.RevocationDB](ball, revocation.OpenDBFromCfg)
	DBModule(ball)
	mud.View[storagenodedb.Config, lazyfilewalker.Config](ball, func(s storagenodedb.Config) lazyfilewalker.Config {
		return s.LazyFilewalkerConfig()
	})

	{ // setup notification service.
		mud.Provide[*notifications.Service](ball, notifications.NewService)
	}

	{
		mud.Provide[*pieces.PieceExpirationStore](ball, func(log *zap.Logger, oldCfg piecestore.OldConfig, storeCfg piecestore.Config, cfg pieces.Config) (*pieces.PieceExpirationStore, error) {
			flatFileStorePath := cfg.FlatExpirationStorePath
			if abs := filepath.IsAbs(flatFileStorePath); !abs {
				if storeCfg.DatabaseDir != "" {
					flatFileStorePath = filepath.Join(storeCfg.DatabaseDir, flatFileStorePath)
				} else {
					flatFileStorePath = filepath.Join(oldCfg.Path, flatFileStorePath)
				}
			}
			return pieces.NewPieceExpirationStore(log, pieces.PieceExpirationConfig{
				DataDir:               flatFileStorePath,
				ConcurrentFileHandles: cfg.FlatExpirationStoreFileHandles,
				MaxBufferTime:         cfg.FlatExpirationStoreMaxBufferTime,
			})
		})
		mud.RegisterInterfaceImplementation[pieces.PieceExpirationDB, *pieces.PieceExpirationStore](ball)
	}

	{ // setup debug
		sdebug.Module(ball)
	}

	cleanup.Module(ball)
	{ // version setup
		mud.Provide[*checker.Service](ball, func(log *zap.Logger, config checker.Config, versionInfo version.Info) *checker.Service {
			return checker.NewService(log, config, versionInfo, "Storagenode")
		})

		versionCheckInterval := 12 * time.Hour

		mud.Provide[*snversion.Chore](ball, func(log *zap.Logger, checker *checker.Service, notificationsService *notifications.Service, nodeID storj.NodeID) *snversion.Chore {
			return snversion.NewChore(log, checker, notificationsService, nodeID, versionCheckInterval)
		})

		mud.Tag[*snversion.Chore, modular.Service](ball, modular.Service{})
	}

	{
		mud.Provide[*healthcheck.Service](ball, func(db reputation.DB, config healthcheck.Config) *healthcheck.Service {
			return healthcheck.NewService(db, config.Details)
		})
		mud.Provide[*healthcheck.Endpoint](ball, healthcheck.NewEndpoint)
		mud.Provide[HttpFallbackHandler](ball, func(endpoint *healthcheck.Endpoint) HttpFallbackHandler {
			return HttpFallbackHandler{
				Handler: endpoint.HandleHTTP,
			}
		})
	}

	{ // setup listener and server
		mud.Provide[rpc.Dialer](ball, rpc.NewDefaultDialer)
		mud.Provide[*tlsopts.Options](ball, tlsopts.NewOptions)
		mud.Provide[*server.Server](ball, func(log *zap.Logger, tlsOptions *tlsopts.Options, config server.Config, fallback HttpFallbackHandler) (*server.Server, error) {
			config.UsePeerCAWhitelist = false
			srv, err := server.New(log, tlsOptions, config)
			if err != nil {
				return nil, err
			}
			if fallback.Handler != nil {
				srv.AddHTTPFallback(fallback.Handler)
			}
			return srv, nil
		})
	}

	{ // setup trust pool
		mud.Provide[*trust.Pool](ball, func(log *zap.Logger, satDb satellites.DB, dialer rpc.Dialer, config trust.Config) (*trust.Pool, error) {
			pool, err := trust.NewPool(log, trust.Dialer(dialer), config, satDb)
			if err != nil {
				return nil, err
			}
			pool.StartWithRefresh = true
			return pool, err
		})
		mud.RegisterInterfaceImplementation[trust.TrustedSatelliteSource, *trust.Pool](ball)
	}

	{
		mud.Provide[*preflight.LocalTime](ball, preflight.NewLocalTime)
	}

	{ // setup contact service
		mud.Provide[contact.NodeInfo](ball, func(ctx context.Context, log *zap.Logger, id storj.NodeID, contactConfig contact.Config, operator operator.Config, versionInfo version.Info, server *server.Server, state *satstore.SatelliteStore, hashstoreConfig hashstore.Config) (contact.NodeInfo, error) {
			externalAddress := contactConfig.ExternalAddress
			if externalAddress == "" {
				externalAddress = server.Addr().String()
			}

			pbVersion, err := versionInfo.Proto()
			if err != nil {
				return contact.NodeInfo{}, err
			}

			noiseKeyAttestation, err := server.NoiseKeyAttestation(ctx)
			if err != nil {
				return contact.NodeInfo{}, err
			}

			nodeInfo := contact.NodeInfo{
				ID:      id,
				Address: externalAddress,
				Operator: pb.NodeOperator{
					Email:          operator.Email,
					Wallet:         operator.Wallet,
					WalletFeatures: operator.WalletFeatures,
				},
				Version:             *pbVersion,
				NoiseKeyAttestation: noiseKeyAttestation,
				DebounceLimit:       server.DebounceLimit(),
				FastOpen:            server.FastOpen(),
				HashstoreMemtbl:     hashstoreConfig.TableDefaultKind.Kind == hashstore.TableKind_MemTbl,
				HashstoreWriteToNew: ReportHashstoreWriteToNew(log, state),
			}

			return nodeInfo, nil
		})

		mud.Provide[*contact.PingStats](ball, func() *contact.PingStats {
			return new(contact.PingStats)
		})
		mud.View[*contact.PingStats, piecestore.PingStatsSource](ball, func(stats *contact.PingStats) piecestore.PingStatsSource {
			return stats
		})

		mud.Provide[*contact.QUICStats](ball, func(server *server.Server) *contact.QUICStats {
			return contact.NewQUICStats(server.IsQUICEnabled())
		})

		mud.Provide[*pb.SignedNodeTagSets](ball, contact.GetTags)

		mud.Provide[*contact.Service](ball, contact.NewService)

		mud.Provide[*contact.Chore](ball, func(log *zap.Logger, contactConfig contact.Config, service *contact.Service) *contact.Chore {
			return contact.NewChore(log, contactConfig.Interval, contactConfig.CheckInTimeout, service)
		})
		mud.Tag[*contact.Chore, modular.Service](ball, modular.Service{})

		mud.Provide[*contact.Endpoint](ball, func(log *zap.Logger, trustSource trust.TrustedSatelliteSource, pingStats *contact.PingStats, srv *server.Server) (*contact.Endpoint, error) {
			ep := contact.NewEndpoint(log, trustSource, pingStats)
			if err := pb.DRPCRegisterContact(srv.DRPC(), ep); err != nil {
				return nil, err
			}
			return ep, nil
		})
		mud.Tag[*contact.Endpoint, modular.Service](ball, modular.Service{})
	}

	// setup bandwidth service
	{
		mud.Provide[*bandwidth.Cache](ball, bandwidth.NewCache)
		mud.Provide[*bandwidth.Service](ball, bandwidth.NewService)
		mud.Tag[*bandwidth.Service, modular.Service](ball, modular.Service{})
	}

	{ // setup storage
		mud.Provide[*pieces.FileWalker](ball, pieces.NewFileWalker)

		executable, err := os.Executable()
		if err != nil {
			panic(err)
		}

		mud.Provide[*lazyfilewalker.Supervisor](ball, func(log *zap.Logger, config lazyfilewalker.Config) *lazyfilewalker.Supervisor {
			return lazyfilewalker.NewSupervisor(log, config, executable)
		})

		mud.Provide[*pieces.Store](ball, pieces.NewStore)

		mud.Provide[*pieces.BlobsUsageCache](ball, func(log *zap.Logger, blobs RawBlobs) *pieces.BlobsUsageCache {
			return pieces.NewBlobsUsageCache(log, blobs)
		})
		mud.Provide[*pieces.CacheService](ball, func(log *zap.Logger, usageCache *pieces.BlobsUsageCache, store *pieces.Store, usedSpaceDB pieces.PieceSpaceUsedDB, storage2Config piecestore.Config) *pieces.CacheService {
			return pieces.NewService(log, usageCache, store, usedSpaceDB, storage2Config.CacheSyncInterval, storage2Config.PieceScanOnStartup)
		})

		mud.View[DB, RawBlobs](ball, func(db DB) RawBlobs {
			return db.Pieces()
		})

		mud.RegisterInterfaceImplementation[blobstore.Blobs, RawBlobs](ball)

		mud.Provide[monitor.SpaceReport](ball, func(log *zap.Logger, oldConfig piecestore.OldConfig, config monitor.Config) monitor.SpaceReport {
			return monitor.NewDedicatedDisk(log, oldConfig.Path, config.MinimumDiskSpace.Int64(), config.ReservedBytes.Int64())
		})
		config.RegisterConfig[monitor.Config](ball, "monitor")

		mud.RegisterInterfaceImplementation[monitor.DiskVerification, *pieces.Store](ball)
		mud.Provide[*monitor.Service](ball, func(log *zap.Logger, verifier monitor.DiskVerification, contactService *contact.Service, report monitor.SpaceReport, config monitor.Config, contactConfig contact.Config) *monitor.Service {
			return monitor.NewService(log, verifier, contactService, report, config, contactConfig.CheckInTimeout)
		})

		mud.Provide[*retain.Service](ball, retain.NewService)
		mud.Provide[*retain.RunOnce](ball, retain.NewRunOnce)

		mud.Provide[*usedserials.Table](ball, func(storage2Config piecestore.Config) *usedserials.Table {
			return usedserials.NewTable(storage2Config.MaxUsedSerialsSize)
		})

		mud.Provide[*orders.FileStore](ball, func(log *zap.Logger, storage2Config piecestore.Config) (*orders.FileStore, error) {
			return orders.NewFileStore(log, storage2Config.Orders.Path, storage2Config.OrderLimitGracePeriod)
		})

		mud.Provide[*pieces.TrashChore](ball, func(log *zap.Logger, trust *trust.Pool, store *pieces.Store) *pieces.TrashChore {
			return pieces.NewTrashChore(
				log,
				24*time.Hour,
				trashExpiryInterval,
				trust, store)
		})
		mud.Provide[*pieces.TrashRunOnce](ball, func(log *zap.Logger, blobs blobstore.Blobs, stop *modular.StopTrigger) *pieces.TrashRunOnce {
			return pieces.NewTrashRunOnce(log, blobs, trashExpiryInterval, stop)
		})

		mud.RegisterInterfaceImplementation[piecestore.RestoreTrash, *pieces.TrashChore](ball)
		mud.RegisterImplementation[[]piecestore.QueueRetain](ball)
		mud.Implementation[[]piecestore.QueueRetain, *retain.Service](ball)

		mud.Provide[*satstore.SatelliteStore](ball, func(cfg hashstore.Config, old piecestore.OldConfig) *satstore.SatelliteStore {
			logsPath, _ := cfg.Directories(old.Path)
			return satstore.NewSatelliteStore(filepath.Join(logsPath, "meta"), "migrate")
		})
		mud.Provide[*piecestore.OldPieceBackend](ball, piecestore.NewOldPieceBackend)
		mud.Provide[*piecestore.HashStoreBackend](ball, func(ctx context.Context, cfg hashstore.Config, old piecestore.OldConfig, bfm *retain.BloomFilterManager, rtm *retain.RestoreTimeManager, log *zap.Logger) (*piecestore.HashStoreBackend, error) {
			logsPath, tablePath := cfg.Directories(old.Path)
			backend, err := piecestore.NewHashStoreBackend(ctx, cfg, logsPath, tablePath, bfm, rtm, log)
			if err != nil {
				return nil, err
			}
			mon.Chain(backend)
			return backend, nil
		})
		mud.Provide[*piecemigrate.Chore](ball, func(log *zap.Logger, cfg piecemigrate.Config, config hashstore.Config, old *pieces.Store, new *piecestore.HashStoreBackend, piecestoreOldConfig piecestore.OldConfig, contactService *contact.Service) *piecemigrate.Chore {
			logsPath, _ := config.Directories(piecestoreOldConfig.Path)
			chore := piecemigrate.NewChore(log, cfg, satstore.NewSatelliteStore(filepath.Join(logsPath, "meta"), "migrate_chore"), old, new, contactService)
			mon.Chain(chore)
			return chore
		})
		mud.Provide[*piecestore.MigratingBackend](ball, func(log *zap.Logger, old *piecestore.OldPieceBackend, new *piecestore.HashStoreBackend, state *satstore.SatelliteStore, chore *piecemigrate.Chore, contactService *contact.Service, cfg piecemigrate.Config) *piecestore.MigratingBackend {
			backend := piecestore.NewMigratingBackend(log, old, new, state, chore, contactService, cfg.SuppressCentralMigration)
			mon.Chain(backend)
			return backend
		})
		config.RegisterConfig[hashstore.Config](ball, "hashstore")

		// default is the old one
		mud.RegisterInterfaceImplementation[piecestore.PieceBackend, *piecestore.OldPieceBackend](ball)

		mud.Provide[*retain.BloomFilterManager](ball, func(cfg hashstore.Config, old piecestore.OldConfig, rcfg retain.Config) (*retain.BloomFilterManager, error) {
			logsPath, _ := cfg.Directories(old.Path)
			return retain.NewBloomFilterManager(filepath.Join(logsPath, "meta"), rcfg.MaxTimeSkew)
		})
		mud.Implementation[[]piecestore.QueueRetain, *retain.BloomFilterManager](ball)
		mud.Provide[*retain.RestoreTimeManager](ball, func(cfg hashstore.Config, old piecestore.OldConfig) *retain.RestoreTimeManager {
			logsPath, _ := cfg.Directories(old.Path)
			return retain.NewRestoreTimeManager(filepath.Join(logsPath, "meta"))
		})

		mud.Provide[*piecestore.Endpoint](ball, piecestore.NewEndpoint)

		mud.Provide[*orders.Service](ball, func(log *zap.Logger, ordersStore *orders.FileStore, trustSource trust.TrustedSatelliteSource, config orders.Config, tlsOptions *tlsopts.Options) *orders.Service {
			// TODO workaround for custom timeout for order sending request (read/write)
			dialer := rpc.NewDefaultDialer(tlsOptions)
			dialer.DialTimeout = config.SenderDialTimeout
			return orders.NewService(log, dialer, ordersStore, trustSource, config)
		})
		mud.Tag[*orders.Service, modular.Service](ball, modular.Service{})
	}

	{ // setup payouts.
		mud.Provide[*payouts.Service](ball, payouts.NewService)
		mud.Provide[*payouts.Endpoint](ball, payouts.NewEndpoint)
	}

	{ // setup reputation service.
		mud.Provide[*reputation.Service](ball, reputation.NewService)
		mud.Provide[*reputation.Chore](ball, reputation.NewChore)
		mud.Tag[*reputation.Chore, modular.Service](ball, modular.Service{})
	}

	{ // setup node stats service
		mud.Provide[*nodestats.Service](ball, nodestats.NewService)
		mud.Provide[nodestats.CacheStorage](ball, func(sdb storageusage.DB, pdb payouts.DB, prdb pricing.DB) nodestats.CacheStorage {
			return nodestats.CacheStorage{
				StorageUsage: sdb,
				Payout:       pdb,
				Pricing:      prdb,
			}
		})
		mud.Provide[*nodestats.Cache](ball, nodestats.NewCache)
		mud.Tag[*nodestats.Cache, modular.Service](ball, modular.Service{})
	}

	{
		mud.Provide[*collector.Service](ball, collector.NewService)
		mud.Provide[collector.RunOnce](ball, collector.NewRunnerOnce)
		config.RegisterConfig[collector.Config](ball, "collector")
		mud.Tag[*collector.Service, modular.Service](ball, modular.Service{})
	}
	// TODO: there is much more elegant way to do this. But we have circular dependency between piecestore endpoint and Server
	// (mainly, because everybody is interested about the actual server port)
	mud.Provide[*EndpointRegistration](ball, func(srv *server.Server, piecestoreEndpoint *piecestore.Endpoint) (*EndpointRegistration, error) {
		if err := pb.DRPCRegisterPiecestore(srv.DRPC(), piecestoreEndpoint); err != nil {
			return nil, err
		}
		if err := pb.DRPCRegisterReplaySafePiecestore(srv.ReplaySafeDRPC(), piecestoreEndpoint); err != nil {
			return nil, err
		}
		return &EndpointRegistration{}, nil
	})
	mud.Tag[*EndpointRegistration, modular.Service](ball, modular.Service{})

	signaturecheck.Module(ball)

	estimatedpayouts.Module(ball)
	console.Module(ball)
	consoleserver.Module(ball, Assets)
}

// EndpointRegistration is a pseudo component to wire server and DRPC endpoints together.
type EndpointRegistration struct{}

// HttpFallbackHandler is an extension to the public DRPC server.
type HttpFallbackHandler struct {
	Handler http.HandlerFunc
}

// ReportHashstoreWriteToNew returns a function that can be used for reporting current WriteToNew status for satellites.
func ReportHashstoreWriteToNew(log *zap.Logger, store *satstore.SatelliteStore) func() bool {
	return func() bool {
		var res bool
		err := store.Range(func(id storj.NodeID, bytes []byte) error {
			var ms piecestore.MigrationState
			err := json.Unmarshal(bytes, &ms)
			if err != nil {
				log.Warn("failed to unmarshal migration state", zap.Error(err), zap.Stringer("satellite", id))
			}
			if ms.WriteToNew {
				res = true
			}
			return nil
		})
		if err != nil {
			log.Warn("Couldn't read migration state", zap.Error(err))
		}
		return res
	}
}
