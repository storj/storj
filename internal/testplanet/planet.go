// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

// Package testplanet implements the full network wiring for testing
package testplanet

import (
	"context"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"

	"storj.io/storj/bootstrap"
	"storj.io/storj/bootstrap/bootstrapdb"
	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/accounting/rollup"
	"storj.io/storj/pkg/accounting/tally"
	"storj.io/storj/pkg/audit"
	"storj.io/storj/pkg/bwagreement"
	"storj.io/storj/pkg/datarepair/checker"
	"storj.io/storj/pkg/datarepair/repairer"
	"storj.io/storj/pkg/discovery"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/piecestore/psserver"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/server"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleweb"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/storagenodedb"
)

// Peer represents one of StorageNode or Satellite
type Peer interface {
	ID() storj.NodeID
	Addr() string
	Local() pb.Node

	Run(context.Context) error
	Close() error

	NewNodeClient() (node.Client, error)
}

// Config describes planet configuration
type Config struct {
	SatelliteCount   int
	StorageNodeCount int
	UplinkCount      int

	Identities  *Identities
	Reconfigure Reconfigure
}

// Reconfigure allows to change node configurations
type Reconfigure struct {
	NewBootstrapDB func(index int) (bootstrap.DB, error)
	Bootstrap      func(index int, config *bootstrap.Config)

	NewSatelliteDB func(index int) (satellite.DB, error)
	Satellite      func(index int, config *satellite.Config)

	NewStorageNodeDB func(index int) (storagenode.DB, error)
	StorageNode      func(index int, config *storagenode.Config)
}

// Planet is a full storj system setup.
type Planet struct {
	log       *zap.Logger
	config    Config
	directory string // TODO: ensure that everything is in-memory to speed things up
	started   bool

	peers     []closablePeer
	databases []io.Closer
	uplinks   []*Uplink

	Bootstrap    *bootstrap.Peer
	Satellites   []*satellite.Peer
	StorageNodes []*storagenode.Peer
	Uplinks      []*Uplink

	identities *Identities

	cancel func()
}

type closablePeer struct {
	peer Peer

	ctx    context.Context
	cancel func()

	close sync.Once
	err   error
}

// Close closes safely the peer.
func (peer *closablePeer) Close() error {
	peer.cancel()
	peer.close.Do(func() {
		peer.err = peer.peer.Close()
	})
	return peer.err
}

// New creates a new full system with the given number of nodes.
func New(t zaptest.TestingT, satelliteCount, storageNodeCount, uplinkCount int) (*Planet, error) {
	var log *zap.Logger
	if t == nil {
		log = zap.NewNop()
	} else {
		log = zaptest.NewLogger(t)
	}

	return NewWithLogger(log, satelliteCount, storageNodeCount, uplinkCount)
}

// NewWithLogger creates a new full system with the given number of nodes.
func NewWithLogger(log *zap.Logger, satelliteCount, storageNodeCount, uplinkCount int) (*Planet, error) {
	return NewCustom(log, Config{
		SatelliteCount:   satelliteCount,
		StorageNodeCount: storageNodeCount,
		UplinkCount:      uplinkCount,
	})
}

// NewCustom creates a new full system with the specified configuration.
func NewCustom(log *zap.Logger, config Config) (*Planet, error) {
	if config.Identities == nil {
		config.Identities = pregeneratedIdentities
	}

	planet := &Planet{
		log:        log,
		config:     config,
		identities: config.Identities,
	}

	var err error
	planet.directory, err = ioutil.TempDir("", "planet")
	if err != nil {
		return nil, err
	}

	planet.Bootstrap, err = planet.newBootstrap()
	if err != nil {
		return nil, errs.Combine(err, planet.Shutdown())
	}

	planet.Satellites, err = planet.newSatellites(config.SatelliteCount)
	if err != nil {
		return nil, errs.Combine(err, planet.Shutdown())
	}

	planet.StorageNodes, err = planet.newStorageNodes(config.StorageNodeCount)
	if err != nil {
		return nil, errs.Combine(err, planet.Shutdown())
	}

	planet.Uplinks, err = planet.newUplinks("uplink", config.UplinkCount, config.StorageNodeCount)
	if err != nil {
		return nil, errs.Combine(err, planet.Shutdown())
	}

	// init Satellites
	for _, satellite := range planet.Satellites {
		if len(satellite.Kademlia.Service.GetBootstrapNodes()) == 0 {
			satellite.Kademlia.Service.SetBootstrapNodes([]pb.Node{planet.Bootstrap.Local()})
		}
	}
	// init storage nodes
	for _, storageNode := range planet.StorageNodes {
		if len(storageNode.Kademlia.Service.GetBootstrapNodes()) == 0 {
			storageNode.Kademlia.Service.SetBootstrapNodes([]pb.Node{planet.Bootstrap.Local()})
		}
	}

	return planet, nil
}

// Start starts all the nodes.
func (planet *Planet) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	planet.cancel = cancel

	for i := range planet.peers {
		peer := &planet.peers[i]
		peer.ctx, peer.cancel = context.WithCancel(ctx)
		go func(peer *closablePeer) {
			err := peer.peer.Run(peer.ctx)
			if err == grpc.ErrServerStopped {
				err = nil
			}
			if err != nil {
				// TODO: better error handling
				panic(err)
			}
		}(peer)
	}

	planet.started = true

	for _, peer := range planet.Satellites {
		peer.Kademlia.Service.WaitForBootstrap()
	}
	for _, peer := range planet.StorageNodes {
		peer.Kademlia.Service.WaitForBootstrap()
	}
}

// StopPeer stops a single peer in the planet
func (planet *Planet) StopPeer(peer Peer) error {
	for i := range planet.peers {
		p := &planet.peers[i]
		if p.peer == peer {
			return p.Close()
		}
	}
	return errors.New("unknown peer")
}

// Size returns number of nodes in the network
func (planet *Planet) Size() int { return len(planet.uplinks) + len(planet.peers) }

// Shutdown shuts down all the nodes and deletes temporary directories.
func (planet *Planet) Shutdown() error {
	if !planet.started {
		return errors.New("Start was never called")
	}
	planet.cancel()

	var errlist errs.Group
	// shutdown in reverse order
	for i := len(planet.uplinks) - 1; i >= 0; i-- {
		node := planet.uplinks[i]
		errlist.Add(node.Shutdown())
	}
	for i := len(planet.peers) - 1; i >= 0; i-- {
		peer := &planet.peers[i]
		errlist.Add(peer.Close())
	}
	for _, db := range planet.databases {
		errlist.Add(db.Close())
	}

	errlist.Add(os.RemoveAll(planet.directory))
	return errlist.Err()
}

// newUplinks creates initializes uplinks
func (planet *Planet) newUplinks(prefix string, count, storageNodeCount int) ([]*Uplink, error) {
	var xs []*Uplink
	for i := 0; i < count; i++ {
		uplink, err := planet.newUplink(prefix+strconv.Itoa(i), storageNodeCount)
		if err != nil {
			return nil, err
		}
		xs = append(xs, uplink)
	}

	return xs, nil
}

// newSatellites initializes satellites
func (planet *Planet) newSatellites(count int) ([]*satellite.Peer, error) {
	var xs []*satellite.Peer
	defer func() {
		for _, x := range xs {
			planet.peers = append(planet.peers, closablePeer{peer: x})
		}
	}()

	for i := 0; i < count; i++ {
		prefix := "satellite" + strconv.Itoa(i)
		log := planet.log.Named(prefix)

		storageDir := filepath.Join(planet.directory, prefix)
		if err := os.MkdirAll(storageDir, 0700); err != nil {
			return nil, err
		}

		identity, err := planet.NewIdentity()
		if err != nil {
			return nil, err
		}

		var db satellite.DB
		if planet.config.Reconfigure.NewSatelliteDB != nil {
			db, err = planet.config.Reconfigure.NewSatelliteDB(i)
		} else {
			db, err = satellitedb.NewInMemory()
		}
		if err != nil {
			return nil, err
		}

		err = db.CreateTables()
		if err != nil {
			return nil, err
		}

		planet.databases = append(planet.databases, db)

		config := satellite.Config{
			Server: server.Config{
				Address:            "127.0.0.1:0",
				RevocationDBURL:    "bolt://" + filepath.Join(planet.directory, "revocation.db"),
				UsePeerCAWhitelist: false, // TODO: enable
				Extensions: peertls.TLSExtConfig{
					Revocation:          true,
					WhitelistSignedLeaf: false,
				},
			},
			Kademlia: kademlia.Config{
				Alpha:  5,
				DBPath: storageDir, // TODO: replace with master db
				Operator: kademlia.OperatorConfig{
					Email:  prefix + "@example.com",
					Wallet: "0x" + strings.Repeat("00", 20),
				},
			},
			Overlay: overlay.Config{
				RefreshInterval: 30 * time.Second,
				Node: overlay.NodeSelectionConfig{
					UptimeRatio:           0,
					UptimeCount:           0,
					AuditSuccessRatio:     0,
					AuditCount:            0,
					NewNodeAuditThreshold: 0,
					NewNodePercentage:     0,
				},
			},
			Discovery: discovery.Config{
				GraveyardInterval: 1 * time.Second,
				DiscoveryInterval: 1 * time.Second,
				RefreshInterval:   1 * time.Second,
				RefreshLimit:      100,
			},
			PointerDB: pointerdb.Config{
				DatabaseURL:          "bolt://" + filepath.Join(storageDir, "pointers.db"),
				MinRemoteSegmentSize: 0, // TODO: fix tests to work with 1024
				MaxInlineSegmentSize: 8000,
				Overlay:              true,
				BwExpiration:         45,
			},
			BwAgreement: bwagreement.Config{},
			Checker: checker.Config{
				Interval: 30 * time.Second,
			},
			Repairer: repairer.Config{
				MaxRepair:     10,
				Interval:      time.Hour,
				OverlayAddr:   "", // overridden in satellite.New
				PointerDBAddr: "", // overridden in satellite.New
				MaxBufferMem:  4 * memory.MB,
				APIKey:        "",
			},
			Audit: audit.Config{
				MaxRetriesStatDB: 0,
				Interval:         30 * time.Second,
			},
			Tally: tally.Config{
				Interval: 30 * time.Second,
			},
			Rollup: rollup.Config{
				Interval: 120 * time.Second,
			},
			Console: consoleweb.Config{
				Address:      "127.0.0.1:0",
				PasswordCost: console.TestPasswordCost,
			},
		}
		if planet.config.Reconfigure.Satellite != nil {
			planet.config.Reconfigure.Satellite(i, &config)
		}

		// TODO: for development only
		config.Console.StaticDir = "./web/satellite"

		peer, err := satellite.New(log, identity, db, &config)
		if err != nil {
			return xs, err
		}

		log.Debug("id=" + peer.ID().String() + " addr=" + peer.Addr())
		xs = append(xs, peer)
	}
	return xs, nil
}

// newStorageNodes initializes storage nodes
func (planet *Planet) newStorageNodes(count int) ([]*storagenode.Peer, error) {
	var xs []*storagenode.Peer
	defer func() {
		for _, x := range xs {
			planet.peers = append(planet.peers, closablePeer{peer: x})
		}
	}()

	for i := 0; i < count; i++ {
		prefix := "storage" + strconv.Itoa(i)
		log := planet.log.Named(prefix)
		storageDir := filepath.Join(planet.directory, prefix)

		if err := os.MkdirAll(storageDir, 0700); err != nil {
			return nil, err
		}

		identity, err := planet.NewIdentity()
		if err != nil {
			return nil, err
		}

		var db storagenode.DB
		if planet.config.Reconfigure.NewStorageNodeDB != nil {
			db, err = planet.config.Reconfigure.NewStorageNodeDB(i)
		} else {
			db, err = storagenodedb.NewInMemory(storageDir)
		}
		if err != nil {
			return nil, err
		}

		err = db.CreateTables()
		if err != nil {
			return nil, err
		}

		planet.databases = append(planet.databases, db)

		config := storagenode.Config{
			Server: server.Config{
				Address:            "127.0.0.1:0",
				RevocationDBURL:    "bolt://" + filepath.Join(storageDir, "revocation.db"),
				UsePeerCAWhitelist: false, // TODO: enable
				Extensions: peertls.TLSExtConfig{
					Revocation:          true,
					WhitelistSignedLeaf: false,
				},
			},
			Kademlia: kademlia.Config{
				Alpha:  5,
				DBPath: storageDir, // TODO: replace with master db
				Operator: kademlia.OperatorConfig{
					Email:  prefix + "@example.com",
					Wallet: "0x" + strings.Repeat("00", 20),
				},
			},
			Storage: psserver.Config{
				Path:                   "", // TODO: this argument won't be needed with master storagenodedb
				AllocatedDiskSpace:     memory.TB,
				AllocatedBandwidth:     memory.TB,
				KBucketRefreshInterval: time.Hour,

				AgreementSenderCheckInterval: time.Hour,
				CollectorInterval:            time.Hour,
			},
		}
		if planet.config.Reconfigure.StorageNode != nil {
			planet.config.Reconfigure.StorageNode(i, &config)
		}

		peer, err := storagenode.New(log, identity, db, config)
		if err != nil {
			return xs, err
		}

		log.Debug("id=" + peer.ID().String() + " addr=" + peer.Addr())
		xs = append(xs, peer)
	}
	return xs, nil
}

// newBootstrap initializes the bootstrap node
func (planet *Planet) newBootstrap() (peer *bootstrap.Peer, err error) {
	defer func() {
		planet.peers = append(planet.peers, closablePeer{peer: peer})
	}()

	prefix := "bootstrap"
	log := planet.log.Named(prefix)
	dbDir := filepath.Join(planet.directory, prefix)

	if err := os.MkdirAll(dbDir, 0700); err != nil {
		return nil, err
	}

	identity, err := planet.NewIdentity()
	if err != nil {
		return nil, err
	}

	var db bootstrap.DB
	if planet.config.Reconfigure.NewBootstrapDB != nil {
		db, err = planet.config.Reconfigure.NewBootstrapDB(0)
	} else {
		db, err = bootstrapdb.NewInMemory(dbDir)
	}

	err = db.CreateTables()
	if err != nil {
		return nil, err
	}

	planet.databases = append(planet.databases, db)

	config := bootstrap.Config{
		Server: server.Config{
			Address:            "127.0.0.1:0",
			RevocationDBURL:    "bolt://" + filepath.Join(dbDir, "revocation.db"),
			UsePeerCAWhitelist: false, // TODO: enable
			Extensions: peertls.TLSExtConfig{
				Revocation:          true,
				WhitelistSignedLeaf: false,
			},
		},
		Kademlia: kademlia.Config{
			Alpha:  5,
			DBPath: dbDir, // TODO: replace with master db
			Operator: kademlia.OperatorConfig{
				Email:  prefix + "@example.com",
				Wallet: "0x" + strings.Repeat("00", 20),
			},
		},
	}
	if planet.config.Reconfigure.Bootstrap != nil {
		planet.config.Reconfigure.Bootstrap(0, &config)
	}

	peer, err = bootstrap.New(log, identity, db, config)
	if err != nil {
		return nil, err
	}

	log.Debug("id=" + peer.ID().String() + " addr=" + peer.Addr())

	return peer, nil
}

// Identities returns the identity provider for this planet.
func (planet *Planet) Identities() *Identities {
	return planet.identities
}

// NewIdentity creates a new identity for a node
func (planet *Planet) NewIdentity() (*identity.FullIdentity, error) {
	return planet.identities.NewIdentity()
}

// NewListener creates a new listener
func (planet *Planet) NewListener() (net.Listener, error) {
	return net.Listen("tcp", "127.0.0.1:0")
}
