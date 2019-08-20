// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/internal/errs2"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/server"
	"storj.io/storj/pkg/signing"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair/checker"
	"storj.io/storj/satellite/repair/irreparable"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/satellite/repair/repairer"
	"storj.io/storj/storage"
)

// RepairProcessDB is the interface to connect to the master database for the satellite's RepairProcess
type RepairProcessDB interface {
	// CreateTables initializes the database
	CreateTables() error
	// Close closes the database
	Close() error

	// CreateSchema sets the schema
	CreateSchema(schema string) error
	// DropSchema drops the schema
	DropSchema(schema string) error

	// OverlayCache returns database for caching overlay information
	OverlayCache() overlay.DB
	// RepairQueue returns queue for segments that need repairing
	RepairQueue() queue.RepairQueue
	// Irreparable returns database for failed repairs
	Irreparable() irreparable.DB
	// Orders returns database for orders
	Orders() orders.DB
	// Buckets returns the database to interact with buckets
	Buckets() metainfo.BucketsDB
}

// RepairProcessConfig is the config used for RepairProcess
type RepairProcessConfig struct {
	Identity identity.Config
	Server   server.Config
	Overlay  overlay.Config
	Metainfo metainfo.Config
	Orders   orders.Config
	Checker  checker.Config
	Repairer repairer.Config
}

// RepairProcess is the repair subsystem for the satellite
type RepairProcess struct {
	Log      *zap.Logger
	Identity *identity.FullIdentity
	DB       RepairProcessDB

	Transport transport.Client

	Overlay struct {
		DB        overlay.DB
		Service   *overlay.Service
		Inspector *overlay.Inspector
	}

	Metainfo struct {
		Database  storage.KeyValueStore
		Service   *metainfo.Service
		Endpoint2 *metainfo.Endpoint
		Loop      *metainfo.Loop
	}

	Orders struct {
		Endpoint *orders.Endpoint
		Service  *orders.Service
	}

	Repair struct {
		Checker  *checker.Checker
		Repairer *repairer.Service
	}
}

// NewRepairProcess creates a new repair subsystem process for the satellite
func NewRepairProcess(log *zap.Logger, full *identity.FullIdentity, db RepairProcessDB,
	revDB extensions.RevocationDB, config *RepairProcessConfig) (*RepairProcess, error) {
	peer := &RepairProcess{
		Log:      log,
		Identity: full,
		DB:       db,
	}

	{ // setup transport client
		log.Debug("Starting transport client for repair process")
		sc := config.Server
		options, err := tlsopts.NewOptions(peer.Identity, sc.Config, revDB)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
		peer.Transport = transport.NewClient(options)
	}

	{ // setup overlay
		log.Debug("Starting overlay for repair process")
		peer.Overlay.DB = overlay.NewCombinedCache(peer.DB.OverlayCache())
		peer.Overlay.Service = overlay.NewService(peer.Log.Named("overlay"), peer.Overlay.DB, config.Overlay)
	}

	{ // setup orders
		log.Debug("Setting up orders for repair process")
		peer.Orders.Service = orders.NewService(
			peer.Log.Named("orders:service"),
			signing.SignerFromFullIdentity(peer.Identity),
			peer.Overlay.Service,
			peer.DB.Orders(),
			config.Orders.Expiration,
			&pb.NodeAddress{
				Transport: pb.NodeTransport_TCP_TLS_GRPC,
				// TODO: we need the SA address here
				// Address:   config.Kademlia.ExternalAddress,
			},
			config.Repairer.MaxExcessRateOptimalThreshold,
		)
	}

	{ // setup metainfo
		log.Debug("Setting up metainfo for repair process")
		db, err := metainfo.NewStore(peer.Log.Named("metainfo:store"), config.Metainfo.DatabaseURL)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
		peer.Metainfo.Database = db
		peer.Metainfo.Service = metainfo.NewService(peer.Log.Named("metainfo:service"),
			peer.Metainfo.Database,
			peer.DB.Buckets(),
		)
		peer.Metainfo.Loop = metainfo.NewLoop(config.Metainfo.Loop, peer.Metainfo.Service)
	}

	{ // setup datarepair
		log.Debug("Setting up datarepair")
		peer.Repair.Checker = checker.NewChecker(
			peer.Log.Named("checker"),
			peer.DB.RepairQueue(),
			peer.DB.Irreparable(),
			peer.Metainfo.Service,
			peer.Metainfo.Loop,
			peer.Overlay.Service,
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
	}

	return peer, nil
}

// Run the satellite's RepairProcess until it is closed or it errors.
func (peer *RepairProcess) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Repair.Checker.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Metainfo.Loop.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Repair.Repairer.Run(ctx))
	})
	return group.Wait()
}

// Close closes all the resources.
// close services in reverse initialization order
func (peer *RepairProcess) Close() error {
	var errlist errs.Group
	if peer.Repair.Repairer != nil {
		errlist.Add(peer.Repair.Repairer.Close())
	}
	if peer.Repair.Checker != nil {
		errlist.Add(peer.Repair.Checker.Close())
	}
	if peer.Metainfo.Database != nil {
		errlist.Add(peer.Metainfo.Database.Close())
	}
	if peer.Overlay.Service != nil {
		errlist.Add(peer.Overlay.Service.Close())
	}
	return errlist.Err()
}

// ID returns the peer ID
func (peer *RepairProcess) ID() storj.NodeID { return peer.Identity.ID }
