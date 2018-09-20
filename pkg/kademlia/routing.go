// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"encoding/binary"
	"encoding/hex"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/utils"
	"storj.io/storj/storage"
	"storj.io/storj/storage/boltdb"
	"storj.io/storj/storage/storelogger"
)

const (
	// KademliaBucket is the string representing the bucket used for the kademlia routing table k-bucket ids
	KademliaBucket = "kbuckets"
	// NodeBucket is the string representing the bucket used for the kademlia routing table node ids
	NodeBucket = "nodes"
)

// RoutingErr is the class for all errors pertaining to routing table operations
var RoutingErr = errs.Class("routing table error")

// RoutingTable implements the RoutingTable interface
type RoutingTable struct {
	self             *pb.Node
	kadBucketDB      storage.KeyValueStore
	nodeBucketDB     storage.KeyValueStore
	transport        *pb.NodeTransport
	mutex            *sync.Mutex
	replacementCache map[string][]*pb.Node
	idLength         int // kbucket and node id bit length (SHA256) = 256
	bucketSize       int // max number of nodes stored in a kbucket = 20 (k)
	rcBucketSize     int // replacementCache bucket max length
}

//RoutingOptions for configuring RoutingTable
type RoutingOptions struct {
	kpath        string
	npath        string
	idLength     int //TODO (JJ): add checks for > 0
	bucketSize   int
	rcBucketSize int
}

// NewRoutingTable returns a newly configured instance of a RoutingTable
func NewRoutingTable(localNode *pb.Node, options *RoutingOptions) (*RoutingTable, error) {
	kdb, err := boltdb.New(options.kpath, KademliaBucket)
	if err != nil {
		return nil, RoutingErr.New("could not create kadBucketDB: %s", err)
	}

	ndb, err := boltdb.New(options.npath, NodeBucket)
	if err != nil {
		return nil, RoutingErr.New("could not create nodeBucketDB: %s", err)
	}
	rp := make(map[string][]*pb.Node)
	rt := &RoutingTable{
		self:             localNode,
		kadBucketDB:      storelogger.New(zap.L(), kdb),
		nodeBucketDB:     storelogger.New(zap.L(), ndb),
		transport:        &defaultTransport,
		mutex:            &sync.Mutex{},
		replacementCache: rp,
		idLength:         options.idLength,
		bucketSize:       options.bucketSize,
		rcBucketSize:     options.rcBucketSize,
	}
	ok, err := rt.addNode(localNode)
	if !ok || err != nil {
		return nil, RoutingErr.New("could not add localNode to routing table: %s", err)
	}
	return rt, nil
}

// Close closes underlying databases
func (rt *RoutingTable) Close() error {
	kerr := rt.kadBucketDB.Close()
	nerr := rt.nodeBucketDB.Close()
	return utils.CombineErrors(kerr, nerr)
}

// Local returns the local nodes ID
func (rt *RoutingTable) Local() pb.Node {
	return *rt.self
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
func (rt *RoutingTable) GetBucket(id string) (bucket dht.Bucket, ok bool) {
	i, err := hex.DecodeString(id)
	if err != nil {
		return &KBucket{}, false
	}
	bucketID, err := rt.getKBucketID(i)
	if err != nil {
		return &KBucket{}, false
	}
	if bucketID == nil {
		return &KBucket{}, false
	}
	unmarshaledNodes, err := rt.getUnmarshaledNodesFromBucket(bucketID)
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
		unmarshaledNodes, err := rt.getUnmarshaledNodesFromBucket(v)
		if err != nil {
			return bs, err
		}
		bs = append(bs, &KBucket{nodes: unmarshaledNodes})
	}
	return bs, nil
}

// FindNear returns the node corresponding to the provided nodeID
// returns all Nodes closest via XOR to the provided nodeID up to the provided limit
// always returns limit + self
func (rt *RoutingTable) FindNear(id dht.NodeID, limit int) ([]*pb.Node, error) {
	// if id is not in the routing table
	nodeIDs, err := rt.nodeBucketDB.List(nil, 0)
	if err != nil {
		return []*pb.Node{}, RoutingErr.New("could not get node ids %s", err)
	}

	sortedIDs := sortByXOR(nodeIDs, id.Bytes())
	if len(sortedIDs) >= limit {
		sortedIDs = sortedIDs[:limit]
	}
	ids, serializedNodes, err := rt.getNodesFromIDs(sortedIDs)
	if err != nil {
		return []*pb.Node{}, RoutingErr.New("could not get nodes %s", err)
	}

	unmarshaledNodes, err := unmarshalNodes(ids, serializedNodes)
	if err != nil {
		return []*pb.Node{}, RoutingErr.New("could not unmarshal nodes %s", err)
	}

	return unmarshaledNodes, nil
}

// ConnectionSuccess updates or adds a node to the routing table when
// a successful connection is made to the node on the network
func (rt *RoutingTable) ConnectionSuccess(node *pb.Node) error {
	v, err := rt.nodeBucketDB.Get(storage.Key(node.Id))
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
	nodeID := storage.Key(node.Id)
	bucketID, err := rt.getKBucketID(nodeID)
	if err != nil {
		return RoutingErr.New("could not get k bucket %s", err)
	}
	err = rt.removeNode(bucketID, nodeID)
	if err != nil {
		return RoutingErr.New("could not remove node %s", err)
	}
	return nil
}

// SetBucketTimestamp updates the last updated time for a bucket
func (rt *RoutingTable) SetBucketTimestamp(id string, now time.Time) error {
	rt.mutex.Lock()
	defer rt.mutex.Unlock()
	err := rt.createOrUpdateKBucket([]byte(id), now)
	if err != nil {
		return NodeErr.New("could not update bucket timestamp %s", err)
	}
	return nil
}

// GetBucketTimestamp retrieves the last updated time for a bucket
func (rt *RoutingTable) GetBucketTimestamp(id string, bucket dht.Bucket) (time.Time, error) {
	t, err := rt.kadBucketDB.Get([]byte(id))
	if err != nil {
		return time.Now(), RoutingErr.New("could not get bucket timestamp %s", err)
	}

	timestamp, _ := binary.Varint(t)

	return time.Unix(0, timestamp).UTC(), nil
}
