// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

const (
	// KademliaBucket is the string representing the bucket used for the kademlia routing table k-bucket ids
	KademliaBucket = "kbuckets"
	// NodeBucket is the string representing the bucket used for the kademlia routing table node ids
	NodeBucket = "nodes"
)

// RoutingErr is the class for all errors pertaining to routing table operations
var RoutingErr = errs.Class("routing table error")

// Bucket IDs exist in the same address space as node IDs
type bucketID [len(storj.NodeID{})]byte

var firstBucketID = bucketID{
	0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF,

	0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF,
}

var emptyBucketID = bucketID{}

// RoutingTableConfig configures the routing table
type RoutingTableConfig struct {
	BucketSize           int `help:"size of each Kademlia bucket" default:"20"`
	ReplacementCacheSize int `help:"size of Kademlia replacement cache" default:"5"`
}

// RoutingTable implements the RoutingTable interface
type RoutingTable struct {
	log              *zap.Logger
	self             pb.Node
	kadBucketDB      storage.KeyValueStore
	nodeBucketDB     storage.KeyValueStore
	transport        *pb.NodeTransport
	mutex            *sync.Mutex
	seen             map[storj.NodeID]*pb.Node
	replacementCache map[bucketID][]*pb.Node
	bucketSize       int // max number of nodes stored in a kbucket = 20 (k)
	rcBucketSize     int // replacementCache bucket max length
}

// NewRoutingTable returns a newly configured instance of a RoutingTable
func NewRoutingTable(logger *zap.Logger, localNode pb.Node, kdb, ndb storage.KeyValueStore, config *RoutingTableConfig) (*RoutingTable, error) {
	localNode.Type.DPanicOnInvalid("new routing table")

	if config == nil || config.BucketSize == 0 || config.ReplacementCacheSize == 0 {
		// TODO: handle this more nicely
		config = &RoutingTableConfig{
			BucketSize:           20,
			ReplacementCacheSize: 5,
		}
	}

	rt := &RoutingTable{
		log:          logger,
		self:         localNode,
		kadBucketDB:  kdb,
		nodeBucketDB: ndb,
		transport:    &defaultTransport,

		mutex:            &sync.Mutex{},
		seen:             make(map[storj.NodeID]*pb.Node),
		replacementCache: make(map[bucketID][]*pb.Node),

		bucketSize:   config.BucketSize,
		rcBucketSize: config.ReplacementCacheSize,
	}
	ok, err := rt.addNode(&localNode)
	if !ok || err != nil {
		return nil, RoutingErr.New("could not add localNode to routing table: %s", err)
	}
	return rt, nil
}

// Close close without closing dependencies
func (rt *RoutingTable) Close() error {
	return nil
}

// Local returns the local nodes ID
func (rt *RoutingTable) Local() pb.Node {
	rt.mutex.Lock()
	defer rt.mutex.Unlock()
	return rt.self
}

// K returns the currently configured maximum of nodes to store in a bucket
func (rt *RoutingTable) K() int {
	return rt.bucketSize
}

// CacheSize returns the total current size of the replacement cache
func (rt *RoutingTable) CacheSize() int {
	return rt.rcBucketSize
}

// GetNodes retrieves nodes within the same kbucket as the given node id
// Note: id doesn't need to be stored at time of search
func (rt *RoutingTable) GetNodes(id storj.NodeID) ([]*pb.Node, bool) {
	bID, err := rt.getKBucketID(id)
	if err != nil {
		return nil, false
	}
	if bID == (bucketID{}) {
		return nil, false
	}
	unmarshaledNodes, err := rt.getUnmarshaledNodesFromBucket(bID)
	if err != nil {
		return nil, false
	}
	return unmarshaledNodes, true
}

// GetBucketIds returns a storage.Keys type of bucket ID's in the Kademlia instance
func (rt *RoutingTable) GetBucketIds() (storage.Keys, error) {
	kbuckets, err := rt.kadBucketDB.List(nil, 0)
	if err != nil {
		return nil, err
	}
	return kbuckets, nil
}

// DumpNodes loops returns all nodes in the nodeBucketDB and marshals them to &pb.Nodes
func (rt *RoutingTable) DumpNodes() ([]*pb.Node, error) {
	var nodes []*pb.Node

	nodeKeys, err := rt.nodeBucketDB.List(nil, 0)
	if err != nil {
		return nil, err
	}

	for _, key := range nodeKeys {
		var id storj.NodeID
		var node = &pb.Node{}

		val, err := rt.nodeBucketDB.Get(key)
		if err != nil {
			continue
		}

		err = id.Unmarshal(key)
		if err != nil {
			fmt.Printf("error unmarshaling node id in DumpNodes %+v\n", err)
			return nil, err
		}

		err = proto.Unmarshal(val, node)
		if err != nil {
			fmt.Printf("error unmarshaling node value in DumpNodes %+v\n", err)
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}

// FindNear returns the node corresponding to the provided nodeID
// returns all Nodes closest via XOR to the provided nodeID up to the provided limit
// always returns limit + self
func (rt *RoutingTable) FindNear(id storj.NodeID, limit int) (nodes []*pb.Node, err error) {
	// if id is not in the routing table
	nodeIDsKeys, err := rt.nodeBucketDB.List(nil, 0)
	if err != nil {
		return nodes, RoutingErr.New("could not get node ids %s", err)
	}
	nodeIDs, err := storj.NodeIDsFromBytes(nodeIDsKeys.ByteSlices())
	if err != nil {
		return nodes, RoutingErr.Wrap(err)
	}
	sortByXOR(nodeIDs, id)
	if len(nodeIDs) >= limit {
		nodeIDs = nodeIDs[:limit]
	}

	nodes, err = rt.getNodesFromIDsBytes(nodeIDs)
	if err != nil {
		return nodes, RoutingErr.New("could not get nodes %s", err)
	}

	return nodes, nil
}

// UpdateSelf updates a node on the routing table
func (rt *RoutingTable) UpdateSelf(node *pb.Node) error {
	// TODO: replace UpdateSelf with UpdateRestrictions and UpdateAddress
	rt.mutex.Lock()
	if node.Id != rt.self.Id {
		rt.mutex.Unlock()
		return RoutingErr.New("self does not have a matching node id")
	}
	rt.self = *node
	rt.seen[node.Id] = node
	rt.mutex.Unlock()

	if err := rt.updateNode(node); err != nil {
		return RoutingErr.New("could not update node %s", err)
	}

	return nil
}

// ConnectionSuccess updates or adds a node to the routing table when
// a successful connection is made to the node on the network
func (rt *RoutingTable) ConnectionSuccess(node *pb.Node) error {
	// valid to connect to node without ID but don't store connection
	if node.Id == (storj.NodeID{}) {
		return nil
	}

	node.Type.DPanicOnInvalid("connection success")

	rt.mutex.Lock()
	rt.seen[node.Id] = node
	rt.mutex.Unlock()
	v, err := rt.nodeBucketDB.Get(storage.Key(node.Id.Bytes()))
	if err != nil && !storage.ErrKeyNotFound.Has(err) {
		return RoutingErr.New("could not get node %s", err)
	}
	if v != nil {
		err = rt.updateNode(node)
		if err != nil {
			return RoutingErr.New("could not update node %s", err)
		}
		return nil
	}
	_, err = rt.addNode(node)
	if err != nil {
		return RoutingErr.New("could not add node %s", err)
	}
	return nil
}

// ConnectionFailed removes a node from the routing table when
// a connection fails for the node on the network
func (rt *RoutingTable) ConnectionFailed(node *pb.Node) error {
	node.Type.DPanicOnInvalid("connection failed")
	err := rt.removeNode(node.Id)
	if err != nil {
		return RoutingErr.New("could not remove node %s", err)
	}
	return nil
}

// SetBucketTimestamp records the time of the last node lookup for a bucket
func (rt *RoutingTable) SetBucketTimestamp(bIDBytes []byte, now time.Time) error {
	rt.mutex.Lock()
	defer rt.mutex.Unlock()
	err := rt.createOrUpdateKBucket(keyToBucketID(bIDBytes), now)
	if err != nil {
		return NodeErr.New("could not update bucket timestamp %s", err)
	}
	return nil
}

// GetBucketTimestamp retrieves time of the last node lookup for a bucket
func (rt *RoutingTable) GetBucketTimestamp(bIDBytes []byte) (time.Time, error) {
	t, err := rt.kadBucketDB.Get(bIDBytes)
	if err != nil {
		return time.Now(), RoutingErr.New("could not get bucket timestamp %s", err)
	}
	timestamp, _ := binary.Varint(t)
	return time.Unix(0, timestamp).UTC(), nil
}

func (rt *RoutingTable) iterate(opts storage.IterateOptions, f func(it storage.Iterator) error) error {
	return rt.nodeBucketDB.Iterate(opts, f)
}

// ConnFailure implements the Transport failure function
func (rt *RoutingTable) ConnFailure(ctx context.Context, node *pb.Node, err error) {
	err2 := rt.ConnectionFailed(node)
	if err2 != nil {
		zap.L().Debug(fmt.Sprintf("error with ConnFailure hook  %+v : %+v", err, err2))
	}
}

// ConnSuccess implements the Transport success function
func (rt *RoutingTable) ConnSuccess(ctx context.Context, node *pb.Node) {
	err := rt.ConnectionSuccess(node)
	if err != nil {
		zap.L().Debug("connection success error:", zap.Error(err))
	}
}
