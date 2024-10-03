// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenode

import (
	"context"
	"os"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/peertls/extensions"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/process"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/common/version"
	"storj.io/storj/private/mud"
	"storj.io/storj/private/server"
	"storj.io/storj/private/version/checker"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/blobstore"
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
	"storj.io/storj/storagenode/storageusage"
	"storj.io/storj/storagenode/trust"
	snversion "storj.io/storj/storagenode/version"
)

// MudPeer is the modular version of the normal storagenode.Peer.
type MudPeer struct {
	ball     *mud.Ball
	log      *zap.Logger
	selector mud.ComponentSelector
}

var _ PeerRunner = (*MudPeer)(nil)

// Service is a marker struct for all components which should be started (with all the dependencies).
type Service struct{}

func (s Service) String() string {
	return "Service"
}

// NewMudPeer creates a new Storage Node.
func NewMudPeer(zapLogger *zap.Logger, full *identity.FullIdentity, db DB, revocationDB extensions.RevocationDB, cfg Config, versionInfo version.Info, selector mud.ComponentSelector) (*MudPeer, error) {
	initializeDiskMon(zapLogger)

	ball := mud.NewBall()
	mud.Supply(ball, zapLogger)
	mud.Supply(ball, full)
	mud.Supply(ball, full.ID)
	mud.Supply(ball, versionInfo)

	// expose DB
	mud.Supply(ball, db.Notifications())
	mud.Supply(ball, db.Reputation())
	mud.Supply(ball, db.Satellites())
	mud.Supply(ball, revocationDB)
	mud.Supply(ball, db.Bandwidth())
	mud.Supply(ball, db.Orders())
	mud.Supply(ball, db.Payout())
	mud.Supply(ball, db.Pricing())
	mud.Supply(ball, db.GCFilewalkerProgress())
	mud.Supply(ball, db.V0PieceInfo())
	mud.Supply(ball, db.PieceExpirationDB())
	mud.Supply(ball, db.PieceSpaceUsedDB())
	mud.Supply(ball, db.StorageUsage())

	// expose config
	mud.Supply(ball, cfg.Debug)
	mud.Supply(ball, cfg.Version.Config)
	mud.Supply(ball, cfg.Healthcheck)
	mud.Supply(ball, cfg.Server.Config)
	mud.Supply(ball, cfg.Server)
	mud.Supply(ball, cfg.Storage2.Trust)
	mud.Supply(ball, cfg.Preflight)
	mud.Supply(ball, cfg.Contact)
	mud.Supply(ball, cfg.Operator)
	mud.Supply(ball, cfg.Bandwidth)
	mud.Supply(ball, cfg.Storage2)
	mud.Supply(ball, cfg.Pieces)
	mud.Supply(ball, cfg.Storage)
	mud.Supply(ball, cfg.Retain)
	mud.Supply(ball, cfg.Storage2.Orders)
	mud.Supply(ball, cfg.Nodestats)
	mud.Supply(ball, db.Config().LazyFilewalkerConfig())

	{ // setup notification service.
		mud.Provide[*notifications.Service](ball, notifications.NewService)
	}

	var err error

	{ // version setup
		if !versionInfo.IsZero() {
			zapLogger.Debug("Version info",
				zap.Stringer("Version", versionInfo.Version.Version),
				zap.String("Commit Hash", versionInfo.CommitHash),
				zap.Stringer("Build Timestamp", versionInfo.Timestamp),
				zap.Bool("Release Build", versionInfo.Release),
			)
		}

		if !cfg.Version.RunMode.Disabled() {
			mud.Provide[*checker.Service](ball, func(log *zap.Logger, config checker.Config, versionInfo version.Info) *checker.Service {
				return checker.NewService(log, config, versionInfo, "Storagenode")
			}, logWrapper("version"))

			versionCheckInterval := 12 * time.Hour

			mud.Provide[*snversion.Chore](ball, func(log *zap.Logger, checker *checker.Service, notificationsService *notifications.Service, nodeID storj.NodeID) *snversion.Chore {
				return snversion.NewChore(process.NamedLog(log, "version:chore"), checker, notificationsService, nodeID, versionCheckInterval)
			})

			mud.Tag[*snversion.Chore, Service](ball, Service{})

		}
	}

	{
		mud.Provide[*healthcheck.Service](ball, func(db reputation.DB, config healthcheck.Config) *healthcheck.Service {
			return healthcheck.NewService(db, config.Details)
		})
		mud.Provide[*healthcheck.Endpoint](ball, healthcheck.NewEndpoint)
	}

	{ // setup listener and server
		sc := cfg.Server
		sc.Config.UsePeerCAWhitelist = false

		mud.Provide[rpc.Dialer](ball, rpc.NewDefaultDialer)
		mud.Provide[*tlsopts.Options](ball, tlsopts.NewOptions)
		mud.Provide[*server.Server](ball, func(log *zap.Logger, tlsOptions *tlsopts.Options, config server.Config, hc healthcheck.Config, endpoint *healthcheck.Endpoint) (*server.Server, error) {
			srv, err := server.New(log, tlsOptions, config)
			if err != nil {
				return nil, err
			}
			if hc.Enabled {
				srv.AddHTTPFallback(endpoint.HandleHTTP)
			}
			return srv, nil
		}, logWrapper("server"))

	}

	{ // setup trust pool
		mud.Provide[*trust.Pool](ball, func(log *zap.Logger, satDb satellites.DB, dialer rpc.Dialer, config trust.Config) (*trust.Pool, error) {
			return trust.NewPool(process.NamedLog(log, "trust"), trust.Dialer(dialer), config, satDb)
		})
		mud.Tag[*trust.Pool, Service](ball, Service{})
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

		mud.Provide[contact.Endpoint](ball, func(log *zap.Logger, trustPool *trust.Pool, pingStats *contact.PingStats, srv *server.Server) (*contact.Endpoint, error) {
			ep := contact.NewEndpoint(log, trustPool, pingStats)
			if err := pb.DRPCRegisterContact(srv.DRPC(), ep); err != nil {
				return nil, err
			}
			return ep, nil
		})

	}

	// setup bandwidth service
	{
		mud.Provide[*bandwidth.Cache](ball, bandwidth.NewCache)
		mud.Provide[*bandwidth.Service](ball, bandwidth.NewService, logWrapper("bandwidth"))
		mud.Tag[*bandwidth.Service, Service](ball, Service{})
	}

	{ // setup storage
		mud.Supply[blobstore.Blobs](ball, db.Pieces())
		mud.Provide[*pieces.BlobsUsageCache](ball, pieces.NewBlobsUsageCache, logWrapper("blobscache"))

		mud.Provide[*pieces.FileWalker](ball, pieces.NewFileWalker)

		if cfg.Pieces.EnableLazyFilewalker {
			executable, err := os.Executable()
			if err != nil {
				return nil, errs.Wrap(err)
			}

			mud.Provide[*lazyfilewalker.Supervisor](ball, func(log *zap.Logger, config lazyfilewalker.Config) *lazyfilewalker.Supervisor {
				return lazyfilewalker.NewSupervisor(log, config, executable)
			}, logWrapper("lazyfilewalker"))
		} else {
			mud.Provide[*lazyfilewalker.Supervisor](ball, func() *lazyfilewalker.Supervisor {
				return nil
			})
		}

		mud.Provide[*pieces.Store](ball, pieces.NewStore, logWrapper("pieces"))

		mud.Provide[*pieces.Deleter](ball, func(log *zap.Logger, store *pieces.Store, storage2Config piecestore.Config) *pieces.Deleter {
			return pieces.NewDeleter(log, store, storage2Config.DeleteWorkers, storage2Config.DeleteQueueSize)
		}, logWrapper("piecedeleter"))
		mud.Tag[*pieces.Deleter, Service](ball, Service{})

		mud.Provide[*pieces.CacheService](ball, func(log *zap.Logger, usageCache *pieces.BlobsUsageCache, store *pieces.Store, storage2Config piecestore.Config) *pieces.CacheService {
			return pieces.NewService(log, usageCache, store, storage2Config.CacheSyncInterval, storage2Config.PieceScanOnStartup)
		}, logWrapper("piecestore:cache"))

		mud.Provide[monitor.SpaceReport](ball, func(log *zap.Logger, store *pieces.Store, config monitor.Config) monitor.SpaceReport {
			return monitor.NewDedicatedDisk(log, store, config.MinimumDiskSpace.Int64(), config.ReservedBytes.Int64())
		})

		mud.Provide[*monitor.Service](ball, func(log *zap.Logger, store *pieces.Store, oldConfig piecestore.OldConfig, contact *contact.Service, spaceReport monitor.SpaceReport) *monitor.Service {
			return monitor.NewService(log, store, contact, oldConfig.KBucketRefreshInterval, spaceReport, monitor.Config{})
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
				7*24*time.Hour,
				trust, store)
		}, logWrapper("pieces:trash"))
		mud.Tag[*pieces.TrashChore, Service](ball, Service{})
		mud.Provide[*piecestore.Endpoint](ball, piecestore.NewEndpoint, logWrapper("piecestore"))

		mud.Provide[*orders.Service](ball, func(log *zap.Logger, ordersStore *orders.FileStore, ordersDB orders.DB, trust *trust.Pool, config orders.Config, tlsOptions *tlsopts.Options) *orders.Service {
			// TODO workaround for custom timeout for order sending request (read/write)
			dialer := rpc.NewDefaultDialer(tlsOptions)
			dialer.DialTimeout = config.SenderDialTimeout
			return orders.NewService(log, dialer, ordersStore, ordersDB, trust, config)
		}, logWrapper("orders"))

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
	}

	{ // setup estimation service
		mud.Provide[estimatedpayouts.Service](ball, estimatedpayouts.NewService)
	}

	// TODO: there is much more elegant way to do this (for example, tagging endpoints with specific tags). But it would require bigger change.
	mud.Provide[*EndpointRegistration](ball, func(srv *server.Server, piecestoreEndpoint *piecestore.Endpoint) (*EndpointRegistration, error) {
		if err := pb.DRPCRegisterPiecestore(srv.DRPC(), piecestoreEndpoint); err != nil {
			return nil, err
		}
		if err := pb.DRPCRegisterReplaySafePiecestore(srv.ReplaySafeDRPC(), piecestoreEndpoint); err != nil {
			return nil, err
		}
		return &EndpointRegistration{}, nil
	})
	mud.Tag[*EndpointRegistration, Service](ball, Service{})

	err = mud.ForEachDependency(ball, selector, func(component *mud.Component) error {
		zapLogger.Debug("Initializing", zap.Stringer("component", component.GetTarget()))
		return component.Init(context.Background())
	}, mud.All)
	return &MudPeer{
		log:      zapLogger,
		ball:     ball,
		selector: selector,
	}, err
}

func logWrapper(name string) any {
	return mud.NewWrapper[*zap.Logger](func(logger *zap.Logger) *zap.Logger {
		return process.NamedLog(logger, name)
	})
}

// Run runs storage node until it's either closed or it errors.
func (peer *MudPeer) Run(ctx context.Context) (
	err error) {
	defer mon.Task()(&ctx)(&err)
	eg := &errgroup.Group{}
	err = mud.ForEachDependency(peer.ball, peer.selector, func(component *mud.Component) error {
		peer.log.Debug("Starting", zap.Stringer("component", component.GetTarget()))
		return component.Run(ctx, eg)
	}, mud.All)
	if err != nil {
		return err
	}
	return eg.Wait()
}

// Close closes all the resources.
func (peer *MudPeer) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err := mud.ForEachDependencyReverse(peer.ball, peer.selector, func(component *mud.Component) error {
		peer.log.Debug("Closing", zap.Stringer("component", component.GetTarget()))
		return component.Close(ctx)
	}, mud.All)
	return err
}

// EndpointRegistration is a pseudo component to wire server and DRPC endpoints together.
type EndpointRegistration struct {
}
