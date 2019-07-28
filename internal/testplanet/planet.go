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
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/storagenode"
	"storj.io/storj/versioncontrol"
)

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
	Satellites     []*satellite.Peer
	StorageNodes   []*storagenode.Peer
	Uplinks        []*Uplink

	identities    *testidentity.Identities
	whitelistPath string // TODO: in-memory

	run    errgroup.Group
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
		whitelistedSatellites = append(whitelistedSatellites, satellite.URL())
	}

	planet.StorageNodes, err = planet.newStorageNodes(config.StorageNodeCount, whitelistedSatellites)
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
			satellite.Kademlia.Service.SetBootstrapNodes([]pb.Node{planet.Bootstrap.Local().Node})
		}
	}
	// init storage nodes
	for _, storageNode := range planet.StorageNodes {
		if len(storageNode.Kademlia.Service.GetBootstrapNodes()) == 0 {
			storageNode.Kademlia.Service.SetBootstrapNodes([]pb.Node{planet.Bootstrap.Local().Node})
		}
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

	planet.started = true

	planet.Bootstrap.Kademlia.Service.WaitForBootstrap()

	for _, peer := range planet.StorageNodes {
		peer.Kademlia.Service.WaitForBootstrap()
	}

	for _, peer := range planet.Satellites {
		peer.Kademlia.Service.WaitForBootstrap()
	}

	planet.Reconnect(ctx)
}

// Reconnect reconnects all nodes with each other.
func (planet *Planet) Reconnect(ctx context.Context) {
	log := planet.log.Named("reconnect")

	var group errgroup.Group

	// TODO: instead of pinging try to use Lookups or natural discovery to ensure
	// everyone finds everyone else

	for _, storageNode := range planet.StorageNodes {
		storageNode := storageNode
		group.Go(func() error {
			_, err := storageNode.Kademlia.Service.Ping(ctx, planet.Bootstrap.Local().Node)
			if err != nil {
				log.Error("storage node did not find bootstrap", zap.Error(err))
			}
			return nil
		})
	}

	for _, satellite := range planet.Satellites {
		satellite := satellite
		group.Go(func() error {
			for _, storageNode := range planet.StorageNodes {
				_, err := satellite.Kademlia.Service.Ping(ctx, storageNode.Local().Node)
				if err != nil {
					log.Error("satellite did not find storage node", zap.Error(err))
				}
			}
			return nil
		})
	}

	_ = group.Wait() // none of the goroutines return an error
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
