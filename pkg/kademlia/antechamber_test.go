// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"bytes"
	"context"
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
	rt := createRoutingTableWith(ctx, storj.NodeID{127, 255}, routingTableOpts{bucketSize: 3})
	defer ctx.Check(rt.Close)

	node1 := &pb.Node{Id: storj.NodeID{191, 255}} // XOR 192
	node2 := &pb.Node{Id: storj.NodeID{255, 255}} // XOR 128
	node3 := &pb.Node{Id: storj.NodeID{143, 255}} // XOR 240
	node4 := &pb.Node{Id: storj.NodeID{133, 255}} // XOR 250
	node5 := &pb.Node{Id: storj.NodeID{63, 255}}  // XOR [127, 255] = 64

	cases := []struct {
		testName      string
		node          *pb.Node
		expectedNodes []*pb.Node
	}{
		{
			testName:      "A: 1 total node",
			node:          node1,
			expectedNodes: []*pb.Node{node1}, // XOR 192
		},
		{
			testName:      "B: node2 added to beginning. 2 total nodes",
			node:          node2,
			expectedNodes: []*pb.Node{node2, node1}, // XOR 128, 19
		},
		{
			testName:      "C: node3 added to end. 3 total nodes",
			node:          node3,
			expectedNodes: []*pb.Node{node2, node1, node3}, // XOR 128, 192, 240
		},
		{
			testName:      "D: node4 is too far away and the antechamber is full. node4 not added",
			node:          node4,
			expectedNodes: []*pb.Node{node2, node1, node3},
		},
		{
			testName:      "E: node5 is closer (smaller XOR) than node3. node5 is added. node3 is removed.",
			node:          node5,
			expectedNodes: []*pb.Node{node5, node2, node1}, // XOR 64, 128, 192
		},
	}
	for _, c := range cases {
		testCase := c
		t.Run(testCase.testName, func(t *testing.T) {
			err := rt.antechamberAddNode(ctx, testCase.node)
			assert.NoError(t, err)
			nodes, err := rt.getAllAntechamberNodes(ctx)
			assert.NoError(t, err)
			for i, v := range testCase.expectedNodes {
				assert.True(t, bytes.Equal(v.Id.Bytes(), nodes[i].Id.Bytes()))
			}
		})
	}
}

func TestAntechamberRemoveNode(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()
	rt := createRoutingTable(ctx, storj.NodeID{127, 255})
	defer ctx.Check(rt.Close)
	// remove non existent node
	node := &pb.Node{Id: storj.NodeID{191, 255}}
	err := rt.antechamberRemoveNode(ctx, node.Id)
	assert.NoError(t, err)

	// add node to antechamber
	err = addNode(ctx, rt, node)
	assert.NoError(t, err)

	// remove node
	err = rt.antechamberRemoveNode(ctx, node.Id)
	assert.NoError(t, err)

	// check if gone
	_, err = rt.antechamber.Get(ctx, node.Id.Bytes())
	assert.Error(t, err)
}

func TestTrimAntechamber(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()
	rt := createRoutingTableWith(ctx, storj.NodeID{127, 255}, routingTableOpts{bucketSize: 2})
	defer ctx.Check(rt.Close)

	node1 := &pb.Node{Id: storj.NodeID{191, 255}} // XOR 192
	node2 := &pb.Node{Id: storj.NodeID{255, 255}} // XOR 128
	node3 := &pb.Node{Id: storj.NodeID{143, 255}} // XOR 240
	node4 := &pb.Node{Id: storj.NodeID{63, 255}}  // XOR 64

	// no nodes
	nodes, err := rt.getAllAntechamberNodes(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(nodes))
	err = rt.trimAntechamber(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(nodes))

	cases := []struct {
		testName      string
		node          *pb.Node
		expectedNodes []*pb.Node
	}{
		{
			testName:      "A: 1 total node",
			node:          node1,
			expectedNodes: []*pb.Node{node1},
		},
		{
			testName:      "B: node2 added to beginning. 2 total nodes",
			node:          node2,
			expectedNodes: []*pb.Node{node2, node1},
		},
		{
			testName:      "C: node3 added to end. removed",
			node:          node3,
			expectedNodes: []*pb.Node{node2, node1},
		},
		{
			testName:      "D: node4 added to end. node1 removed",
			node:          node4,
			expectedNodes: []*pb.Node{node4, node2},
		},
	}
	for _, c := range cases {
		testCase := c
		t.Run(testCase.testName, func(t *testing.T) {
			err := addNode(ctx, rt, testCase.node)
			assert.NoError(t, err)
			err = rt.trimAntechamber(ctx)
			assert.NoError(t, err)
			nodes, err := rt.getAllAntechamberNodes(ctx)
			assert.NoError(t, err)
			for i, v := range testCase.expectedNodes {
				assert.True(t, bytes.Equal(v.Id.Bytes(), nodes[i].Id.Bytes()))
			}
		})
	}

	// test trimming multiple nodes
	err = addNode(ctx, rt, node3)
	assert.NoError(t, err)
	err = addNode(ctx, rt, node1)
	assert.NoError(t, err)
	err = rt.trimAntechamber(ctx)
	assert.NoError(t, err)
	nodes, err = rt.getAllAntechamberNodes(ctx)
	assert.NoError(t, err)
	expectedNodes := []*pb.Node{node4, node2}
	for i, v := range expectedNodes {
		assert.True(t, bytes.Equal(v.Id.Bytes(), nodes[i].Id.Bytes()))
	}

}

func TestGetAllAntechamberNodes(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()
	rt := createRoutingTableWith(ctx, storj.NodeID{127, 255}, routingTableOpts{bucketSize: 3})
	defer ctx.Check(rt.Close)

	node1 := &pb.Node{Id: storj.NodeID{191, 255}} // XOR 192
	node2 := &pb.Node{Id: storj.NodeID{255, 255}} // XOR 128
	node3 := &pb.Node{Id: storj.NodeID{143, 255}} // XOR 240

	// No nodes in antechamber
	nodes, err := rt.getAllAntechamberNodes(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(nodes))

	cases := []struct {
		testName      string
		node          *pb.Node
		expectedNodes []*pb.Node
	}{
		{
			testName:      "A: 1 total node",
			node:          node1,
			expectedNodes: []*pb.Node{node1}, // XOR 192
		},
		{
			testName:      "B: node2 added to beginning. 2 total nodes",
			node:          node2,
			expectedNodes: []*pb.Node{node2, node1}, // XOR 128, 19
		},
		{
			testName:      "C: node3 added to end. 3 total nodes",
			node:          node3,
			expectedNodes: []*pb.Node{node2, node1, node3}, // XOR 128, 192, 240
		},
	}
	for _, c := range cases {
		testCase := c
		t.Run(testCase.testName, func(t *testing.T) {
			err := addNode(ctx, rt, testCase.node)
			assert.NoError(t, err)
			nodes, err := rt.getAllAntechamberNodes(ctx)
			assert.NoError(t, err)

			for i, v := range testCase.expectedNodes {
				assert.True(t, bytes.Equal(v.Id.Bytes(), nodes[i].Id.Bytes()))
			}
		})
	}
}

func addNode(ctx context.Context, rt *RoutingTable, node *pb.Node) error {
	v, err := proto.Marshal(node)
	if err != nil {
		return err
	}
	err = rt.antechamber.Put(ctx, xorNodeID(node.Id, rt.self.Id).Bytes(), v)
	if err != nil {
		return err
	}
	return nil
}
