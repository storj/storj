// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"bytes"
	"fmt"
	"sync"
	"time"
	"errors"

	pb "github.com/golang/protobuf/proto"
	"go.uber.org/zap"
	
	proto "storj.io/storj/protos/overlay"
	"storj.io/storj/storage"
	"storj.io/storj/storage/boltdb"
)


// RoutingTable implements the RoutingTable interface
// Note: k-bucket and kad bucket are interchangable
type RoutingTable struct {
	Self         	*proto.Node
	kadBucketDB  	*storage.KeyValueStore
	nodeBucketDB 	*storage.KeyValueStore
	transport    	*proto.NodeTransport
	mutex        	*sync.Mutex
	idLength	 	int // kbucket and node id bit length (SHA256) = 256
	maxBucketSize   int // max number of nodes stored in a kbucket = 20 (k)
}

// NewRoutingTable returns a newly configured instance of a RoutingTable
func NewRoutingTable(localNode *proto.Node, kpath string, npath string, idLength int, maxBucketSize int) (*RoutingTable, error) {
	logger, _ := zap.NewDevelopment()
	kdb, err := boltdb.NewClient(logger, kpath, boltdb.KBucket)
	if err != nil {
		return nil, fmt.Errorf("create KBucket bucket: %s", err)
	}
	ndb, err := boltdb.NewClient(logger, npath, boltdb.NodeBucket)
	if err != nil {
		return nil, fmt.Errorf("create NodeBucket bucket: %s", err)
	}
	return &RoutingTable{
		Self:         	localNode,
		kadBucketDB:  	&kdb,
		nodeBucketDB: 	&ndb,
		transport:    	&defaultTransport,
		mutex:        	&sync.Mutex{},
		idLength:     	idLength,
		maxBucketSize:  maxBucketSize,
	}, nil
}

// addNode attempts to add a new contact to the routing table
// Not sure where this will be used, or if it will be need to be exported
// Note: Local Node must be added to the routing table first
func (rt RoutingTable) addNode(node *proto.Node) error {
	rt.mutex.Lock()
	defer rt.mutex.Unlock()
	nodeKey := storage.Key(node.Id)
	nodeValue := node
	if bytes.Equal(nodeKey, storage.Key(rt.Self.Id)) {
		err := rt.createOrUpdateKBucket(rt.createFirstBucketID())
		if err != nil {
			return err
		}
		err = rt.marshalNode(nodeKey, nodeValue)
		if err != nil {
			return err
		}
		return nil
	}
	kadBucketID, err := rt.getKBucketID(nodeKey)
	if err != nil {
		return fmt.Errorf("could not get k bucket id within add node: %s", err)
	}
	hasRoom := rt.kadBucketHasRoom(kadBucketID) 
	containsLocal := rt.kadBucketContainsLocalNode(kadBucketID)
	withinK := rt.nodeIsWithinNearestK(nodeKey)

	for !hasRoom {
		if  containsLocal || withinK {
			depth := rt.determineLeafDepth(kadBucketID)
			kadBucketID = rt.splitBucket(kadBucketID, depth)
			rt.createOrUpdateKBucket(kadBucketID)
			kadBucketID, err = rt.getKBucketID(nodeKey)
			if err != nil {
				return fmt.Errorf("could not get k bucket Id within add node split bucket checks: %s", err)
			}
			hasRoom = rt.kadBucketHasRoom(kadBucketID)
			containsLocal = rt.kadBucketContainsLocalNode(kadBucketID)
		} else { //disregarding node
			return nil
		}
	}
	err = rt.marshalNode(nodeKey, nodeValue)
	if err != nil {
		return err
	}
	rt.createOrUpdateKBucket(kadBucketID)
	return nil
}

func (rt RoutingTable) marshalNode(nodeKey storage.Key, nodeValue *proto.Node) error {
	val, err := pb.Marshal(nodeValue)
	if err != nil {
		return fmt.Errorf("marshaling error: %s", err)
	}
	err = (*rt.nodeBucketDB).Put(nodeKey, val)
	if err != nil {
		return fmt.Errorf("put error within marshal node: %s", err)
	}
	return nil
}

// createOrUpdateKBucket: helper, adds or updates given kbucket
func (rt RoutingTable) createOrUpdateKBucket(kadID storage.Key) error {
	dateTime := time.Now().UTC().Format("20060102150405")
	err := (*rt.kadBucketDB).Put(kadID, []byte(dateTime))
	if err != nil {
		return fmt.Errorf("add or update k bucket: %s", err)
	}
	return nil
}

// getKBucketID: helper, returns the id of the corresponding k bucket given a node id
func (rt RoutingTable) getKBucketID(nodeID storage.Key) (storage.Key, error) {
	kadBucketIDs, err := (*rt.kadBucketDB).List(nil, 0)
	if err != nil {
		return nil, fmt.Errorf("get k bucket id: %s", err)
	}
	smallestKey := rt.createZeroAsStorageKey()
	var keys storage.Keys
	keys = append(keys, smallestKey)
	for j := 0; j < len(kadBucketIDs); j++ {
		keys = append(keys, kadBucketIDs[j])
	}
	for i := 0; i < len(keys)-1; i++ {
		if bytes.Compare(nodeID, keys[i]) > 0 && bytes.Compare(nodeID, keys[i+1]) <= 0 {
			return keys[i+1], nil
		}
	}
	//shouldn't happen BUT return error if no matching kbucket...
	err = errors.New("can't find k bucket")
	return nil, err
}

// nodeIsWithinNearestK: helper, returns true if the node in question is within the nearest k from local node
func (rt RoutingTable) nodeIsWithinNearestK(nodeID storage.Key) bool {
	nodeRange := storage.Limit(rt.maxBucketSize / 2 + 1)
	localNodeID := storage.Key(rt.Self.Id)
	lesserNodes, _ := (*rt.nodeBucketDB).ReverseList(localNodeID, nodeRange)
	greaterNodes, _ := (*rt.nodeBucketDB).List(localNodeID, nodeRange)
	smallestLesser := localNodeID
	largestGreater := localNodeID
	if len(lesserNodes) > 1 {
		smallestLesser = lesserNodes[len(lesserNodes) - 1] 
	}
	if len(greaterNodes) > 1 {
		largestGreater = greaterNodes[len(greaterNodes) - 1] 
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
func (rt RoutingTable) kadBucketContainsLocalNode(kadID storage.Key) bool {
	key := storage.Key(rt.Self.Id)
	bucket, _ := rt.getKBucketID(key)
	if bytes.Compare(bucket, kadID) == 0 {
		return true
	}
	return false
}

// kadBucketHasRoom: helper, returns true if it has fewer than k nodes
func (rt RoutingTable) kadBucketHasRoom(kadID storage.Key) bool {
	if len(rt.getNodeIDsWithinKBucket(kadID)) < rt.maxBucketSize {
		return true
	}
	return false
}

// getNodeIDsWithinKBucket: helper, returns a collection of all the node ids contained within the kbucket
func (rt RoutingTable) getNodeIDsWithinKBucket(kadID storage.Key) storage.Keys {
	endpoints := rt.getKBucketRange(kadID)
	left := endpoints[0]
	right := endpoints[1]
	var allNodeIDs storage.Keys
	var nodeIDs storage.Keys
	allNodeIDs, _ = (*rt.nodeBucketDB).List(nil, 0)
	for i := 0; i < len(allNodeIDs); i++ {
		if (bytes.Compare(allNodeIDs[i], left) > 0) && (bytes.Compare(allNodeIDs[i], right) <= 0) {
			nodeIDs = append(nodeIDs, allNodeIDs[i])
			if len(nodeIDs) == rt.maxBucketSize {
				break
			}
		}
	}
	if len(nodeIDs) > 0 {
		return nodeIDs
	}
	return nil
}

// getKBucketRange: helper, returns the left and right endpoints of the range of node ids contained within the bucket
func (rt RoutingTable) getKBucketRange(kadID storage.Key) storage.Keys {
	key := storage.Key(kadID)
	kadIDs, _ := (*rt.kadBucketDB).ReverseList(key, 2)
	coords := make(storage.Keys, 2)
	if len(kadIDs) < 2 {
		coords[0] = rt.createZeroAsStorageKey()
	} else {
		coords[0] = kadIDs[1]
	}
	coords[1] = kadIDs[0]
	return coords
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
func (rt RoutingTable) determineLeafDepth(kadID storage.Key) int {
	keys, _ := (*rt.kadBucketDB).List(nil, 0)
	firstBucket := rt.createFirstBucketID()
	max := storage.Key(firstBucket)
	mid := rt.splitBucket(firstBucket, 0)
	compareMax := bytes.Compare(kadID, max)
	compareMid := bytes.Compare(kadID, mid)
	if len(keys) == 1 {
		return 0
	} else if len(keys) == 2 {
		return 1
	} else if compareMid < 0 || (compareMid > 0 && compareMax < 0) {
		nextKeys, _ := (*rt.kadBucketDB).List(kadID, 2)
		return determineDifferingBitIndex(kadID, nextKeys[1]) + 1
	} else if compareMid == 0 || compareMax == 0 {
		prevKeys, _ := (*rt.kadBucketDB).ReverseList(kadID, 2)
		return determineDifferingBitIndex(kadID, prevKeys[1]) + 1
	} else {
		return -1
	}
}

// determineDifferingBitIndex: helper, returns the binary tree level of the id in question
func determineDifferingBitIndex(kadID storage.Key, comparisonID storage.Key) int {
	var xorArr []byte
	var differingBytes []int
	for i := 0; i < len(kadID); i++ {
		xor := kadID[i]^comparisonID[i]
		xorArr = append(xorArr, xor)
	}
	for j := 0; j< len(xorArr); j++ {
		if xorArr[j] != byte(0) {
			differingBytes = append(differingBytes, j)
		}
	}
	target := differingBytes[len(differingBytes)-1]
	var h int
	for h = 0; h < 8; h++ {
		mask := byte(1 << uint(h))
		if mask == xorArr[target] {
			break
		}
	}
	
	bitInByteIndex := 7 - h
	byteIndex := target
	bitIndex := byteIndex * 8 + bitInByteIndex
	return bitIndex
}

// splitBucket: helper, returns the smaller of the two new bucket ids
// the original bucket id becomes the greater of the 2 new
func (rt RoutingTable) splitBucket(kadID []byte, depth int) []byte {
	newID := make([]byte, len(kadID))
	copy(newID, kadID)
	bitIndex := depth
	byteIndex := bitIndex / 8
	bitInByteIndex := 7 - (bitIndex % 8)
	toggle := byte(1 << uint(bitInByteIndex))
	newID[byteIndex] ^= toggle	
	return newID
}
