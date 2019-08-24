package storagenode

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/satellite/overlay"
)

type NodeService struct {
	log        *zap.Logger
	self       *overlay.NodeDossier
	mu         sync.Mutex
	lastPinged time.Time
}

// NewNodeService returns a newly configured node service instance
func NewNodeService(log *zap.Logger, transport transport.Client, config Config) (*NodeService, error) {
	ns := &NodeService{
		log: log, //todo update
	}
	return ns, nil
}

// LastPinged returns last time someone pinged this node.
func (ns *NodeService) LastPinged() time.Time {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	return ns.lastPinged
}

// Pinged notifies the service it has been remotely pinged.
func (ns *NodeService) Pinged() {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	ns.lastPinged = time.Now()
}

// Local returns the local node
func (ns *NodeService) Local() overlay.NodeDossier {
	return *ns.self
}

// FetchPeerIdentity connects to a node and returns its peer identity
func (ns *NodeService) FetchPeerIdentity(ctx context.Context, nodeID storj.NodeID) (_ *identity.PeerIdentity, err error) {
	defer mon.Task()(&ctx)(&err)

	return k.dialer.FetchPeerIdentity(ctx, node)
}

// Ping checks that the provided node is still accessible on the network
func (k *Kademlia) Ping(ctx context.Context, node pb.Node) (_ pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)
	ok, err := k.dialer.PingNode(ctx, node)
	if err != nil {
		return pb.Node{}, NodeErr.Wrap(err)
	}
	if !ok {
		return pb.Node{}, NodeErr.New("%s : %s failed to ping node ID %s", k.routingTable.self.Type.String(), k.routingTable.self.Id.String(), node.Id.String())
	}
	return node, nil
}

// FetchInfo connects to a node address and returns the node info
func (k *Kademlia) FetchInfo(ctx context.Context, node pb.Node) (_ *pb.InfoResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	info, err := k.dialer.FetchInfo(ctx, node)
	if err != nil {
		return nil, NodeErr.Wrap(err)
	}
	return info, nil
}
