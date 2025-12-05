// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/identity"
	"storj.io/common/identity/testidentity"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/versioncontrol"
)

var mon = monkit.Package()

const defaultInterval = 15 * time.Second

// Peer represents one of StorageNode or Satellite.
type Peer interface {
	Label() string

	ID() storj.NodeID
	Addr() string
	URL() string
	NodeURL() storj.NodeURL

	Run(context.Context) error
	Close() error
}

// Config describes planet configuration.
type Config struct {
	SatelliteCount   int
	StorageNodeCount int
	UplinkCount      int
	MultinodeCount   int

	IdentityVersion *storj.IDVersion
	LastNetFunc     overlay.LastNetFunc
	Reconfigure     Reconfigure

	Name        string
	Host        string
	NonParallel bool
	Timeout     time.Duration

	applicationName string

	// SkipSpanner is a flag used to tell tests to skip Spanner tests.
	SkipSpanner bool
	// ExerciseJobq is a flag used to tell tests to exercise the jobq implementation of the repair queue.
	ExerciseJobq bool
}

// DatabaseConfig defines connection strings for database.
type DatabaseConfig struct {
	SatelliteDB string
}

// Planet is a full storj system setup.
type Planet struct {
	ctx       *testcontext.Context
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
	Multinodes     []*Multinode
	Uplinks        []*Uplink

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
func NewCustom(ctx *testcontext.Context, log *zap.Logger, config Config, satelliteDatabases satellitedbtest.SatelliteDatabases) (*Planet, error) {
	if config.IdentityVersion == nil {
		version := storj.LatestIDVersion()
		config.IdentityVersion = &version
	}

	if config.Host == "" {
		config.Host = "127.0.0.1"
		if hostlist := os.Getenv("STORJ_TEST_HOST"); hostlist != "" {
			hosts := strings.Split(hostlist, ";")
			config.Host = hosts[testrand.Intn(len(hosts))]
		}
	}

	if config.applicationName == "" {
		config.applicationName = "testplanet"
	}

	planet := &Planet{
		ctx:    ctx,
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
	planet.directory, err = os.MkdirTemp("", "planet")
	if err != nil {
		return nil, errs.Wrap(err)
	}

	whitelistPath, err := planet.WriteWhitelist(*config.IdentityVersion)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	planet.whitelistPath = whitelistPath

	err = planet.createPeers(ctx, satelliteDatabases)
	if err != nil {
		return nil, errs.Combine(err, planet.Shutdown())
	}
	return planet, nil
}

func (planet *Planet) createPeers(ctx context.Context, satelliteDatabases satellitedbtest.SatelliteDatabases) (err error) {
	planet.VersionControl, err = planet.newVersionControlServer()
	if err != nil {
		return errs.Wrap(err)
	}

	planet.Satellites, err = planet.newSatellites(ctx, planet.config.SatelliteCount, satelliteDatabases)
	if err != nil {
		return errs.Wrap(err)
	}

	whitelistedSatellites := make(storj.NodeURLs, 0, len(planet.Satellites))
	for _, satellite := range planet.Satellites {
		whitelistedSatellites = append(whitelistedSatellites, satellite.NodeURL())
	}

	planet.StorageNodes, err = planet.newStorageNodes(ctx, planet.config.StorageNodeCount, whitelistedSatellites)
	if err != nil {
		return errs.Wrap(err)
	}

	planet.Multinodes, err = planet.newMultinodes(ctx, "multinode", planet.config.MultinodeCount)
	if err != nil {
		return errs.Wrap(err)
	}

	planet.Uplinks, err = planet.newUplinks(ctx, "uplink", planet.config.UplinkCount)
	if err != nil {
		return errs.Wrap(err)
	}

	for _, satellite := range planet.Satellites {
		for _, node := range planet.StorageNodes {
			if err := checkInManually(ctx, satellite, node); err != nil {
				return errs.Wrap(err)
			}
		}
	}

	return nil
}

func checkInManually(ctx context.Context, satellite *Satellite, node *StorageNode) error {
	err := satellite.DB.PeerIdentities().Set(ctx, node.ID(), node.Identity.PeerIdentity())
	if err != nil {
		return errs.Wrap(err)
	}

	_, _, lastNet, err := satellite.Overlay.Service.ResolveIPAndNetwork(ctx, node.Addr())
	if err != nil {
		return errs.Wrap(err)
	}
	availableSpace, err := node.Storage2.Monitor.AvailableSpace(ctx)
	if err != nil {
		return errs.Wrap(err)
	}
	self := node.Contact.Service.Local()
	return satellite.DB.OverlayCache().UpdateCheckIn(ctx, overlay.NodeCheckInInfo{
		NodeID:     node.ID(),
		Address:    &pb.NodeAddress{Address: node.Addr()},
		IsUp:       true,
		Version:    &self.Version,
		LastNet:    lastNet,
		LastIPPort: node.Addr(),
		Capacity: &pb.NodeCapacity{
			FreeDisk: availableSpace,
		},
		Operator: &pb.NodeOperator{
			Email:  node.Config.Operator.Email,
			Wallet: node.Config.Operator.Wallet,
		},
	}, time.Now(), satellite.Config.Overlay.Node)
}

// Start starts all the nodes.
func (planet *Planet) Start(ctx context.Context) error {
	defer mon.Task()(&ctx)(nil)

	ctx, cancel := context.WithCancel(ctx)
	planet.cancel = cancel

	pprof.Do(ctx, pprof.Labels("peer", "version-control"), func(ctx context.Context) {
		planet.run.Go(func() error {
			return planet.VersionControl.Run(ctx)
		})
	})

	for i := range planet.peers {
		peer := &planet.peers[i]
		peer.ctx, peer.cancel = context.WithCancel(ctx)
		pprof.Do(peer.ctx, pprof.Labels("peer", peer.peer.Label()), func(ctx context.Context) {
			planet.run.Go(func() error {
				defer close(peer.runFinished)

				err := peer.peer.Run(ctx)
				return err
			})
		})
	}

	var group errgroup.Group
	for _, peer := range planet.StorageNodes {
		peer := peer
		pprof.Do(ctx, pprof.Labels("peer", peer.Label(), "startup", "contact"), func(ctx context.Context) {
			group.Go(func() error {
				peer.Storage2.Monitor.Loop.TriggerWait()
				return nil
			})
		})
	}

	if err := group.Wait(); err != nil {
		return err
	}

	planet.started = true
	return nil
}

// StopPeer stops a single peer in the planet.
func (planet *Planet) StopPeer(peer Peer) error {
	if peer == nil {
		return errors.New("peer is nil")
	}
	for i := range planet.peers {
		p := &planet.peers[i]
		if p.peer == peer {
			return p.Close()
		}
	}
	return errors.New("unknown peer")
}

// StopNodeAndUpdate stops storage node and updates satellite overlay.
func (planet *Planet) StopNodeAndUpdate(ctx context.Context, node *StorageNode) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = planet.StopPeer(node)
	if err != nil {
		return err
	}

	for _, satellite := range planet.Satellites {
		err := satellite.DB.OverlayCache().UpdateCheckIn(ctx, overlay.NodeCheckInInfo{
			NodeID:  node.ID(),
			Address: &pb.NodeAddress{Address: node.Addr()},
			IsUp:    true,
			Version: &pb.NodeVersion{
				Version:    "v0.0.0",
				CommitHash: "",
				Timestamp:  time.Time{},
				Release:    false,
			},
		}, time.Now().Add(-4*time.Hour), satellite.Config.Overlay.Node)
		if err != nil {
			return err
		}
	}

	return nil
}

// Size returns number of nodes in the network.
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

// Log returns the root logger.
func (planet *Planet) Log() *zap.Logger { return planet.log }

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
		timer := time.NewTimer(10 * time.Second)
		defer timer.Stop()
		select {
		case <-timer.C:
			planet.log.Error("Planet took too long to shutdown\n" + planet.ctx.StackTrace())
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

	errlist.Add(planet.VersionControl.Close())

	errlist.Add(os.RemoveAll(planet.directory))

	var filtered errs.Group
	for _, err := range errlist {
		errmsg := err.Error()
		// workaround for not being able to catch context.Canceled error from net package
		if strings.Contains(errmsg, "operation was canceled") {
			continue
		}
		// workaround for not being able to catch context.Canceled from Spanner
		//
		// TODO(spanner): figure out why it's not possible to catch this earlier
		if strings.Contains(errmsg, "context canceled") {
			continue
		}
		filtered.Add(err)
	}

	return filtered.Err()
}

// Identities returns the identity provider for this planet.
func (planet *Planet) Identities() *testidentity.Identities {
	return planet.identities
}

// NewIdentity creates a new identity for a node.
func (planet *Planet) NewIdentity() (*identity.FullIdentity, error) {
	return planet.identities.NewIdentity()
}

// NewListenAddress returns an address for listening.
func (planet *Planet) NewListenAddress() string {
	return net.JoinHostPort(planet.config.Host, "0")
}

// NewListener creates a new listener.
func (planet *Planet) NewListener() (net.Listener, error) {
	return net.Listen("tcp", planet.NewListenAddress())
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
