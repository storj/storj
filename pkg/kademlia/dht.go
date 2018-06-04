// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"crypto/rand"
	"fmt"
	"strconv"
	"strings"
	"time"

	bkad "github.com/coyle/kademlia"
	"github.com/zeebo/errs"

	proto "storj.io/storj/protos/overlay"
)

// NodeErr is the class for all errros petaining to node operations
var NodeErr = errs.Class("node error")

//TODO: shouldn't default to TCP but not sure what to do yet
var defaultTransport = proto.NodeTransport_TCP

// Kademlia is an implementation of kademlia adhering to the DHT interface.
type Kademlia struct {
	rt             RoutingTable
	bootstrapNodes []proto.Node
	ip             string
	port           string
	stun           bool
	dht            *bkad.DHT
}

// NewKademlia returns a newly configured Kademlia instance
func NewKademlia(bootstrapNodes []proto.Node, ip string, port string, stun bool) Kademlia {
	bb := convertProtoNodes(bootstrapNodes)
	bdht, _ := bkad.NewDHT(&bkad.MemoryStore{}, &bkad.Options{
		ID:             []byte(newID()),
		IP:             ip,
		Port:           port,
		BootstrapNodes: bb,
	})

	rt := RouteTable{
		ht:  bdht.HT,
		dht: bdht,
	}

	return Kademlia{
		rt:             rt,
		bootstrapNodes: bootstrapNodes,
		ip:             ip,
		port:           port,
		stun:           stun,
		dht:            bdht,
	}
}

// GetNodes returns all nodes from a starting node up to a maximum limit stored in the local routing table
func (k Kademlia) GetNodes(ctx context.Context, start string, limit int) ([]*proto.Node, error) {
	nn, err := k.dht.FindNodes(ctx, start, limit)
	if err != nil {
		return []*proto.Node{}, err
	}
	return convertNetworkNodes(nn), nil
}

// GetRoutingTable provides the routing table for the Kademlia DHT
func (k Kademlia) GetRoutingTable(ctx context.Context) (RoutingTable, error) {
	return RouteTable{
		ht:  k.dht.HT,
		dht: k.dht,
	}, nil
}

// Bootstrap contacts one of a set of pre defined trusted nodes on the network and
// begins populating the local Kademlia node
func (k Kademlia) Bootstrap(ctx context.Context) error {
	if err := k.dht.CreateSocket(); err != nil {
		return err
	}

	go k.dht.Listen()

	return k.dht.Bootstrap()

}

// BootstrapNetwork creates a new DHT and bootstraps it with the passed IP and Port
func BootstrapNetwork(ip, port, bootstrapIP, bootstrapPort string) error {
	id := newID()
	fmt.Println("created new node id %s", id)
	p, _ := strconv.Atoi(port)
	dht, err := bkad.NewDHT(&bkad.MemoryStore{}, &bkad.Options{
		ID:   id,
		IP:   ip,
		Port: strconv.Itoa(p),
		BootstrapNodes: []*bkad.NetworkNode{
			bkad.NewNetworkNode(bootstrapIP, bootstrapPort),
		},
	})

	// listen for connections
	go dht.Listen()

	// call bootstrap on initialized dht
	go func(dht *bkad.DHT) {
		if err := dht.Bootstrap(); err != nil {
			NodeErr.New("error bootstrapping node", err)
		}
	}(dht)

	fmt.Printf("bootstrapped network dht: %+v\n", dht)

	return err
}

// BootstrapTestNetwork spins up a kademlia network locally for testing purposes
func BootstrapTestNetwork(ip, port string) []*bkad.DHT {
	dhts := []*bkad.DHT{}
	p, err := strconv.Atoi(port)
	if err != nil {
		panic(err)
	}

	for i := 0; i < 20; i++ {
		id := newID()
		dht, _ := bkad.NewDHT(&bkad.MemoryStore{}, &bkad.Options{
			ID:   id,
			IP:   ip,
			Port: strconv.Itoa(p),
			BootstrapNodes: []*bkad.NetworkNode{
				bkad.NewNetworkNode("127.0.0.1", strconv.Itoa(p-1)),
			},
		})
		p++
		dhts = append(dhts, dht)
		if err := dht.CreateSocket(); err != nil {
			panic(err)
		}
	}

	for _, dht := range dhts {
		go dht.Listen()
		go func(dht *bkad.DHT) {
			if err := dht.Bootstrap(); err != nil {
				panic(err)
			}
		}(dht)

		time.Sleep(200 * time.Millisecond)
		return dhts
	}

	return dhts
}

// Ping checks that the provided node is still accessible on the network
func (k Kademlia) Ping(ctx context.Context, node proto.Node) (proto.Node, error) {
	n, ok := convert(node).(*bkad.NetworkNode)
	if !ok {
		return proto.Node{}, NodeErr.New("unable to convert to expected type")
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
func (k Kademlia) FindNode(ctx context.Context, ID NodeID) (proto.Node, error) {
	nodes, err := k.dht.FindNode([]byte(ID))
	fmt.Println(nodes)

	if err != nil {
		return proto.Node{}, err

	}

	for _, v := range nodes {
		if string(v.ID) == string(ID) {
			return proto.Node{Id: string(v.ID), Address: &proto.NodeAddress{
				Transport: defaultTransport,
				Address:   fmt.Sprintf("%s:%d", v.IP.String(), v.Port),
			},
			}, nil
		}
	}
	return proto.Node{}, NodeErr.New("node not found")
}

func convertProtoNodes(n []proto.Node) []*bkad.NetworkNode {
	nn := make([]*bkad.NetworkNode, len(n))
	for i, v := range n {
		if bnn, ok := convert(v).(*bkad.NetworkNode); !ok {
			continue
		} else {
			nn[i] = bnn
		}
	}

	return nn
}

func convertNetworkNodes(n []*bkad.NetworkNode) []*proto.Node {
	nn := make([]*proto.Node, len(n))
	for i, v := range n {
		if bnn, ok := convert(*v).(proto.Node); !ok {
			continue
		} else {
			nn[i] = &bnn
		}
	}

	return nn
}

func convert(i interface{}) interface{} {

	switch v := i.(type) {
	case proto.Node:
		ip := strings.Split(v.GetAddress().GetAddress(), ":")
		if len(ip) == 1 {
			ip = append(ip, "0")
		}

		nn := bkad.NewNetworkNode(ip[0], ip[1])
		nn.ID = []byte(v.GetId())

		return nn
	case bkad.NetworkNode:
		nn := proto.Node{
			Id:      string(v.ID),
			Address: &proto.NodeAddress{Transport: defaultTransport, Address: fmt.Sprintf("%s:%d", v.IP.String(), v.Port)},
		}
		return &nn
	default:
		return nil
	}

}

// newID generates a new random ID
// TODO: Add a proof of work scheme for node ID generation
func newID() []byte {
	result := make([]byte, 20)
	rand.Read(result)
	return result
}
