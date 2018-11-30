// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"encoding/binary"
	"sync"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/utils"
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

// RoutingTable implements the RoutingTable interface
type RoutingTable struct {
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
func NewRoutingTable(localNode pb.Node, kdb, ndb storage.KeyValueStore) (*RoutingTable, error) {
	rt := &RoutingTable{
		self:         localNode,
		kadBucketDB:  kdb,
		nodeBucketDB: ndb,
		transport:    &defaultTransport,

		mutex:            &sync.Mutex{},
		seen:             make(map[storj.NodeID]*pb.Node),
		replacementCache: make(map[bucketID][]*pb.Node),

		bucketSize:   *flagBucketSize,
		rcBucketSize: *flagReplacementCacheSize,
	}
	ok, err := rt.addNode(&localNode)
	if !ok || err != nil {
		return nil, RoutingErr.New("could not add localNode to routing table: %s", err)
	}
	return rt, nil
}

// Close closes underlying databases
func (rt *RoutingTable) Close() error {
	return utils.CombineErrors(
		rt.kadBucketDB.Close(),
		rt.nodeBucketDB.Close(),
	)
}

// Local returns the local nodes ID
func (rt *RoutingTable) Local() pb.Node {
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

// GetBucket retrieves the corresponding kbucket from node id
// Note: id doesn't need to be stored at time of search
func (rt *RoutingTable) GetBucket(id storj.NodeID) (bucket dht.Bucket, ok bool) {
	bID, err := rt.getKBucketID(id)
	if err != nil {
		return &KBucket{}, false
	}
	if bID == (bucketID{}) {
		return &KBucket{}, false
	}
	unmarshaledNodes, err := rt.getUnmarshaledNodesFromBucket(bID)
	if err != nil {
		return &KBucket{}, false
	}
	return &KBucket{nodes: unmarshaledNodes}, true
}

// GetBuckets retrieves all buckets from the local node
func (rt *RoutingTable) GetBuckets() (k []dht.Bucket, err error) {
	bs := []dht.Bucket{}
	kbuckets, err := rt.kadBucketDB.List(nil, 0)
	if err != nil {
		return bs, RoutingErr.New("could not get bucket ids %s", err)
	}
	for _, v := range kbuckets {
		unmarshaledNodes, err := rt.getUnmarshaledNodesFromBucket(keyToBucketID(v))
		if err != nil {
			return bs, err
		}
		bs = append(bs, &KBucket{nodes: unmarshaledNodes})
	}
	return bs, nil
}

// GetBucketIds returns a storage.Keys type of bucket ID's in the Kademlia instance
func (rt *RoutingTable) GetBucketIds() (storage.Keys, error) {
	kbuckets, err := rt.kadBucketDB.List(nil, 0)
	if err != nil {
		return nil, err
	}
	return kbuckets, nil
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
	sortByXOR(nodeIDsKeys, id.Bytes())
	if len(nodeIDsKeys) >= limit {
		nodeIDsKeys = nodeIDsKeys[:limit]
	}
	nodeIDs, err := storj.NodeIDsFromBytes(nodeIDsKeys.ByteSlices())
	if err != nil {
		return nodes, RoutingErr.Wrap(err)
	}

	nodes, err = rt.getNodesFromIDsBytes(nodeIDs)
	if err != nil {
		return nodes, RoutingErr.New("could not get nodes %s", err)
	}

	return nodes, nil
}

// ConnectionSuccess updates or adds a node to the routing table when
// a successful connection is made to the node on the network
func (rt *RoutingTable) ConnectionSuccess(node *pb.Node) error {
	// valid to connect to node without ID but don't store connection
	if node.Id == (storj.NodeID{}) {
		return nil
	}

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
	err := rt.removeNode(node.Id)
	if err != nil {
		return RoutingErr.New("could not remove node %s", err)
	}
	return nil
}

// SetBucketTimestamp updates the last updated time for a bucket
func (rt *RoutingTable) SetBucketTimestamp(bIDBytes []byte, now time.Time) error {
	rt.mutex.Lock()
	defer rt.mutex.Unlock()
	err := rt.createOrUpdateKBucket(keyToBucketID(bIDBytes), now)
	if err != nil {
		return NodeErr.New("could not update bucket timestamp %s", err)
	}
	return nil
}

// GetBucketTimestamp retrieves the last updated time for a bucket
func (rt *RoutingTable) GetBucketTimestamp(bIDBytes []byte, bucket dht.Bucket) (time.Time, error) {
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
