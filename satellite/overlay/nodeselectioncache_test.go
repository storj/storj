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

func TestInit(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		cache := overlay.NewNodeSelectionCache(zap.NewNop(),
			db.OverlayCache(),
			time.Nanosecond,
			nodeCfg,
		)
		// the cache should have no nodes to start
		err := cache.Init(ctx)
		require.NoError(t, err)
		reputable, new := cache.Size()
		require.Equal(t, 0, reputable)
		require.Equal(t, 0, new)

		// add some nodes to the database
		const nodeCount = 2
		addNodesToNodesTable(ctx, t, db.OverlayCache(), nodeCount)

		// confirm nodes are in the cache once initialized
		err = cache.Init(ctx)
		require.NoError(t, err)
		reputable, new = cache.Size()
		require.Equal(t, 2, new)
		require.Equal(t, 0, reputable)
	})
}

func addNodesToNodesTable(ctx context.Context, t *testing.T, db overlay.DB, count int) {
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
	}
}

type mockdb struct {
	mu        sync.Mutex
	callCount int
}

func (m *mockdb) SelectAllStorageNodesUpload(ctx context.Context, selectionCfg overlay.NodeSelectionConfig) (reputable, new []*overlay.SelectedNode, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	return []*overlay.SelectedNode{}, []*overlay.SelectedNode{}, nil
}
func TestRefreshConcurrent(t *testing.T) {
	ctx := testcontext.New(t)

	// concurrent refresh with high staleness
	staleWhenAfter := time.Hour
	mockDB := mockdb{}
	cache := overlay.NewNodeSelectionCache(zap.NewNop(),
		&mockDB,
		staleWhenAfter,
		nodeCfg,
	)

	var group errgroup.Group
	group.Go(func() error {
		return cache.Init(ctx)
	})
	group.Go(func() error {
		return cache.Init(ctx)
	})
	err := group.Wait()
	require.NoError(t, err)

	require.Equal(t, 1, mockDB.callCount)

	// concurrent refresh with low staleness
	staleWhenAfter = time.Nanosecond
	mockDB = mockdb{}
	cache = overlay.NewNodeSelectionCache(zap.NewNop(),
		&mockDB,
		staleWhenAfter,
		nodeCfg,
	)
	group.Go(func() error {
		return cache.Init(ctx)
	})
	group.Go(func() error {
		return cache.Init(ctx)
	})
	err = group.Wait()
	require.NoError(t, err)

	require.Equal(t, 2, mockDB.callCount)
}

func TestGetNode(t *testing.T) {
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
			time.Nanosecond,
			nodeCfg,
		)
		// the cache should have no nodes to start
		reputable, new := cache.Size()
		require.Equal(t, 0, reputable)
		require.Equal(t, 0, new)

		// add some nodes to the database
		const nodeCount = 4
		addNodesToNodesTable(ctx, t, db.OverlayCache(), nodeCount)

		// confirm get nodes returns those nodes
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
func TestGetNodeConcurrent(t *testing.T) {
	ctx := testcontext.New(t)

	// concurrent GetNodes with high staleness
	staleWhenAfter := time.Hour
	mockDB := mockdb{}
	cache := overlay.NewNodeSelectionCache(zap.NewNop(),
		&mockDB,
		staleWhenAfter,
		nodeCfg,
	)

	var group errgroup.Group
	group.Go(func() error {
		_, err := cache.GetNodes(ctx, overlay.FindStorageNodesRequest{})
		return err
	})
	group.Go(func() error {
		_, err := cache.GetNodes(ctx, overlay.FindStorageNodesRequest{})
		return err
	})
	err := group.Wait()
	require.NoError(t, err)
	// expect only one call to GetNodes
	require.Equal(t, 1, mockDB.callCount)

	// concurrent get nodes with low staleness
	staleWhenAfter = time.Nanosecond
	mockDB = mockdb{}
	cache = overlay.NewNodeSelectionCache(zap.NewNop(),
		&mockDB,
		staleWhenAfter,
		nodeCfg,
	)

	group.Go(func() error {
		_, err := cache.GetNodes(ctx, overlay.FindStorageNodesRequest{})
		return err
	})
	group.Go(func() error {
		_, err := cache.GetNodes(ctx, overlay.FindStorageNodesRequest{})
		return err
	})
	err = group.Wait()
	require.NoError(t, err)
	// expect two calls to GetNodes
	require.Equal(t, 2, mockDB.callCount)
}

func TestGetNodeError(t *testing.T) {
	ctx := testcontext.New(t)
	staleWhenAfter := time.Hour
	mockDB := mockdb{}
	cache := overlay.NewNodeSelectionCache(zap.NewNop(),
		&mockDB,
		staleWhenAfter,
		nodeCfg,
	)

	// there should be 0 nodes in the cache
	reputable, new := cache.Size()
	require.Equal(t, 0, reputable)
	require.Equal(t, 0, new)

	// since the cache has no nodes, we should not be able
	// to get 2 storage nodes from it
	_, err := cache.GetNodes(ctx, overlay.FindStorageNodesRequest{RequestedCount: 2})
	require.Error(t, err)
}
