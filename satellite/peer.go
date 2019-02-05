// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/accounting/rollup"
	"storj.io/storj/pkg/accounting/tally"
	"storj.io/storj/pkg/audit"
	"storj.io/storj/pkg/auth/grpcauth"
	"storj.io/storj/pkg/bwagreement"
	"storj.io/storj/pkg/datarepair/checker"
	"storj.io/storj/pkg/datarepair/irreparable"
	"storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/datarepair/repairer"
	"storj.io/storj/pkg/discovery"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/server"
	"storj.io/storj/pkg/statdb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/console/consoleweb"
	"storj.io/storj/storage"
	"storj.io/storj/storage/boltdb"
	"storj.io/storj/storage/storelogger"
)

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

	// BandwidthAgreement returns database for storing bandwidth agreements
	BandwidthAgreement() bwagreement.DB
	// StatDB returns database for storing node statistics
	StatDB() statdb.DB
	// OverlayCache returns database for caching overlay information
	OverlayCache() overlay.DB
	// Accounting returns database for storing information about data use
	Accounting() accounting.DB
	// RepairQueue returns queue for segments that need repairing
	RepairQueue() queue.RepairQueue
	// Irreparable returns database for failed repairs
	Irreparable() irreparable.DB
	// Console returns database for satellite console
	Console() console.DB
}

// Config is the global config satellite
type Config struct {
	Identity identity.Config

	// TODO: switch to using server.Config when Identity has been removed from it
	Server server.Config

	Kademlia  kademlia.Config
	Overlay   overlay.Config
	Discovery discovery.Config

	PointerDB   pointerdb.Config
	BwAgreement bwagreement.Config // TODO: decide whether to keep empty configs for consistency

	Checker  checker.Config
	Repairer repairer.Config
	Audit    audit.Config

	Tally  tally.Config
	Rollup rollup.Config

	Console consoleweb.Config
}

// Peer is the satellite
type Peer struct {
	// core dependencies
	Log      *zap.Logger
	Identity *identity.FullIdentity
	DB       DB

	// servers
	Public struct {
		Listener net.Listener
		Server   *server.Server
	}

	// services and endpoints
	Kademlia struct {
		kdb, ndb storage.KeyValueStore // TODO: move these into DB

		RoutingTable *kademlia.RoutingTable
		Service      *kademlia.Kademlia
		Endpoint     *node.Server
		Inspector    *kademlia.Inspector
	}

	Overlay struct {
		Service   *overlay.Cache
		Endpoint  *overlay.Server
		Inspector *overlay.Inspector
	}

	Discovery struct {
		Service *discovery.Discovery
	}

	Reputation struct {
		Inspector *statdb.Inspector
	}

	Metainfo struct {
		Database   storage.KeyValueStore // TODO: move into pointerDB
		Allocation *pointerdb.AllocationSigner
		Service    *pointerdb.Service
		Endpoint   *pointerdb.Server
	}

	Agreements struct {
		Endpoint *bwagreement.Server
	}

	Repair struct {
		Checker  checker.Checker // TODO: convert to actual struct
		Repairer *repairer.Service
	}
	Audit struct {
		Service *audit.Service
	}

	Accounting struct {
		Tally  *tally.Tally
		Rollup *rollup.Rollup
	}

	Console struct {
		Listener net.Listener
		Service  *console.Service
		Endpoint *consoleweb.Server
	}
}

// New creates a new satellite
func New(log *zap.Logger, full *identity.FullIdentity, db DB, config *Config) (*Peer, error) {
	peer := &Peer{
		Log:      log,
		Identity: full,
		DB:       db,
	}

	var err error

	{ // setup listener and server
		peer.Public.Listener, err = net.Listen("tcp", config.Server.Address)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		publicConfig := server.Config{Address: peer.Public.Listener.Addr().String()}
		publicOptions, err := server.NewOptions(peer.Identity, publicConfig)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Public.Server, err = server.New(publicOptions, peer.Public.Listener, grpcauth.NewAPIKeyInterceptor())
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
	}

	{ // setup kademlia
		config := config.Kademlia
		// TODO: move this setup logic into kademlia package
		if config.ExternalAddress == "" {
			config.ExternalAddress = peer.Public.Server.Addr().String()
		}

		self := pb.Node{
			Id:   peer.ID(),
			Type: pb.NodeType_SATELLITE,
			Address: &pb.NodeAddress{
				Address: config.ExternalAddress,
			},
			Metadata: &pb.NodeMetadata{
				Email:  config.Operator.Email,
				Wallet: config.Operator.Wallet,
			},
		}

		{ // setup routing table
			// TODO: clean this up, should be part of database
			bucketIdentifier := peer.ID().String()[:5] // need a way to differentiate between nodes if running more than one simultaneously
			dbpath := filepath.Join(config.DBPath, fmt.Sprintf("kademlia_%s.db", bucketIdentifier))

			if err := os.MkdirAll(config.DBPath, 0777); err != nil && !os.IsExist(err) {
				return nil, err
			}

			dbs, err := boltdb.NewShared(dbpath, kademlia.KademliaBucket, kademlia.NodeBucket)
			if err != nil {
				return nil, errs.Combine(err, peer.Close())
			}
			peer.Kademlia.kdb, peer.Kademlia.ndb = dbs[0], dbs[1]

			peer.Kademlia.RoutingTable, err = kademlia.NewRoutingTable(peer.Log.Named("routing"), self, peer.Kademlia.kdb, peer.Kademlia.ndb, &config.RoutingTableConfig)
			if err != nil {
				return nil, errs.Combine(err, peer.Close())
			}
		}

		// TODO: reduce number of arguments
		peer.Kademlia.Service, err = kademlia.NewService(peer.Log.Named("kademlia"), self, config.BootstrapNodes(), peer.Identity, config.Alpha, peer.Kademlia.RoutingTable)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Kademlia.Endpoint = node.NewServer(peer.Log.Named("kademlia:endpoint"), peer.Kademlia.Service)
		pb.RegisterNodesServer(peer.Public.Server.GRPC(), peer.Kademlia.Endpoint)

		peer.Kademlia.Inspector = kademlia.NewInspector(peer.Kademlia.Service, peer.Identity)
		pb.RegisterKadInspectorServer(peer.Public.Server.GRPC(), peer.Kademlia.Inspector)
	}

	{ // setup overlay
		config := config.Overlay
		peer.Overlay.Service = overlay.NewCache(peer.DB.OverlayCache(), peer.DB.StatDB())

		nodeSelectionConfig := &overlay.NodeSelectionConfig{
			UptimeCount:           config.Node.UptimeCount,
			UptimeRatio:           config.Node.UptimeRatio,
			AuditSuccessRatio:     config.Node.AuditSuccessRatio,
			AuditCount:            config.Node.AuditCount,
			NewNodeAuditThreshold: config.Node.NewNodeAuditThreshold,
			NewNodePercentage:     config.Node.NewNodePercentage,
		}

		peer.Overlay.Endpoint = overlay.NewServer(peer.Log.Named("overlay:endpoint"), peer.Overlay.Service, nodeSelectionConfig)
		pb.RegisterOverlayServer(peer.Public.Server.GRPC(), peer.Overlay.Endpoint)

		peer.Overlay.Inspector = overlay.NewInspector(peer.Overlay.Service)
		pb.RegisterOverlayInspectorServer(peer.Public.Server.GRPC(), peer.Overlay.Inspector)
	}

	{ // setup reputation
		// TODO: find better structure with overlay
		peer.Reputation.Inspector = statdb.NewInspector(peer.DB.StatDB())
		pb.RegisterStatDBInspectorServer(peer.Public.Server.GRPC(), peer.Reputation.Inspector)
	}

	{ // setup discovery
		config := config.Discovery
		peer.Discovery.Service = discovery.New(peer.Log.Named("discovery"), peer.Overlay.Service, peer.Kademlia.Service, peer.DB.StatDB(), config)
	}

	{ // setup metainfo
		db, err := pointerdb.NewStore(config.PointerDB.DatabaseURL)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Metainfo.Database = storelogger.New(peer.Log.Named("pdb"), db)
		peer.Metainfo.Service = pointerdb.NewService(peer.Log.Named("pointerdb"), peer.Metainfo.Database)
		peer.Metainfo.Allocation = pointerdb.NewAllocationSigner(peer.Identity, config.PointerDB.BwExpiration)
		peer.Metainfo.Endpoint = pointerdb.NewServer(peer.Log.Named("pointerdb:endpoint"), peer.Metainfo.Service, peer.Metainfo.Allocation, peer.Overlay.Service, config.PointerDB, peer.Identity)
		pb.RegisterPointerDBServer(peer.Public.Server.GRPC(), peer.Metainfo.Endpoint)
	}

	{ // setup agreements
		bwServer := bwagreement.NewServer(peer.DB.BandwidthAgreement(), peer.Log.Named("agreements"), peer.Identity.ID)
		peer.Agreements.Endpoint = bwServer
		pb.RegisterBandwidthServer(peer.Public.Server.GRPC(), peer.Agreements.Endpoint)
	}

	{ // setup datarepair
		// TODO: simplify argument list somehow
		peer.Repair.Checker = checker.NewChecker(
			peer.Metainfo.Service,
			peer.DB.StatDB(), peer.DB.RepairQueue(),
			peer.Overlay.Endpoint, peer.DB.Irreparable(),
			0, peer.Log.Named("checker"),
			config.Checker.Interval)

		peer.Repair.Repairer = repairer.NewService(peer.DB.RepairQueue(), &config.Repairer, peer.Identity, config.Repairer.Interval, config.Repairer.MaxRepair)
	}

	{ // setup audit
		config := config.Audit

		// TODO: use common transport Client and close to avoid leak
		transportClient := transport.NewClient(peer.Identity)

		peer.Audit.Service, err = audit.NewService(peer.Log.Named("audit"),
			peer.DB.StatDB(),
			config.Interval, config.MaxRetriesStatDB,
			peer.Metainfo.Service, peer.Metainfo.Allocation,
			transportClient, peer.Overlay.Service,
			peer.Identity,
		)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
	}

	{ // setup accounting
		peer.Accounting.Tally = tally.New(peer.Log.Named("tally"), peer.DB.Accounting(), peer.DB.BandwidthAgreement(), peer.Metainfo.Service, peer.Overlay.Endpoint, 0, config.Tally.Interval)
		peer.Accounting.Rollup = rollup.New(peer.Log.Named("rollup"), peer.DB.Accounting(), config.Rollup.Interval)
	}

	{ // setup console
		config := config.Console

		peer.Console.Listener, err = net.Listen("tcp", config.Address)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Console.Service, err = console.NewService(peer.Log.Named("console:service"),
			// TODO: use satellite key
			&consoleauth.Hmac{Secret: []byte("my-suppa-secret-key")},
			peer.DB.Console(),
			config.PasswordCost,
		)

		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Console.Endpoint = consoleweb.NewServer(peer.Log.Named("console:endpoint"),
			config,
			peer.Console.Service,
			peer.Console.Listener)
	}

	return peer, nil
}

func ignoreCancel(err error) error {
	if err == context.Canceled || err == grpc.ErrServerStopped || err == http.ErrServerClosed {
		return nil
	}
	return err
}

// Run runs storage node until it's either closed or it errors.
func (peer *Peer) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var group errgroup.Group
	group.Go(func() error {
		return ignoreCancel(peer.Kademlia.Service.Bootstrap(ctx))
	})
	group.Go(func() error {
		return ignoreCancel(peer.Kademlia.Service.RunRefresh(ctx))
	})
	group.Go(func() error {
		return ignoreCancel(peer.Discovery.Service.Run(ctx))
	})
	group.Go(func() error {
		return ignoreCancel(peer.Repair.Checker.Run(ctx))
	})
	group.Go(func() error {
		return ignoreCancel(peer.Repair.Repairer.Run(ctx))
	})
	group.Go(func() error {
		return ignoreCancel(peer.Accounting.Tally.Run(ctx))
	})
	group.Go(func() error {
		return ignoreCancel(peer.Accounting.Rollup.Run(ctx))
	})
	group.Go(func() error {
		return ignoreCancel(peer.Audit.Service.Run(ctx))
	})
	group.Go(func() error {
		// TODO: move the message into Server instead
		peer.Log.Sugar().Infof("Node %s started on %s", peer.Identity.ID, peer.Public.Server.Addr().String())
		return ignoreCancel(peer.Public.Server.Run(ctx))
	})
	group.Go(func() error {
		return ignoreCancel(peer.Console.Endpoint.Run(ctx))
	})

	return group.Wait()
}

// Close closes all the resources.
func (peer *Peer) Close() error {
	var errlist errs.Group

	// TODO: ensure that Close can be called on nil-s that way this code won't need the checks.

	// close servers, to avoid new connections to closing subsystems
	if peer.Public.Server != nil {
		errlist.Add(peer.Public.Server.Close())
	} else {
		// peer.Public.Server automatically closes listener
		if peer.Public.Listener != nil {
			errlist.Add(peer.Public.Listener.Close())
		}
	}

	if peer.Console.Endpoint != nil {
		errlist.Add(peer.Console.Endpoint.Close())
	} else {
		if peer.Console.Endpoint != nil {
			errlist.Add(peer.Public.Listener.Close())
		}
	}

	// close services in reverse initialization order
	if peer.Repair.Repairer != nil {
		errlist.Add(peer.Repair.Repairer.Close())
	}
	if peer.Repair.Checker != nil {
		errlist.Add(peer.Repair.Checker.Close())
	}

	if peer.Agreements.Endpoint != nil {
		errlist.Add(peer.Agreements.Endpoint.Close())
	}

	if peer.Metainfo.Endpoint != nil {
		errlist.Add(peer.Metainfo.Endpoint.Close())
	}
	if peer.Metainfo.Database != nil {
		errlist.Add(peer.Metainfo.Database.Close())
	}

	if peer.Discovery.Service != nil {
		errlist.Add(peer.Discovery.Service.Close())
	}

	if peer.Overlay.Endpoint != nil {
		errlist.Add(peer.Overlay.Endpoint.Close())
	}
	if peer.Overlay.Service != nil {
		errlist.Add(peer.Overlay.Service.Close())
	}

	// TODO: add kademlia.Endpoint for consistency
	if peer.Kademlia.Service != nil {
		errlist.Add(peer.Kademlia.Service.Close())
	}
	if peer.Kademlia.RoutingTable != nil {
		errlist.Add(peer.Kademlia.RoutingTable.Close())
	}

	if peer.Kademlia.ndb != nil || peer.Kademlia.kdb != nil {
		errlist.Add(peer.Kademlia.kdb.Close())
		errlist.Add(peer.Kademlia.ndb.Close())
	}

	return errlist.Err()
}

// ID returns the peer ID.
func (peer *Peer) ID() storj.NodeID { return peer.Identity.ID }

// Local returns the peer local node info.
func (peer *Peer) Local() pb.Node { return peer.Kademlia.RoutingTable.Local() }

// Addr returns the public address.
func (peer *Peer) Addr() string { return peer.Public.Server.Addr().String() }
