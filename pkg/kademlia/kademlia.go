// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"net"
	"strconv"

	bkad "github.com/coyle/kademlia"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/dht"
	proto "storj.io/storj/protos/overlay"
)

// NodeErr is the class for all errors pertaining to node operations
var NodeErr = errs.Class("node error")

//TODO: shouldn't default to TCP but not sure what to do yet
var defaultTransport = proto.NodeTransport_TCP

// Kademlia is an implementation of kademlia adhering to the DHT interface.
type Kademlia struct {
	routingTable   RoutingTable
	bootstrapNodes []proto.Node
	ip             string
	port           string
	stun           bool
	dht            *bkad.DHT
}

// NewKademlia returns a newly configured Kademlia instance
func NewKademlia(id dht.NodeID, bootstrapNodes []proto.Node, ip string, port string) (*Kademlia, error) {
	if port == "" {
		return nil, NodeErr.New("must specify port in request to NewKademlia")
	}

	ips, err := net.LookupIP(ip)
	if err != nil {
		return nil, err
	}

	if len(ips) <= 0 {
		return nil, errs.New("Invalid IP")
	}

	ip = ips[0].String()

	bnodes, err := convertProtoNodes(bootstrapNodes)
	if err != nil {
		return nil, err
	}

	bdht, err := bkad.NewDHT(&bkad.MemoryStore{}, &bkad.Options{
		ID:             id.Bytes(),
		IP:             ip,
		Port:           port,
		BootstrapNodes: bnodes,
	})

	if err != nil {
		return nil, err
	}

	rt := RoutingTable{
		// ht:  bdht.HT,
		// dht: bdht,
	}

	return &Kademlia{
		routingTable:   rt,
		bootstrapNodes: bootstrapNodes,
		ip:             ip,
		port:           port,
		stun:           true,
		dht:            bdht,
	}, nil
}

// Disconnect safely closes connections to the Kademlia network
func (k Kademlia) Disconnect() error {
	return k.dht.Disconnect()
}

// GetNodes returns all nodes from a starting node up to a maximum limit
// stored in the local routing table limiting the result by the specified restrictions
func (k Kademlia) GetNodes(ctx context.Context, start string, limit int, restrictions ...proto.Restriction) ([]*proto.Node, error) {
	if start == "" {
		start = k.dht.GetSelfID()
	}

	nn, err := k.dht.FindNodes(ctx, start, limit)
	if err != nil {
		return []*proto.Node{}, err
	}

	nodes := convertNetworkNodes(nn)

	for _, r := range restrictions {
		nodes = restrict(r, nodes)
	}
	return nodes, nil
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
	return k.dht.Bootstrap()
}

// Ping checks that the provided node is still accessible on the network
func (k *Kademlia) Ping(ctx context.Context, node proto.Node) (proto.Node, error) {
	n, err := convertProtoNode(node)
	if err != nil {
		return proto.Node{}, err
	}

	ok, err := k.dht.Ping(n)
	if err != nil {
		return proto.Node{}, err
	}
	if !ok {
		return proto.Node{}, NodeErr.New("node unavailable")
	}
	return node, nil
}

// FindNode looks up the provided NodeID first in the local Node, and if it is not found
// begins searching the network for the NodeID. Returns and error if node was not found
func (k *Kademlia) FindNode(ctx context.Context, ID dht.NodeID) (proto.Node, error) {
	nodes, err := k.dht.FindNode(ID.Bytes())
	if err != nil {
		return proto.Node{}, err

	}

	for _, v := range nodes {
		if string(v.ID) == ID.String() {
			return proto.Node{Id: string(v.ID), Address: &proto.NodeAddress{
				Transport: defaultTransport,
				Address:   fmt.Sprintf("%s:%d", v.IP.String(), v.Port),
			},
			}, nil
		}
	}
	return proto.Node{}, NodeErr.New("node not found")
}

// ListenAndServe connects the kademlia node to the network and listens for incoming requests
func (k *Kademlia) ListenAndServe() error {
	if err := k.dht.CreateSocket(); err != nil {
		return err
	}

	go func() {
		if err := k.dht.Listen(); err != nil {
			log.Printf("Failed to listen on the dht: %s\n", err)
		}
	}()

	return nil
}

func convertProtoNodes(n []proto.Node) ([]*bkad.NetworkNode, error) {
	nn := make([]*bkad.NetworkNode, len(n))
	for i, v := range n {
		node, err := convertProtoNode(v)
		if err != nil {
			return nil, err
		}
		nn[i] = node
	}

	return nn, nil
}

func convertNetworkNodes(n []*bkad.NetworkNode) []*proto.Node {
	nn := make([]*proto.Node, len(n))
	for i, v := range n {
		nn[i] = convertNetworkNode(v)
	}

	return nn
}

func convertNetworkNode(v *bkad.NetworkNode) *proto.Node {
	return &proto.Node{
		Id:      string(v.ID),
		Address: &proto.NodeAddress{Transport: defaultTransport, Address: net.JoinHostPort(v.IP.String(), strconv.Itoa(v.Port))},
	}
}

func convertProtoNode(v proto.Node) (*bkad.NetworkNode, error) {
	host, port, err := net.SplitHostPort(v.GetAddress().GetAddress())
	if err != nil {
		return nil, err
	}

	nn := bkad.NewNetworkNode(host, port)
	nn.ID = []byte(v.GetId())

	return nn, nil
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
