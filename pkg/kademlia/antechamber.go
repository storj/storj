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
// Called when node moves into RT, node is outside neighborhood (check when any node is added to RT), or node failed contact
func (rt *RoutingTable) antechamberRemoveNode(ctx context.Context, nodeID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)
	rt.acMutex.Lock()
	defer rt.acMutex.Unlock()
	err = rt.antechamber.Delete(ctx, xorNodeID(nodeID, rt.self.Id).Bytes())
	if err != nil && !storage.ErrKeyNotFound.Has(err) {
		return AntechamberErr.New("could not delete node with xo%s", err)
	}
	return nil
}
