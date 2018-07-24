// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/protos/overlay"
	"storj.io/storj/storage"
	"storj.io/storj/storage/boltdb"
	"storj.io/storj/storage/redis"
)

// ErrNodeNotFound error standardization
var ErrNodeNotFound = errs.New("Node not found")

// OverlayError creates class of errors for stack traces
var OverlayError = errs.Class("Overlay Error")

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
	nodes, err := o.DHT.GetNodes(ctx, "0", 1280)

	if err != nil {
		zap.Error(OverlayError.New("Error getting nodes from DHT", err))
	}

	for _, v := range nodes {
		found, err := o.DHT.FindNode(ctx, kademlia.StringToNodeID(v.Id))
		if err != nil {
			zap.Error(ErrNodeNotFound)
		}
		addr, err := proto.Marshal(found.Address)
		if err != nil {
			return err
		}

		if err := o.DB.Put([]byte(found.Id), addr); err != nil {
			return err
		}
	}
	// called after kademlia is bootstrapped
	// needs to take RoutingTable and start to persist it into the cache
	// take bootstrap node
	// get their route table
	// loop through nodes in RT and get THEIR route table
	// keep going forever and ever

	// Other Possibilities: Randomly generate node ID's to ask for?

	_, err = o.DHT.GetRoutingTable(ctx)

	return err
}

// Refresh walks the network looking for new nodes and pings existing nodes to eliminate stale addresses
func (o *Cache) Refresh(ctx context.Context) error {
	log.Print("refresh function called")
	// iterate over all nodes
	// compare responses to find new nodes
	// listen for responses from existing nodes
	// if no response from existing, then do nothing
	// if responds, it refreshes in cache
	table, err := o.DHT.GetRoutingTable(ctx)

	if err != nil {
		zap.Error(OverlayError.New("Error getting routing table", err))
		return err
	}

	k := table.K()

	nodes, err := o.DHT.GetNodes(ctx, "0", k)

	log.Print("nodes %+v\n", nodes)

	for _, node := range nodes {
		fmt.Printf("node::: %+v\n", node)
		fmt.Printf("pointer node %+v\n", *node)
		pinged, err := o.DHT.Ping(ctx, *node)
		if err != nil {
			// penalize node for not being online
			zap.Error(ErrNodeNotFound)
		}
		log.Print("pinged node id %s address: %s", pinged.Id, pinged.Address.Address)
		o.DB.Put([]byte(pinged.Id), []byte(pinged.Address.Address))
	}

	log.Print("finished refreshing cache")
	return nil
}

// Walk iterates over each node in each bucket to traverse the network
func (o *Cache) Walk(ctx context.Context) error {
	table, _ := o.DHT.GetRoutingTable(ctx)
	k := table.K()

	nodes, err := o.DHT.GetNodes(ctx, "0", k)
	if err != nil {
		return err
	}

	for _, node := range nodes {
		fmt.Printf("node::: %+v\n", node)
		_, err := o.DHT.FindNode(ctx, kademlia.StringToNodeID(node.Id))
		if err != nil {
			zap.Error(ErrNodeNotFound)
			return err
		}
	}

	return nil
}

func schedule(fn func() error, interval time.Duration) chan error {
	ticker := time.NewTicker(interval * time.Second)
	quit := make(chan struct{})
	errors := make(chan error)
	go func() {
		for {
			select {
			case <-ticker.C:
				err := fn()
				if err != nil {
					errors <- err
				}
			case <-quit:
				ticker.Stop()
			}
		}
	}()
	return errors
}
