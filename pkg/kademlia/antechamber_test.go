// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

func TestAntechamberAddNode(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()
	rt := createRoutingTableWith(ctx, storj.NodeID{127, 255}, routingTableOpts{bucketSize: 2})
	defer ctx.Check(rt.Close)

	// Add node to antechamber even if there are no neighborhood nodes
	node := &pb.Node{Id: storj.NodeID{63, 255}}
	err := rt.antechamberAddNode(ctx, node)
	assert.NoError(t, err)
	val, err := rt.antechamber.Get(ctx, node.Id.Bytes())
	assert.NoError(t, err)
	unmarshaled := &pb.Node{}
	err = proto.Unmarshal(val, unmarshaled)
	assert.NoError(t, err)
	assert.Equal(t, node.Id, unmarshaled.Id)

	// Add two nodes to routing table
	node1 := &pb.Node{Id: storj.NodeID{191, 255}} // [191, 255] XOR [127, 255] = 192
	ok, err := rt.addNode(ctx, node1)
	assert.True(t, ok)
	assert.NoError(t, err)

	node2 := &pb.Node{Id: storj.NodeID{143, 255}} // [143, 255] XOR [127, 255] = 240
	ok, err = rt.addNode(ctx, node2)
	assert.True(t, ok)
	assert.NoError(t, err)

	// node not in neighborhood, should not be added to antechamber
	node3 := &pb.Node{Id: storj.NodeID{133, 255}} // [133, 255] XOR [127, 255] = 250 > 240 neighborhood XOR boundary
	err = rt.antechamberAddNode(ctx, node3)
	assert.NoError(t, err)
	_, err = rt.antechamber.Get(ctx, node3.Id.Bytes())
	assert.Error(t, err)

	// node in neighborhood, should be added to antechamber
	node4 := &pb.Node{Id: storj.NodeID{255, 255}} // [255, 255] XOR [127, 255] = 128 < 240
	err = rt.antechamberAddNode(ctx, node4)
	assert.NoError(t, err)
	val, err = rt.antechamber.Get(ctx, node4.Id.Bytes())
	assert.NoError(t, err)
	unmarshaled = &pb.Node{}
	err = proto.Unmarshal(val, unmarshaled)
	assert.NoError(t, err)
	assert.Equal(t, node4.Id, unmarshaled.Id)
}

func TestAntechamberRemoveNode(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()
	rt := createRoutingTable(ctx, storj.NodeID{127, 255})
	defer ctx.Check(rt.Close)
	// remove non existent node
	node := &pb.Node{Id: storj.NodeID{191, 255}}
	err := rt.antechamberRemoveNode(ctx, node)
	assert.NoError(t, err)

	// add node to antechamber
	err = rt.antechamberAddNode(ctx, node)
	assert.NoError(t, err)

	// remove node
	err = rt.antechamberRemoveNode(ctx, node)
	assert.NoError(t, err)

	// check if gone
	_, err = rt.antechamber.Get(ctx, node.Id.Bytes())
	assert.Error(t, err)
}

func TestAntechamberFindNear(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()
	nodeID := storj.NodeID{127, 255}
	rt := createRoutingTable(ctx, nodeID)
	defer ctx.Check(rt.Close)

	// Check empty antechamber, expect empty findNear
	nodes, err := rt.antechamberFindNear(ctx, nodeID, 2)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(nodes))

	// add 4 nodes
	node1 := &pb.Node{Id: storj.NodeID{191, 255}} // [191, 255] XOR [127, 255] = 192 -> second closest
	err = rt.antechamberAddNode(ctx, node1)
	assert.NoError(t, err)
	node2 := &pb.Node{Id: storj.NodeID{143, 255}}
	err = rt.antechamberAddNode(ctx, node2)
	assert.NoError(t, err)
	node3 := &pb.Node{Id: storj.NodeID{133, 255}}
	err = rt.antechamberAddNode(ctx, node3)
	assert.NoError(t, err)
	node4 := &pb.Node{Id: storj.NodeID{255, 255}} // [255, 255] XOR [127, 255] = 128 -> closest node
	err = rt.antechamberAddNode(ctx, node4)
	assert.NoError(t, err)

	// select 2 closest
	nodes, err = rt.antechamberFindNear(ctx, nodeID, 2)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(nodes))
	assert.Equal(t, node4.Id, nodes[0].Id)
	assert.Equal(t, node1.Id, nodes[1].Id)
}
