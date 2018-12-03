// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"bytes"
	"encoding/binary"
	"sort"
	"time"

	"github.com/gogo/protobuf/proto"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/utils"
	"storj.io/storj/storage"
)

// addNode attempts to add a new contact to the routing table
// Requires node not already in table
// Returns true if node was added successfully
func (rt *RoutingTable) addNode(node *pb.Node) (bool, error) {
	rt.mutex.Lock()
	defer rt.mutex.Unlock()
	nodeIDBytes := node.Id.Bytes()

	if bytes.Equal(nodeIDBytes, rt.self.Id.Bytes()) {
		err := rt.createOrUpdateKBucket(rt.createFirstBucketID(), time.Now())
		if err != nil {
			return false, RoutingErr.New("could not create initial K bucket: %s", err)
		}
		err = rt.putNode(node)
		if err != nil {
			return false, RoutingErr.New("could not add initial node to nodeBucketDB: %s", err)
		}
		return true, nil
	}
	kadBucketID, err := rt.getKBucketID(node.Id)
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

	withinK, err := rt.nodeIsWithinNearestK(node.Id)
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
			kadBucketID, err = rt.getKBucketID(node.Id)
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
	err = rt.putNode(node)
	if err != nil {
		return false, RoutingErr.New("could not add node to nodeBucketDB: %s", err)
	}
	err = rt.createOrUpdateKBucket(kadBucketID, time.Now())
	if err != nil {
		return false, RoutingErr.New("could not create or update K bucket: %s", err)
	}
	return true, nil
}

// updateNode will update the node information given that
// the node is already in the routing table.
func (rt *RoutingTable) updateNode(node *pb.Node) error {
	if err := rt.putNode(node); err != nil {
		return RoutingErr.New("could not update node: %v", err)
	}
	return nil
}

// removeNode will remove churned nodes and replace those entries with nodes from the replacement cache.
func (rt *RoutingTable) removeNode(nodeID storj.NodeID) error {
	kadBucketID, err := rt.getKBucketID(nodeID)
	if err != nil {
		return RoutingErr.New("could not get k bucket %s", err)
	}
	_, err = rt.nodeBucketDB.Get(nodeID.Bytes())
	if storage.ErrKeyNotFound.Has(err) {
		return nil
	} else if err != nil {
		return RoutingErr.New("could not get node %s", err)
	}
	err = rt.nodeBucketDB.Delete(nodeID.Bytes())
	if err != nil {
		return RoutingErr.New("could not delete node %s", err)
	}
	nodes := rt.replacementCache[kadBucketID]
	if len(nodes) == 0 {
		return nil
	}
	err = rt.putNode(nodes[len(nodes)-1])
	if err != nil {
		return err
	}
	rt.replacementCache[kadBucketID] = nodes[:len(nodes)-1]
	return nil
}

// putNode: helper, adds or updates Node and ID to nodeBucketDB
func (rt *RoutingTable) putNode(node *pb.Node) error {
	v, err := proto.Marshal(node)
	if err != nil {
		return RoutingErr.Wrap(err)
	}

	err = rt.nodeBucketDB.Put(node.Id.Bytes(), v)
	if err != nil {
		return RoutingErr.New("could not add key value pair to nodeBucketDB: %s", err)
	}
	return nil
}

// createOrUpdateKBucket: helper, adds or updates given kbucket
func (rt *RoutingTable) createOrUpdateKBucket(bID bucketID, now time.Time) error {
	dateTime := make([]byte, binary.MaxVarintLen64)
	binary.PutVarint(dateTime, now.UnixNano())
	err := rt.kadBucketDB.Put(bID[:], dateTime)
	if err != nil {
		return RoutingErr.New("could not add or update k bucket: %s", err)
	}
	return nil
}

// getKBucketID: helper, returns the id of the corresponding k bucket given a node id.
// The node doesn't have to be in the routing table at time of search
func (rt *RoutingTable) getKBucketID(nodeID storj.NodeID) (bucketID, error) {
	kadBucketIDs, err := rt.kadBucketDB.List(nil, 0)
	if err != nil {
		return bucketID{}, RoutingErr.New("could not list all k bucket ids: %s", err)
	}
	var keys []bucketID
	keys = append(keys, bucketID{})
	for _, k := range kadBucketIDs {
		keys = append(keys, keyToBucketID(k))
	}

	for i := 0; i < len(keys)-1; i++ {
		if bytes.Compare(nodeID.Bytes(), keys[i][:]) > 0 && bytes.Compare(nodeID.Bytes(), keys[i+1][:]) <= 0 {
			return keys[i+1], nil
		}
	}

	// shouldn't happen BUT return error if no matching kbucket...
	return bucketID{}, RoutingErr.New("could not find k bucket")
}

// compareByXor compares left, right xorred by reference
func compareByXor(left, right, reference storage.Key) int {
	n := len(reference)
	if n > len(left) {
		n = len(left)
	}
	if n > len(right) {
		n = len(right)
	}
	left = left[:n]
	right = right[:n]
	reference = reference[:n]

	for i, r := range reference {
		a, b := left[i]^r, right[i]^r
		if a != b {
			if a < b {
				return -1
			}
			return 1
		}
	}

	return 0
}

func sortByXOR(nodeIDs storage.Keys, ref storage.Key) {
	sort.Slice(nodeIDs, func(i, k int) bool {
		return compareByXor(nodeIDs[i], nodeIDs[k], ref) < 0
	})
}

func nodeIDsToKeys(ids storj.NodeIDList) (nodeIDKeys storage.Keys) {
	for _, n := range ids {
		nodeIDKeys = append(nodeIDKeys, n.Bytes())
	}
	return nodeIDKeys
}

func keysToNodeIDs(keys storage.Keys) (ids storj.NodeIDList, err error) {
	var idErrs []error
	for _, k := range keys {
		id, err := storj.NodeIDFromBytes(k[:])
		if err != nil {
			idErrs = append(idErrs, err)
		}
		ids = append(ids, id)
	}
	if err := utils.CombineErrors(idErrs...); err != nil {
		return nil, err
	}

	return ids, nil
}

func keyToBucketID(key storage.Key) (bID bucketID) {
	copy(bID[:], key)
	return bID
}

// determineFurthestIDWithinK: helper, determines the furthest node within the k closest to local node
func (rt *RoutingTable) determineFurthestIDWithinK(nodeIDs storj.NodeIDList) (storj.NodeID, error) {
	nodeIDKeys := nodeIDsToKeys(nodeIDs)
	sortByXOR(nodeIDKeys, rt.self.Id.Bytes())
	if len(nodeIDs) < rt.bucketSize+1 { //adding 1 since we're not including local node in closest k
		return storj.NodeIDFromBytes(nodeIDKeys[len(nodeIDKeys)-1])
	}
	return storj.NodeIDFromBytes(nodeIDKeys[rt.bucketSize])
}

// xorTwoIds: helper, finds the xor distance between two byte slices
func xorTwoIds(id, comparisonID []byte) []byte {
	var xorArr []byte
	s := len(id)
	if s > len(comparisonID) {
		s = len(comparisonID)
	}

	for i := 0; i < s; i++ {
		xor := id[i] ^ comparisonID[i]
		xorArr = append(xorArr, xor)
	}
	return xorArr
}

// nodeIsWithinNearestK: helper, returns true if the node in question is within the nearest k from local node
func (rt *RoutingTable) nodeIsWithinNearestK(nodeID storj.NodeID) (bool, error) {
	nodeKeys, err := rt.nodeBucketDB.List(nil, 0)
	if err != nil {
		return false, RoutingErr.New("could not get nodes: %s", err)
	}
	nodeCount := len(nodeKeys)
	if nodeCount < rt.bucketSize+1 { //adding 1 since we're not including local node in closest k
		return true, nil
	}
	nodeIDs, err := keysToNodeIDs(nodeKeys)
	if err != nil {
		return false, RoutingErr.Wrap(err)
	}
	furthestIDWithinK, err := rt.determineFurthestIDWithinK(nodeIDs)
	if err != nil {
		return false, RoutingErr.New("could not determine furthest id within k: %s", err)
	}
	existingXor := xorTwoIds(furthestIDWithinK.Bytes(), rt.self.Id.Bytes())
	newXor := xorTwoIds(nodeID.Bytes(), rt.self.Id.Bytes())
	if bytes.Compare(newXor, existingXor) < 0 {
		return true, nil
	}
	return false, nil
}

// kadBucketContainsLocalNode returns true if the kbucket in question contains the local node
func (rt *RoutingTable) kadBucketContainsLocalNode(queryID bucketID) (bool, error) {
	bID, err := rt.getKBucketID(rt.self.Id)
	if err != nil {
		return false, err
	}
	return bytes.Equal(queryID[:], bID[:]), nil
}

// kadBucketHasRoom: helper, returns true if it has fewer than k nodes
func (rt *RoutingTable) kadBucketHasRoom(bID bucketID) (bool, error) {
	nodes, err := rt.getNodeIDsWithinKBucket(bID)
	if err != nil {
		return false, err
	}
	if len(nodes) < rt.bucketSize {
		return true, nil
	}
	return false, nil
}

// getNodeIDsWithinKBucket: helper, returns a collection of all the node ids contained within the kbucket
func (rt *RoutingTable) getNodeIDsWithinKBucket(bID bucketID) (storj.NodeIDList, error) {
	endpoints, err := rt.getKBucketRange(bID)
	if err != nil {
		return nil, err
	}
	left := endpoints[0]
	right := endpoints[1]
	var nodeIDsBytes [][]byte
	allNodeIDsBytes, err := rt.nodeBucketDB.List(nil, 0)
	if err != nil {
		return nil, RoutingErr.New("could not list nodes %s", err)
	}
	for _, v := range allNodeIDsBytes {
		if (bytes.Compare(v, left[:]) > 0) && (bytes.Compare(v, right[:]) <= 0) {
			nodeIDsBytes = append(nodeIDsBytes, v)
			if len(nodeIDsBytes) == rt.bucketSize {
				break
			}
		}
	}
	nodeIDs, err := storj.NodeIDsFromBytes(nodeIDsBytes)
	if err != nil {
		return nil, err
	}
	if len(nodeIDsBytes) > 0 {
		return nodeIDs, nil
	}
	return nil, nil
}

// getNodesFromIDsBytes: helper, returns array of encoded nodes from node ids
func (rt *RoutingTable) getNodesFromIDsBytes(nodeIDs storj.NodeIDList) ([]*pb.Node, error) {
	var marshaledNodes []storage.Value
	for _, v := range nodeIDs {
		n, err := rt.nodeBucketDB.Get(v.Bytes())
		if err != nil {
			return nil, RoutingErr.New("could not get node id %v, %s", v, err)
		}
		marshaledNodes = append(marshaledNodes, n)
	}
	return unmarshalNodes(marshaledNodes)
}

// unmarshalNodes: helper, returns slice of reconstructed node pointers given a map of nodeIDs:serialized nodes
func unmarshalNodes(nodes []storage.Value) ([]*pb.Node, error) {
	var unmarshaled []*pb.Node
	for _, n := range nodes {
		node := &pb.Node{}
		err := proto.Unmarshal(n, node)
		if err != nil {
			return unmarshaled, RoutingErr.New("could not unmarshal node %s", err)
		}
		unmarshaled = append(unmarshaled, node)
	}
	return unmarshaled, nil
}

// getUnmarshaledNodesFromBucket: helper, gets nodes within kbucket
func (rt *RoutingTable) getUnmarshaledNodesFromBucket(bID bucketID) ([]*pb.Node, error) {
	nodeIDsBytes, err := rt.getNodeIDsWithinKBucket(bID)
	if err != nil {
		return []*pb.Node{}, RoutingErr.New("could not get nodeIds within kbucket %s", err)
	}
	nodes, err := rt.getNodesFromIDsBytes(nodeIDsBytes)
	if err != nil {
		return []*pb.Node{}, RoutingErr.New("could not get node values %s", err)
	}
	return nodes, nil
}

// getKBucketRange: helper, returns the left and right endpoints of the range of node ids contained within the bucket
func (rt *RoutingTable) getKBucketRange(bID bucketID) ([]bucketID, error) {
	kadIDs, err := rt.kadBucketDB.ReverseList(bID[:], 2)
	if err != nil {
		return nil, RoutingErr.New("could not reverse list k bucket ids %s", err)
	}
	coords := make([]bucketID, 2)
	if len(kadIDs) < 2 {
		coords[0] = bucketID{}
	} else {
		copy(coords[0][:], kadIDs[1])
	}
	copy(coords[1][:], kadIDs[0])
	return coords, nil
}

// createFirstBucketID creates byte slice representing 11..11
func (rt *RoutingTable) createFirstBucketID() bucketID {
	var id bucketID
	x := byte(255)
	for i := 0; i < len(id); i++ {
		id[i] = x
	}
	return id
}

// determineLeafDepth determines the level of the bucket id in question.
// Eg level 0 means there is only 1 bucket, level 1 means the bucket has been split once, and so on
func (rt *RoutingTable) determineLeafDepth(bID bucketID) (int, error) {
	bucketRange, err := rt.getKBucketRange(bID)
	if err != nil {
		return -1, RoutingErr.New("could not get k bucket range %s", err)
	}
	smaller := bucketRange[0]
	diffBit, err := rt.determineDifferingBitIndex(bID, smaller)
	if err != nil {
		return diffBit + 1, RoutingErr.New("could not determine differing bit %s", err)
	}
	return diffBit + 1, nil
}

// determineDifferingBitIndex: helper, returns the last bit differs starting from prefix to suffix
func (rt *RoutingTable) determineDifferingBitIndex(bID, comparisonID bucketID) (int, error) {
	if bytes.Equal(bID[:], comparisonID[:]) {
		return -2, RoutingErr.New("compared two equivalent k bucket ids")
	}
	emptyBID := bucketID{}
	if bytes.Equal(comparisonID[:], emptyBID[:]) {
		comparisonID = rt.createFirstBucketID()
	}

	var differingByteIndex int
	var differingByteXor byte
	xorArr := xorTwoIds(bID[:], comparisonID[:])

	firstBID := rt.createFirstBucketID()
	if bytes.Equal(xorArr, firstBID[:]) {
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
func (rt *RoutingTable) splitBucket(bID bucketID, depth int) bucketID {
	var newID bucketID
	copy(newID[:], bID[:])
	byteIndex := depth / 8
	bitInByteIndex := 7 - (depth % 8)
	toggle := byte(1 << uint(bitInByteIndex))
	newID[byteIndex] ^= toggle
	return newID
}
