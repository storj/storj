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
	"golang.org/x/sync/errgroup"

	"storj.io/common/identity"
	"storj.io/common/identity/testidentity"
	"storj.io/common/storj"
	"storj.io/storj/pkg/server"
	"storj.io/storj/private/dbutil/pgutil"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/versioncontrol"
)

const defaultInterval = 15 * time.Second

// Peer represents one of StorageNode or Satellite
type Peer interface {
	ID() storj.NodeID
	Addr() string
	URL() string
	NodeURL() storj.NodeURL
	Local() overlay.NodeDossier

	Run(context.Context) error
	Close() error
}

// Config describes planet configuration
type Config struct {
	SatelliteCount   int
	StorageNodeCount int
	UplinkCount      int

	IdentityVersion *storj.IDVersion
	Reconfigure     Reconfigure

	Name        string
	NonParallel bool
}

// DatabaseConfig defines connection strings for database.
type DatabaseConfig struct {
	SatelliteDB        string
	SatellitePointerDB string
}

// Planet is a full storj system setup.
type Planet struct {
	id        string
	log       *zap.Logger
	config    Config
	directory string // TODO: ensure that everything is in-memory to speed things up

	started  bool
	shutdown bool

	peers     []closablePeer
	databases []io.Closer
	uplinks   []*Uplink

	VersionControl *versioncontrol.Peer
	Satellites     []*Satellite
	StorageNodes   []*StorageNode
	Uplinks        []*Uplink

	ReferralManager *server.Server

	identities    *testidentity.Identities
	whitelistPath string // TODO: in-memory

	run    errgroup.Group
	cancel func()
}

type closablePeer struct {
	peer Peer

	ctx         context.Context
	cancel      func()
	runFinished chan struct{} // it is closed after peer.Run returns

	close sync.Once
	err   error
}

func newClosablePeer(peer Peer) closablePeer {
	return closablePeer{
		peer:        peer,
		runFinished: make(chan struct{}),
	}
}

// Close closes safely the peer.
func (peer *closablePeer) Close() error {
	peer.cancel()

	peer.close.Do(func() {
		<-peer.runFinished // wait for Run to complete
		peer.err = peer.peer.Close()
	})

	return peer.err
}

// NewCustom creates a new full system with the specified configuration.
func NewCustom(log *zap.Logger, config Config, satelliteDatabases satellitedbtest.SatelliteDatabases) (*Planet, error) {
	if config.IdentityVersion == nil {
		version := storj.LatestIDVersion()
		config.IdentityVersion = &version
	}

	planet := &Planet{
		log:    log,
		id:     config.Name + "/" + pgutil.CreateRandomTestingSchemaName(6),
		config: config,
	}

	if config.Reconfigure.Identities != nil {
		planet.identities = config.Reconfigure.Identities(log, *config.IdentityVersion)
	} else {
		planet.identities = testidentity.NewPregeneratedSignedIdentities(*config.IdentityVersion)
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

	planet.ReferralManager, err = planet.newReferralManager()
	if err != nil {
		return nil, errs.Combine(err, planet.Shutdown())
	}

	planet.Satellites, err = planet.newSatellites(config.SatelliteCount, satelliteDatabases)
	if err != nil {
		return nil, errs.Combine(err, planet.Shutdown())
	}

	whitelistedSatellites := make(storj.NodeURLs, 0, len(planet.Satellites))
	for _, satellite := range planet.Satellites {
		whitelistedSatellites = append(whitelistedSatellites, satellite.NodeURL())
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

	if planet.ReferralManager != nil {
		planet.run.Go(func() error {
			return planet.ReferralManager.Run(ctx)
		})
	}

	for i := range planet.peers {
		peer := &planet.peers[i]
		peer.ctx, peer.cancel = context.WithCancel(ctx)
		planet.run.Go(func() error {
			defer close(peer.runFinished)

			err := peer.peer.Run(peer.ctx)
			return err
		})
	}

	var group errgroup.Group
	for _, peer := range planet.StorageNodes {
		peer := peer
		group.Go(func() error {
			peer.Storage2.Monitor.Loop.TriggerWait()
			peer.Contact.Chore.TriggerWait(ctx)
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

// FindNode is a helper to retrieve a storage node record by its node ID.
func (planet *Planet) FindNode(nodeID storj.NodeID) *StorageNode {
	for _, node := range planet.StorageNodes {
		if node.ID() == nodeID {
			return node
		}
	}
	return nil
}

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

	for i := len(planet.databases) - 1; i >= 0; i-- {
		db := planet.databases[i]
		errlist.Add(db.Close())
	}

	if planet.ReferralManager != nil {
		errlist.Add(planet.ReferralManager.Close())
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
