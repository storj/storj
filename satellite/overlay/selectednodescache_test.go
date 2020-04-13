// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

var nodeCfg = overlay.NodeSelectionConfig{
	AuditCount:       100,
	UptimeCount:      100,
	NewNodeFraction:  0.2,
	MinimumVersion:   "v1.0.0",
	OnlineWindow:     4 * time.Hour,
	DistinctIP:       true,
	MinimumDiskSpace: 100 * memory.MiB,
}

func TestAddRemoveNode(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		c := overlay.NewSelectedNodesCache(ctx, zap.NewNop(),
			db.OverlayCache(), nodeCfg,
		)

		newNodeID := storj.NodeID{1}
		reputableNodeId := storj.NodeID{2}
		// add new node
		c.AddNewNode(ctx, overlay.CachedNode{
			ID:         newNodeID,
			Address:    "9.8.7.6:8080",
			LastNet:    "9.8.7",
			LastIPPort: "9.8.7.6:8080",
		})
		r, n := c.Size(ctx)
		require.Equal(t, 0, r)
		require.Equal(t, 1, n)

		// add reputable
		c.AddReputableNode(ctx, overlay.CachedNode{
			ID:         reputableNodeId,
			Address:    "9.8.8.6:8080",
			LastNet:    "9.8.8",
			LastIPPort: "9.8.8.6:8080",
		})
		r, n = c.Size(ctx)
		require.Equal(t, 1, r)
		require.Equal(t, 1, n)

		// remove not found node
		c.RemoveNode(ctx, storj.NodeID{3})
		r, n = c.Size(ctx)
		require.Equal(t, 1, r)
		require.Equal(t, 1, n)

		// remove a new node
		c.RemoveNode(ctx, newNodeID)
		r, n = c.Size(ctx)
		require.Equal(t, 1, r)
		require.Equal(t, 0, n)

		// remove a reputable node
		c.RemoveNode(ctx, reputableNodeId)
		r, n = c.Size(ctx)
		require.Equal(t, 0, r)
		require.Equal(t, 0, n)

	})
}
