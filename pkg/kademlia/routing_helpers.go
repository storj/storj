// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"bytes"
	"math/rand"
	"encoding/binary"
	"time"

	pb "github.com/golang/protobuf/proto"

	proto "storj.io/storj/protos/overlay"
	"storj.io/storj/storage"
)

// addNode attempts to add a new contact to the routing table
// Requires node not already in table
// Returns true if node was added successfully
func (rt *RoutingTable) addNode(node *proto.Node) (bool, error) {
	rt.mutex.Lock()
	defer rt.mutex.Unlock()
	nodeKey := storage.Key(node.Id)
	if bytes.Equal(nodeKey, storage.Key(rt.self.Id)) {
		err := rt.createOrUpdateKBucket(rt.createFirstBucketID(), time.Now())
		if err != nil {
			return false, RoutingErr.New("could not create initial K bucket: %s", err)
		}
		nodeValue, err := marshalNode(*node)
		if err != nil {
			return false, RoutingErr.New("could not marshal initial node: %s", err)
		}
		err = rt.putNode(nodeKey, nodeValue)
		if err != nil {
			return false, RoutingErr.New("could not add initial node to nodeBucketDB: %s", err)
		}
		return true, nil
	}
	kadBucketID, err := rt.getKBucketID(nodeKey)
	if err != nil {
		return false, RoutingErr.New("could not getKBucketID: %s", err)
	}
	hasRoom, err := rt.kadBucketHasRoom(kadBucketID)
	if err != nil {
		return false, err
	}
	containsLocal, err := rt.kadBucketContainsLocalNode(kadBucketID)
	if err != nil {
		return false, err
	}

	withinK, err := rt.nodeIsWithinNearestK(nodeKey)
	if err != nil {
		return false, RoutingErr.New("could not determine if node is within k: %s", err)
	}
	for !hasRoom {
		if containsLocal || withinK {
			depth, err := rt.determineLeafDepth(kadBucketID)
			if err != nil {
				return false, RoutingErr.New("could not determine leaf depth: %s", err)
			}
			kadBucketID = rt.splitBucket(kadBucketID, depth)
			err = rt.createOrUpdateKBucket(kadBucketID, time.Now())
			if err != nil {
				return false, RoutingErr.New("could not split and create K bucket: %s", err)
			}
			kadBucketID, err = rt.getKBucketID(nodeKey)
			if err != nil {
				return false, RoutingErr.New("could not get k bucket Id within add node split bucket checks: %s", err)
			}
			hasRoom, err = rt.kadBucketHasRoom(kadBucketID)
			if err != nil {
				return false, err
			}
			containsLocal, err = rt.kadBucketContainsLocalNode(kadBucketID)
			if err != nil {
				return false, err
			}

		} else {
			rt.addToReplacementCache(kadBucketID, node)
			return false, nil
		}
	}
	nodeValue, err := marshalNode(*node)
	if err != nil {
		return false, RoutingErr.New("could not marshal node: %s", err)
	}
	err = rt.putNode(nodeKey, nodeValue)
	if err != nil {
		return false, RoutingErr.New("could not add node to nodeBucketDB: %s", err)
	}
	err = rt.createOrUpdateKBucket(kadBucketID, time.Now())
	if err != nil {
		return false, RoutingErr.New("could not create or update K bucket: %s", err)
	}
	return true, nil
}

// nodeAlreadyExists will return true if the given node ID exists within nodeBucketDB
func (rt *RoutingTable) nodeAlreadyExists(nodeID storage.Key) (bool, error) {
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
func (rt *RoutingTable) updateNode(node *proto.Node) error {
	marshaledNode, err := marshalNode(*node)
	if err != nil {
		return err
	}
	err = rt.putNode(storage.Key(node.Id), marshaledNode)
	if err != nil {
		return RoutingErr.New("could not update node: %v", err)
	}
	return nil
}

// removeNode will remove churned nodes and replace those entries with nodes from the replacement cache.
func (rt *RoutingTable) removeNode(kadBucketID storage.Key, nodeID storage.Key) error {
	err := rt.nodeBucketDB.Delete(nodeID)
	if err != nil {
		return RoutingErr.New("could not delete node %s", err)
	}
	nodes := rt.replacementCache[string(kadBucketID)]
	if len(nodes) == 0 {
		return nil
	}
	last := nodes[len(nodes)-1]
	val, err := marshalNode(*last)
	if err != nil {
		return err
	}
	err = rt.putNode(storage.Key(last.Id), val)
	if err != nil {
		return err
	}
	rt.replacementCache[string(kadBucketID)] = nodes[:len(nodes)-1]
	return nil
}

// marshalNode: helper, sanitizes proto Node for db insertion
func marshalNode(node proto.Node) ([]byte, error) {
	node.Id = "-"
	nodeVal, err := pb.Marshal(&node)
	if err != nil {
		return nil, RoutingErr.New("could not marshal proto node: %s", err)
	}
	return nodeVal, nil
}

// putNode: helper, adds or updates proto Node and ID to nodeBucketDB
func (rt *RoutingTable) putNode(nodeKey storage.Key, nodeValue storage.Value) error {
	err := rt.nodeBucketDB.Put(nodeKey, nodeValue)
	if err != nil {
		return RoutingErr.New("could not add key value pair to nodeBucketDB: %s", err)
	}
	return nil
}

// createOrUpdateKBucket: helper, adds or updates given kbucket
func (rt *RoutingTable) createOrUpdateKBucket(bucketID storage.Key, now time.Time) error {
	dateTime := make([]byte, binary.MaxVarintLen64)
	binary.PutVarint(dateTime, now.UnixNano())
	err := rt.kadBucketDB.Put(bucketID, dateTime)
	if err != nil {
		return RoutingErr.New("could not add or update k bucket: %s", err)
	}
	return nil
}

// getKBucketID: helper, returns the id of the corresponding k bucket given a node id.
// The node doesn't have to be in the routing table at time of search
func (rt *RoutingTable) getKBucketID(nodeID storage.Key) (storage.Key, error) {
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

// sortByXOR: helper, quick sorts node IDs by xor from the reference Node, smallest xor to largest
func sortByXOR(nodeIDs storage.Keys, referenceNode storage.Key) storage.Keys {
	if len(nodeIDs) < 2 {
		return nodeIDs
	}
	left, right := 0, len(nodeIDs)-1
	pivot := rand.Int() % len(nodeIDs)
	nodeIDs[pivot], nodeIDs[right] = nodeIDs[right], nodeIDs[pivot]
	for i := range nodeIDs {
		xorI := xorTwoIds(nodeIDs[i], referenceNode)
		xorR := xorTwoIds(nodeIDs[right], referenceNode)
		if bytes.Compare(xorI, xorR) < 0 {
			nodeIDs[left], nodeIDs[i] = nodeIDs[i], nodeIDs[left]
			left++
		}
	}
	nodeIDs[left], nodeIDs[right] = nodeIDs[right], nodeIDs[left]
	sortByXOR(nodeIDs[:left], referenceNode)
	sortByXOR(nodeIDs[left+1:], referenceNode)
	return nodeIDs
}

// determineFurthestIDWithinK: helper, determines the furthest node within the k closest to local node
func (rt *RoutingTable) determineFurthestIDWithinK(nodeIDs storage.Keys) ([]byte, error) {
	sortedNodes := sortByXOR(nodeIDs, []byte(rt.self.Id))
	if len(sortedNodes) < rt.bucketSize+1 { //adding 1 since we're not including local node in closest k
		return sortedNodes[len(sortedNodes)-1], nil
	}
	return sortedNodes[rt.bucketSize], nil
}

// xorTwoIds: helper, finds the xor distance between two byte slices
func xorTwoIds(id []byte, comparisonID []byte) []byte {
	var xorArr []byte
	for i := 0; i < len(id); i++ {
		xor := id[i] ^ comparisonID[i]
		xorArr = append(xorArr, xor)
	}
	return xorArr
}

// nodeIsWithinNearestK: helper, returns true if the node in question is within the nearest k from local node
func (rt *RoutingTable) nodeIsWithinNearestK(nodeID storage.Key) (bool, error) {
	nodes, err := rt.nodeBucketDB.List(nil, 0)
	if err != nil {
		return false, RoutingErr.New("could not get nodes: %s", err)
	}
	nodeCount := len(nodes)
	if nodeCount < rt.bucketSize+1 { //adding 1 since we're not including local node in closest k
		return true, nil
	}
	furthestIDWithinK, err := rt.determineFurthestIDWithinK(nodes)
	if err != nil {
		return false, RoutingErr.New("could not determine furthest id within k: %s", err)
	}
	localNodeID := rt.self.Id
	existingXor := xorTwoIds(furthestIDWithinK, []byte(localNodeID))
	newXor := xorTwoIds(nodeID, []byte(localNodeID))
	if bytes.Compare(newXor, existingXor) < 0 {
		return true, nil
	}
	return false, nil
}

// kadBucketContainsLocalNode returns true if the kbucket in question contains the local node
func (rt *RoutingTable) kadBucketContainsLocalNode(bucketID storage.Key) (bool, error) {
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
func (rt *RoutingTable) kadBucketHasRoom(bucketID storage.Key) (bool, error) {
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
func (rt *RoutingTable) getNodeIDsWithinKBucket(bucketID storage.Key) (storage.Keys, error) {
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

// getNodesFromIDs: helper, returns
func (rt *RoutingTable) getNodesFromIDs(nodeIDs storage.Keys) (storage.Keys, []storage.Value, error) {
	var nodes []storage.Value
	for _, v := range nodeIDs {
		n, err := rt.nodeBucketDB.Get(v)
		if err != nil {
			return nodeIDs, nodes, RoutingErr.New("could not get node id %v, %s", v, err)
		}
		nodes = append(nodes, n)
	}
	return nodeIDs, nodes, nil
}

// unmarshalNodes: helper, returns slice of reconstructed proto node pointers given a map of nodeIDs:serialized nodes
func unmarshalNodes(nodeIDs storage.Keys, nodes []storage.Value) ([]*proto.Node, error) {
	if len(nodeIDs) != len(nodes) {
		return []*proto.Node{}, RoutingErr.New("length mismatch between nodeIDs and nodes")
	}
	var unmarshaled []*proto.Node
	for i, n := range nodes {
		node := &proto.Node{}
		err := pb.Unmarshal(n, node)
		if err != nil {
			return unmarshaled, RoutingErr.New("could not unmarshal node %s", err)
		}
		node.Id = string(nodeIDs[i])
		unmarshaled = append(unmarshaled, node)
	}
	return unmarshaled, nil
}

// getUnmarshaledNodesFromBucket: helper, gets proto nodes within kbucket
func (rt *RoutingTable) getUnmarshaledNodesFromBucket(bucketID storage.Key) ([]*proto.Node, error) {
	nodeIDs, err := rt.getNodeIDsWithinKBucket(bucketID)
	if err != nil {
		return []*proto.Node{}, RoutingErr.New("could not get nodeIds within kbucket %s", err)
	}
	ids, serializedNodes, err := rt.getNodesFromIDs(nodeIDs)
	if err != nil {
		return []*proto.Node{}, RoutingErr.New("could not get node values %s", err)
	}
	unmarshaledNodes, err := unmarshalNodes(ids, serializedNodes)
	if err != nil {
		return []*proto.Node{}, RoutingErr.New("could not unmarshal nodes %s", err)
	}
	return unmarshaledNodes, nil
}

// getKBucketRange: helper, returns the left and right endpoints of the range of node ids contained within the bucket
func (rt *RoutingTable) getKBucketRange(bucketID storage.Key) (storage.Keys, error) {
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
func (rt *RoutingTable) createFirstBucketID() []byte {
	var id []byte
	x := byte(255)
	bytesLength := rt.idLength / 8
	for i := 0; i < bytesLength; i++ {
		id = append(id, x)
	}
	return id
}

// createZeroAsStorageKey creates storage Key representation of 00..00
func (rt *RoutingTable) createZeroAsStorageKey() storage.Key {
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
func (rt *RoutingTable) determineLeafDepth(bucketID storage.Key) (int, error) {
	bucketRange, err := rt.getKBucketRange(bucketID)
	if err != nil {
		return -1, RoutingErr.New("could not get k bucket range %s", err)
	}
	smaller := bucketRange[0]
	diffBit, err := rt.determineDifferingBitIndex(bucketID, smaller)
	if err != nil {
		return diffBit + 1, RoutingErr.New("could not determine differing bit %s", err)
	}
	return diffBit + 1, nil
}

// determineDifferingBitIndex: helper, returns the last bit differs starting from prefix to suffix
func (rt *RoutingTable) determineDifferingBitIndex(bucketID storage.Key, comparisonID storage.Key) (int, error) {
	if bytes.Equal(bucketID, comparisonID) {
		return -2, RoutingErr.New("compared two equivalent k bucket ids")
	}
	if bytes.Equal(comparisonID, rt.createZeroAsStorageKey()) {
		comparisonID = rt.createFirstBucketID()
	}

	var differingByteIndex int
	var differingByteXor byte
	xorArr := xorTwoIds(bucketID, comparisonID)

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
		tempXor ^= toggle
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
func (rt *RoutingTable) splitBucket(bucketID []byte, depth int) []byte {
	newID := make([]byte, len(bucketID))
	copy(newID, bucketID)
	bitIndex := depth
	byteIndex := bitIndex / 8
	bitInByteIndex := 7 - (bitIndex % 8)
	toggle := byte(1 << uint(bitInByteIndex))
	newID[byteIndex] ^= toggle
	return newID
}
