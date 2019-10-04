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
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/bootstrap"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/internal/version"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/rpc"
	"storj.io/storj/pkg/server"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/accounting/live"
	"storj.io/storj/satellite/accounting/rollup"
	"storj.io/storj/satellite/accounting/tally"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleweb"
	"storj.io/storj/satellite/contact"
	"storj.io/storj/satellite/dbcleanup"
	"storj.io/storj/satellite/discovery"
	"storj.io/storj/satellite/gc"
	"storj.io/storj/satellite/inspector"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/marketingweb"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/nodestats"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair/checker"
	"storj.io/storj/satellite/repair/irreparable"
	"storj.io/storj/satellite/repair/repairer"
	"storj.io/storj/satellite/vouchers"
	"storj.io/storj/storagenode"
	"storj.io/storj/versioncontrol"
)

const defaultInterval = 15 * time.Second

// Peer represents one of StorageNode or Satellite
type Peer interface {
	ID() storj.NodeID
	Addr() string
	URL() storj.NodeURL
	Local() overlay.NodeDossier

	Run(context.Context) error
	Close() error
}

// Config describes planet configuration
type Config struct {
	SatelliteCount   int
	StorageNodeCount int
	UplinkCount      int

	Identities      *testidentity.Identities
	IdentityVersion *storj.IDVersion
	Reconfigure     Reconfigure
}

// Planet is a full storj system setup.
type Planet struct {
	log       *zap.Logger
	config    Config
	directory string // TODO: ensure that everything is in-memory to speed things up

	started  bool
	shutdown bool

	peers     []closablePeer
	databases []io.Closer
	uplinks   []*Uplink

	Bootstrap      *bootstrap.Peer
	VersionControl *versioncontrol.Peer
	Satellites     []*SatelliteSystem
	StorageNodes   []*storagenode.Peer
	Uplinks        []*Uplink

	identities    *testidentity.Identities
	whitelistPath string // TODO: in-memory

	run    errgroup.Group
	cancel func()
}

// SatelliteSystem contains all the processes needed to run a full Satellite setup
type SatelliteSystem struct {
	Peer *satellite.Peer
	API  *satellite.API

	Log      *zap.Logger
	Identity *identity.FullIdentity
	DB       satellite.DB

	Dialer  rpc.Dialer
	Server  *server.Server
	Version *version.Service

	Contact struct {
		Service   *contact.Service
		Endpoint  *contact.Endpoint
		KEndpoint *contact.KademliaEndpoint
	}
	Overlay struct {
		DB        overlay.DB
		Service   *overlay.Service
		Inspector *overlay.Inspector
	}
	Discovery struct {
		Service *discovery.Discovery
	}
	Metainfo struct {
		Database  metainfo.PointerDB
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
		Queue    *audit.Queue
		Worker   *audit.Worker
		Chore    *audit.Chore
		Verifier *audit.Verifier
		Reporter *audit.Reporter
	}
	GarbageCollection struct {
		Service *gc.Service
	}
	DBCleanup struct {
		Chore *dbcleanup.Chore
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

// ID returns the ID of the Satellite system.
func (system *SatelliteSystem) ID() storj.NodeID { return system.API.Identity.ID }

// Local returns the peer local node info from the Satellite system API.
func (system *SatelliteSystem) Local() overlay.NodeDossier { return system.API.Contact.Service.Local() }

// Addr returns the public address from the Satellite system API.
func (system *SatelliteSystem) Addr() string { return system.API.Server.Addr().String() }

// URL returns the storj.NodeURL from the Satellite system API.
func (system *SatelliteSystem) URL() storj.NodeURL {
	return storj.NodeURL{ID: system.API.ID(), Address: system.API.Addr()}
}

// Close closes all the subsystems in the Satellite system
func (system *SatelliteSystem) Close() error {
	return errs.Combine(system.API.Close(), system.Peer.Close())
}

// Run runs all the subsystems in the Satellite system
func (system *SatelliteSystem) Run(ctx context.Context) (err error) {
	return errs.Combine(system.API.Run(ctx), system.Peer.Run(ctx))
}

// PrivateAddr returns the private address from the Satellite system API.
func (system *SatelliteSystem) PrivateAddr() string { return system.API.Server.PrivateAddr().String() }

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

// NewWithIdentityVersion creates a new full system with the given version for node identities and the given number of nodes.
func NewWithIdentityVersion(t zaptest.TestingT, identityVersion *storj.IDVersion, satelliteCount, storageNodeCount, uplinkCount int) (*Planet, error) {
	var log *zap.Logger
	if t == nil {
		log = zap.NewNop()
	} else {
		log = zaptest.NewLogger(t)
	}

	return NewCustom(log, Config{
		SatelliteCount:   satelliteCount,
		StorageNodeCount: storageNodeCount,
		UplinkCount:      uplinkCount,
		IdentityVersion:  identityVersion,
	})
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
	if config.IdentityVersion == nil {
		version := storj.LatestIDVersion()
		config.IdentityVersion = &version
	}
	if config.Identities == nil {
		config.Identities = testidentity.NewPregeneratedSignedIdentities(*config.IdentityVersion)
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

	whitelistPath, err := planet.WriteWhitelist(*config.IdentityVersion)
	if err != nil {
		return nil, err
	}
	planet.whitelistPath = whitelistPath

	planet.VersionControl, err = planet.newVersionControlServer()
	if err != nil {
		return nil, errs.Combine(err, planet.Shutdown())
	}

	planet.Bootstrap, err = planet.newBootstrap()
	if err != nil {
		return nil, errs.Combine(err, planet.Shutdown())
	}

	planet.Satellites, err = planet.newSatellites(config.SatelliteCount)
	if err != nil {
		return nil, errs.Combine(err, planet.Shutdown())
	}

	whitelistedSatellites := make(storj.NodeURLs, 0, len(planet.Satellites))
	for _, satellite := range planet.Satellites {
		whitelistedSatellites = append(whitelistedSatellites, satellite.API.URL())
	}

	planet.StorageNodes, err = planet.newStorageNodes(config.StorageNodeCount, whitelistedSatellites)
	if err != nil {
		return nil, errs.Combine(err, planet.Shutdown())
	}

	planet.Uplinks, err = planet.newUplinks("uplink", config.UplinkCount, config.StorageNodeCount)
	if err != nil {
		return nil, errs.Combine(err, planet.Shutdown())
	}

	return planet, nil
}

// Start starts all the nodes.
func (planet *Planet) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	planet.cancel = cancel

	planet.run.Go(func() error {
		return planet.VersionControl.Run(ctx)
	})

	for i := range planet.peers {
		peer := &planet.peers[i]
		peer.ctx, peer.cancel = context.WithCancel(ctx)
		planet.run.Go(func() error {
			return peer.peer.Run(peer.ctx)
		})
	}

	var group errgroup.Group
	for _, peer := range planet.StorageNodes {
		peer := peer
		group.Go(func() error {
			peer.Storage2.Monitor.Loop.TriggerWait()
			peer.Contact.Chore.Loop.TriggerWait()
			return nil
		})
	}
	_ = group.Wait()

	planet.started = true
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
	if planet.shutdown {
		panic("double Shutdown")
	}
	planet.shutdown = true

	planet.cancel()

	var errlist errs.Group

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		// TODO: add diagnostics to see what hasn't been properly shut down
		timer := time.NewTimer(30 * time.Second)
		defer timer.Stop()
		select {
		case <-timer.C:
			panic("planet took too long to shutdown")
		case <-ctx.Done():
		}
	}()
	errlist.Add(planet.run.Wait())
	cancel()

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
	errlist.Add(planet.VersionControl.Close())

	errlist.Add(os.RemoveAll(planet.directory))
	return errlist.Err()
}

// Identities returns the identity provider for this planet.
func (planet *Planet) Identities() *testidentity.Identities {
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

// WriteWhitelist writes the pregenerated signer's CA cert to a "CA whitelist", PEM-encoded.
func (planet *Planet) WriteWhitelist(version storj.IDVersion) (string, error) {
	whitelistPath := filepath.Join(planet.directory, "whitelist.pem")
	signer := testidentity.NewPregeneratedSigner(version)
	err := identity.PeerCAConfig{
		CertPath: whitelistPath,
	}.Save(signer.PeerCA())

	return whitelistPath, err
}
