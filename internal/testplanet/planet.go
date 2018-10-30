// planet implements the full network wiring for testing
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
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	pieceserver "storj.io/storj/pkg/piecestore/rpc/server"
	"storj.io/storj/pkg/piecestore/rpc/server/psdb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/pkg/utils"
	"storj.io/storj/storage/teststore"
)

type Planet struct {
	directory string // TODO: ensure that everything is in-memory to speed things up

	nodes     []pb.Node
	nodeLinks []string

	satellites   []*Node
	storageNodes []*Node
	uplinks      []*Node

	identities *Identities
}

func New(satelliteCount, storageNodeCount, uplinkCount int) (*Planet, error) {
	planet := &Planet{
		identities: pregeneratedIdentities.Clone(),
	}

	var err error
	planet.directory, err = ioutil.TempDir("", "planet")
	if err != nil {
		return nil, err
	}

	planet.satellites, err = planet.NewNodes(satelliteCount)
	if err != nil {
		return nil, utils.CombineErrors(err, planet.Shutdown())
	}

	planet.storageNodes, err = planet.NewNodes(storageNodeCount)
	if err != nil {
		return nil, utils.CombineErrors(err, planet.Shutdown())
	}

	planet.uplinks, err = planet.NewNodes(uplinkCount)
	if err != nil {
		return nil, utils.CombineErrors(err, planet.Shutdown())
	}

	for _, node := range planet.allNodes() {
		err := node.InitOverlay(planet)
		if err != nil {
			return nil, utils.CombineErrors(err, planet.Shutdown())
		}
	}

	// init satellites
	for _, node := range planet.satellites {
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
	for _, node := range planet.storageNodes {
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

	// init uplinks

	return planet, nil
}

func (planet *Planet) allNodes() []*Node {
	var all []*Node
	all = append(all, planet.satellites...)
	all = append(all, planet.storageNodes...)
	all = append(all, planet.uplinks...)
	return all
}

func (planet *Planet) Start(ctx context.Context) {
	for _, node := range planet.allNodes() {
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

func (planet *Planet) Shutdown() error {
	var errs []error
	for _, node := range planet.allNodes() {
		errs = append(errs, node.Shutdown())
	}
	errs = append(errs, os.RemoveAll(planet.directory))
	return utils.CombineErrors(errs...)
}

func (planet *Planet) NewNode() (*Node, error) {
	identity, err := planet.NewIdentity()
	if err != nil {
		return nil, err
	}

	listener, err := planet.NewListener()
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

	planet.nodes = append(planet.nodes, node.Info)
	planet.nodeLinks = append(planet.nodeLinks, node.Info.Id+":"+node.Listener.Addr().String())

	return node, nil
}

func (planet *Planet) NewNodes(count int) ([]*Node, error) {
	var xs []*Node
	for i := 0; i < count; i++ {
		node, err := planet.NewNode()
		if err != nil {
			return nil, err
		}

		xs = append(xs, node)
	}

	return xs, nil
}

func (node *Node) InitOverlay(planet *Planet) error {
	routing, err := kademlia.NewRoutingTable(node.Info, teststore.New(), teststore.New())
	if err != nil {
		return err
	}

	kad, err := kademlia.NewKademliaWithRoutingTable(node.Info, planet.nodes, node.Identity, 5, routing)
	if err != nil {
		return utils.CombineErrors(err, routing.Close())
	}

	node.Kademlia = kad
	node.Overlay = overlay.NewOverlayCache(teststore.New(), node.Kademlia)

	return nil
}

func (planet *Planet) NewIdentity() (*provider.FullIdentity, error) {
	return planet.identities.NewIdentity()
}

func (planet *Planet) NewListener() (net.Listener, error) {
	return net.Listen("tcp", "127.0.0.1:0")
}

func (planet *Planet) Satellite(index int) *Node { return planet.satellites[index] }
func (planet *Planet) SatelliteCount() int       { return len(planet.satellites) }

func (planet *Planet) StorageNode(index int) *Node { return planet.storageNodes[index] }
func (planet *Planet) StorageNodeCount() int       { return len(planet.storageNodes) }

func (planet *Planet) Uplink(index int) *Node { return planet.uplinks[index] }
func (planet *Planet) UplinkCount() int       { return len(planet.uplinks) }
