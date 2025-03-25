// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package nodes_test

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/multinode"
	"storj.io/storj/multinode/multinodedb/multinodedbtest"
	"storj.io/storj/multinode/nodes"
	"storj.io/storj/private/multinodeauth"
)

func TestNodesDB(t *testing.T) {
	multinodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db multinode.DB) {
		nodesRepository := db.Nodes()

		nodeID := testrand.NodeID()
		apiSecret := multinodeauth.Secret{uint8(0)}
		publicAddress := "228.13.38.1:8081"

		err := nodesRepository.Add(ctx, nodes.Node{ID: nodeID, APISecret: apiSecret, PublicAddress: publicAddress})
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

		t.Run("pagination tests", func(t *testing.T) {
			const nodesAmount = 10
			nodeList := make([]nodes.Node, 0)
			for i := 0; i < nodesAmount; i++ {
				node := nodes.Node{
					ID:            testrand.NodeID(),
					APISecret:     multinodeauth.Secret{uint8(i)},
					PublicAddress: strconv.Itoa(i),
					Name:          strconv.Itoa(i),
				}
				nodeList = append(nodeList, node)
				err := nodesRepository.Add(ctx, node)
				require.NoError(t, err)
			}
			page, err := nodesRepository.ListPaged(ctx, nodes.Cursor{
				Limit: 2,
				Page:  1,
			})
			assert.NoError(t, err)
			assert.Equal(t, page.TotalCount, int64(nodesAmount))
			assert.Equal(t, 2, len(page.Nodes))
			assert.Equal(t, 0, bytes.Compare(nodeList[0].ID.Bytes(), page.Nodes[0].ID.Bytes()))
			assert.Equal(t, 0, bytes.Compare(nodeList[1].ID.Bytes(), page.Nodes[1].ID.Bytes()))
			page, err = nodesRepository.ListPaged(ctx, nodes.Cursor{
				Limit: 2,
				Page:  2,
			})
			assert.NoError(t, err)
			assert.Equal(t, page.TotalCount, int64(nodesAmount))
			assert.Equal(t, 2, len(page.Nodes))
			assert.Equal(t, 0, bytes.Compare(nodeList[2].ID.Bytes(), page.Nodes[0].ID.Bytes()))
			assert.Equal(t, 0, bytes.Compare(nodeList[3].ID.Bytes(), page.Nodes[1].ID.Bytes()))
			page, err = nodesRepository.ListPaged(ctx, nodes.Cursor{
				Limit: 2,
				Page:  3,
			})
			assert.NoError(t, err)
			assert.Equal(t, page.TotalCount, int64(nodesAmount))
			assert.Equal(t, 2, len(page.Nodes))
			assert.Equal(t, 0, bytes.Compare(nodeList[4].ID.Bytes(), page.Nodes[0].ID.Bytes()))
			assert.Equal(t, 0, bytes.Compare(nodeList[5].ID.Bytes(), page.Nodes[1].ID.Bytes()))
			page, err = nodesRepository.ListPaged(ctx, nodes.Cursor{
				Limit: 2,
				Page:  4,
			})
			assert.NoError(t, err)
			assert.Equal(t, page.TotalCount, int64(nodesAmount))
			assert.Equal(t, 2, len(page.Nodes))
			assert.Equal(t, 0, bytes.Compare(nodeList[6].ID.Bytes(), page.Nodes[0].ID.Bytes()))
			assert.Equal(t, 0, bytes.Compare(nodeList[7].ID.Bytes(), page.Nodes[1].ID.Bytes()))
			page, err = nodesRepository.ListPaged(ctx, nodes.Cursor{
				Limit: 2,
				Page:  5,
			})
			assert.NoError(t, err)
			assert.Equal(t, page.TotalCount, int64(nodesAmount))
			assert.Equal(t, 2, len(page.Nodes))
			assert.Equal(t, 0, bytes.Compare(nodeList[8].ID.Bytes(), page.Nodes[0].ID.Bytes()))
			assert.Equal(t, 0, bytes.Compare(nodeList[9].ID.Bytes(), page.Nodes[1].ID.Bytes()))
		})
	})
}
