// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/statdb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

const (
	// OverlayBucket is the string representing the bucket used for a bolt-backed overlay dht cache
	OverlayBucket = "overlay"
)

// ErrDelete is returned when there was an error deleting something
var ErrDelete = errs.New("error deleting key:")

// ErrEmptyNode is returned when the nodeID is empty
var ErrEmptyNode = errs.New("empty node ID")

// ErrNodeNotFound is returned if a node does not exist in database
var ErrNodeNotFound = errs.New("Node not found")

// ErrBucketNotFound is returned if a bucket is unable to be found in the routing table
var ErrBucketNotFound = errs.New("Bucket not found")

// OverlayError creates class of errors for stack traces
var OverlayError = errs.Class("Overlay Error")

// Cache is used to store overlay data in Redis
type Cache struct {
	DB     storage.KeyValueStore
	DHT    dht.DHT
	StatDB statdb.DB
}

// NewOverlayCache returns a new Cache
func NewOverlayCache(db storage.KeyValueStore, dht dht.DHT, sdb statdb.DB) *Cache {
	return &Cache{DB: db, DHT: dht, StatDB: sdb}
}

// Get looks up the provided nodeID from the overlay cache
func (o *Cache) Get(ctx context.Context, nodeID storj.NodeID) (*pb.Node, error) {
	if nodeID.IsZero() {
		return nil, ErrEmptyNode
	}

	b, err := o.DB.Get(nodeID.Bytes())
	if err != nil {
		if storage.ErrKeyNotFound.Has(err) {
			return nil, ErrNodeNotFound
		}
		return nil, err
	}
	if b == nil {
		return nil, ErrNodeNotFound
	}

	na := &pb.Node{}
	if err := proto.Unmarshal(b, na); err != nil {
		return nil, err
	}
	return na, nil
}

// GetAll looks up the provided nodeIDs from the overlay cache
func (o *Cache) GetAll(ctx context.Context, nodeIDs storj.NodeIDList) ([]*pb.Node, error) {
	if len(nodeIDs) == 0 {
		return nil, OverlayError.New("no nodeIDs provided")
	}
	var ks storage.Keys
	for _, v := range nodeIDs {
		ks = append(ks, v.Bytes())
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
func (o *Cache) Put(ctx context.Context, nodeID storj.NodeID, value pb.Node) error {
	// If we get a Node without an ID (i.e. bootstrap node)
	// we don't want to add to the routing tbale
	if nodeID.IsZero() {
		return nil
	}

	// get existing node rep, or create a new statdb node with 0 rep
	stats, err := o.StatDB.CreateEntryIfNotExists(ctx, nodeID)
	if err != nil {
		return err
	}
	value.Reputation = &pb.NodeStats{
		AuditSuccessRatio:  stats.AuditSuccessRatio,
		AuditSuccessCount:  stats.AuditSuccessCount,
		AuditCount:         stats.AuditCount,
		UptimeRatio:        stats.UptimeRatio,
		UptimeSuccessCount: stats.UptimeSuccessCount,
		UptimeCount:        stats.UptimeCount,
	}

	data, err := proto.Marshal(&value)
	if err != nil {
		return err
	}

	return o.DB.Put(nodeID.Bytes(), data)
}

// Delete will remove the node from the cache. Used when a node hard disconnects or fails
// to pass a PING multiple times.
func (o *Cache) Delete(ctx context.Context, id storj.NodeID) error {
	if id.IsZero() {
		return ErrDelete
	}

	err := o.DB.Delete(id.Bytes())
	if err != nil {
		return ErrDelete
	}

	return nil
}
