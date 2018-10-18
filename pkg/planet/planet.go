package planet

import (
	"context"
	"net"

	"storj.io/storj/pkg/auth/grpcauth"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/provider"
)

type Planet struct {
	Context      context.Context
	
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

func New(ctx context.Context, satelliteCount, storageNodeCount int) *Planet {
	planet := &Planet{Context: ctx}

	planet.Satellites = planet.NewNodes(satelliteCount)
	planet.StorageNodes = planet.NewNodes(storageNodes)

	return planet
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
	}
	planet.NodeLinks = append(planet.NodeLinks, node.Identity.String()+":"+node.Listener.Addr().String())
	
	return node
}

func (planet *Planet) NewNodes(count int) []*Node {
	var xs []*Node
	for i := 0; i < count; i++ {
		xs = append(xs, planet.NewNode())
	}
	return xs
}

func (planet *Planet) InitOverlay(node *Node) {
	kad, err := NewKademliaWithStore(
		node.Identity().ID, 
		node.Nodes,
		node.Listener.Addr().String(),
		teststore.New(),
	)
	node.Kademlia = 
}

	rt, err := NewRoutingTable(&self, &RoutingOptions{
		kpath:        filepath.Join(path, fmt.Sprintf("kbucket_%s.db", bucketIdentifier)),
		npath:        filepath.Join(path, fmt.Sprintf("nbucket_%s.db", bucketIdentifier)),
		idLength:     kadconfig.DefaultIDLength,
		bucketSize:   kadconfig.DefaultBucketSize,
		rcBucketSize: kadconfig.DefaultReplacementCacheSize,
	})

if err != nil {
	return nil, BootstrapErr.Wrap(err)
}


func (planet *Planet) NewIdentity() *provider.FullIdentity {
	// TODO: create a static table
	ca, err := provider.NewCA(ctx, 14, 4)
	if err != nil {
		panic(err)
	}

	identity, err := ca.NewIdentity()
	if err != nil {
		panic(err)
	}

	return identity
}

func (planet *Planet) NewListener() net.Listener {
	listener, err := net.Listen("127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	return listener
}

func (planet *Planet) Shutdown() {}

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
