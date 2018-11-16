// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"crypto/rand"
	"log"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"storj.io/storj/pkg/storj"

	"storj.io/storj/pkg/dht"
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
	return &Cache{
		DB:  db,
		DHT: dht,
	}
}

// Get looks up the provided nodeID from the overlay cache
func (o *Cache) Get(ctx context.Context, nodeID storj.NodeID) (storj.Node, error) {
	b, err := o.DB.Get(nodeID.Bytes())
	if err != nil {
		return storj.Node{}, err
	}
	if b.IsZero() {
		// TODO: log? return an error?
		return storj.Node{}, nil
	}

	pbNode := &pb.Node{}
	if err := proto.Unmarshal(b, pbNode); err != nil {
		return storj.Node{}, err
	}

	return storj.NewNodeWithID(nodeID, pbNode), nil
}

// GetAll looks up the provided nodeIDs from the overlay cache
func (o *Cache) GetAll(ctx context.Context, nodeIDs storj.NodeIDList) ([]storj.Node, error) {
	if len(nodeIDs) == 0 {
		return nil, OverlayError.New("no nodeIDs provided")
	}
	var ks storage.Keys
	for _, v := range nodeIDs {
		ks = append(ks, storage.Key(v.Bytes()))
	}
	vs, err := o.DB.GetAll(ks)
	if err != nil {
		return nil, err
	}
	var pbNodes []*pb.Node
	for _, v := range vs {
		if v == nil {
			pbNodes = append(pbNodes, nil)
			continue
		}
		pbNode := &pb.Node{}
		err := proto.Unmarshal(v, pbNode)
		if err != nil {
			return nil, OverlayError.New("could not unmarshal non-nil node: %v", err)
		}
		pbNodes = append(pbNodes, pbNode)
	}
	return storj.NewNodes(pbNodes)
}

// Put adds a nodeID to the redis cache with a binary representation of proto defined Node
func (o *Cache) Put(node storj.Node) error {
	data, err := proto.Marshal(node.Node)
	if err != nil {
		return err
	}
	return o.DB.Put(node.Id.Bytes(), data)
}

// Bootstrap walks the initialized network and populates the cache
func (o *Cache) Bootstrap(ctx context.Context) error {
	nodes, err := o.DHT.GetNodes(ctx, storj.EmptyNodeID, 1280)
	if err != nil {
		zap.Error(OverlayError.New("Error getting nodes from DHT: %v", err))
	}

	for _, v := range nodes {
		found, err := o.DHT.FindNode(ctx, v.Id)
		if err != nil {
			zap.Error(ErrNodeNotFound)
		}

		data, err := proto.Marshal(found.Node)
		if err != nil {
			return err
		}

		if err := o.DB.Put(found.Id.Bytes(), data); err != nil {
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
	r, err := randomID()
	if err != nil {
		return err
	}

	near, err := o.DHT.GetNodes(ctx, r, 128)
	if err != nil {
		return err
	}

	for _, node := range near {
		pinged, err := o.DHT.Ping(ctx, node)
		if err != nil {
			return err
		}
		err = o.DB.Put(pinged.Id.Bytes(), []byte(pinged.Address.Address))
		if err != nil {
			return err
		}
	}

	// TODO: Kademlia hooks to do this automatically rather than at interval
	nodes, err := o.DHT.GetNodes(ctx, storj.EmptyNodeID, 128)
	if err != nil {
		return err
	}

	for _, node := range nodes {
		pinged, err := o.DHT.Ping(ctx, node)
		if err != nil {
			zap.Error(ErrNodeNotFound)
			return err
		}
		err = o.DB.Put(pinged.Id.Bytes(), []byte(pinged.Address.Address))
		if err != nil {
			return err
		}

	}

	return err
}

// Walk iterates over each node in each bucket to traverse the network
func (o *Cache) Walk(ctx context.Context) error {
	// TODO: This should walk the cache, rather than be a duplicate of refresh
	return nil
}

func randomID() (storj.NodeID, error) {
	r := make([]byte, storj.IdentityLength)
	_, err := rand.Read(r)
	id, err := storj.NodeIDFromBytes(r)
	if err != nil {
		return storj.EmptyNodeID, err
	}

	return id, err
}
