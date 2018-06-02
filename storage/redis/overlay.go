// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gogo/protobuf/proto"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/protos/overlay"
)

const defaultNodeExpiration = 61 * time.Minute

// ErrNodeNotFound standardizes errors here
var ErrNodeNotFound = errors.New("Node not found")

// OverlayClient is used to store overlay data in Redis
type OverlayClient struct {
	DB  Client
	DHT kademlia.DHT
}

// NewOverlayClient returns a pointer to a new OverlayClient instance with an initalized connection to Redis.
func NewOverlayClient(address, password string, db int, DHT kademlia.DHT) (*OverlayClient, error) {
	rc, err := NewRedisClient(address, password, db)
	if err != nil {
		return nil, err
	}

	return &OverlayClient{
		DB:  rc,
		DHT: DHT,
	}, nil
}

// Get looks up the provided nodeID from the redis cache
func (o *OverlayClient) Get(ctx context.Context, key string) (*overlay.NodeAddress, error) {
	d, err := o.DB.Get(key)

	if d == nil {
		return nil, ErrNodeNotFound
	}

	if err != nil {
		return nil, err
	}

	na := &overlay.NodeAddress{}
	return na, proto.Unmarshal(d, na)
}

// Set adds a nodeID to the redis cache with a binary representation of proto defined NodeAddress
func (o *OverlayClient) Set(nodeID string, value overlay.NodeAddress) error {
	data, err := proto.Marshal(&value)
	if err != nil {
		return err
	}

	return o.DB.Set(nodeID, data, defaultNodeExpiration)
}

// Bootstrap walks the initialized network and populates the cache
func (o *OverlayClient) Bootstrap(ctx context.Context) error {
	fmt.Println("Bootstrap function reached")
	routetable, err := o.DHT.GetRoutingTable(ctx)

	fmt.Println(routetable)

	// called after kademlia is bootstrapped
	// needs to take RoutingTable and start to persist it into the cache
	// take bootstrap node
	// get their route table
	// loop through nodes in RT and get THEIR route table
	// keep going forever and ever

	// Other Possibilities: Randomly generate node ID's to ask for?

	rt, err := o.DHT.GetRoutingTable(ctx)

	fmt.Printf("Boostrap routing table:  %+v\n", rt)
	fmt.Printf("Bootstrap DHT: %+v\n", o.DHT)

	if err != nil {
		return err
	}

	fmt.Println("Routing Table: ", rt)
	return nil
}

// Refresh walks the network looking for new nodes and pings existing nodes to eliminate stale addresses
func (o *OverlayClient) Refresh(ctx context.Context) error {
	// iterate over all nodes
	// compare responses to find new nodes
	// listen for responses from existing nodes
	// if no response from existing, then mark it as offline for time period
	// if responds, it refreshes in DHT
	rt, rtErr := o.DHT.GetRoutingTable(ctx)

	if rtErr != nil {
		return rtErr
	}

	fmt.Printf("RT:: %+v\n", rt)

	nodes, err := o.DHT.GetNodes(ctx, "0", 128)

	if err != nil {
		return err
	}

	fmt.Printf("Nodes:: %+v\n", nodes)

	for k, v := range nodes {
		fmt.Printf("key: ", k, "value: ", v)
	}

	return nil
}

// Walk iterates over buckets to traverse the network
func (o *OverlayClient) Walk(ctx context.Context) error {
	return errors.New("Walk function needs to be implemented")
}
