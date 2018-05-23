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

// OverlayClient is used to store overlay data in Redis
type OverlayClient struct {
	DB             Client
	DHT            kademlia.DHT
	bootstrapNodes []overlay.Node
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
func (o *OverlayClient) Get(key string) (*overlay.NodeAddress, error) {
	d, err := o.DB.Get(key)

	if d == nil {
		// if not found in cache, we do another lookup in DHT
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
	rt, err := o.DHT.GetRoutingTable(ctx)
	buckets, _ := rt.GetBuckets()

	fmt.Println("Buckets: ", buckets)
	fmt.Println("Routing table: ", rt)

	if err != nil {
		return errors.New("Error getting routing table")
	}

	return errors.New("BOOTSTRAP TODO")

	// Merge Dennis' code
	// loop through bootstrap nodes asking for random IDs
	// nodes, err := o.DHT. (ctx, "random node ID", 100)
	// if err != nil {
	// 	fmt.Println(err)
	// }
}

// Refresh walks the network looking for new nodes and pings existing nodes to eliminate stale addresses
func (o *OverlayClient) Refresh(ctx context.Context) error {
	// iterate over all nodes
	// compare responses to find new nodes
	// listen for responses from existing nodes
	// if no response from existing, then mark it as offline for time period
	// if responds, it refreshes in DHT

	return errors.New("REFRESH TODO")
}

// Walk iterates over buckets to walk the network
func (o *OverlayClient) Walk(ctx context.Context) error {
	return errors.New("Walk function needs to be implemented")
}
