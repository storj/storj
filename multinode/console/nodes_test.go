// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package console_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/multinode"
	"storj.io/storj/multinode/console"
	"storj.io/storj/multinode/multinodedb/multinodedbtest"
)

func TestNodesDB(t *testing.T) {
	multinodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db multinode.DB) {
		nodes := db.Nodes()

		nodeID := testrand.NodeID()
		apiSecret := []byte("secret")
		publicAddress := "228.13.38.1:8081"

		err := nodes.Add(ctx, nodeID, apiSecret, publicAddress)
		assert.NoError(t, err)

		node, err := nodes.GetByID(ctx, nodeID)
		assert.NoError(t, err)
		assert.Equal(t, node.ID.Bytes(), nodeID.Bytes())
		assert.Equal(t, node.APISecret, apiSecret)
		assert.Equal(t, node.PublicAddress, publicAddress)

		allNodes, err := nodes.GetAll(ctx)
		assert.NoError(t, err)
		assert.Equal(t, len(allNodes), 1)
		assert.Equal(t, node.ID.Bytes(), allNodes[0].ID.Bytes())
		assert.Equal(t, node.APISecret, allNodes[0].APISecret)
		assert.Equal(t, node.PublicAddress, allNodes[0].PublicAddress)

		err = nodes.Remove(ctx, nodeID)
		assert.NoError(t, err)

		_, err = nodes.GetAll(ctx)
		assert.Error(t, err)
		assert.True(t, console.ErrNoNode.Has(err))

		node, err = nodes.GetByID(ctx, nodeID)
		assert.Error(t, err)
		assert.True(t, console.ErrNoNode.Has(err))
	})
}
