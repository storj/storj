// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"errors"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/provider"
	proto "storj.io/storj/protos/overlay"
)

const (
	alpha                       = 5
	defaultIDLength             = 256
	defaultBucketSize           = 20
	defaultReplacementCacheSize = 5
)

// NodeErr is the class for all errors pertaining to node operations
var NodeErr = errs.Class("node error")

// BootstrapErr is the class for all errors pertaining to bootstrapping a node
var BootstrapErr = errs.Class("bootstrap node error")

//TODO: shouldn't default to TCP but not sure what to do yet
var defaultTransport = proto.NodeTransport_TCP

// NodeNotFound is returned when a lookup can not produce the requested node
var NodeNotFound = NodeErr.New("node not found")

type lookupOpts struct {
	amount int
}

// Kademlia is an implementation of kademlia adhering to the DHT interface.
type Kademlia struct {
	routingTable   *RoutingTable
	bootstrapNodes []proto.Node
	address        string
	stun           bool
	nodeClient     node.Client
	alpha          int
}

// NewKademlia returns a newly configured Kademlia instance
func NewKademlia(id dht.NodeID, bootstrapNodes []proto.Node, address string) (*Kademlia, error) {
	self := proto.Node{Id: id.String(), Address: &proto.NodeAddress{Address: address}}
	rt, err := NewRoutingTable(&self, &RoutingOptions{
		kpath:        "kbucket.db",
		npath:        "nbucket.db",
		idLength:     defaultIDLength,
		bucketSize:   defaultBucketSize,
		rcBucketSize: defaultReplacementCacheSize,
	})
	if err != nil {
		return nil, BootstrapErr.Wrap(err)
	}

	for _, v := range bootstrapNodes {
		ok, err := rt.addNode(&v)
		if !ok || err != nil {
			return nil, err
		}
	}

	k := &Kademlia{
		alpha:          alpha,
		routingTable:   rt,
		bootstrapNodes: bootstrapNodes,
		address:        address,
		stun:           true,
	}

	ca, err := provider.NewCA(context.Background(), 12, 4)
	if err != nil {
		return nil, err
	}
	identity, err := ca.NewIdentity()
	if err != nil {
		return nil, err
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
	return errors.New("TODO")
}

// GetNodes returns all nodes from a starting node up to a maximum limit
// stored in the local routing table limiting the result by the specified restrictions
func (k *Kademlia) GetNodes(ctx context.Context, start string, limit int, restrictions ...proto.Restriction) ([]*proto.Node, error) {
	// TODO(coyle)
	return []*proto.Node{}, errors.New("TODO")
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

	_, err := k.lookup(ctx, node.StringToID(k.routingTable.self.GetId()), lookupOpts{amount: 5})
	return err
}

func (k *Kademlia) lookup(ctx context.Context, target dht.NodeID, opts lookupOpts) (*proto.Node, error) {
	conc := opts.amount
	kb := k.routingTable.K()
	if conc <= 0 {
		conc = kb
	}

	// look in routing table for targetID
	nodes, err := k.routingTable.FindNear(target, kb)
	if err != nil {
		return nil, err
	}

	// if we have the node in our routing table and that node is not this node (bootstrap) just return the node
	if len(nodes) == 1 {
		return nodes[0], nil
	}

	// begin the work looking for the node by spinning up alpha workers
	// and asking for the node we want from nodes we know in our routing table
	ch := make(chan []*proto.Node, conc)
	w := newWorker(ctx, k.routingTable, nodes, k.nodeClient, target, kb)
	ctx, w.cancel = context.WithCancel(ctx)
	defer w.cancel()
	for i := 0; i < conc; i++ {
		go w.work(ctx, ch)
	}

	for {
		select {
		case v := <-ch:
			for _, node := range v {
				if node.GetId() == target.String() {
					return node, nil
				}
			}
		case <-ctx.Done():
			return nil, NodeNotFound
		}
	}
}

// Ping checks that the provided node is still accessible on the network
func (k *Kademlia) Ping(ctx context.Context, node proto.Node) (proto.Node, error) {
	// TODO(coyle)
	return proto.Node{}, errors.New("TODO")
}

// FindNode looks up the provided NodeID first in the local Node, and if it is not found
// begins searching the network for the NodeID. Returns and error if node was not found
func (k *Kademlia) FindNode(ctx context.Context, ID dht.NodeID) (proto.Node, error) {
	//TODO(coyle)
	return proto.Node{}, NodeErr.New("TODO")
}

// ListenAndServe connects the kademlia node to the network and listens for incoming requests
func (k *Kademlia) ListenAndServe() error {
	//TODO(coyle)

	return errors.New("TODO")
}

// GetIntroNode determines the best node to bootstrap a new node onto the network
func GetIntroNode(addr string) (*proto.Node, error) {
	if addr == "" {
		addr = "bootstrap.storj.io:8080"
	}

	return &proto.Node{
		Address: &proto.NodeAddress{
			Transport: defaultTransport,
			Address:   addr,
		},
	}, nil
}

func restrict(r proto.Restriction, n []*proto.Node) []*proto.Node {
	oper := r.GetOperand()
	op := r.GetOperator()
	val := r.GetValue()
	var comp int64

	results := []*proto.Node{}
	for _, v := range n {
		switch oper {
		case proto.Restriction_freeBandwidth:
			comp = v.GetRestrictions().GetFreeBandwidth()
		case proto.Restriction_freeDisk:
			comp = v.GetRestrictions().GetFreeDisk()
		}

		switch op {
		case proto.Restriction_EQ:
			if comp != val {
				results = append(results, v)
				continue
			}
		case proto.Restriction_LT:
			if comp < val {
				results = append(results, v)
				continue
			}
		case proto.Restriction_LTE:
			if comp <= val {
				results = append(results, v)
				continue
			}
		case proto.Restriction_GT:
			if comp > val {
				results = append(results, v)
				continue
			}
		case proto.Restriction_GTE:
			if comp >= val {
				results = append(results, v)
				continue
			}

		}

	}

	return results
}
