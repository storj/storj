// planet implements the full network wiring for testing
package testplanet

import (
	"context"
	"io/ioutil"
	"net"
	"os"

	"storj.io/storj/pkg/auth/grpcauth"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/utils"
	"storj.io/storj/storage/teststore"
)

type Planet struct {
	Directory string // TODO: ensure that everything is in-memory to speed things up

	Nodes     []pb.Node
	NodeLinks []string

	Satellites   []*Node
	StorageNodes []*Node

	identities *Identities
}

type Node struct {
	Info     pb.Node
	Identity *provider.FullIdentity
	Listener net.Listener
	Provider *provider.Provider
	Kademlia *kademlia.Kademlia

	Responsibilites []provider.Responsibility
}

func (node *Node) ID() string   { return node.Info.Id }
func (node *Node) Addr() string { return node.Info.Address.Address }

func (node *Node) Shutdown() error {
	var errs []error
	if node.Provider != nil {
		errs = append(errs, node.Provider.Close())
	}
	if node.Listener != nil {
		errs = append(errs, node.Listener.Close())
	}
	if node.Kademlia != nil {
		errs = append(errs, node.Kademlia.Disconnect())
	}
	return utils.CombineErrors(errs...)
}

func New(satelliteCount, storageNodeCount int) (*Planet, error) {
	planet := &Planet{
		identities: pregeneratedIdentities.Clone(),
	}

	var err error
	planet.Directory, err = ioutil.TempDir("", "planet")
	if err != nil {
		return nil, err
	}

	planet.Satellites, err = planet.NewNodes(satelliteCount)
	if err != nil {
		return nil, utils.CombineErrors(err, planet.Shutdown())
	}

	planet.StorageNodes, err = planet.NewNodes(storageNodeCount)
	if err != nil {
		return nil, utils.CombineErrors(err, planet.Shutdown())
	}

	for _, node := range planet.allNodes() {
		err := node.InitOverlay(planet)
		if err != nil {
			return nil, utils.CombineErrors(err, planet.Shutdown())
		}
	}

	return planet, nil
}

func (planet *Planet) allNodes() []*Node {
	var all []*Node
	all = append(all, planet.Satellites...)
	all = append(all, planet.StorageNodes...)
	return all
}

func (planet *Planet) Start(ctx context.Context) error {
	return nil
}

func (planet *Planet) Shutdown() error {
	var errs []error
	for _, node := range planet.allNodes() {
		errs = append(errs, node.Shutdown())
	}
	errs = append(errs, os.RemoveAll(planet.Directory))
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

	planet.Nodes = append(planet.Nodes, node.Info)
	planet.NodeLinks = append(planet.NodeLinks, node.Info.Id+":"+node.Listener.Addr().String())

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
	routing, err := kademlia.NewRoutingTable(node.Info, teststore.New(), teststore.New(),
		&kademlia.RoutingOptions{
			IDLength:     16,
			BucketSize:   6,
			RCBucketSize: 2,
		},
	)
	if err != nil {
		return err
	}

	kad, err := kademlia.NewKademliaWithRoutingTable(node.Info, planet.Nodes, node.Identity, 5, routing)
	if err != nil {
		return utils.CombineErrors(err, routing.Close())
	}

	node.Kademlia = kad
	return nil
}

func (planet *Planet) NewIdentity() (*provider.FullIdentity, error) {
	return planet.identities.NewIdentity()
}

func (planet *Planet) NewListener() (net.Listener, error) {
	return net.Listen("tcp", "127.0.0.1:0")
}

func (planet *Planet) Satellite(index int) *Node { return planet.Satellites[index] }
func (planet *Planet) SatelliteCount() int       { return len(planet.Satellites) }

func (planet *Planet) StorageNode(index int) *Node { return planet.StorageNodes[index] }
func (planet *Planet) StorageNodeCount() int       { return len(planet.StorageNodes) }
