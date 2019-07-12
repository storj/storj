// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

// AntechamberErr is the class for all errors pertaining to antechamber operations
var AntechamberErr = errs.Class("antechamber error")

// antechamberAddNode attempts to add a node the antechamber. Only allowed in if within rt neighborhood
func (rt *RoutingTable) antechamberAddNode(ctx context.Context, node *pb.Node) (err error) {
	defer mon.Task()(&ctx)(&err)
	rt.mutex.Lock()
	rt.acMutex.Lock()
	defer rt.mutex.Unlock()
	defer rt.acMutex.Unlock()

	inNeighborhood, err := rt.wouldBeInNearestK(ctx, node.Id)
	if err != nil {
		return AntechamberErr.New("could not check node neighborhood: %s", err)
	}
	if inNeighborhood {
		v, err := proto.Marshal(node)
		if err != nil {
			return AntechamberErr.New("could not marshall node: %s", err)
		}
		err = rt.antechamber.Put(ctx, xorNodeID(node.Id, rt.self.Id).Bytes(), v)
		if err != nil {
			return AntechamberErr.New("could not add key value pair to antechamber: %s", err)
		}
	}
	return nil
}

// antechamberRemoveNode removes a node from the antechamber
// Called when node moves into RT, node is outside neighborhood (check when any node is added to RT), or node failed contact
func (rt *RoutingTable) antechamberRemoveNode(ctx context.Context, nodeID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)
	rt.acMutex.Lock()
	defer rt.acMutex.Unlock()
	err = rt.antechamber.Delete(ctx, xorNodeID(nodeID, rt.self.Id).Bytes())
	if err != nil && !storage.ErrKeyNotFound.Has(err) {
		return AntechamberErr.New("could not delete node %s", err)
	}
	return nil
}

// antechamberFindNear returns the closest nodes to target from the antechamber up to the limit
// it is called in conjunction with RT FindNear in some circumstances
func (rt *RoutingTable) antechamberFindNear(ctx context.Context, target storj.NodeID, limit int) (_ []*pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)
	rt.acMutex.Lock()
	defer rt.acMutex.Unlock()
	closestNodes := make([]*pb.Node, 0, limit+1)
	err = rt.iterateAntechamber(ctx, storj.NodeID{}, func(ctx context.Context, newID storj.NodeID, protoNode []byte) error {
		newPos := len(closestNodes)
		for ; newPos > 0 && compareByXor(closestNodes[newPos-1].Id, newID, target) > 0; newPos-- { //todo update comparebyxor with xor self, target... newID should be xor
		}
		if newPos != limit {
			newNode := pb.Node{}
			err := proto.Unmarshal(protoNode, &newNode)
			if err != nil {
				return err
			}
			closestNodes = append(closestNodes, &newNode)
			if newPos != len(closestNodes) { //reorder
				copy(closestNodes[newPos+1:], closestNodes[newPos:])
				closestNodes[newPos] = &newNode
				if len(closestNodes) > limit {
					closestNodes = closestNodes[:limit]
				}
			}
		}
		return nil
	})
	return closestNodes, Error.Wrap(err)
}

// findOutsiderNodes removes the nodes outside the rt node neighborhood
func (rt *RoutingTable) findOutsiderNodes(ctx context.Context) (keys storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)
	rt.mutex.Lock()
	rt.acMutex.Lock()
	defer rt.mutex.Unlock()
	defer rt.acMutex.Unlock()
	neighborhood, err := rt.FindNear(ctx, rt.self.Id, rt.bucketSize)
	if err != nil {
		return keys, AntechamberErr.New("could not Find Near: %s", err)
	}
	size := len(neighborhood)
	if size <= rt.bucketSize {
		// node neighborhood has room, no trimming needed
		return keys, nil
	}
	furthest := neighborhood[size-1]
	// take xor of furthest node
	err = rt.iterateAntechamber(ctx, storj.NodeID{}, func(ctx context.Context, newID storj.NodeID, protoNode []byte) error {
		// compare values until we find the nodes farther than the furthest node newnodexor > furthestxor
		// possibly iterate backwards furthestxor < newnodexor
		// add values to a slice to delete
	})
	return keys, nil
}

// iterateAntechamber is a helper method that iterates through the whole antechamber table
func (rt *RoutingTable) iterateAntechamber(ctx context.Context, start storj.NodeID, f func(context.Context, storj.NodeID, []byte) error) (err error) {
	defer mon.Task()(&ctx)(&err)
	return rt.antechamber.Iterate(ctx, storage.IterateOptions{First: storage.Key(start.Bytes()), Recurse: true},
		func(ctx context.Context, it storage.Iterator) error {
			var item storage.ListItem
			for it.Next(ctx, &item) {
				nodeID, err := storj.NodeIDFromBytes(item.Key)
				if err != nil {
					return err
				}
				err = f(ctx, nodeID, item.Value)
				if err != nil {
					return err
				}
			}
			return nil
		},
	)
}
