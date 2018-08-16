// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"errors"
	"math/rand"
	"sync"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/node"
	proto "storj.io/storj/protos/overlay"
)

// NodeErr is the class for all errors pertaining to node operations
var NodeErr = errs.Class("node error")

// BootstrapErr is the class for all errors pertaining to bootstrapping a node
var BootstrapErr = errs.Class("bootstrap node error")

//TODO: shouldn't default to TCP but not sure what to do yet
var defaultTransport = proto.NodeTransport_TCP

// NodeNotFound is returned when a lookup can not produce the requested node
var NodeNotFound = NodeErr.New("node not found")

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
	rt := RoutingTable{}

	return &Kademlia{
		routingTable:   &rt,
		bootstrapNodes: bootstrapNodes,
		address:        address,
		stun:           true,
	}, nil
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
	return &RoutingTable{
		// ht:  k.dht.HT,
		// dht: k.dht,
	}, nil

}

// Bootstrap contacts one of a set of pre defined trusted nodes on the network and
// begins populating the local Kademlia node
func (k *Kademlia) Bootstrap(ctx context.Context) error {
	// What I want to do here is do a normal lookup for myself
	// so call lookup(ctx, nodeImLookingFor)
	if len(k.bootstrapNodes) == 0 {
		return BootstrapErr.New("no bootstrap nodes provided")
	}

	_, err := k.lookup(ctx, StringToNodeID(k.routingTable.self.GetId()))
	return err
}

func (k *Kademlia) lookup(ctx context.Context, target dht.NodeID) (*proto.Node, error) {
	// look in routing table for targetID
	nodes, err := k.routingTable.FindNear(target, k.alpha)
	if err != nil {
		return nil, err
	}

	// if we have the node in our routing table just return the node
	if len(nodes) == 1 && StringToNodeID(nodes[0].GetId()) == target {
		return nodes[0], nil
	}

	// begin the work looking for the node by spinning up alpha workers
	// and asking for the node we want from nodes we know in our routing table
	ch := make(chan []*proto.Node)
	w := newWorker(ctx, k.routingTable, nodes, k.nodeClient, target, k.routingTable.K())
	for i := 0; i < k.alpha; i++ {
		go w.work(ctx, ch)
	}

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

	return nil, NodeNotFound
}

func (k *Kademlia) query(ctx context.Context, nodes []*proto.Node) ([]*proto.Node, error) {
	if len(nodes) <= 0 {
		return nil, NodeErr.New("no nodes provided for lookup")
	}

	self := k.routingTable.Local()
	wg := sync.WaitGroup{}
	n := len(nodes)

	wg.Add(n)
	c := make(chan []*proto.Node, n)
	e := make(chan error, n)

	for _, v := range nodes {
		go func(v *proto.Node, wg *sync.WaitGroup, c chan []*proto.Node, e chan error) {
			//TODO() ctx with timeout
			defer wg.Done()
			nodes, err := k.nodeClient.Lookup(ctx, *v, self)
			if err != nil {
				e <- err
			}
			c <- nodes
		}(v, &wg, c, e)

	}
	wg.Wait()
	close(e)
	close(c)

	var err error
	if len(e) > 0 {
		for err = range e {
			err = BootstrapErr.Wrap(err)
		}
		return nil, err
	}

	results := []*proto.Node{}
	for ns := range c {
		results = append(results, ns...)
	}

	return results, nil
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

// newID generates a new random ID.
// This purely to get things working. We shouldn't use this as the ID in the actual network
func newID() ([]byte, error) {
	result := make([]byte, 20)
	_, err := rand.Read(result)
	return result, err
}

// GetIntroNode determines the best node to bootstrap a new node onto the network
func GetIntroNode(id, ip, port string) (*proto.Node, error) {
	addr := "bootstrap.storj.io:8080"
	if ip != "" && port != "" {
		addr = ip + ":" + port
	}

	if id == "" {
		i, err := newID()
		if err != nil {
			return nil, err
		}

		id = string(i)
	}

	return &proto.Node{
		Id: id,
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
