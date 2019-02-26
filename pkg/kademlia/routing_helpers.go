// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"encoding/binary"
	"time"

	"github.com/gogo/protobuf/proto"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

// addNode attempts to add a new contact to the routing table
// Requires node not already in table
// Returns true if node was added successfully
func (rt *RoutingTable) addNode(node *pb.Node) (bool, error) {
	rt.mutex.Lock()
	defer rt.mutex.Unlock()

	if node.Id == rt.self.Id {
		err := rt.createOrUpdateKBucket(firstBucketID, time.Now())
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

	withinK, err := rt.wouldBeInNearestK(node.Id)
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
func (rt *RoutingTable) removeNode(node *pb.Node) error {
	rt.mutex.Lock()
	defer rt.mutex.Unlock()
	kadBucketID, err := rt.getKBucketID(node.Id)

	if err != nil {
		return RoutingErr.New("could not get k bucket %s", err)
	}

	existingMarshalled, err := rt.nodeBucketDB.Get(node.Id.Bytes())
	if storage.ErrKeyNotFound.Has(err) {
		//check replacement cache
		rt.removeFromReplacementCache(kadBucketID, node)
		return nil
	} else if err != nil {
		return RoutingErr.New("could not get node %s", err)
	}

	var existing pb.Node
	err = proto.Unmarshal(existingMarshalled, &existing)
	if err != nil {
		return RoutingErr.New("could not unmarshal node %s", err)
	}

	if !pb.AddressEqual(existing.Address, node.Address) {
		// don't remove a node if the address is different
		return nil
	}
	err = rt.nodeBucketDB.Delete(node.Id.Bytes())
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
	match := bucketID{}
	err := rt.kadBucketDB.Iterate(storage.IterateOptions{First: storage.Key{}, Recurse: true},
		func(it storage.Iterator) error {
			var item storage.ListItem
			for it.Next(&item) {
				match = keyToBucketID(item.Key)
				if nodeID.Less(match) {
					break
				}
			}
			return nil
		},
	)
	if err != nil {
		return bucketID{}, RoutingErr.Wrap(err)
	}
	return match, nil
}

// wouldBeInNearestK: helper, returns true if the node in question is within the nearest k from local node
func (rt *RoutingTable) wouldBeInNearestK(nodeID storj.NodeID) (bool, error) {
	closestNodes, err := rt.FindNear(rt.self.Id, rt.bucketSize)
	if err != nil {
		return false, RoutingErr.Wrap(err)
	}
	if len(closestNodes) < rt.bucketSize {
		return true, nil
	}
	var furthestIDWithinK storj.NodeID
	if len(closestNodes) <= rt.bucketSize {
		furthestIDWithinK = closestNodes[len(closestNodes)-1].Id
	} else {
		furthestIDWithinK = closestNodes[rt.bucketSize].Id
	}

	existingXor := xorNodeID(furthestIDWithinK, rt.self.Id)
	newXor := xorNodeID(nodeID, rt.self.Id)
	return newXor.Less(existingXor), nil
}

// kadBucketContainsLocalNode returns true if the kbucket in question contains the local node
func (rt *RoutingTable) kadBucketContainsLocalNode(queryID bucketID) (bool, error) {
	bID, err := rt.getKBucketID(rt.self.Id)
	if err != nil {
		return false, err
	}
	return queryID == bID, nil
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
	var ids []storj.NodeID

	err = rt.iterateNodes(left, func(nodeID storj.NodeID, protoNode []byte) error {
		if left.Less(nodeID) && (nodeID.Less(right) || nodeID == right) {
			ids = append(ids, nodeID)
		}
		return nil
	}, false)
	if err != nil {
		return nil, err
	}
	return ids, nil
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
	previousBucket := bucketID{}
	endpoints := []bucketID{}
	err := rt.kadBucketDB.Iterate(storage.IterateOptions{First: storage.Key{}, Recurse: true},
		func(it storage.Iterator) error {
			var item storage.ListItem
			for it.Next(&item) {
				thisBucket := keyToBucketID(item.Key)
				if thisBucket == bID {
					endpoints = []bucketID{previousBucket, bID}
					break
				}
				previousBucket = thisBucket
			}
			return nil
		},
	)
	if err != nil {
		return endpoints, RoutingErr.Wrap(err)
	}
	return endpoints, nil
}

// determineLeafDepth determines the level of the bucket id in question.
// Eg level 0 means there is only 1 bucket, level 1 means the bucket has been split once, and so on
func (rt *RoutingTable) determineLeafDepth(bID bucketID) (int, error) {
	bucketRange, err := rt.getKBucketRange(bID)
	if err != nil {
		return -1, RoutingErr.New("could not get k bucket range %s", err)
	}
	smaller := bucketRange[0]
	diffBit, err := determineDifferingBitIndex(bID, smaller)
	if err != nil {
		return diffBit + 1, RoutingErr.New("could not determine differing bit %s", err)
	}
	return diffBit + 1, nil
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
