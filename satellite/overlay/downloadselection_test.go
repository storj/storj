// Copyright (C) 2021 Storj Labs, Incache.
// See LICENSE for copying information.

package overlay_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

var downloadSelectionCacheConfig = overlay.DownloadSelectionCacheConfig{
	Staleness:      lowStaleness,
	OnlineWindow:   time.Hour,
	AsOfSystemTime: overlay.AsOfSystemTimeConfig{Enabled: true, DefaultInterval: time.Minute},
}

func TestDownloadSelectionCacheState_Refresh(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		cache, err := overlay.NewDownloadSelectionCache(zap.NewNop(),
			db.OverlayCache(),
			downloadSelectionCacheConfig,
		)
		require.NoError(t, err)

		cacheCtx, cacheCancel := context.WithCancel(ctx)
		defer cacheCancel()
		ctx.Go(func() error { return cache.Run(cacheCtx) })

		// the cache should have no nodes to start
		err = cache.Refresh(ctx)
		require.NoError(t, err)
		count, err := cache.Size(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, count)

		// add some nodes to the database
		const nodeCount = 2
		addNodesToNodesTable(ctx, t, db.OverlayCache(), nodeCount, 0)

		// confirm nodes are in the cache once
		err = cache.Refresh(ctx)
		require.NoError(t, err)
		count, err = cache.Size(ctx)
		require.NoError(t, err)
		require.Equal(t, nodeCount, count)
	})
}

func TestDownloadSelectionCacheState_GetNodeIPs(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		cache, err := overlay.NewDownloadSelectionCache(zap.NewNop(),
			db.OverlayCache(),
			downloadSelectionCacheConfig,
		)
		require.NoError(t, err)

		cacheCtx, cacheCancel := context.WithCancel(ctx)
		defer cacheCancel()
		ctx.Go(func() error { return cache.Run(cacheCtx) })

		// add some nodes to the database
		const nodeCount = 2
		ids := addNodesToNodesTable(ctx, t, db.OverlayCache(), nodeCount, 0)

		// confirm nodes are in the cache once
		nodeips, err := cache.GetNodeIPs(ctx, ids)
		require.NoError(t, err)
		for _, id := range ids {
			require.NotEmpty(t, nodeips[id])
		}
	})
}

func TestDownloadSelectionCacheState_IPs(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	node := &overlay.SelectedNode{
		ID: testrand.NodeID(),
		Address: &pb.NodeAddress{
			Address: "1.0.1.1:8080",
		},
		LastNet:    "1.0.1",
		LastIPPort: "1.0.1.1:8080",
	}

	state := overlay.NewDownloadSelectionCacheState([]*overlay.SelectedNode{node})
	require.Equal(t, state.Size(), 1)

	ips := state.IPs([]storj.NodeID{testrand.NodeID(), node.ID})
	require.Len(t, ips, 1)
	require.Equal(t, node.LastIPPort, ips[node.ID])
}
