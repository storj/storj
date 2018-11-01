// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

// testplanet implements the full network wiring for testing
package testplanet

import (
	"context"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/auth/grpcauth"
	"storj.io/storj/pkg/pb"
	pieceserver "storj.io/storj/pkg/piecestore/rpc/server"
	"storj.io/storj/pkg/piecestore/rpc/server/psdb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/pkg/utils"
	"storj.io/storj/storage/teststore"
)

// Planet is a full storj system setup.
type Planet struct {
	directory string // TODO: ensure that everything is in-memory to speed things up

	nodeInfos []pb.Node
	nodeLinks []string

	nodes        []*Node
	Satellites   []*Node // DO NOT MODIFY
	StorageNodes []*Node // DO NOT MODIFY
	Uplinks      []*Node // DO NOT MODIFY

	identities *Identities
}

// New creates a new full sytem with the given number of nodes.
func New(satelliteCount, storageNodeCount, uplinkCount int) (*Planet, error) {
	planet := &Planet{
		identities: pregeneratedIdentities.Clone(),
	}

	var err error
	planet.directory, err = ioutil.TempDir("", "planet")
	if err != nil {
		return nil, err
	}

	planet.Satellites, err = planet.newNodes(satelliteCount)
	if err != nil {
		return nil, utils.CombineErrors(err, planet.Shutdown())
	}

	planet.StorageNodes, err = planet.newNodes(storageNodeCount)
	if err != nil {
		return nil, utils.CombineErrors(err, planet.Shutdown())
	}

	planet.Uplinks, err = planet.newNodes(uplinkCount)
	if err != nil {
		return nil, utils.CombineErrors(err, planet.Shutdown())
	}

	for _, node := range planet.nodes {
		err := node.initOverlay(planet)
		if err != nil {
			return nil, utils.CombineErrors(err, planet.Shutdown())
		}
	}

	// init Satellites
	for _, node := range planet.Satellites {
		server := pointerdb.NewServer(
			teststore.New(), node.Overlay,
			zap.NewNop(),
			pointerdb.Config{
				MinRemoteSegmentSize: 1240,
				MaxInlineSegmentSize: 8000,
				Overlay:              true,
			},
			node.Identity)
		pb.RegisterPointerDBServer(node.Provider.GRPC(), server)
		node.Dependencies = append(node.Dependencies, server)
	}

	// init storage nodes
	for _, node := range planet.StorageNodes {
		storageDir := filepath.Join(planet.directory, node.ID())

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
		node.Dependencies = append(node.Dependencies, server)
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
}

// Shutdown shuts down all the nodes and deletes temporary directories.
func (planet *Planet) Shutdown() error {
	var errs []error
	// shutdown in reverse order
	for i := len(planet.nodes) - 1; i >= 0; i-- {
		node := planet.nodes[i]
		errs = append(errs, node.Shutdown())
	}
	errs = append(errs, os.RemoveAll(planet.directory))
	return utils.CombineErrors(errs...)
}

// newNode creates a new node.
func (planet *Planet) newNode() (*Node, error) {
	identity, err := planet.newIdentity()
	if err != nil {
		return nil, err
	}

	listener, err := planet.newListener()
	if err != nil {
		return nil, err
	}

	node := &Node{
		Identity: identity,
		Listener: listener,
	}

	node.Transport = transport.NewClient(identity)

	node.Provider, err = provider.NewProvider(node.Identity, node.Listener, grpcauth.NewAPIKeyInterceptor())
	if err != nil {
		return nil, utils.CombineErrors(err, listener.Close())
	}

	node.Info = pb.Node{
		Id: node.Identity.ID.String(),
		Address: &pb.NodeAddress{
			Transport: pb.NodeTransport_TCP_TLS_GRPC,
			Address:   node.Listener.Addr().String(),
		},
	}

	planet.nodes = append(planet.nodes, node)
	planet.nodeInfos = append(planet.nodeInfos, node.Info)
	planet.nodeLinks = append(planet.nodeLinks, node.Info.Id+":"+node.Listener.Addr().String())

	return node, nil
}

// newNodes creates initializes multiple nodes
func (planet *Planet) newNodes(count int) ([]*Node, error) {
	var xs []*Node
	for i := 0; i < count; i++ {
		node, err := planet.newNode()
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
