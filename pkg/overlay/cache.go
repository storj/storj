// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/storage"
	"storj.io/storj/storage/boltdb"
	"storj.io/storj/storage/redis"
	"storj.io/storj/storage/storelogger"
)

const (
	// OverlayBucket is the string representing the bucket used for a bolt-backed overlay dht cache
	OverlayBucket = "overlay"
)

// ErrNodeNotFound error standardization
var ErrNodeNotFound = errs.New("Node not found")

// ErrBucketNotFound returns if a bucket is unable to be found in the routing table
var ErrBucketNotFound = errs.New("Bucket not found")

// OverlayError creates class of errors for stack traces
var OverlayError = errs.Class("Overlay Error")

// Cache is used to store overlay data in Redis
type Cache struct {
	DB  storage.KeyValueStore
	DHT dht.DHT
}

// NewRedisOverlayCache returns a pointer to a new Cache instance with an initialized connection to Redis.
func NewRedisOverlayCache(address, password string, dbindex int, dht dht.DHT) (*Cache, error) {
	db, err := redis.NewClient(address, password, dbindex)
	if err != nil {
		return nil, err
	}
	return NewOverlayCache(storelogger.New(zap.L(), db), dht), nil
}

// NewBoltOverlayCache returns a pointer to a new Cache instance with an initialized connection to a Bolt db.
func NewBoltOverlayCache(dbPath string, dht dht.DHT) (*Cache, error) {
	db, err := boltdb.New(dbPath, OverlayBucket)
	if err != nil {
		return nil, err
	}
	return NewOverlayCache(storelogger.New(zap.L(), db), dht), nil
}

// NewOverlayCache returns a new Cache
func NewOverlayCache(db storage.KeyValueStore, dht dht.DHT) *Cache {
	return &Cache{DB: db, DHT: dht}
}

// Get looks up the provided nodeID from the overlay cache
func (o *Cache) Get(ctx context.Context, key string) (*pb.Node, error) {
	b, err := o.DB.Get([]byte(key))
	if err != nil {
		return nil, err
	}
	if b.IsZero() {
		// TODO: log? return an error?
		return nil, nil
	}
	na := &pb.Node{}
	if err := proto.Unmarshal(b, na); err != nil {
		return nil, err
	}
	return na, nil
}

// GetAll looks up the provided nodeIDs from the overlay cache
func (o *Cache) GetAll(ctx context.Context, keys []string) ([]*pb.Node, error) {
	if len(keys) == 0 {
		return nil, OverlayError.New("no keys provided")
	}
	var ks storage.Keys
	for _, v := range keys {
		ks = append(ks, storage.Key(v))
	}
	vs, err := o.DB.GetAll(ks)
	if err != nil {
		return nil, err
	}
	var ns []*pb.Node
	for _, v := range vs {
		if v == nil {
			ns = append(ns, nil)
			continue
		}
		na := &pb.Node{}
		err := proto.Unmarshal(v, na)
		if err != nil {
			return nil, OverlayError.New("could not unmarshal non-nil node: %v", err)
		}
		ns = append(ns, na)
	}
	return ns, nil
}

// Put adds a nodeID to the redis cache with a binary representation of proto defined Node
func (o *Cache) Put(nodeID string, value pb.Node) error {
	data, err := proto.Marshal(&value)
	if err != nil {
		return err
	}
	return o.DB.Put(node.IDFromString(nodeID).Bytes(), data)
}

// Bootstrap walks the initialized network and populates the cache
func (o *Cache) Bootstrap(ctx context.Context) error {
	nodes, err := o.DHT.GetNodes(ctx, "", 1280)
	if err != nil {
		return OverlayError.New("Error getting nodes from DHT: %v", err)
	}
	for _, v := range nodes {
		found, err := o.DHT.FindNode(ctx, node.IDFromString(v.Id))
		if err != nil {
			zap.L().Info("Node find failed", zap.String("nodeID", v.Id))
			continue
		}
		n, err := proto.Marshal(&found)
		if err != nil {
			zap.L().Error("Node marshall failed", zap.String("nodeID", v.Id))
			continue
		}
		if err := o.DB.Put(node.IDFromString(found.Id).Bytes(), n); err != nil {
			zap.L().Error("Node cache put failed", zap.String("nodeID", v.Id))
			continue
		}
	}
	return err
}

// Refresh updates the cache db with the current DHT.
// We currently do not penalize nodes that are unresponsive,
// but should in the future.
func (o *Cache) Refresh(ctx context.Context) error {
	log.Print("starting cache refresh")
	r, err := randomID()
	if err != nil {
		return err
	}
	rid := node.ID(r)
	near, err := o.DHT.GetNodes(ctx, rid.String(), 128)
	if err != nil {
		return err
	}
	for _, n := range near {
		pinged, err := o.DHT.Ping(ctx, *n)
		if err != nil {
			zap.L().Info("Node ping failed", zap.String("nodeID", n.GetId()))
			continue
		}
		data, err := proto.Marshal(&pinged)
		if err != nil {
			zap.L().Error("Node marshall failed", zap.String("nodeID", n.GetId()))
			continue
		}
		err = o.DB.Put(node.IDFromString(pinged.Id).Bytes(), data)
		if err != nil {
			zap.L().Error("Node cache put failed", zap.String("nodeID", n.GetId()))
			continue
		}
	}
	// TODO: Kademlia hooks to do this automatically rather than at interval
	nodes, err := o.DHT.GetNodes(ctx, "", 128)
	if err != nil {
		return err
	}
	for _, n := range nodes {
		pinged, err := o.DHT.Ping(ctx, *n)
		if err != nil {
			zap.L().Info("Node ping failed", zap.String("nodeID", n.GetId()))
			continue
		}
		data, err := proto.Marshal(&pinged)
		if err != nil {
			zap.L().Error("Node marshall failed", zap.String("nodeID", n.GetId()))
			continue
		}
		err = o.DB.Put(node.IDFromString(pinged.Id).Bytes(), data)
		if err != nil {
			zap.L().Error("Node cache put failed", zap.String("nodeID", n.GetId()))
			continue
		}
	}

	return nil
}

// Walk iterates over each node in each bucket to traverse the network
func (o *Cache) Walk(ctx context.Context) error {
	// TODO: This should walk the cache, rather than be a duplicate of refresh
	return nil
}

/*
* Kademlia bucket interaction methods
*	These methods allow external inspection of the kademlia service via gRPC
 */

// DumpBuckets returns a list of buckets in the current Kademlia instance
func (o *Cache) DumpBuckets(ctx context.Context) error {
	table, err := o.DHT.GetRoutingTable(ctx)
	if err != nil {
		return err
	}
	buckets, err := table.GetBuckets()
	if err != nil {
		return err
	}
	zap.S().Info(fmt.Printf("Buckets: %+v\n", buckets))
	return nil
}

// GetBucket returns all the nodes in a given bucket
func (o *Cache) GetBucket(ctx context.Context, id string) error {
	table, err := o.DHT.GetRoutingTable(ctx)
	if err != nil {
		return err
	}

	bucket, ok := table.GetBucket(id)
	if !ok {
		return ErrBucketNotFound
	}

	zap.S().Info(fmt.Printf("Bucket: %+v\n", bucket))

	return nil
}

func randomID() ([]byte, error) {
	result := make([]byte, 64)
	_, err := rand.Read(result)
	return result, err
}
