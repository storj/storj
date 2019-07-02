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
		err = rt.antechamber.Put(ctx, node.Id.Bytes(), v)
		if err != nil {
			return AntechamberErr.New("could not add key value pair to antechamber: %s", err)
		}
	}
	return nil
}

// antechamberRemoveNode removes a node from the antechamber
// Called when node moves into RT, node is outside neighborhood (check when any node is added to RT), or node failed contact
func (rt *RoutingTable) antechamberRemoveNode(ctx context.Context, node *pb.Node) (err error) {
	defer mon.Task()(&ctx)(&err)
	rt.acMutex.Lock()
	defer rt.acMutex.Unlock()
	err = rt.antechamber.Delete(ctx, node.Id.Bytes())
	if err != nil && !storage.ErrKeyNotFound.Has(err) {
		return AntechamberErr.New("could not delete node %s", err)
	}
	return nil
}

// antechamberFindNear returns the closest nodes to self from the antechamber up to the limit
// it is called in conjunction with RT FindNear in some circumstances
func (rt *RoutingTable) antechamberFindNear(ctx context.Context, target storj.NodeID, limit int) (_ []*pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)
	rt.acMutex.Lock()
	defer rt.acMutex.Unlock()
	closestNodes := make([]*pb.Node, 0, limit+1)
	err = rt.iterateAntechamber(ctx, storj.NodeID{}, func(ctx context.Context, newID storj.NodeID, protoNode []byte) error {
		newPos := len(closestNodes)
		for ; newPos > 0 && compareByXor(closestNodes[newPos-1].Id, newID, target) > 0; newPos-- {
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
