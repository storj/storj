// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"crypto/rand"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/storage"
	"storj.io/storj/pkg/statdb/sdbclient"
)

const (
	// OverlayBucket is the string representing the bucket used for a bolt-backed overlay dht cache
	OverlayBucket = "overlay"
)

// OverlayError creates class of errors for stack traces
var OverlayError = errs.Class("Overlay Error")

// Cache is used to store overlay data in Redis
type Cache struct {
	DB  storage.KeyValueStore
	DHT dht.DHT
	statdb sdbclient.Client
}

// NewOverlayCache returns a new Cache
func NewOverlayCache(db storage.KeyValueStore, dht dht.DHT, statdb sdbclient.Client) *Cache {
	return &Cache{DB: db, DHT: dht, statdb: statdb}
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
	zap.L().Info("starting cache refresh")
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

func randomID() ([]byte, error) {
	result := make([]byte, 64)
	_, err := rand.Read(result)
	return result, err
}
