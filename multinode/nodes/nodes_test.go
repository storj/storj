// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package nodes_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/multinode"
	"storj.io/storj/multinode/multinodedb/multinodedbtest"
	"storj.io/storj/multinode/nodes"
)

func TestNodesDB(t *testing.T) {
	multinodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db multinode.DB) {
		nodesRepository := db.Nodes()

		nodeID := testrand.NodeID()
		apiSecret := []byte("secret")
		publicAddress := "228.13.38.1:8081"

		err := nodesRepository.Add(ctx, nodeID, apiSecret, publicAddress)
		assert.NoError(t, err)

		node, err := nodesRepository.Get(ctx, nodeID)
		assert.NoError(t, err)
		assert.Equal(t, node.ID.Bytes(), nodeID.Bytes())
		assert.Equal(t, node.APISecret, apiSecret)
		assert.Equal(t, node.PublicAddress, publicAddress)

		allNodes, err := nodesRepository.List(ctx)
		assert.NoError(t, err)
		assert.Equal(t, len(allNodes), 1)
		assert.Equal(t, node.ID.Bytes(), allNodes[0].ID.Bytes())
		assert.Equal(t, node.APISecret, allNodes[0].APISecret)
		assert.Equal(t, node.PublicAddress, allNodes[0].PublicAddress)

		newName := "Alice"
		err = nodesRepository.UpdateName(ctx, nodeID, newName)
		assert.NoError(t, err)

		node, err = nodesRepository.Get(ctx, nodeID)
		assert.NoError(t, err)
		assert.Equal(t, node.Name, newName)

		err = nodesRepository.Remove(ctx, nodeID)
		assert.NoError(t, err)

		_, err = nodesRepository.List(ctx)
		assert.Error(t, err)
		assert.True(t, nodes.ErrNoNode.Has(err))

		_, err = nodesRepository.Get(ctx, nodeID)
		assert.Error(t, err)
		assert.True(t, nodes.ErrNoNode.Has(err))
	})
}
