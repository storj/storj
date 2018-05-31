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
	"golang.org/x/sync/errgroup"

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

// GetNodes returns all nodes from a starting node up to a maximum limit stored in the local routing table
func (k Kademlia) GetNodes(ctx context.Context, start string, limit int) ([]proto.Node, error) {
	nn, err := k.dht.FindNodes(ctx, start, limit)
	if err != nil {
		return []proto.Node{}, nil
	}
	return convertNetworkNodes(nn), nil
}

// GetRoutingTable provides the routing table for the Kademlia DHT
func (k Kademlia) GetRoutingTable(ctx context.Context) (RoutingTable, error) {
	return k.rt, nil
}

// Bootstrap contacts one of a set of pre defined trusted nodes on the network and
// begins populating the local Kademlia node
func (k Kademlia) Bootstrap(ctx context.Context) error {
	dht, err := bkad.NewDHT(&bkad.MemoryStore{}, &bkad.Options{
		BootstrapNodes: convertProtoNodes(k.bootstrapNodes),
		IP:             k.ip,
		Port:           k.port,
		UseStun:        k.stun,
	})
	if err != nil {
		return err
	}

	if err := dht.CreateSocket(); err != nil {
		return err
	}

	g := errgroup.Group{}

	g.Go(func() error {
		return dht.Listen()
	})

	g.Go(func() error {
		return dht.Bootstrap()
	})

	return g.Wait()
}

// newID creates a new Node ID
func newID() []byte {
	result := make([]byte, 20)
	rand.Read(result)
	return result
}

// BootstrapNetwork creates a new DHT and bootstraps it with the passed IP and Port
func BootstrapNetwork(ip, port, bootstrapIP, bootstrapPort string) (*bkad.DHT, error) {
	id := newID()
	p, _ := strconv.Atoi(port)
	dht, err := bkad.NewDHT(&bkad.MemoryStore{}, &bkad.Options{
		ID:   id,
		IP:   ip,
		Port: strconv.Itoa(p),
		BootstrapNodes: []*bkad.NetworkNode{
			bkad.NewNetworkNode(bootstrapIP, bootstrapPort),
		},
	})
	return dht, err
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

	if len(nodes) <= 0 || string(nodes[0].ID) != string(ID) {
		// check if the IDs don't match since dht.FindNode will
		// return the closest node if the node it's looking for
		// is not found
		return proto.Node{}, NodeErr.New("node not found")
	}

	node := nodes[0]

	return proto.Node{
		Id: string(node.ID),
		Address: &proto.NodeAddress{
			Transport: defaultTransport, // TODO: defaulting to this, probably needs to be determined during lookup
			Address:   fmt.Sprintf("%s:%d", node.IP.String(), node.Port),
		},
	}, nil
}

func convertProtoNodes(n []proto.Node) []*bkad.NetworkNode {
	nn := []*bkad.NetworkNode{}
	for i, v := range n {
		if bnn, ok := convert(v).(*bkad.NetworkNode); !ok {
			continue
		} else {
			nn[i] = bnn
		}
	}

	return nn
}

func convertNetworkNodes(n []*bkad.NetworkNode) []proto.Node {
	nn := []proto.Node{}
	for i, v := range n {
		if bnn, ok := convert(*v).(proto.Node); !ok {
			continue
		} else {
			nn[i] = bnn
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
