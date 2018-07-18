// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"bytes"
	"fmt"
	"sync"
	"time"
	"errors"
	protobuf "github.com/golang/protobuf/proto"
	"go.uber.org/zap"
	"storj.io/storj/pkg/dht"
	proto "storj.io/storj/protos/overlay"
	"storj.io/storj/storage"
	"storj.io/storj/storage/boltdb"
)


const (
	//decimal representation of 11111111
	maxByteVal = 255

	//number of bits in a byte
	bitsInByte = 8
)

// RoutingTable implements the RoutingTable interface
// Note: k-bucket and kad bucket are interchangable
type RoutingTable struct {
	Self         *proto.Node
	kadBucketDB  *storage.KeyValueStore
	nodeBucketDB *storage.KeyValueStore
	transport    *proto.NodeTransport
	mutex        *sync.Mutex
	b			 int //kbucket key length 256 bits (SHA256) b = 256
	k			 int // max number of nodes stored in a kbucket k = 20
}

// NewRoutingTable returns a newly configured instance of a RoutingTable
func NewRoutingTable(localNode *proto.Node, kpath string, npath string, b int, k int) (*RoutingTable, error) {
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
		Self:         localNode,
		kadBucketDB:  &kdb,
		nodeBucketDB: &ndb,
		transport:    &defaultTransport,
		mutex:        &sync.Mutex{},
		b:    		  b,
		k:            k,
	}, nil
}


//INTERFACE METHODS -------------------

// Local returns the local node
//TODO
func (rt RoutingTable) Local() proto.Node {
	return proto.Node{}
}

// K returns the currently configured maximum of nodes to store in a bucket
func (rt RoutingTable) K() int {
	return rt.k
}

// CacheSize returns the total current size of the cache
// TODO
func (rt RoutingTable) CacheSize() int {
	return 0
}

// GetBucket retrieves a bucket from the local node
// TODO
func (rt RoutingTable) GetBucket(id string) (dht.Bucket, bool) {
	return &KadBucket{}, true
}

// GetBuckets retrieves all buckets from the local node
// TODO
func (rt RoutingTable) GetBuckets() ([]dht.Bucket, error) {
	return []dht.Bucket{}, nil
}

// FindNear finds all Nodes near the provided nodeID up to the provided limit
// TODO
func (rt RoutingTable) FindNear(id NodeID, limit int) ([]*proto.Node, error) {
	return []*proto.Node{}, nil
}

// ConnectionSuccess handles the details of what kademlia should do when
// a successful connection is made to node on the network
// TODO
func (rt RoutingTable) ConnectionSuccess(id string, address proto.NodeAddress) {
	return
}

// ConnectionFailed handles the details of what kademlia should do when
// a connection fails for a node on the network
// TODO
func (rt RoutingTable) ConnectionFailed(id string, address proto.NodeAddress) {
	return
}

// SetBucketTimestamp updates the last updated time for a bucket
func (rt RoutingTable) SetBucketTimestamp(id string, now time.Time) error {
	//WIP - doesn't use time
	err := rt.createOrUpdateKBucket(storage.Key(id))
	if err != nil {
		return fmt.Errorf("set bucket timestamp: %s", err)
	}
	return nil
}

// GetBucketTimestamp retrieves the last updated time for a bucket
func (rt RoutingTable) GetBucketTimestamp(id string, bucket dht.Bucket) (time.Time, error) {
	//WIP - doesn't use bucket
	pathKey := storage.Key(id)
	val, _ := (*rt.kadBucketDB).Get(pathKey)
	t, err := time.Parse("20060102150405", string(val))
	return t, err
}

//HELPER METHODS -------------------

// addNode attempts to add a new contact to the routing table
// Not sure where this will be used, or if it will be need to be exported
func (rt RoutingTable) addNode(node *proto.Node) error {
	rt.mutex.Lock()
	defer rt.mutex.Unlock()
	key := storage.Key(node.Id)
	value := node
	if node == rt.Self { //change: bytes.Compare actual ids
		err := rt.createOrUpdateKBucket(rt.createFirstBucketID()) //this requires local node to be added first
		if err != nil {
			return err
		}
		err = rt.marshalNode(key, value)
		if err != nil {
			return err
		}
		return nil
	}
	KBucketID, err := rt.getKBucketID(key)
	if err != nil {
		return fmt.Errorf("add node: %s", err)
	}
	hasRoom := rt.kadBucketHasRoom(KBucketID) 
	containsLocal := rt.kadBucketContainsLocalNode(KBucketID)
	withinK := rt.nodeIsWithinNearestK(key)

	for !hasRoom {
		if  containsLocal || withinK {
			depth := rt.determineLeafDepth(KBucketID)
			fmt.Printf("\n depth: %v", depth)
			KBucketID = rt.splitBucket(KBucketID, depth)
			fmt.Printf("bucket after split %v: \n", KBucketID)

			rt.createOrUpdateKBucket(KBucketID)
			KBucketID, _ = rt.getKBucketID(key)
			fmt.Printf("bucket after get %v: \n", KBucketID)
			hasRoom = rt.kadBucketHasRoom(KBucketID)
			containsLocal = rt.kadBucketContainsLocalNode(KBucketID)
		} else { //disregarding node
			return nil
		}
	}
	err = rt.marshalNode(key, value)
	if err != nil {
		return err
	}
	rt.createOrUpdateKBucket(KBucketID)
	return nil
}

func (rt RoutingTable) marshalNode(key storage.Key, value *proto.Node) error {
	val, err := protobuf.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshaling error: %s", err)
	}
	err = (*rt.nodeBucketDB).Put([]byte(key), val)
	if err != nil {
		return fmt.Errorf("add node: %s", err)
	}
	return nil
}

// createOrUpdateKBucket: helper, adds or updates kbucket of id
func (rt RoutingTable) createOrUpdateKBucket(id storage.Key) error {
	dateTime := time.Now().UTC().Format("20060102150405")
	err := (*rt.kadBucketDB).Put([]byte(id), []byte(dateTime))
	if err != nil {
		return fmt.Errorf("add or update k bucket: %s", err)
	}
	return nil
}

// getKBucketID: helper, returns the id of the corresponding k bucket given a node id
func (rt RoutingTable) getKBucketID(id storage.Key) (storage.Key, error) {
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
		if bytes.Compare(id, keys[i]) == 1 && bytes.Compare(keys[i+1], id) > 0 {
			return keys[i+1], nil
		}
	}
	//shouldn't happen BUT return error if no matching kbucket...
	err = errors.New("can't find k bucket")
	return nil, err
}

// nodeIsWithinNearestK: helper, returns true if the node in question is within the nearest k from local node
func (rt RoutingTable) nodeIsWithinNearestK(id storage.Key) bool {
	nodeRange := storage.Limit(rt.k / 2 + 1)
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
	if bytes.Compare(id, smallestLesser) == 1 && bytes.Compare(id, largestGreater) == -1 {
		return true
	} else if bytes.Compare(id, localNodeID) == -1 && len(lesserNodes) <= 1 {
		return true
	} else if bytes.Compare(id, localNodeID) == 1 && len(greaterNodes) <= 1 { 
		return true
	} else {
		return false
	}
}

// kadBucketContainsLocalNode returns true if the kbucket in question contains the local node
func (rt RoutingTable) kadBucketContainsLocalNode(id storage.Key) bool {
	key := storage.Key(rt.Self.Id)
	bucket, _ := rt.getKBucketID(key)
	if bytes.Compare(bucket, id) == 0 {
		return true
	}
	return false
}

// kadBucketHasRoom: helper, returns true if it has fewer than k nodes
func (rt RoutingTable) kadBucketHasRoom(id storage.Key) bool {
	if len(rt.getNodeIDsWithinKBucket(id)) < rt.k {
		return true
	}
	return false
}

// getNodeIDsWithinKBucket: helper, returns a collection of all the node ids contained within the kbucket
func (rt RoutingTable) getNodeIDsWithinKBucket(id storage.Key) storage.Keys {
	endpoints := rt.getKBucketRange(id)
	left := endpoints[0]
	right := endpoints[1]
	var allNodeIDs storage.Keys
	var nodeIDs storage.Keys
	allNodeIDs, _ = (*rt.nodeBucketDB).List(nil, 0)
	for i := 0; i < len(allNodeIDs); i++ {
		if (bytes.Compare(allNodeIDs[i], left) > 0) && (bytes.Compare(allNodeIDs[i], right) <= 0) {
			nodeIDs = append(nodeIDs, allNodeIDs[i])
			if len(nodeIDs) == rt.k {
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
func (rt RoutingTable) getKBucketRange(id storage.Key) storage.Keys {
	key := storage.Key(id)
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
	x := byte(maxByteVal)
	bytesLength := rt.b / bitsInByte
	for i := 0; i < bytesLength; i++ {
		id = append(id, x)
	}
	return id
}

// createZeroAsStorageKey creates storage Key representation of 00..00
func (rt RoutingTable) createZeroAsStorageKey() storage.Key {
	var id []byte
	x := byte(0)
	bytesLength := rt.b / bitsInByte
	for i := 0; i < bytesLength; i++ {
		id = append(id, x)
	}
	return id
}

// determineLeafDepth determines the level of the bucket id in question. 
// Eg level 0 means there is only 1 bucket, level 1 means the bucket has been split once, and so on
func (rt RoutingTable) determineLeafDepth(id storage.Key) int {
	keys, _ := (*rt.kadBucketDB).List(nil, 0)
	firstBucket := rt.createFirstBucketID()
	max := storage.Key(firstBucket)
	mid := rt.splitBucket(firstBucket, 0)
	compareMax := bytes.Compare(id, max)
	compareMid := bytes.Compare(id, mid)
	if len(keys) == 1 {
		return 0
	} else if len(keys) == 2 {
		return 1
	} else if compareMid < 0 || (compareMid > 0 && compareMax < 0) {
		nextKeys, _ := (*rt.kadBucketDB).List(id, 2)
		fmt.Printf("id %v \n", id)
		fmt.Printf("next keys %v \n", nextKeys)
		return determineDifferingBitIndex(id, nextKeys[1]) + 1
	} else if compareMid == 0 || compareMax == 0 {
		prevKeys, _ := (*rt.kadBucketDB).ReverseList(id, 2)
		return determineDifferingBitIndex(id, prevKeys[1]) + 1
	} else {
		return -1
	}
}

// determineDifferingBitIndex: helper, returns the binary tree level of the id in question
func determineDifferingBitIndex(id storage.Key, comparisonID storage.Key) int {
	var xorArr []byte
	var differingBytes []int
	for i := 0; i < len(id); i++ {
		xor := id[i]^comparisonID[i]
		xorArr = append(xorArr, xor)
	}
	for j := 0; j< len(xorArr); j++ {
		if xorArr[j] != byte(0) {
			differingBytes = append(differingBytes, j)
		}
	}
	target := differingBytes[len(differingBytes)-1]
	var h int
	for h = 0; h < bitsInByte; h++ {
		mask := byte(1 << uint(h))
		if mask == xorArr[target] {
			break
		}
	}
	
	bitInByteIndex := 7 - h
	byteIndex := target
	bitIndex := byteIndex * bitsInByte + bitInByteIndex
	return bitIndex
}

// splitBucket: helper, returns the smaller of the two new bucket ids
// the original bucket id becomes the greater of the 2 new
func (rt RoutingTable) splitBucket(id []byte, depth int) []byte {
	newID := make([]byte, len(id))
	copy(newID, id)
	bitIndex := depth
	byteIndex := bitIndex / bitsInByte
	bitInByteIndex := 7 - (bitIndex % bitsInByte)
	toggle := byte(1 << uint(bitInByteIndex))
	newID[byteIndex] ^= toggle	
	return newID
}
