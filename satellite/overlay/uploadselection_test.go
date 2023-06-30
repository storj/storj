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
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/nodeselection/uploadselection"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

var nodeSelectionConfig = overlay.NodeSelectionConfig{
	NewNodeFraction:  0.2,
	MinimumVersion:   "v1.0.0",
	OnlineWindow:     4 * time.Hour,
	DistinctIP:       true,
	MinimumDiskSpace: 100 * memory.MiB,

	AsOfSystemTime: overlay.AsOfSystemTimeConfig{
		Enabled:         true,
		DefaultInterval: -time.Microsecond,
	},
}

const (
	// staleness is how stale the cache can be before we sync with
	// the database to refresh the cache.

	// using a low time will force the cache to refresh every time.
	lowStaleness = 2 * time.Nanosecond

	// using a positive time will make it so that the cache is only refreshed when
	// it hasn't been in the past hour.
	highStaleness = time.Hour
)

func TestRefresh(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		cache, err := overlay.NewUploadSelectionCache(zap.NewNop(),
			db.OverlayCache(),
			lowStaleness,
			nodeSelectionConfig,
		)
		require.NoError(t, err)

		cacheCtx, cacheCancel := context.WithCancel(ctx)
		defer cacheCancel()
		ctx.Go(func() error { return cache.Run(cacheCtx) })

		// the cache should have no nodes to start
		err = cache.Refresh(ctx)
		require.NoError(t, err)
		reputable, new, err := cache.Size(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, reputable)
		require.Equal(t, 0, new)

		// add some nodes to the database
		const nodeCount = 2
		addNodesToNodesTable(ctx, t, db.OverlayCache(), nodeCount, 0)

		// confirm nodes are in the cache once
		err = cache.Refresh(ctx)
		require.NoError(t, err)
		reputable, new, err = cache.Size(ctx)
		require.NoError(t, err)
		require.Equal(t, 2, new)
		require.Equal(t, 0, reputable)
	})
}

func addNodesToNodesTable(ctx context.Context, t *testing.T, db overlay.DB, count, makeReputable int) (ids []storj.NodeID) {
	for i := 0; i < count; i++ {
		subnet := strconv.Itoa(i) + ".1.2"
		addr := subnet + ".3:8080"
		n := overlay.NodeCheckInInfo{
			NodeID: storj.NodeID{byte(i)},
			Address: &pb.NodeAddress{
				Address: addr,
			},
			LastNet:    subnet,
			LastIPPort: addr,
			IsUp:       true,
			Capacity: &pb.NodeCapacity{
				FreeDisk: 200 * memory.MiB.Int64(),
			},
			Version: &pb.NodeVersion{
				Version:    "v1.1.0",
				CommitHash: "",
				Timestamp:  time.Time{},
				Release:    true,
			},
		}
		err := db.UpdateCheckIn(ctx, n, time.Now().UTC(), nodeSelectionConfig)
		require.NoError(t, err)

		// make designated nodes reputable
		if i < makeReputable {
			vettedAt, err := db.TestVetNode(ctx, storj.NodeID{byte(i)})
			require.NoError(t, err)
			require.NoError(t, err)
			require.NotNil(t, vettedAt)
			ids = append(ids, storj.NodeID{byte(i)})
		}
	}
	return ids
}

type mockdb struct {
	mu        sync.Mutex
	callCount int
	reputable []*uploadselection.SelectedNode
	new       []*uploadselection.SelectedNode
}

func (m *mockdb) SelectAllStorageNodesUpload(ctx context.Context, selectionCfg overlay.NodeSelectionConfig) (reputable, new []*uploadselection.SelectedNode, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sync2.Sleep(ctx, 500*time.Millisecond)
	m.callCount++

	reputable = make([]*uploadselection.SelectedNode, len(m.reputable))
	for i, n := range m.reputable {
		reputable[i] = n.Clone()
	}
	new = make([]*uploadselection.SelectedNode, len(m.new))
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
	cache, err := overlay.NewUploadSelectionCache(zap.NewNop(),
		&mockDB,
		highStaleness,
		nodeSelectionConfig,
	)
	require.NoError(t, err)

	cacheCtx, cacheCancel := context.WithCancel(ctx)
	defer cacheCancel()
	ctx.Go(func() error { return cache.Run(cacheCtx) })

	var group errgroup.Group
	group.Go(func() error {
		return cache.Refresh(ctx)
	})
	group.Go(func() error {
		return cache.Refresh(ctx)
	})
	require.NoError(t, group.Wait())

	require.Equal(t, 1, mockDB.callCount)

	// concurrent cache.Refresh with low staleness, where low staleness
	// means that the cache will refresh *every time* cache.Refresh is called
	mockDB = mockdb{}
	cache, err = overlay.NewUploadSelectionCache(zap.NewNop(),
		&mockDB,
		lowStaleness,
		nodeSelectionConfig,
	)
	require.NoError(t, err)
	ctx.Go(func() error { return cache.Run(cacheCtx) })
	group.Go(func() error {
		return cache.Refresh(ctx)
	})
	group.Go(func() error {
		return cache.Refresh(ctx)
	})
	err = group.Wait()
	require.NoError(t, err)

	require.True(t, 1 <= mockDB.callCount && mockDB.callCount <= 2, "calls %d", mockDB.callCount)
}

func TestGetNodes(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		var nodeSelectionConfig = overlay.NodeSelectionConfig{
			NewNodeFraction:  0.2,
			MinimumVersion:   "v1.0.0",
			OnlineWindow:     4 * time.Hour,
			DistinctIP:       true,
			MinimumDiskSpace: 100 * memory.MiB,
		}
		cache, err := overlay.NewUploadSelectionCache(zap.NewNop(),
			db.OverlayCache(),
			lowStaleness,
			nodeSelectionConfig,
		)
		require.NoError(t, err)

		cacheCtx, cacheCancel := context.WithCancel(ctx)
		defer cacheCancel()
		ctx.Go(func() error { return cache.Run(cacheCtx) })

		// the cache should have no nodes to start
		reputable, new, err := cache.Size(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, reputable)
		require.Equal(t, 0, new)

		// add 4 nodes to the database and vet 2
		const nodeCount = 4
		nodeIds := addNodesToNodesTable(ctx, t, db.OverlayCache(), nodeCount, 2)
		require.Len(t, nodeIds, 2)

		// confirm cache.GetNodes returns the correct nodes
		selectedNodes, err := cache.GetNodes(ctx, overlay.FindStorageNodesRequest{RequestedCount: 2})
		require.NoError(t, err)
		reputable, new, err = cache.Size(ctx)
		require.NoError(t, err)
		require.Equal(t, 2, new)
		require.Equal(t, 2, reputable)
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

func TestGetNodesExcludeCountryCodes(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 2, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		err := planet.Satellites[0].Overlay.Service.TestNodeCountryCode(ctx, planet.StorageNodes[0].ID(), "FR")
		require.NoError(t, err)

		cache := planet.Satellites[0].Overlay.Service.UploadSelectionCache

		// confirm cache.GetNodes returns the correct nodes
		selectedNodes, err := cache.GetNodes(ctx, overlay.FindStorageNodesRequest{RequestedCount: 2})
		// we only expect one node to be returned, even though we requested two, so there will be an error
		require.Error(t, err)

		_, new, err := cache.Size(ctx)
		require.NoError(t, err)
		require.Equal(t, 2, new)
		require.Equal(t, 1, len(selectedNodes))
		// the node that was returned should be the one that does not have the "FR" country code
		require.Equal(t, planet.StorageNodes[1].ID(), selectedNodes[0].ID)
	})
}

func TestGetNodesConcurrent(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	reputableNodes := []*uploadselection.SelectedNode{{
		ID:         storj.NodeID{1},
		Address:    &pb.NodeAddress{Address: "127.0.0.9"},
		LastNet:    "127.0.0",
		LastIPPort: "127.0.0.9:8000",
	}}
	newNodes := []*uploadselection.SelectedNode{{
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
	cache, err := overlay.NewUploadSelectionCache(zap.NewNop(),
		&mockDB,
		highStaleness,
		nodeSelectionConfig,
	)
	require.NoError(t, err)

	cacheCtx, cacheCancel := context.WithCancel(ctx)
	defer cacheCancel()
	ctx.Go(func() error { return cache.Run(cacheCtx) })

	var group errgroup.Group
	group.Go(func() error {
		nodes, err := cache.GetNodes(ctx, overlay.FindStorageNodesRequest{
			RequestedCount: 1,
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
			RequestedCount: 1,
		})
		for i := range nodes {
			nodes[i].ID = storj.NodeID{byte(i)}
			nodes[i].Address.Address = "123.123.123.123"
		}
		nodes[0] = nil
		return err
	})

	require.NoError(t, group.Wait())
	// expect only one call to the db via cache.GetNodes
	require.Equal(t, 1, mockDB.callCount)

	// concurrent get nodes with low staleness, where low staleness means that
	// the cache will refresh each time cache.GetNodes is called
	mockDB = mockdb{
		reputable: reputableNodes,
		new:       newNodes,
	}
	cache, err = overlay.NewUploadSelectionCache(zap.NewNop(),
		&mockDB,
		lowStaleness,
		nodeSelectionConfig,
	)
	require.NoError(t, err)

	ctx.Go(func() error { return cache.Run(cacheCtx) })

	group.Go(func() error {
		nodes, err := cache.GetNodes(ctx, overlay.FindStorageNodesRequest{
			RequestedCount: 1,
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
			RequestedCount: 1,
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
	// expect up to two calls to the db via cache.GetNodes
	require.True(t, 1 <= mockDB.callCount && mockDB.callCount <= 2, "calls %d", mockDB.callCount)
}

func TestGetNodesDistinct(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	reputableNodes := []*uploadselection.SelectedNode{{
		ID:         testrand.NodeID(),
		Address:    &pb.NodeAddress{Address: "127.0.0.9"},
		LastNet:    "127.0.0",
		LastIPPort: "127.0.0.9:8000",
	}, {
		ID:         testrand.NodeID(),
		Address:    &pb.NodeAddress{Address: "127.0.0.6"},
		LastNet:    "127.0.0",
		LastIPPort: "127.0.0.6:8000",
	}, {
		ID:         testrand.NodeID(),
		Address:    &pb.NodeAddress{Address: "127.0.1.7"},
		LastNet:    "127.0.1",
		LastIPPort: "127.0.1.7:8000",
	}, {
		ID:         testrand.NodeID(),
		Address:    &pb.NodeAddress{Address: "127.0.2.7"},
		LastNet:    "127.0.2",
		LastIPPort: "127.0.2.7:8000",
	}}

	newNodes := []*uploadselection.SelectedNode{{
		ID:         testrand.NodeID(),
		Address:    &pb.NodeAddress{Address: "127.0.0.10"},
		LastNet:    "127.0.0",
		LastIPPort: "127.0.0.10:8000",
	}, {
		ID:         testrand.NodeID(),
		Address:    &pb.NodeAddress{Address: "127.0.1.8"},
		LastNet:    "127.0.1",
		LastIPPort: "127.0.1.8:8000",
	}, {
		ID:         testrand.NodeID(),
		Address:    &pb.NodeAddress{Address: "127.0.2.8"},
		LastNet:    "127.0.2",
		LastIPPort: "127.0.2.8:8000",
	}}

	mockDB := mockdb{
		reputable: reputableNodes,
		new:       newNodes,
	}

	{
		// test that distinct ip doesn't return same last net
		config := nodeSelectionConfig
		config.NewNodeFraction = 0.5
		config.DistinctIP = true
		cache, err := overlay.NewUploadSelectionCache(zap.NewNop(),
			&mockDB,
			highStaleness,
			config,
		)
		require.NoError(t, err)

		cacheCtx, cacheCancel := context.WithCancel(ctx)
		defer cacheCancel()
		ctx.Go(func() error { return cache.Run(cacheCtx) })

		// selecting 3 should be possible
		nodes, err := cache.GetNodes(ctx, overlay.FindStorageNodesRequest{
			RequestedCount: 3,
		})
		require.NoError(t, err)
		seen := make(map[string]bool)
		for _, n := range nodes {
			require.False(t, seen[n.LastNet])
			seen[n.LastNet] = true
		}

		// selecting 6 is impossible
		_, err = cache.GetNodes(ctx, overlay.FindStorageNodesRequest{
			RequestedCount: 6,
		})
		require.Error(t, err)
	}

	{ // test that distinctIP=true allows selecting 6 nodes
		// emulate DistinctIP=false behavior by filling in LastNets with unique addresses
		for _, nodeList := range [][]*uploadselection.SelectedNode{reputableNodes, newNodes} {
			for i := range nodeList {
				nodeList[i].LastNet = nodeList[i].LastIPPort
			}
		}
		config := nodeSelectionConfig
		config.NewNodeFraction = 0.5
		config.DistinctIP = false
		cache, err := overlay.NewUploadSelectionCache(zap.NewNop(),
			&mockDB,
			highStaleness,
			config,
		)
		require.NoError(t, err)

		cacheCtx, cacheCancel := context.WithCancel(ctx)
		defer cacheCancel()
		ctx.Go(func() error { return cache.Run(cacheCtx) })

		_, err = cache.GetNodes(ctx, overlay.FindStorageNodesRequest{
			RequestedCount: 6,
		})
		require.NoError(t, err)
	}
}

func TestGetNodesError(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	mockDB := mockdb{}
	cache, err := overlay.NewUploadSelectionCache(zap.NewNop(),
		&mockDB,
		highStaleness,
		nodeSelectionConfig,
	)
	require.NoError(t, err)

	cacheCtx, cacheCancel := context.WithCancel(ctx)
	defer cacheCancel()
	ctx.Go(func() error { return cache.Run(cacheCtx) })

	// there should be 0 nodes in the cache
	reputable, new, err := cache.Size(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, reputable)
	require.Equal(t, 0, new)

	// since the cache has no nodes, we should not be able
	// to get 2 storage nodes from it and we expect an error
	_, err = cache.GetNodes(ctx, overlay.FindStorageNodesRequest{RequestedCount: 2})
	require.Error(t, err)
}

func TestNewNodeFraction(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		newNodeFraction := 0.2
		var nodeSelectionConfig = overlay.NodeSelectionConfig{
			NewNodeFraction:  newNodeFraction,
			MinimumVersion:   "v1.0.0",
			OnlineWindow:     4 * time.Hour,
			DistinctIP:       true,
			MinimumDiskSpace: 10 * memory.MiB,
		}
		cache, err := overlay.NewUploadSelectionCache(zap.NewNop(),
			db.OverlayCache(),
			lowStaleness,
			nodeSelectionConfig,
		)
		require.NoError(t, err)

		cacheCtx, cacheCancel := context.WithCancel(ctx)
		defer cacheCancel()
		ctx.Go(func() error { return cache.Run(cacheCtx) })

		// the cache should have no nodes to start
		err = cache.Refresh(ctx)
		require.NoError(t, err)
		reputable, new, err := cache.Size(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, reputable)
		require.Equal(t, 0, new)

		// add some nodes to the database, some are reputable and some are new nodes
		const nodeCount = 10
		repIDs := addNodesToNodesTable(ctx, t, db.OverlayCache(), nodeCount, 4)
		require.Len(t, repIDs, 4)
		// confirm nodes are in the cache once
		err = cache.Refresh(ctx)
		require.NoError(t, err)
		reputable, new, err = cache.Size(ctx)
		require.NoError(t, err)
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
		require.Equal(t, len(n)-reputableCount, int(5*newNodeFraction)) // 1, 1
	})
}
