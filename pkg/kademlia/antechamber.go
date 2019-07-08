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

// antechamberAddNode attempts to add a node the antechamber.
// Only added if there are fewer than k nodes in the antechamber or the node is closer than the furthest node.
// If the the node is closest than the furthest node, we remove the furthest node to maintain the limit of k
func (rt *RoutingTable) antechamberAddNode(ctx context.Context, node *pb.Node) (err error) {
	defer mon.Task()(&ctx)(&err)
	rt.acMutex.Lock()
	defer rt.acMutex.Unlock()

	//check size of antechamber and furthest node
	v, err := proto.Marshal(node)
	if err != nil {
		return AntechamberErr.New("could not marshall node: %s", err)
	}
	err = rt.antechamber.Put(ctx, xorNodeID(node.Id, rt.self.Id).Bytes(), v)
	if err != nil {
		return AntechamberErr.New("could not add key value pair to antechamber: %s", err)
	}
	return nil
}

// antechamberRemoveNode removes a node from the antechamber
// Called when node moves into RT, node is outside the closest k antechamber nodes, or node failed contact
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

// trimAntechamber removes nodes outside the closest 20
func (rt *RoutingTable) trimAntechamber(ctx context.Context) (err error) {
	keys, err := rt.antechamber.List(ctx, nil, 0)
	if err != nil {
		return AntechamberErr.New("could not list nodes %s", err)
	}
	size := len(keys)
	for diff := size - rt.bucketSize; diff > 0; diff-- {
		xor, err := storj.NodeIDFromBytes(keys[size-diff-1])
		if err != nil {
			return AntechamberErr.New("could not get xor from key %s", err)
		}
		nodeID := xorNodeID(xor, rt.self.Id)
		err = rt.antechamberRemoveNode(ctx, nodeID)
		if err != nil {
			return AntechamberErr.New("could not remove node %s", err)
		}
	}
	return nil
}

// getAllAntechamberNodes returns all the nodes from the antechamber
func (rt *RoutingTable) getAllAntechamberNodes(ctx context.Context) (nodes []*pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)
	rt.acMutex.Lock()
	defer rt.acMutex.Unlock()

	var nodeErrors errs.Group

	err = rt.iterateAntechamber(ctx, storj.NodeID{}, func(ctx context.Context, xor storj.NodeID, protoNode []byte) error {
		newNode := pb.Node{}
		err := proto.Unmarshal(protoNode, &newNode)
		if err != nil {
			nodeErrors.Add(err)
		}
		nodes = append(nodes, &newNode)
		return nil
	})

	if err != nil {
		nodeErrors.Add(err)
	}

	return nodes, nodeErrors.Err()
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
