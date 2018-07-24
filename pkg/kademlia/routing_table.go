// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	pb "github.com/golang/protobuf/proto"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	proto "storj.io/storj/protos/overlay"
	"storj.io/storj/storage"
	"storj.io/storj/storage/boltdb"
)

// RoutingErr is the class for all errors pertaining to routing table operations
var RoutingErr = errs.Class("routing table error")

// RoutingTable implements the RoutingTable interface
type RoutingTable struct {
	self          *proto.Node
	kadBucketDB   storage.KeyValueStore
	nodeBucketDB  storage.KeyValueStore
	transport     *proto.NodeTransport
	mutex         *sync.Mutex
	idLength      int // kbucket and node id bit length (SHA256) = 256
	bucketSize    int // max number of nodes stored in a kbucket = 20 (k)
	nearestKNodes storage.Keys
}

// NewRoutingTable returns a newly configured instance of a RoutingTable
func NewRoutingTable(localNode *proto.Node, kpath string, npath string, idLength int, bucketSize int) (*RoutingTable, error) {
	logger := zap.L()
	kdb, err := boltdb.NewClient(logger, kpath, boltdb.KBucket)
	if err != nil {
		return nil, RoutingErr.New("could not create kadBucketDB: %s", err)
	}
	ndb, err := boltdb.NewClient(logger, npath, boltdb.NodeBucket)
	if err != nil {
		return nil, RoutingErr.New("could not create nodeBucketDB: %s", err)
	}
	return &RoutingTable{
		self:          localNode,
		kadBucketDB:   kdb,
		nodeBucketDB:  ndb,
		transport:     &defaultTransport,
		mutex:         &sync.Mutex{},
		idLength:      idLength,
		bucketSize:    bucketSize,
		nearestKNodes: storage.Keys{},
	}, nil
}

// addNode attempts to add a new contact to the routing table
// Note: Local Node must be added to the routing table first
// Requires node not already in table
func (rt RoutingTable) addNode(node *proto.Node) error {
	rt.mutex.Lock()
	defer rt.mutex.Unlock()
	nodeKey := storage.Key(node.Id)
	if bytes.Equal(nodeKey, storage.Key(rt.self.Id)) {
		err := rt.createOrUpdateKBucket(rt.createFirstBucketID(), time.Now())
		if err != nil {
			return RoutingErr.New("could not create initial K bucket: %s", err)
		}
		nodeValue, err := rt.marshalNode(node)
		if err != nil {
			return RoutingErr.New("could not marshal initial node: %s", err)
		}
		err = rt.putNode(nodeKey, nodeValue)
		if err != nil {
			return RoutingErr.New("could not add initial node to nodeBucketDB: %s", err)
		}
		return nil
	}
	kadBucketID, err := rt.getKBucketID(nodeKey)
	if err != nil {
		return RoutingErr.New("could not getKBucketID: %s", err)
	}
	hasRoom, err := rt.kadBucketHasRoom(kadBucketID)
	if err != nil {
		return err
	}
	containsLocal, err := rt.kadBucketContainsLocalNode(kadBucketID)
	if err != nil {
		return err
	}
	withinK := rt.nodeIsWithinNearestK(nodeKey)
	z := 0
	for !hasRoom && z < 10 {
		z++
		fmt.Printf("kbucket after get %v\n", kadBucketID)
		if containsLocal || withinK {
			depth, err := rt.determineLeafDepth(kadBucketID)
			fmt.Printf("depth %v\n", depth)
			if err != nil {
				return RoutingErr.New("could not determine leaf depth: %s", err)
			}
			fmt.Printf("kbucket before split %v\n", kadBucketID)

			kadBucketID = rt.splitBucket(kadBucketID, depth)
			fmt.Printf("kbucket after split %v\n", kadBucketID)
			err = rt.createOrUpdateKBucket(kadBucketID, time.Now())
			if err != nil {
				return RoutingErr.New("could not split and create K bucket: %s", err)
			}
			kadBucketID, err = rt.getKBucketID(nodeKey)
			fmt.Printf("kbucket after get %v\n", kadBucketID)
			if err != nil {
				return RoutingErr.New("could not get k bucket Id within add node split bucket checks: %s", err)
			}
			hasRoom, err = rt.kadBucketHasRoom(kadBucketID)
			if err != nil {
				return err
			}
			containsLocal, err = rt.kadBucketContainsLocalNode(kadBucketID)
			if err != nil {
				return err
			}
			fmt.Printf("kbucket at end %v\n", kadBucketID)

		} else {
			return nil
		}
	}
	nodeValue, err := rt.marshalNode(node)
	if err != nil {
		return RoutingErr.New("could not marshal node: %s", err)
	}
	err = rt.putNode(nodeKey, nodeValue)
	if err != nil {
		return RoutingErr.New("could not add node to nodeBucketDB: %s", err)
	}
	err = rt.createOrUpdateKBucket(kadBucketID, time.Now())
	if err != nil {
		return RoutingErr.New("could not create or update K bucket: %s", err)
	}
	return nil
}

// nodeAlreadyExists will return true if the given node ID exists within nodeBucketDB
func (rt RoutingTable) nodeAlreadyExists(nodeID storage.Key) (bool, error) {
	node, err := rt.nodeBucketDB.Get(nodeID)
	if err != nil {
		return false, err
	}
	if node == nil {
		return false, nil
	}
	return true, nil
}

// updateNode will update the node information given that
// the node is already in the routing table.
func (rt RoutingTable) updateNode(node *proto.Node) error {
	//TODO (JJ)
	return nil
}

// removeNode will remove nodes and replace those entries with nodes
// from the replacement cache.
// We want to replace churned nodes (no longer online)
func (rt RoutingTable) removeNode(nodeID storage.Key) error {
	//TODO (JJ)
	return nil
}

// marshalNode: helper, sanitizes proto Node for db insertion
func (rt RoutingTable) marshalNode(node *proto.Node) ([]byte, error) {
	nodeVal, err := pb.Marshal(node)
	if err != nil {
		return nil, RoutingErr.New("could not marshal proto node: %s", err)
	}
	return nodeVal, nil
}

// putNode: helper, adds proto Node and ID to nodeBucketDB
func (rt RoutingTable) putNode(nodeKey storage.Key, nodeValue storage.Value) error {
	err := rt.nodeBucketDB.Put(nodeKey, nodeValue)
	if err != nil {
		return RoutingErr.New("could not add key value pair to nodeBucketDB: %s", err)
	}
	return nil
}

// createOrUpdateKBucket: helper, adds or updates given kbucket
func (rt RoutingTable) createOrUpdateKBucket(bucketID storage.Key, now time.Time) error {
	dateTime := string(now.UTC().UnixNano())
	err := rt.kadBucketDB.Put(bucketID, []byte(dateTime))
	if err != nil {
		return RoutingErr.New("could not add or update k bucket: %s", err)
	}
	return nil
}

// getKBucketID: helper, returns the id of the corresponding k bucket given a node id
func (rt RoutingTable) getKBucketID(nodeID storage.Key) (storage.Key, error) {
	kadBucketIDs, err := rt.kadBucketDB.List(nil, 0)
	if err != nil {
		return nil, RoutingErr.New("could not list all k bucket ids: %s", err)
	}
	smallestKey := rt.createZeroAsStorageKey()
	var keys storage.Keys
	keys = append(keys, smallestKey)
	keys = append(keys, kadBucketIDs...)

	for i := 0; i < len(keys)-1; i++ {
		if bytes.Compare(nodeID, keys[i]) > 0 && bytes.Compare(nodeID, keys[i+1]) <= 0 {
			return keys[i+1], nil
		}
	}
	//shouldn't happen BUT return error if no matching kbucket...
	return nil, RoutingErr.New("could not find k bucket")
}

func (rt RoutingTable) updateNearestKNodes() {

}

// nodeIsWithinNearestK: helper, returns true if the node in question is within the nearest k from local node
func (rt RoutingTable) nodeIsWithinNearestK(nodeID storage.Key) bool {
	nodeRange := storage.Limit(rt.bucketSize/2 + 1)
	localNodeID := storage.Key(rt.self.Id)
	lesserNodes, _ := rt.nodeBucketDB.ReverseList(localNodeID, nodeRange)
	greaterNodes, _ := rt.nodeBucketDB.List(localNodeID, nodeRange)
	smallestLesser := localNodeID
	largestGreater := localNodeID
	if len(lesserNodes) > 1 {
		smallestLesser = lesserNodes[len(lesserNodes)-1]
	}
	if len(greaterNodes) > 1 {
		largestGreater = greaterNodes[len(greaterNodes)-1]
	}
	if bytes.Compare(nodeID, smallestLesser) == 1 && bytes.Compare(nodeID, largestGreater) == -1 {
		return true
	} else if bytes.Compare(nodeID, localNodeID) == -1 && len(lesserNodes) <= 1 {
		return true
	} else if bytes.Compare(nodeID, localNodeID) == 1 && len(greaterNodes) <= 1 {
		return true
	} else {
		return false
	}
}

// kadBucketContainsLocalNode returns true if the kbucket in question contains the local node
func (rt RoutingTable) kadBucketContainsLocalNode(bucketID storage.Key) (bool, error) {
	key := storage.Key(rt.self.Id)
	bucket, err := rt.getKBucketID(key)
	if err != nil {
		return false, err
	}
	if bytes.Compare(bucket, bucketID) == 0 {
		return true, nil
	}
	return false, nil
}

// kadBucketHasRoom: helper, returns true if it has fewer than k nodes
func (rt RoutingTable) kadBucketHasRoom(bucketID storage.Key) (bool, error) {
	nodes, err := rt.getNodeIDsWithinKBucket(bucketID)
	if err != nil {
		return false, err
	}
	if len(nodes) < rt.bucketSize {
		return true, nil
	}
	return false, nil
}

// getNodeIDsWithinKBucket: helper, returns a collection of all the node ids contained within the kbucket
func (rt RoutingTable) getNodeIDsWithinKBucket(bucketID storage.Key) (storage.Keys, error) {
	endpoints, err := rt.getKBucketRange(bucketID)
	if err != nil {
		return nil, err
	}
	left := endpoints[0]
	right := endpoints[1]
	var nodeIDs storage.Keys
	allNodeIDs, err := rt.nodeBucketDB.List(nil, 0)
	if err != nil {
		return nil, RoutingErr.New("could not list nodes %s", err)
	}
	for _, v := range allNodeIDs {
		if (bytes.Compare(v, left) > 0) && (bytes.Compare(v, right) <= 0) {
			nodeIDs = append(nodeIDs, v)
			if len(nodeIDs) == rt.bucketSize {
				break
			}
		}
	}
	if len(nodeIDs) > 0 {
		return nodeIDs, nil
	}
	return nil, nil
}

// getKBucketRange: helper, returns the left and right endpoints of the range of node ids contained within the bucket
func (rt RoutingTable) getKBucketRange(bucketID storage.Key) (storage.Keys, error) {
	key := storage.Key(bucketID)
	kadIDs, err := rt.kadBucketDB.ReverseList(key, 2)
	if err != nil {
		return nil, RoutingErr.New("could not reverse list k bucket ids %s", err)
	}
	coords := make(storage.Keys, 2)
	if len(kadIDs) < 2 {
		coords[0] = rt.createZeroAsStorageKey()
	} else {
		coords[0] = kadIDs[1]
	}
	coords[1] = kadIDs[0]
	return coords, nil
}

// createFirstBucketID creates byte slice representing 11..11
func (rt RoutingTable) createFirstBucketID() []byte {
	var id []byte
	x := byte(255)
	bytesLength := rt.idLength / 8
	for i := 0; i < bytesLength; i++ {
		id = append(id, x)
	}
	return id
}

// createZeroAsStorageKey creates storage Key representation of 00..00
func (rt RoutingTable) createZeroAsStorageKey() storage.Key {
	var id []byte
	x := byte(0)
	bytesLength := rt.idLength / 8
	for i := 0; i < bytesLength; i++ {
		id = append(id, x)
	}
	return id
}

// determineLeafDepth determines the level of the bucket id in question.
// Eg level 0 means there is only 1 bucket, level 1 means the bucket has been split once, and so on
func (rt RoutingTable) determineLeafDepth(bucketID storage.Key) (int, error) {
	bucketRange, err := rt.getKBucketRange(bucketID)
	if err != nil {
		return -1, RoutingErr.New("could not get k bucket range %s", err)
	}
	smaller := bucketRange[0]
	fmt.Print(smaller)
	diffBit, err := rt.determineDifferingBitIndex(bucketID, smaller)
	fmt.Printf("diffBit %v\n                                                                                                  ", diffBit)
	if err != nil {
		return diffBit + 1, RoutingErr.New("could not determine differing bit %s", err)
	}
	return diffBit + 1, nil
}

// determineDifferingBitIndex: helper, returns the last bit differs starting from prefix to suffix
func (rt RoutingTable) determineDifferingBitIndex(bucketID storage.Key, comparisonID storage.Key) (int, error) {
	if bytes.Equal(bucketID, comparisonID) {
		return -2, RoutingErr.New("compared two equivalent k bucket ids")
	}
	if bytes.Equal(comparisonID, rt.createZeroAsStorageKey()) {
		comparisonID = rt.createFirstBucketID()
	}

	var xorArr []byte
	var differingByteIndex int
	var differingByteXor byte
	for i := 0; i < len(bucketID); i++ {
		xor := bucketID[i] ^ comparisonID[i]
		xorArr = append(xorArr, xor)
	}
	if bytes.Equal(xorArr, rt.createFirstBucketID()) {
		return -1, nil
	}

	for j, v := range xorArr {
		if v != byte(0) {
			differingByteIndex = j
			differingByteXor = v
			break
		}
	}

	h := 0
	for ; h < 8; h++ {
		toggle := byte(1 << uint(h))
		tempXor := differingByteXor
		// fmt.Printf("%b\n", tempXor)
		tempXor ^= toggle
		// fmt.Printf("%b\n", tempXor)
		if tempXor < differingByteXor {
			break
		}

	}
	bitInByteIndex := 7 - h
	byteIndex := differingByteIndex
	bitIndex := byteIndex*8 + bitInByteIndex

	return bitIndex, nil
}

// splitBucket: helper, returns the smaller of the two new bucket ids
// the original bucket id becomes the greater of the 2 new
func (rt RoutingTable) splitBucket(bucketID []byte, depth int) []byte {
	newID := make([]byte, len(bucketID))
	copy(newID, bucketID)
	bitIndex := depth
	byteIndex := bitIndex / 8
	bitInByteIndex := 7 - (bitIndex % 8)
	toggle := byte(1 << uint(bitInByteIndex))
	newID[byteIndex] ^= toggle
	return newID
}
