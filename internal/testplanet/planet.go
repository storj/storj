// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

// Package testplanet implements the full network wiring for testing
package testplanet

import (
	"context"
	"errors"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	pieceserver "storj.io/storj/pkg/piecestore/psserver"
	"storj.io/storj/pkg/piecestore/psserver/psdb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/utils"
	"storj.io/storj/storage/teststore"
)

// Planet is a full storj system setup.
type Planet struct {
	log       *zap.Logger
	directory string // TODO: ensure that everything is in-memory to speed things up
	started   bool

	nodeInfos []pb.Node
	nodeLinks []string
	nodes     []*Node

	Satellites   []*Node
	StorageNodes []*Node
	Uplinks      []*Node

	identities *Identities
}

// New creates a new full system with the given number of nodes.
func New(t zaptest.TestingT, satelliteCount, storageNodeCount, uplinkCount int) (*Planet, error) {
	var log *zap.Logger
	if t == nil {
		log = zap.NewNop()
	} else {
		log = zaptest.NewLogger(t)
	}

	planet := &Planet{
		log:        log,
		identities: pregeneratedIdentities.Clone(),
	}

	var err error
	planet.directory, err = ioutil.TempDir("", "planet")
	if err != nil {
		return nil, err
	}

	planet.Satellites, err = planet.newNodes("satellite", satelliteCount, pb.NodeType_ADMIN)
	if err != nil {
		return nil, utils.CombineErrors(err, planet.Shutdown())
	}

	planet.StorageNodes, err = planet.newNodes("storage", storageNodeCount, pb.NodeType_STORAGE)
	if err != nil {
		return nil, utils.CombineErrors(err, planet.Shutdown())
	}

	planet.Uplinks, err = planet.newNodes("uplink", uplinkCount, pb.NodeType_ADMIN) // TODO: fix the node type here
	if err != nil {
		return nil, utils.CombineErrors(err, planet.Shutdown())
	}

	for _, node := range planet.nodes {
		err = node.initOverlay(planet)
		if err != nil {
			return nil, utils.CombineErrors(err, planet.Shutdown())
		}
	}

	for _, n := range planet.nodes {
		server := node.NewServer(n.Kademlia)
		pb.RegisterNodesServer(n.Provider.GRPC(), server)
		// TODO: shutdown
	}

	// init Satellites
	for _, node := range planet.Satellites {
		pointerServer := pointerdb.NewServer(
			teststore.New(), node.Overlay,
			node.Log.Named("pdb"),
			pointerdb.Config{
				MinRemoteSegmentSize: 1240,
				MaxInlineSegmentSize: 8000,
				Overlay:              true,
			},
			node.Identity)
		pb.RegisterPointerDBServer(node.Provider.GRPC(), pointerServer)
		// bootstrap satellite kademlia node
		go func(n *Node) {
			if err := n.Kademlia.Bootstrap(context.Background()); err != nil {
				log.Error(err.Error())
			}
		}(node)

		overlayServer := overlay.NewServer(node.Log.Named("overlay"), node.Overlay, node.Kademlia)
		pb.RegisterOverlayServer(node.Provider.GRPC(), overlayServer)

		node.Dependencies = append(node.Dependencies,
			closerFunc(func() error {
				// TODO: implement
				return nil
			}))

		go func(n *Node) {
			// refresh the interval every 500ms
			t := time.NewTicker(500 * time.Millisecond).C
			for {
				<-t
				if err := n.Overlay.Refresh(context.Background()); err != nil {
					log.Error(err.Error())
				}
			}
		}(node)
	}

	// init storage nodes
	for _, node := range planet.StorageNodes {
		storageDir := filepath.Join(planet.directory, node.ID().String())

		serverdb, err := psdb.OpenInMemory(context.Background(), storageDir)
		if err != nil {
			return nil, utils.CombineErrors(err, planet.Shutdown())
		}

		server := pieceserver.New(storageDir, serverdb, pieceserver.Config{
			Path:               storageDir,
			AllocatedDiskSpace: memory.GB.Int64(),
			AllocatedBandwidth: 100 * memory.GB.Int64(),
		}, node.Identity.Key)

		pb.RegisterPieceStoreRoutesServer(node.Provider.GRPC(), server)

		node.Dependencies = append(node.Dependencies,
			closerFunc(func() error {
				return server.Stop(context.Background())
			}))
		// bootstrap all the kademlia nodes
		go func(n *Node) {
			if err := n.Kademlia.Bootstrap(context.Background()); err != nil {
				log.Error(err.Error())
			}
		}(node)
	}

	// init Uplinks
	for _, uplink := range planet.Uplinks {
		// TODO: do we need here anything?
		_ = uplink
	}

	return planet, nil
}

// Start starts all the nodes.
func (planet *Planet) Start(ctx context.Context) {
	for _, node := range planet.nodes {
		go func(node *Node) {
			err := node.Provider.Run(ctx)
			if err == grpc.ErrServerStopped {
				err = nil
			}
			if err != nil {
				// TODO: better error handling
				panic(err)
			}
		}(node)
	}
	planet.started = true
}

// Shutdown shuts down all the nodes and deletes temporary directories.
func (planet *Planet) Shutdown() error {
	var errs []error
	if !planet.started {
		errs = append(errs, errors.New("Start was never called"))
	}

	// shutdown in reverse order
	for i := len(planet.nodes) - 1; i >= 0; i-- {
		node := planet.nodes[i]
		errs = append(errs, node.Shutdown())
	}
	errs = append(errs, os.RemoveAll(planet.directory))
	return utils.CombineErrors(errs...)
}

// newNodes creates initializes multiple nodes
func (planet *Planet) newNodes(prefix string, count int, nodeType pb.NodeType) ([]*Node, error) {
	var xs []*Node
	for i := 0; i < count; i++ {
		node, err := planet.newNode(prefix+strconv.Itoa(i), nodeType)
		if err != nil {
			return nil, err
		}
		xs = append(xs, node)
	}

	return xs, nil
}

// newIdentity creates a new identity for a node
func (planet *Planet) newIdentity() (*provider.FullIdentity, error) {
	return planet.identities.NewIdentity()
}

// newListener creates a new listener
func (planet *Planet) newListener() (net.Listener, error) {
	return net.Listen("tcp", "127.0.0.1:0")
}
