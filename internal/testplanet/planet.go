// planet implements the full network wiring for testing
package testplanet

import (
	"context"
	"net"

	"storj.io/storj/pkg/auth/grpcauth"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/provider"
)

const (
	kademliaAlpha = 5
)

type Planet struct {
	Nodes     []pb.Node
	NodeLinks []string

	Satellites   []*Node
	StorageNodes []*Node
}

type Node struct {
	Identity *provider.FullIdentity
	Listener net.Listener
	Provider *provider.Provider
	Kademlia *kademlia.Kademlia

	Responsibilites []provider.Responsibility
}

func (node *Node) Shutdown() error {
	return utils.CombineErrors(
		node.Listener.Close(),
		node.Kademlia.Shutdown(),
	)
}

func New(ctx context.Context, satelliteCount, storageNodeCount int) (*Planet, error) {
	planet := &Planet{Context: ctx}

	var err error
	planet.Satellites, err = planet.NewNodes(satelliteCount)
	if err != nil {
		return err
	}

	planet.StorageNodes, err = planet.NewNodes(storageNodes)
	if err != nil {
		return err
	}

	return planet, nil
}

func (planet *Planet) Start(ctx context.Context) error {
	
}

func (planet *Planet) NewNode() *Node {
	node := &Node{
		Identity: planet.NewIdentity(),
		Listener: planet.NewListener(),
	}
	node.Provider = provider.NewProvider(node.Identity, node.Listener, grpcauth.NewAPIKeyInterceptor())
	
	planet.Nodes = append(planet.Nodes, pb.Node{
		Id: node.Identity.String(),
		Address: &pb.NodeAddress{
			Transport: pb.NodeTransport_TCP_TLS_GRPC,
			Address: node.Listener.Addr().String(),
		},
	})

	planet.NodeLinks = append(planet.NodeLinks, node.Identity.String()+":"+node.Listener.Addr().String())
	
	return node
}

func (planet *Planet) NewNodes(count int) ([]*Node, error) {
	var xs []*Node
	for i := 0; i < count; i++ {
		node, err := planet.NewNode()
		if err != nil {
			errs := []error{err}
			for _, x := range xs {
				errs = append(errs, x.Shutdown())
			}
			return nil, utils.CombineErrors(errs...)
		}

		xs = append(xs, node)
	}

	return xs, nil
}

func (node *Node) InitOverlay(planet *Planet) error {
	self := pb.Node{
		Id: node.Identity.String(),
		Address: &pb.NodeAddress{Address: node.Listener.Addr().String()},
	}

	rt, err := NewRoutingTable(self, teststore.New(), teststore.New(),
		&RoutingOptions{
			IDLength:     16,
			BucketSize:   6,
			RCBucketSize: 2,
		},
	)
	if err != nil {
		return err
	}

	kad, err := kademlia.NewKademliaWithRoutingTable(self, planet.Nodes, 5, rt)
	if err != nil {
		return utils.CombineErrors(err, rt.Close()
	}

	node.Kademlia = kad
	return nil
}

func (planet *Planet) NewIdentity() (*provider.FullIdentity, error) {
	// TODO: create a static table
	ca, err := provider.NewCA(ctx, 14, 4)
	if err != nil {
		return nil, err
	}

	identity, err := ca.NewIdentity()
	if err != nil {
		return nil, err
	}

	return identity, nil
}

func (planet *Planet) NewListener() (net.Listener, error) {
	return net.Listen("127.0.0.1:0")
}

func (planet *Planet) Shutdown() error {
	return nil
}

func (planet *Planet) SatelliteAddr(index int) string {
	return ""
}

func (planet *Planet) SatelliteCount() int {
	return 0
}

func (planet *Planet) StorageNodeAddr(index int) string {
	return ""
}

func (planet *Planet) StorageNodeCount() int {
	return 0
}
