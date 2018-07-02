// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"fmt"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/protos/overlay"
	"storj.io/storj/storage"
	"storj.io/storj/storage/boltdb"
	"storj.io/storj/storage/redis"
)

// ErrNodeNotFound standardizes errors here
var ErrNodeNotFound = errs.Class("Node not found")

// Cache is used to store overlay data in Redis
type Cache struct {
	DB  storage.KeyValueStore
	DHT dht.DHT
}

// NewRedisOverlayCache returns a pointer to a new Cache instance with an initalized connection to Redis.
func NewRedisOverlayCache(address, password string, db int, DHT dht.DHT) (*Cache, error) {
	rc, err := redis.NewClient(address, password, db)
	if err != nil {
		return nil, err
	}

	return &Cache{
		DB:  rc,
		DHT: DHT,
	}, nil
}

// NewBoltOverlayCache returns a pointer to a new Cache instance with an initalized connection to a Bolt db.
func NewBoltOverlayCache(dbPath string, DHT dht.DHT) (*Cache, error) {
	bc, err := boltdb.NewClient(nil, dbPath, boltdb.OverlayBucket)
	if err != nil {
		return nil, err
	}

	return &Cache{
		DB:  bc,
		DHT: DHT,
	}, nil
}

// Get looks up the provided nodeID from the redis cache
func (o *Cache) Get(ctx context.Context, key string) (*overlay.NodeAddress, error) {
	b, err := o.DB.Get([]byte(key))
	if err != nil {
		return nil, err
	}
	if b.IsZero() {
		// TODO: log? return an error?
		return nil, nil
	}

	na := &overlay.NodeAddress{}
	if err := proto.Unmarshal(b, na); err != nil {
		return nil, err
	}

	return na, nil
}

// Put adds a nodeID to the redis cache with a binary representation of proto defined NodeAddress
func (o *Cache) Put(nodeID string, value overlay.NodeAddress) error {
	data, err := proto.Marshal(&value)
	if err != nil {
		return err
	}

	return o.DB.Put([]byte(nodeID), []byte(data))
}

// Bootstrap walks the initialized network and populates the cache
func (o *Cache) Bootstrap(ctx context.Context) error {
	fmt.Println("bootstrapping cache")
	nodes, err := o.DHT.GetNodes(ctx, "0", 1280)

	for _, v := range nodes {
		found, err := o.DHT.FindNode(ctx, kademlia.StringToNodeID(v.Id))
		if err != nil {
			fmt.Println("could not find node in network", err, v.Id)
		}
		addr, err := proto.Marshal(found.Address)
		o.DB.Put([]byte(found.Id), addr)
	}
	// called after kademlia is bootstrapped
	// needs to take RoutingTable and start to persist it into the cache
	// take bootstrap node
	// get their route table
	// loop through nodes in RT and get THEIR route table
	// keep going forever and ever

	// Other Possibilities: Randomly generate node ID's to ask for?

	_, err = o.DHT.GetRoutingTable(ctx)

	if err != nil {
		return err
	}

	return nil
}

// Refresh walks the network looking for new nodes and pings existing nodes to eliminate stale addresses
func (o *Cache) Refresh(ctx context.Context) error {
	// iterate over all nodes
	// compare responses to find new nodes
	// listen for responses from existing nodes
	// if no response from existing, then mark it as offline for time period
	// if responds, it refreshes in DHT
	_, rtErr := o.DHT.GetRoutingTable(ctx)

	if rtErr != nil {
		return rtErr
	}

	_, err := o.DHT.GetNodes(ctx, "0", 128)

	if err != nil {
		return err
	}

	return nil
}

// Walk iterates over buckets to traverse the network
func (o *Cache) Walk(ctx context.Context) error {
	nodes, err := o.DHT.GetNodes(ctx, "0", 128)
	if err != nil {
		return err
	}

	for _, v := range nodes {
		if _, err := o.DHT.FindNode(ctx, kademlia.StringToNodeID(v.Id)); err != nil {
			fmt.Println("could not find node in network", err, v.Id)
		}
	}

	return nil
}
