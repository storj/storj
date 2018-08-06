// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"fmt"
	"log"

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
	bc, err := boltdb.NewClient(zap.L(), dbPath, boltdb.OverlayBucket)
	if err != nil {
		return nil, err
	}

	return &Cache{
		DB:  bc,
		DHT: DHT,
	}, nil
}

// Get looks up the provided nodeID from the redis cache
func (o *Cache) Get(ctx context.Context, key string) (*overlay.Node, error) {
	b, err := o.DB.Get([]byte(key))
	if err != nil {
		return nil, err
	}
	if b.IsZero() {
		// TODO: log? return an error?
		return nil, nil
	}

	na := &overlay.Node{}
	if err := proto.Unmarshal(b, na); err != nil {
		return nil, err
	}

	return na, nil
}

// Put adds a nodeID to the redis cache with a binary representation of proto defined Node
func (o *Cache) Put(nodeID string, value overlay.Node) error {
	data, err := proto.Marshal(&value)
	if err != nil {
		return err
	}

	return o.DB.Put(kademlia.StringToNodeID(nodeID).Bytes(), []byte(data))
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

		node, err := proto.Marshal(&found)
		if err != nil {
			return err
		}

		if err := o.DB.Put(kademlia.StringToNodeID(found.Id).Bytes(), node); err != nil {
			return err
		}
	}

	return err
}

// Refresh updates the cache db with the current DHT.
// We currently do not penalize nodes that are unresponsive,
// but should in the future.
func (o *Cache) Refresh(ctx context.Context) error {
	log.Print("starting cache refresh")
	table, err := o.DHT.GetRoutingTable(ctx)
	if err != nil {
		zap.Error(OverlayError.New("Error getting routing table", err))
		return err
	}

	buckets, _ := table.GetBuckets()

	for _, b := range buckets {
		log.Printf("refreshing bucket %+v\n", b)
		bucketNodes := b.Nodes()
		for _, bn := range bucketNodes {
			log.Printf("bucket node %+v\n", bn)
		}
	}

	nodes, err := o.DHT.GetNodes(ctx, "", 1024)

	/// kick off concurrent refreshes
	randomID, err := kademlia.NewID()

	if err != nil {
		zap.Error(OverlayError.New("Error generating random ID", err))
		log.Printf("error finding random ID %+v\n", err)
	}

	nn, err := o.DHT.FindNearNodes(ctx, randomID)
	if err != nil {
		zap.Error(OverlayError.New("Error finding node", err))
	}

	fmt.Printf("found near nodes %+v\n", nn)

	for _, n := range nn {
		log.Printf("getting routing table for node %+v\n", n)
		t, err := kademlia.GetNodeRoutingTable(ctx, kademlia.NodeID(n.Id))
		if err != nil {
			zap.Error(OverlayError.New("Error getting node routing table", err))
			log.Printf("error getting routing table from foreign node %+v\n", err)
		}
		log.Printf("got routing table back %+v\n", t)
	}

	for _, node := range nodes {
		pinged, err := o.DHT.Ping(ctx, *node)
		if err != nil {
			zap.Error(ErrNodeNotFound)
		} else {
			o.DB.Put([]byte(pinged.Id), []byte(pinged.Address.Address))
		}
	}

	return err
}

// Walk iterates over each node in each bucket to traverse the network
func (o *Cache) Walk(ctx context.Context) error {
	table, _ := o.DHT.GetRoutingTable(ctx)
	k := table.K()

	nodes, err := o.DHT.GetNodes(ctx, "0", k)
	if err != nil {
		zap.Error(OverlayError.New("Error getting routing table", err))
		return err
	}

	for _, node := range nodes {
		_, err := o.DHT.FindNode(ctx, kademlia.StringToNodeID(node.Id))
		if err != nil {
			zap.Error(ErrNodeNotFound)
			return err
		}
	}

	return nil
}
