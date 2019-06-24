// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

func (rt *RoutingTable) antechamberAddNode(ctx context.Context, node *pb.Node) {} //attempts to add a node the antechamber. Only allowed in if within neighborhood

func (rt *RoutingTable) antechamberRemoveNode(ctx context.Context, node *pb.Node) {} //removes a node from the antechamber. Called when node moves into RT or node is outside neighborhood

func (rt *RoutingTable) antechamberFindNear(ctx context.Context, target storj.NodeID, limit int) (_ []*pb.Node, err error) {
	closestNodes := make([]*pb.Node, 0, limit+1)
	return closestNodes, nil
}

//checks whether the node in question has a valid voucher. If true, continue attempt to add to Rt. If false, call antechamberAddNode
func (rt *RoutingTable) hasAcceptableVoucher(ctx context.Context, node *pb.Node) bool {
	return false
}