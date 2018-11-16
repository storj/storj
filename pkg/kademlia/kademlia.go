// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/golang/protobuf/proto"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"storj.io/storj/pkg/storj"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/utils"
	"storj.io/storj/storage"
	"storj.io/storj/storage/boltdb"
)

var (
	// NodeErr is the class for all errors pertaining to node operations
	NodeErr = errs.Class("node error")
	// BootstrapErr is the class for all errors pertaining to bootstrapping a node
	BootstrapErr = errs.Class("bootstrap node error")
	// NodeNotFound is returned when a lookup can not produce the requested node
	NodeNotFound = NodeErr.New("node not found")
	// TODO: shouldn't default to TCP but not sure what to do yet
	defaultTransport = pb.NodeTransport_TCP_TLS_GRPC
	defaultRetries   = 3
)

type discoveryOptions struct {
	concurrency int
	retries     int
	bootstrap   bool
}

// Kademlia is an implementation of kademlia adhering to the DHT interface.
type Kademlia struct {
	alpha          int // alpha is a system wide concurrency parameter
	routingTable   *RoutingTable
	bootstrapNodes []storj.Node
	address        string
	nodeClient     node.Client
	identity       *provider.FullIdentity
}

// NewKademlia returns a newly configured Kademlia instance
func NewKademlia(id storj.NodeID, bootstrapNodes []storj.Node, address string, identity *provider.FullIdentity, path string, alpha int) (*Kademlia, error) {
	self := storj.NewNodeWithID(id, &pb.Node{Address: &pb.NodeAddress{Address: address}})

	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0777); err != nil {
			return nil, err
		}
	}

	bucketIdentifier := id.String()[:5] // need a way to differentiate between nodes if running more than one simultaneously
	dbpath := filepath.Join(path, fmt.Sprintf("kademlia_%s.db", bucketIdentifier))

	dbs, err := boltdb.NewShared(dbpath, KademliaBucket, NodeBucket)
	if err != nil {
		return nil, BootstrapErr.Wrap(err)
	}
	kdb, ndb := dbs[0], dbs[1]

	rt, err := NewRoutingTable(self, kdb, ndb)
	if err != nil {
		return nil, BootstrapErr.Wrap(err)
	}

	return NewKademliaWithRoutingTable(self, bootstrapNodes, identity, alpha, rt)
}

// NewKademliaWithRoutingTable returns a newly configured Kademlia instance
func NewKademliaWithRoutingTable(self storj.Node, bootstrapNodes []storj.Node, identity *provider.FullIdentity, alpha int, rt *RoutingTable) (*Kademlia, error) {
	for _, v := range bootstrapNodes {
		ok, err := rt.addNode(v)
		if err != nil {
			return nil, err
		}
		if !ok {
			zap.L().Warn("Failed to add node", zap.String("NodeID", v.Id.String()))
		}
	}

	k := &Kademlia{
		alpha:          alpha,
		routingTable:   rt,
		bootstrapNodes: bootstrapNodes,
		address:        self.Address.Address,
		identity:       identity,
	}

	nc, err := node.NewNodeClient(identity, self, k)
	if err != nil {
		return nil, BootstrapErr.Wrap(err)
	}

	k.nodeClient = nc

	return k, nil
}

// Disconnect safely closes connections to the Kademlia network
func (k *Kademlia) Disconnect() error {
	return utils.CombineErrors(
		k.nodeClient.Disconnect(),
		k.routingTable.Close(),
	)
}

// GetNodes returns all nodes from a starting node up to a maximum limit
// stored in the local routing table limiting the result by the specified restrictions
func (k *Kademlia) GetNodes(ctx context.Context, start storj.NodeID, limit int, restrictions ...pb.Restriction) ([]storj.Node, error) {
	var nodes []storj.Node
	iteratorMethod := func(it storage.Iterator) error {
		var item storage.ListItem
		maxLimit := storage.LookupLimit
		for ; maxLimit > 0 && it.Next(&item); maxLimit-- {
			var (
				id = make([]byte, len(item.Key))
				v  = make([]byte, len(item.Value))
			)
			copy(id, item.Key)
			copy(v, item.Value)
			n := &pb.Node{}
			err := proto.Unmarshal(v, n)
			if err != nil {
				return Error.Wrap(err)
			}
			n.Id = id
			node, err := storj.NewNode(n)
			if err != nil {
				return err
			}
			if meetsRestrictions(restrictions, node) {
				nodes = append(nodes, node)
			}
			if len(nodes) == limit {
				return nil
			}
		}
		return nil
	}
	err := k.routingTable.iterate(
		storage.IterateOptions{
			First:   storage.Key(start.Bytes()),
			Recurse: true,
		},
		iteratorMethod,
	)
	if err != nil {
		return []storj.Node{}, Error.Wrap(err)
	}
	return nodes, nil
}

// GetRoutingTable provides the routing table for the Kademlia DHT
func (k *Kademlia) GetRoutingTable(ctx context.Context) (dht.RoutingTable, error) {
	return k.routingTable, nil
}

// Bootstrap contacts one of a set of pre defined trusted nodes on the network and
// begins populating the local Kademlia node
func (k *Kademlia) Bootstrap(ctx context.Context) error {
	// What I want to do here is do a normal lookup for myself
	// so call lookup(ctx, nodeImLookingFor)
	if len(k.bootstrapNodes) == 0 {
		return BootstrapErr.New("no bootstrap nodes provided")
	}

	nodeID, err := storj.NodeIDFromBytes(k.routingTable.self.GetId().Bytes())
	if err != nil {
		return BootstrapErr.Wrap(err)
	}
	return k.lookup(ctx, nodeID, discoveryOptions{
		concurrency: k.alpha, retries: defaultRetries, bootstrap: true,
	})
}

func (k *Kademlia) lookup(ctx context.Context, target storj.NodeID, opts discoveryOptions) error {
	kb := k.routingTable.K()
	// look in routing table for targetID
	nodes, err := k.routingTable.FindNear(target, kb)
	if err != nil {
		return err
	}

	lookup := newPeerDiscovery(nodes, k.nodeClient, target, opts)
	err = lookup.Run(ctx)
	if err != nil {
		zap.L().Warn("lookup failed", zap.Error(err))
	}

	return nil
}

// Ping checks that the provided node is still accessible on the network
func (k *Kademlia) Ping(ctx context.Context, node storj.Node) (storj.Node, error) {
	ok, err := k.nodeClient.Ping(ctx, node)
	if err != nil {
		return storj.Node{}, NodeErr.Wrap(err)
	}

	if !ok {
		return storj.Node{}, NodeErr.New("Failed pinging node")
	}

	return node, nil
}

// FindNode looks up the provided NodeID first in the local Node, and if it is not found
// begins searching the network for the NodeID. Returns and error if node was not found
func (k *Kademlia) FindNode(ctx context.Context, ID storj.NodeID) (storj.Node, error) {
	// TODO(coyle)
	return storj.Node{}, NodeErr.New("TODO FindNode")
}

// ListenAndServe connects the kademlia node to the network and listens for incoming requests
func (k *Kademlia) ListenAndServe() error {
	identOpt, err := k.identity.ServerOption()
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer(identOpt)
	mn := node.NewServer(k)

	pb.RegisterNodesServer(grpcServer, mn)
	lis, err := net.Listen("tcp", k.address)
	if err != nil {
		return err
	}
	if err := grpcServer.Serve(lis); err != nil {
		return err
	}
	defer grpcServer.Stop()

	return nil
}

// GetIntroNode determines the best node to bootstrap a new node onto the network
func GetIntroNode(addr string) (storj.Node, error) {
	if addr == "" {
		addr = "bootstrap.storj.io:8080"
	}

	return storj.Node{
		Id: storj.EmptyNodeID,
		Node: &pb.Node{
			Address: &pb.NodeAddress{
				Transport: defaultTransport,
				Address:   addr,
			},
		},
	}, nil
}

// Restrict is used to limit nodes returned that don't match the miniumum storage requirements
func Restrict(r pb.Restriction, n []storj.Node) []storj.Node {
	oper := r.GetOperand()
	op := r.GetOperator()
	val := r.GetValue()
	var comp int64

	results := []storj.Node{}
	for _, v := range n {
		switch oper {
		case pb.Restriction_freeBandwidth:
			comp = v.GetRestrictions().GetFreeBandwidth()
		case pb.Restriction_freeDisk:
			comp = v.GetRestrictions().GetFreeDisk()
		}

		switch op {
		case pb.Restriction_EQ:
			if comp == val {
				results = append(results, v)
				continue
			}
		case pb.Restriction_LT:
			if comp < val {
				results = append(results, v)
				continue
			}
		case pb.Restriction_LTE:
			if comp <= val {
				results = append(results, v)
				continue
			}
		case pb.Restriction_GT:
			if comp > val {
				results = append(results, v)
				continue
			}
		case pb.Restriction_GTE:
			if comp >= val {
				results = append(results, v)
				continue
			}

		}

	}

	return results
}

func meetsRestrictions(rs []pb.Restriction, n storj.Node) bool {
	for _, r := range rs {
		oper := r.GetOperand()
		op := r.GetOperator()
		val := r.GetValue()
		var comp int64
		switch oper {
		case pb.Restriction_freeBandwidth:
			comp = n.GetRestrictions().GetFreeBandwidth()
		case pb.Restriction_freeDisk:
			comp = n.GetRestrictions().GetFreeDisk()
		}
		switch op {
		case pb.Restriction_EQ:
			if comp != val {
				return false
			}
		case pb.Restriction_LT:
			if comp >= val {
				return false
			}
		case pb.Restriction_LTE:
			if comp > val {
				return false
			}
		case pb.Restriction_GT:
			if comp <= val {
				return false
			}
		case pb.Restriction_GTE:
			if comp < val {
				return false
			}
		}
	}
	return true
}
