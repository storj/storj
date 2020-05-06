// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/common/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

var nodeCfg = overlay.NodeSelectionConfig{
	AuditCount:       1,
	UptimeCount:      1,
	NewNodeFraction:  0.2,
	MinimumVersion:   "v1.0.0",
	OnlineWindow:     4 * time.Hour,
	DistinctIP:       true,
	MinimumDiskSpace: 100 * memory.MiB,
}

const (
	// staleness is how stale the cache can be before we sync with
	// the database to refresh the cache

	// using a negative time will force the cache to refresh every time
	lowStaleness = -time.Hour

	// using a positive time will make it so that the cache is only refreshed when
	// it hasn't been in the past hour
	highStaleness = time.Hour
)

func TestRefresh(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		cache := overlay.NewNodeSelectionCache(zap.NewNop(),
			db.OverlayCache(),
			lowStaleness,
			nodeCfg,
		)
		// the cache should have no nodes to start
		err := cache.Refresh(ctx)
		require.NoError(t, err)
		reputable, new := cache.Size()
		require.Equal(t, 0, reputable)
		require.Equal(t, 0, new)

		// add some nodes to the database
		const nodeCount = 2
		addNodesToNodesTable(ctx, t, db.OverlayCache(), nodeCount, false)

		// confirm nodes are in the cache once
		err = cache.Refresh(ctx)
		require.NoError(t, err)
		reputable, new = cache.Size()
		require.Equal(t, 2, new)
		require.Equal(t, 0, reputable)
	})
}

func addNodesToNodesTable(ctx context.Context, t *testing.T, db overlay.DB, count int, makeReputable bool) []storj.NodeID {
	var reputableIds = []storj.NodeID{}
	for i := 0; i < count; i++ {
		subnet := strconv.Itoa(i) + ".1.2"
		addr := subnet + ".3:8080"
		n := overlay.NodeCheckInInfo{
			NodeID: storj.NodeID{byte(i)},
			Address: &pb.NodeAddress{
				Address:   addr,
				Transport: pb.NodeTransport_TCP_TLS_GRPC,
			},
			LastNet:    subnet,
			LastIPPort: addr,
			IsUp:       true,
			Capacity: &pb.NodeCapacity{
				FreeDisk:      200 * memory.MiB.Int64(),
				FreeBandwidth: 1 * memory.TB.Int64(),
			},
			Version: &pb.NodeVersion{
				Version:    "v1.1.0",
				CommitHash: "",
				Timestamp:  time.Time{},
				Release:    true,
			},
		}
		err := db.UpdateCheckIn(ctx, n, time.Now().UTC(), nodeCfg)
		require.NoError(t, err)

		// make half of the nodes reputable
		if makeReputable && i > count/2 {
			_, err = db.UpdateStats(ctx, &overlay.UpdateRequest{
				NodeID:       storj.NodeID{byte(i)},
				IsUp:         true,
				AuditOutcome: overlay.AuditSuccess,
				AuditLambda:  1, AuditWeight: 1, AuditDQ: 0.5,
			})
			require.NoError(t, err)
			reputableIds = append(reputableIds, storj.NodeID{byte(i)})
		}
	}
	return reputableIds
}

type mockdb struct {
	mu        sync.Mutex
	callCount int
	reputable []*overlay.SelectedNode
	new       []*overlay.SelectedNode
}

func (m *mockdb) SelectAllStorageNodesUpload(ctx context.Context, selectionCfg overlay.NodeSelectionConfig) (reputable, new []*overlay.SelectedNode, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sync2.Sleep(ctx, 500*time.Millisecond)
	m.callCount++

	reputable = make([]*overlay.SelectedNode, len(m.reputable))
	for i, n := range m.reputable {
		reputable[i] = n.Clone()
	}
	new = make([]*overlay.SelectedNode, len(m.new))
	for i, n := range m.new {
		new[i] = n.Clone()
	}

	return reputable, new, nil
}

func TestRefreshConcurrent(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// concurrent cache.Refresh with high staleness, where high staleness means the
	// cache should only be refreshed the first time we call cache.Refresh
	mockDB := mockdb{}
	cache := overlay.NewNodeSelectionCache(zap.NewNop(),
		&mockDB,
		highStaleness,
		nodeCfg,
	)

	var group errgroup.Group
	group.Go(func() error {
		return cache.Refresh(ctx)
	})
	group.Go(func() error {
		return cache.Refresh(ctx)
	})
	err := group.Wait()
	require.NoError(t, err)

	require.Equal(t, 1, mockDB.callCount)

	// concurrent cache.Refresh with low staleness, where low staleness
	// means that the cache will refresh *every time* cache.Refresh is called
	mockDB = mockdb{}
	cache = overlay.NewNodeSelectionCache(zap.NewNop(),
		&mockDB,
		lowStaleness,
		nodeCfg,
	)
	group.Go(func() error {
		return cache.Refresh(ctx)
	})
	group.Go(func() error {
		return cache.Refresh(ctx)
	})
	err = group.Wait()
	require.NoError(t, err)

	require.Equal(t, 2, mockDB.callCount)
}

func TestGetNodes(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		var nodeCfg = overlay.NodeSelectionConfig{
			AuditCount:       0,
			UptimeCount:      0,
			NewNodeFraction:  0.2,
			MinimumVersion:   "v1.0.0",
			OnlineWindow:     4 * time.Hour,
			DistinctIP:       true,
			MinimumDiskSpace: 100 * memory.MiB,
		}
		cache := overlay.NewNodeSelectionCache(zap.NewNop(),
			db.OverlayCache(),
			lowStaleness,
			nodeCfg,
		)
		// the cache should have no nodes to start
		reputable, new := cache.Size()
		require.Equal(t, 0, reputable)
		require.Equal(t, 0, new)

		// add some nodes to the database
		const nodeCount = 4
		addNodesToNodesTable(ctx, t, db.OverlayCache(), nodeCount, false)

		// confirm cache.GetNodes returns the correct nodes
		selectedNodes, err := cache.GetNodes(ctx, overlay.FindStorageNodesRequest{RequestedCount: 2})
		require.NoError(t, err)
		reputable, new = cache.Size()
		require.Equal(t, 0, new)
		require.Equal(t, 4, reputable)
		require.Equal(t, 2, len(selectedNodes))
		for _, node := range selectedNodes {
			require.NotEqual(t, node.ID, "")
			require.NotEqual(t, node.Address.Address, "")
			require.NotEqual(t, node.LastIPPort, "")
			require.NotEqual(t, node.LastNet, "")
			require.NotEqual(t, node.LastNet, "")
		}
	})
}

func TestGetNodesConcurrent(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	reputableNodes := []*overlay.SelectedNode{{
		ID:         storj.NodeID{1},
		Address:    &pb.NodeAddress{Address: "127.0.0.9"},
		LastNet:    "127.0.0",
		LastIPPort: "127.0.0.9:8000",
	}}
	newNodes := []*overlay.SelectedNode{{
		ID:         storj.NodeID{1},
		Address:    &pb.NodeAddress{Address: "127.0.0.10"},
		LastNet:    "127.0.0",
		LastIPPort: "127.0.0.10:8000",
	}}

	// concurrent GetNodes with high staleness, where high staleness means the
	// cache should only be refreshed the first time we call cache.GetNodes
	mockDB := mockdb{
		reputable: reputableNodes,
		new:       newNodes,
	}
	cache := overlay.NewNodeSelectionCache(zap.NewNop(),
		&mockDB,
		highStaleness,
		nodeCfg,
	)

	var group errgroup.Group
	group.Go(func() error {
		nodes, err := cache.GetNodes(ctx, overlay.FindStorageNodesRequest{
			MinimumRequiredNodes: 1,
		})
		for i := range nodes {
			nodes[i].ID = storj.NodeID{byte(i)}
			nodes[i].Address.Address = "123.123.123.123"
		}
		nodes[0] = nil
		return err
	})
	group.Go(func() error {
		nodes, err := cache.GetNodes(ctx, overlay.FindStorageNodesRequest{
			MinimumRequiredNodes: 1,
		})
		for i := range nodes {
			nodes[i].ID = storj.NodeID{byte(i)}
			nodes[i].Address.Address = "123.123.123.123"
		}
		nodes[0] = nil
		return err
	})
	err := group.Wait()
	require.NoError(t, err)
	// expect only one call to the db via cache.GetNodes
	require.Equal(t, 1, mockDB.callCount)

	// concurrent get nodes with low staleness, where low staleness means that
	// the cache will refresh each time cache.GetNodes is called
	mockDB = mockdb{
		reputable: reputableNodes,
		new:       newNodes,
	}
	cache = overlay.NewNodeSelectionCache(zap.NewNop(),
		&mockDB,
		lowStaleness,
		nodeCfg,
	)

	group.Go(func() error {
		nodes, err := cache.GetNodes(ctx, overlay.FindStorageNodesRequest{
			MinimumRequiredNodes: 1,
		})
		for i := range nodes {
			nodes[i].ID = storj.NodeID{byte(i)}
			nodes[i].Address.Address = "123.123.123.123"
		}
		nodes[0] = nil
		return err
	})
	group.Go(func() error {
		nodes, err := cache.GetNodes(ctx, overlay.FindStorageNodesRequest{
			MinimumRequiredNodes: 1,
		})
		for i := range nodes {
			nodes[i].ID = storj.NodeID{byte(i)}
			nodes[i].Address.Address = "123.123.123.123"
		}
		nodes[0] = nil
		return err
	})
	err = group.Wait()
	require.NoError(t, err)
	// expect two calls to the db via cache.GetNodes
	require.Equal(t, 2, mockDB.callCount)
}

func TestGetNodesError(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	mockDB := mockdb{}
	cache := overlay.NewNodeSelectionCache(zap.NewNop(),
		&mockDB,
		highStaleness,
		nodeCfg,
	)

	// there should be 0 nodes in the cache
	reputable, new := cache.Size()
	require.Equal(t, 0, reputable)
	require.Equal(t, 0, new)

	// since the cache has no nodes, we should not be able
	// to get 2 storage nodes from it and we expect an error
	_, err := cache.GetNodes(ctx, overlay.FindStorageNodesRequest{RequestedCount: 2})
	require.Error(t, err)
}

func TestNewNodeFraction(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		newNodeFraction := 0.2
		var nodeCfg = overlay.NodeSelectionConfig{
			AuditCount:       1,
			UptimeCount:      1,
			NewNodeFraction:  newNodeFraction,
			MinimumVersion:   "v1.0.0",
			OnlineWindow:     4 * time.Hour,
			DistinctIP:       true,
			MinimumDiskSpace: 10 * memory.MiB,
		}
		cache := overlay.NewNodeSelectionCache(zap.NewNop(),
			db.OverlayCache(),
			lowStaleness,
			nodeCfg,
		)
		// the cache should have no nodes to start
		err := cache.Refresh(ctx)
		require.NoError(t, err)
		reputable, new := cache.Size()
		require.Equal(t, 0, reputable)
		require.Equal(t, 0, new)

		// add some nodes to the database, some are reputable and some are new nodes
		const nodeCount = 10
		repIDs := addNodesToNodesTable(ctx, t, db.OverlayCache(), nodeCount, true)

		// confirm nodes are in the cache once
		err = cache.Refresh(ctx)
		require.NoError(t, err)
		reputable, new = cache.Size()
		require.Equal(t, 6, new)
		require.Equal(t, 4, reputable)

		// select nodes and confirm correct new node fraction
		n, err := cache.GetNodes(ctx, overlay.FindStorageNodesRequest{RequestedCount: 5})
		require.NoError(t, err)
		require.Equal(t, len(n), 5)
		var reputableCount int
		for _, id := range repIDs {
			for _, node := range n {
				if id == node.ID {
					reputableCount++
				}
			}
		}
		require.Equal(t, len(n)-reputableCount, int(5*newNodeFraction))
	})
}
