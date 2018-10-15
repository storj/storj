// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
)

// NodeErr is the class for all errors pertaining to node operations
var NodeErr = errs.Class("node error")

// BootstrapErr is the class for all errors pertaining to bootstrapping a node
var BootstrapErr = errs.Class("bootstrap node error")

//TODO: shouldn't default to TCP but not sure what to do yet
var defaultTransport = pb.NodeTransport_TCP_TLS_GRPC

// NodeNotFound is returned when a lookup can not produce the requested node
var NodeNotFound = NodeErr.New("node not found")

type lookupOpts struct {
	amount    int
	bootstrap bool
}

// Kademlia is an implementation of kademlia adhering to the DHT interface.
type Kademlia struct {
	alpha          int // alpha is a system wide concurrency parameter
	routingTable   *RoutingTable
	bootstrapNodes []pb.Node
	address        string
	nodeClient     node.Client
	identity       *provider.FullIdentity
}

// NewKademlia returns a newly configured Kademlia instance
func NewKademlia(id dht.NodeID, bootstrapNodes []pb.Node, address string, identity *provider.FullIdentity, path string, kadconfig KadConfig) (*Kademlia, error) {
	self := pb.Node{Id: id.String(), Address: &pb.NodeAddress{Address: address}}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0777); err != nil {
			return nil, err
		}
	}
	bucketIdentifier := id.String()[:5] // need a way to differentiate between nodes if running more than one simultaneously
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

	for _, v := range bootstrapNodes {
		ok, err := rt.addNode(&v)
		if err != nil {
			return nil, err
		}
		if !ok {
			zap.L().Warn("Failed to add node", zap.String("NodeID", v.Id))
		}
	}

	k := &Kademlia{
		alpha:          kadconfig.Alpha,
		routingTable:   rt,
		bootstrapNodes: bootstrapNodes,
		address:        address,
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
	// TODO(coyle)
	return errors.New("TODO Disconnect")
}

// GetNodes returns all nodes from a starting node up to a maximum limit
// stored in the local routing table limiting the result by the specified restrictions
func (k *Kademlia) GetNodes(ctx context.Context, start string, limit int, restrictions ...pb.Restriction) ([]*pb.Node, error) {
	// TODO(coyle)
	return []*pb.Node{}, errors.New("TODO GetNodes")
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

	return k.lookup(ctx, node.IDFromString(k.routingTable.self.GetId()), lookupOpts{amount: 5, bootstrap: true})
}

func (k *Kademlia) lookup(ctx context.Context, target dht.NodeID, opts lookupOpts) error {
	kb := k.routingTable.K()
	// look in routing table for targetID
	nodes, err := k.routingTable.FindNear(target, kb)
	if err != nil {
		return err
	}

	lookup := newSequentialLookup(k.routingTable, nodes, k.nodeClient, target, opts.amount, opts.bootstrap)
	err = lookup.Run(ctx)
	if err != nil {
		zap.L().Warn("lookup failed", zap.Error(err))
	}

	return nil
}

// Ping checks that the provided node is still accessible on the network
func (k *Kademlia) Ping(ctx context.Context, node pb.Node) (pb.Node, error) {
	// TODO(coyle)
	return pb.Node{}, nil
}

// FindNode looks up the provided NodeID first in the local Node, and if it is not found
// begins searching the network for the NodeID. Returns and error if node was not found
func (k *Kademlia) FindNode(ctx context.Context, ID dht.NodeID) (pb.Node, error) {
	//TODO(coyle)
	return pb.Node{}, NodeErr.New("TODO FindNode")
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
func GetIntroNode(addr string) (*pb.Node, error) {
	if addr == "" {
		addr = "bootstrap.storj.io:8080"
	}

	return &pb.Node{
		Address: &pb.NodeAddress{
			Transport: defaultTransport,
			Address:   addr,
		},
	}, nil
}

// Restrict is used to limit nodes returned that don't match the miniumum storage requirements
func Restrict(r pb.Restriction, n []*pb.Node) []*pb.Node {
	oper := r.GetOperand()
	op := r.GetOperator()
	val := r.GetValue()
	var comp int64

	results := []*pb.Node{}
	for _, v := range n {
		switch oper {
		case pb.Restriction_freeBandwidth:
			comp = v.GetRestrictions().GetFreeBandwidth()
		case pb.Restriction_freeDisk:
			comp = v.GetRestrictions().GetFreeDisk()
		}

		switch op {
		case pb.Restriction_EQ:
			if comp != val {
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
