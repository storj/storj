// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenode

import (
	"context"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"

	"storj.io/common/debug"
	"storj.io/common/pb"
	"storj.io/common/peertls/extensions"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/process"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/common/version"
	"storj.io/storj/private/mud"
	"storj.io/storj/private/revocation"
	"storj.io/storj/private/server"
	"storj.io/storj/private/version/checker"
	sdebug "storj.io/storj/shared/debug"
	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/blobstore"
	"storj.io/storj/storagenode/blobstore/filestore"
	"storj.io/storj/storagenode/collector"
	"storj.io/storj/storagenode/contact"
	"storj.io/storj/storagenode/healthcheck"
	"storj.io/storj/storagenode/monitor"
	"storj.io/storj/storagenode/nodestats"
	"storj.io/storj/storagenode/notifications"
	"storj.io/storj/storagenode/operator"
	"storj.io/storj/storagenode/orders"
	"storj.io/storj/storagenode/payouts"
	"storj.io/storj/storagenode/payouts/estimatedpayouts"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/pieces/lazyfilewalker"
	"storj.io/storj/storagenode/piecestore"
	"storj.io/storj/storagenode/piecestore/usedserials"
	"storj.io/storj/storagenode/preflight"
	"storj.io/storj/storagenode/pricing"
	"storj.io/storj/storagenode/reputation"
	"storj.io/storj/storagenode/retain"
	"storj.io/storj/storagenode/satellites"
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
	config.RegisterConfig[contact.Config](ball, "contact")
	config.RegisterConfig[server.Config](ball, "server")
	config.RegisterConfig[preflight.Config](ball, "preflight")
	config.RegisterConfig[piecestore.Config](ball, "storage2")
	config.RegisterConfig[piecestore.OldConfig](ball, "storage")
	config.RegisterConfig[debug.Config](ball, "debug")
	config.RegisterConfig[filestore.Config](ball, "filestore")
	config.RegisterConfig[pieces.Config](ball, "pieces")
	config.RegisterConfig[healthcheck.Config](ball, "healthcheck")
	config.RegisterConfig[nodestats.Config](ball, "nodestats")
	config.RegisterConfig[operator.Config](ball, "operator")
	config.RegisterConfig[retain.Config](ball, "retain")
	config.RegisterConfig[bandwidth.Config](ball, "bandwidth")
	config.RegisterConfig[checker.Config](ball, "version")

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
	mud.Supply[version.Info](ball, version.Build)
	mud.Provide[extensions.RevocationDB](ball, revocation.OpenDBFromCfg)
	DBModule(ball)
	mud.View[storagenodedb.Config, lazyfilewalker.Config](ball, func(s storagenodedb.Config) lazyfilewalker.Config {
		return s.LazyFilewalkerConfig()
	})

	{ // setup notification service.
		mud.Provide[*notifications.Service](ball, notifications.NewService)

	}

	{ // setup debug
		sdebug.Module(ball)
	}

	{ // version setup
		mud.Provide[*checker.Service](ball, func(log *zap.Logger, config checker.Config, versionInfo version.Info) *checker.Service {
			return checker.NewService(log, config, versionInfo, "Storagenode")
		}, logWrapper("version"))

		versionCheckInterval := 12 * time.Hour

		mud.Provide[*snversion.Chore](ball, func(log *zap.Logger, checker *checker.Service, notificationsService *notifications.Service, nodeID storj.NodeID) *snversion.Chore {
			return snversion.NewChore(process.NamedLog(log, "version:chore"), checker, notificationsService, nodeID, versionCheckInterval)
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
		}, logWrapper("server"))

	}

	{ // setup trust pool
		mud.Provide[*trust.Pool](ball, func(log *zap.Logger, satDb satellites.DB, dialer rpc.Dialer, config trust.Config) (*trust.Pool, error) {
			pool, err := trust.NewPool(process.NamedLog(log, "trust"), trust.Dialer(dialer), config, satDb)
			pool.StartWithRefresh = true
			return pool, err
		})
		mud.Tag[*trust.Pool, modular.Service](ball, modular.Service{})
	}

	{
		mud.Provide[*preflight.LocalTime](ball, preflight.NewLocalTime, logWrapper("preflight:localtime"))
	}

	{ // setup contact service
		mud.Provide[contact.NodeInfo](ball, func(id storj.NodeID, contactConfig contact.Config, operator operator.Config, versionInfo version.Info, server *server.Server) (contact.NodeInfo, error) {
			externalAddress := contactConfig.ExternalAddress
			if externalAddress == "" {
				externalAddress = server.Addr().String()
			}

			pbVersion, err := versionInfo.Proto()
			if err != nil {
				return contact.NodeInfo{}, err
			}

			noiseKeyAttestation, err := server.NoiseKeyAttestation(context.Background())
			if err != nil {
				return contact.NodeInfo{}, err
			}

			return contact.NodeInfo{
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
			}, nil
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

		mud.Provide[*pb.SignedNodeTagSets](ball, func(config contact.Config) *pb.SignedNodeTagSets {
			tags := pb.SignedNodeTagSets(config.Tags)
			return &tags
		})

		mud.Provide[*contact.Service](ball, contact.NewService, logWrapper("contact:service"))

		mud.Provide[*contact.Chore](ball, func(log *zap.Logger, contactConfig contact.Config, service *contact.Service) *contact.Chore {
			return contact.NewChore(log, contactConfig.Interval, service)
		}, logWrapper("contact:chore"))
		mud.Tag[*contact.Chore, modular.Service](ball, modular.Service{})

		mud.Provide[*contact.Endpoint](ball, func(log *zap.Logger, trustPool *trust.Pool, pingStats *contact.PingStats, srv *server.Server) (*contact.Endpoint, error) {
			ep := contact.NewEndpoint(log, trustPool, pingStats)
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
		mud.Provide[*bandwidth.Service](ball, bandwidth.NewService, logWrapper("bandwidth"))
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
		}, logWrapper("lazyfilewalker"))

		mud.Provide[*pieces.Store](ball, pieces.NewStore, logWrapper("pieces"))

		mud.Provide[*pieces.Deleter](ball, func(log *zap.Logger, store *pieces.Store, storage2Config piecestore.Config) *pieces.Deleter {
			return pieces.NewDeleter(log, store, storage2Config.DeleteWorkers, storage2Config.DeleteQueueSize)
		}, logWrapper("piecedeleter"))
		mud.Tag[*pieces.Deleter, modular.Service](ball, modular.Service{})

		mud.Provide[*pieces.BlobsUsageCache](ball, func(log *zap.Logger, blobs RawBlobs) *pieces.BlobsUsageCache {
			return pieces.NewBlobsUsageCache(log, blobs)
		}, logWrapper("blobscache"))
		mud.Provide[*pieces.CacheService](ball, func(log *zap.Logger, usageCache *pieces.BlobsUsageCache, store *pieces.Store, usedSpaceDB pieces.PieceSpaceUsedDB, storage2Config piecestore.Config) *pieces.CacheService {
			return pieces.NewService(log, usageCache, store, usedSpaceDB, storage2Config.CacheSyncInterval, storage2Config.PieceScanOnStartup)
		}, logWrapper("piecestore:cache"))

		mud.View[DB, RawBlobs](ball, func(db DB) RawBlobs {
			return db.Pieces()
		})
		// TODO: use this later for cached blobs. Without this, the normal implementation will be used.
		// mud.View[*pieces.BlobsUsageCache, blobstore.Blobs](ball, func(cache *pieces.BlobsUsageCache) blobstore.Blobs {
		//	return cache
		// })
		mud.View[RawBlobs, blobstore.Blobs](ball, func(blobs RawBlobs) blobstore.Blobs {
			return blobs
		})

		mud.Provide[monitor.SpaceReport](ball, func(log *zap.Logger, store *pieces.Store, config monitor.Config) monitor.SpaceReport {
			return monitor.NewDedicatedDisk(log, store, config.MinimumDiskSpace.Int64(), config.ReservedBytes.Int64())
		})
		config.RegisterConfig[monitor.Config](ball, "monitor")

		mud.Provide[*monitor.Service](ball, func(log *zap.Logger, store *pieces.Store, oldConfig piecestore.OldConfig, contact *contact.Service, spaceReport monitor.SpaceReport, config monitor.Config) *monitor.Service {
			return monitor.NewService(log, store, contact, oldConfig.KBucketRefreshInterval, spaceReport, config)
		}, logWrapper("piecestore:monitor"))

		mud.Provide[*retain.Service](ball, retain.NewService, logWrapper("retain"))

		mud.Provide[*usedserials.Table](ball, func(storage2Config piecestore.Config) *usedserials.Table {
			return usedserials.NewTable(storage2Config.MaxUsedSerialsSize)
		})

		mud.Provide[*orders.FileStore](ball, func(log *zap.Logger, storage2Config piecestore.Config) (*orders.FileStore, error) {
			return orders.NewFileStore(log, storage2Config.Orders.Path, storage2Config.OrderLimitGracePeriod)
		}, logWrapper("ordersfilestore"))

		mud.Provide[*pieces.TrashChore](ball, func(log *zap.Logger, trust *trust.Pool, store *pieces.Store) *pieces.TrashChore {
			return pieces.NewTrashChore(
				log,
				24*time.Hour,
				trashExpiryInterval,
				trust, store)
		}, logWrapper("pieces:trash"))
		mud.Provide[*pieces.TrashRunOnce](ball, func(log *zap.Logger, trust *trust.Pool, store *pieces.Store, stop *modular.StopTrigger) *pieces.TrashRunOnce {
			return pieces.NewTrashRunOnce(log, trust, store, trashExpiryInterval, stop)
		})
		mud.Tag[*pieces.TrashChore, modular.Service](ball, modular.Service{})
		mud.Provide[*piecestore.Endpoint](ball, piecestore.NewEndpoint, logWrapper("piecestore"))

		mud.Provide[*orders.Service](ball, func(log *zap.Logger, ordersStore *orders.FileStore, ordersDB orders.DB, trust *trust.Pool, config orders.Config, tlsOptions *tlsopts.Options) *orders.Service {
			// TODO workaround for custom timeout for order sending request (read/write)
			dialer := rpc.NewDefaultDialer(tlsOptions)
			dialer.DialTimeout = config.SenderDialTimeout
			return orders.NewService(log, dialer, ordersStore, ordersDB, trust, config)
		}, logWrapper("orders"))
		mud.Tag[*orders.Service, modular.Service](ball, modular.Service{})

	}

	{ // setup payouts.
		mud.Provide[*payouts.Service](ball, payouts.NewService, logWrapper("payouts:service"))
		mud.Provide[*payouts.Endpoint](ball, payouts.NewEndpoint, logWrapper("payouts:endpoint"))
	}

	{ // setup reputation service.
		mud.Provide[*reputation.Service](ball, reputation.NewService, logWrapper("reputation:service"))
	}

	{ // setup node stats service
		mud.Provide[*nodestats.Service](ball, nodestats.NewService, logWrapper("nodestats:service"))
		mud.Provide[nodestats.CacheStorage](ball, func(rdb reputation.DB, sdb storageusage.DB, pdb payouts.DB, prdb pricing.DB) nodestats.CacheStorage {
			return nodestats.CacheStorage{
				Reputation:   rdb,
				StorageUsage: sdb,
				Payout:       pdb,
				Pricing:      prdb,
			}
		})
		mud.Provide[*nodestats.Cache](ball, nodestats.NewCache, logWrapper("nodestats:cache"))
		mud.Tag[*nodestats.Cache, modular.Service](ball, modular.Service{})
	}

	{ // setup estimation service
		mud.Provide[estimatedpayouts.Service](ball, estimatedpayouts.NewService)
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

}

func logWrapper(name string) any {
	return mud.NewWrapper[*zap.Logger](func(logger *zap.Logger) *zap.Logger {
		return process.NamedLog(logger, name)
	})
}

// EndpointRegistration is a pseudo component to wire server and DRPC endpoints together.
type EndpointRegistration struct{}

// HttpFallbackHandler is an extension to the public DRPC server.
type HttpFallbackHandler struct {
	Handler http.HandlerFunc
}
