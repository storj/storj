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

//
//func TestAntechamberAddNode(t *testing.T) {
//	ctx := testcontext.New(t)
//	defer ctx.Cleanup()
//	rt := createRoutingTableWith(ctx, storj.NodeID{127, 255}, routingTableOpts{bucketSize: 3})
//	defer ctx.Check(rt.Close)
//
//	node1 := &pb.Node{Id: storj.NodeID{191, 255}} // XOR 192
//	node2 := &pb.Node{Id: storj.NodeID{255, 255}} // XOR 128
//	node3 := &pb.Node{Id: storj.NodeID{143, 255}} // XOR 240
//	node4 := &pb.Node{Id: storj.NodeID{133, 255}} // XOR 250
//	node5 := &pb.Node{Id: storj.NodeID{63, 255}}  // XOR [127, 255] = 64
//
//	cases := []struct {
//		testName      string
//		node          *pb.Node
//		expectedNodes []*pb.Node
//	}{
//		{
//			testName:      "A: 1 total node",
//			node:          node1,
//			expectedNodes: []*pb.Node{node1}, // XOR 192
//		},
//		{
//			testName:      "B: node2 added to beginning. 2 total nodes",
//			node:          node2,
//			expectedNodes: []*pb.Node{node2, node1}, // XOR 128, 19
//		},
//		{
//			testName:      "C: node3 added to end. 3 total nodes",
//			node:          node3,
//			expectedNodes: []*pb.Node{node2, node1, node3}, // XOR 128, 192, 240
//		},
//		{
//			testName:      "D: node4 is too far away and the antechamber is full. node4 not added",
//			node:          node4,
//			expectedNodes: []*pb.Node{node2, node1, node3},
//		},
//		{
//			testName:      "E: node5 is closer (smaller XOR) than node3. node5 is added. node3 is removed.",
//			node:          node5,
//			expectedNodes: []*pb.Node{node5, node2, node1}, // XOR 64, 128, 192
//		},
//	}
//	for _, c := range cases {
//		testCase := c
//		t.Run(testCase.testName, func(t *testing.T) {
//			err := rt.antechamberAddNode(ctx, testCase.node)
//			assert.NoError(t, err)
//			nodes, err := rt.getAllAntechamberNodes(ctx)
//			assert.NoError(t, err)
//			for i, v := range testCase.expectedNodes {
//				assert.True(t, bytes.Equal(v.Id.Bytes(), nodes[i].Id.Bytes()))
//			}
//		})
//	}
//}
//
//func TestAntechamberRemoveNode(t *testing.T) {
//	ctx := testcontext.New(t)
//	defer ctx.Cleanup()
//	rt := createRoutingTable(ctx, storj.NodeID{127, 255})
//	defer ctx.Check(rt.Close)
//	// remove non existent node
//	node := &pb.Node{Id: storj.NodeID{191, 255}}
//	err := rt.antechamberRemoveNode(ctx, node.Id)
//	assert.NoError(t, err)
//
//	// add node to antechamber
//	err = addNode(ctx, rt, node)
//	assert.NoError(t, err)
//
//	// remove node
//	err = rt.antechamberRemoveNode(ctx, node.Id)
//	assert.NoError(t, err)
//
//	// check if gone
//	_, err = rt.antechamber.Get(ctx, node.Id.Bytes())
//	assert.Error(t, err)
//}
//
//func TestTrimAntechamber(t *testing.T) {
//	ctx := testcontext.New(t)
//	defer ctx.Cleanup()
//	rt := createRoutingTableWith(ctx, storj.NodeID{127, 255}, routingTableOpts{bucketSize: 2})
//	defer ctx.Check(rt.Close)
//
//	node1 := &pb.Node{Id: storj.NodeID{191, 255}} // XOR 192
//	node2 := &pb.Node{Id: storj.NodeID{255, 255}} // XOR 128
//	node3 := &pb.Node{Id: storj.NodeID{143, 255}} // XOR 240
//	node4 := &pb.Node{Id: storj.NodeID{63, 255}}  // XOR 64
//
//	// no nodes
//	nodes, err := rt.getAllAntechamberNodes(ctx)
//	assert.NoError(t, err)
//	assert.Equal(t, 0, len(nodes))
//	err = rt.trimAntechamber(ctx)
//	assert.NoError(t, err)
//	assert.Equal(t, 0, len(nodes))
//
//	cases := []struct {
//		testName      string
//		node          *pb.Node
//		expectedNodes []*pb.Node
//	}{
//		{
//			testName:      "A: 1 total node",
//			node:          node1,
//			expectedNodes: []*pb.Node{node1},
//		},
//		{
//			testName:      "B: node2 added to beginning. 2 total nodes",
//			node:          node2,
//			expectedNodes: []*pb.Node{node2, node1},
//		},
//		{
//			testName:      "C: node3 added to end. removed",
//			node:          node3,
//			expectedNodes: []*pb.Node{node2, node1},
//		},
//		{
//			testName:      "D: node4 added to end. node1 removed",
//			node:          node4,
//			expectedNodes: []*pb.Node{node4, node2},
//		},
//	}
//	for _, c := range cases {
//		testCase := c
//		t.Run(testCase.testName, func(t *testing.T) {
//			err := addNode(ctx, rt, testCase.node)
//			assert.NoError(t, err)
//			err = rt.trimAntechamber(ctx)
//			assert.NoError(t, err)
//			nodes, err := rt.getAllAntechamberNodes(ctx)
//			assert.NoError(t, err)
//			for i, v := range testCase.expectedNodes {
//				assert.True(t, bytes.Equal(v.Id.Bytes(), nodes[i].Id.Bytes()))
//			}
//		})
//	}
//
//	// test trimming multiple nodes
//	err = addNode(ctx, rt, node3)
//	assert.NoError(t, err)
//	err = addNode(ctx, rt, node1)
//	assert.NoError(t, err)
//	err = rt.trimAntechamber(ctx)
//	assert.NoError(t, err)
//	nodes, err = rt.getAllAntechamberNodes(ctx)
//	assert.NoError(t, err)
//	expectedNodes := []*pb.Node{node4, node2}
//	for i, v := range expectedNodes {
//		assert.True(t, bytes.Equal(v.Id.Bytes(), nodes[i].Id.Bytes()))
//	}
//
//}
//
//func TestGetAllAntechamberNodes(t *testing.T) {
//	ctx := testcontext.New(t)
//	defer ctx.Cleanup()
//	rt := createRoutingTableWith(ctx, storj.NodeID{127, 255}, routingTableOpts{bucketSize: 3})
//	defer ctx.Check(rt.Close)
//
//	node1 := &pb.Node{Id: storj.NodeID{191, 255}} // XOR 192
//	node2 := &pb.Node{Id: storj.NodeID{255, 255}} // XOR 128
//	node3 := &pb.Node{Id: storj.NodeID{143, 255}} // XOR 240
//
//	// No nodes in antechamber
//	nodes, err := rt.getAllAntechamberNodes(ctx)
//	assert.NoError(t, err)
//	assert.Equal(t, 0, len(nodes))
//
//	cases := []struct {
//		testName      string
//		node          *pb.Node
//		expectedNodes []*pb.Node
//	}{
//		{
//			testName:      "A: 1 total node",
//			node:          node1,
//			expectedNodes: []*pb.Node{node1}, // XOR 192
//		},
//		{
//			testName:      "B: node2 added to beginning. 2 total nodes",
//			node:          node2,
//			expectedNodes: []*pb.Node{node2, node1}, // XOR 128, 19
//		},
//		{
//			testName:      "C: node3 added to end. 3 total nodes",
//			node:          node3,
//			expectedNodes: []*pb.Node{node2, node1, node3}, // XOR 128, 192, 240
//		},
//	}
//	for _, c := range cases {
//		testCase := c
//		t.Run(testCase.testName, func(t *testing.T) {
//			err := addNode(ctx, rt, testCase.node)
//			assert.NoError(t, err)
//			nodes, err := rt.getAllAntechamberNodes(ctx)
//			assert.NoError(t, err)
//
//			for i, v := range testCase.expectedNodes {
//				assert.True(t, bytes.Equal(v.Id.Bytes(), nodes[i].Id.Bytes()))
//			}
//		})
//	}
//}
//
//func addNode(ctx context.Context, rt *RoutingTable, node *pb.Node) error {
//	v, err := proto.Marshal(node)
//	if err != nil {
//		return err
//	}
//	err = rt.antechamber.Put(ctx, xorNodeID(node.Id, rt.self.Id).Bytes(), v)
//	if err != nil {
//		return err
//	}
//	return nil
//}
//
//func TestX(t *testing.T) {
//	ctx := testcontext.New(t)
//	defer ctx.Cleanup()
//	rt := createRoutingTableWith(ctx, storj.NodeID{127, 255}, routingTableOpts{bucketSize: 3})
//	defer ctx.Check(rt.Close)
//
//	node1 := &pb.Node{Id: storj.NodeID{191, 255}} // XOR 192
//	node2 := &pb.Node{Id: storj.NodeID{255, 255}} // XOR 128
//	node3 := &pb.Node{Id: storj.NodeID{143, 255}} // XOR 240
//	node4 := &pb.Node{Id: storj.NodeID{133, 255}} // XOR 250
//	node5 := &pb.Node{Id: storj.NodeID{63, 255}}  // XOR [127, 255] = 64
//
//	cases := []struct {
//		testName      string
//		node          *pb.Node
//		expectedNodes []*pb.Node
//	}{
//		{
//			testName:      "A: 1 total node",
//			node:          node1,
//			expectedNodes: []*pb.Node{node1}, // XOR 192
//		},
//		{
//			testName:      "B: node2 added to beginning. 2 total nodes",
//			node:          node2,
//			expectedNodes: []*pb.Node{node2, node1}, // XOR 128, 19
//		},
//		{
//			testName:      "C: node3 added to end. 3 total nodes",
//			node:          node3,
//			expectedNodes: []*pb.Node{node2, node1, node3}, // XOR 128, 192, 240
//		},
//		{
//			testName:      "D: node4 is too far away and the antechamber is full. node4 not added",
//			node:          node4,
//			expectedNodes: []*pb.Node{node2, node1, node3},
//		},
//		{
//			testName:      "E: node5 is closer (smaller XOR) than node3. node5 is added. node3 is removed.",
//			node:          node5,
//			expectedNodes: []*pb.Node{node5, node2, node1}, // XOR 64, 128, 192
//		},
//	}
//	for _, c := range cases {
//		testCase := c
//		t.Run(testCase.testName, func(t *testing.T) {
//			err := addNode(ctx, rt, testCase.node)
//			assert.NoError(t, err)
//			nodes, err := rt.getAllAntechamberNodes(ctx)
//			assert.NoError(t, err)
//			for i, v := range testCase.expectedNodes {
//				assert.True(t, bytes.Equal(v.Id.Bytes(), nodes[i].Id.Bytes()))
//			}
//		})
//	}
//}
