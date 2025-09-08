// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/identity/testidentity"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/shared/location"
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
	satellitedbtest.RunWithConfig(t, satellitedbtest.Config{
		NonParallel: true,
	}, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		cache, err := overlay.NewUploadSelectionCache(zap.NewNop(),
			db.OverlayCache(),
			lowStaleness,
			nodeSelectionConfig,
			nodeselection.NodeFilters{},
			nodeselection.TestPlacementDefinitions(),
		)
		require.NoError(t, err)

		cacheCtx, cacheCancel := context.WithCancel(ctx)
		defer cacheCancel()
		ctx.Go(func() error { return cache.Run(cacheCtx) })

		// the cache should have no nodes to start
		err = cache.Refresh(ctx)
		require.NoError(t, err)

		// add some nodes to the database
		const nodeCount = 2
		addNodesToNodesTable(ctx, t, db.OverlayCache(), nodeCount, 0)

		// confirm nodes are in the cache once
		err = cache.Refresh(ctx)
		monkit.Default.Stats(func(key monkit.SeriesKey, field string, val float64) {
			if key.Measurement == "placement" && (field == "UploadCount" || field == "Count") {
				require.Equal(t, float64(nodeCount), val)
			}
		})
		require.NoError(t, err)
	})
}

func addNodesToNodesTable(ctx context.Context, t *testing.T, db overlay.DB, count, makeReputable int) (ids []storj.NodeID) {
	for i := 0; i < count; i++ {
		subnet := strconv.Itoa(i/3) + ".1.2"
		addr := fmt.Sprintf("%s.%d:8080", subnet, i%3+1)
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
			CountryCode: location.Germany + location.CountryCode(i%2),
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

func TestRefreshConcurrent(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// concurrent cache.Refresh with high staleness, where high staleness means the
	// cache should only be refreshed the first time we call cache.Refresh
	mockDB := overlay.Mockdb{}
	cache, err := overlay.NewUploadSelectionCache(zap.NewNop(),
		&mockDB,
		highStaleness,
		nodeSelectionConfig,
		nodeselection.NodeFilters{},
		nodeselection.TestPlacementDefinitions(),
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

	require.Equal(t, 1, mockDB.CallCount)

	// concurrent cache.Refresh with low staleness, where low staleness
	// means that the cache will refresh *every time* cache.Refresh is called
	mockDB = overlay.Mockdb{}
	cache, err = overlay.NewUploadSelectionCache(zap.NewNop(),
		&mockDB,
		lowStaleness,
		nodeSelectionConfig,
		nodeselection.NodeFilters{},
		nodeselection.TestPlacementDefinitions(),
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

	require.True(t, 1 <= mockDB.CallCount && mockDB.CallCount <= 2, "calls %d", mockDB.CallCount)
}

func TestSelectNodes(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		var nodeSelectionConfig = overlay.NodeSelectionConfig{
			NewNodeFraction:  0.2,
			MinimumVersion:   "v1.0.0",
			OnlineWindow:     4 * time.Hour,
			DistinctIP:       true,
			MinimumDiskSpace: 100 * memory.MiB,
		}
		placementRules := nodeselection.TestPlacementDefinitionsWithFraction(nodeSelectionConfig.NewNodeFraction)
		placementRules.AddPlacementRule(storj.PlacementConstraint(5), nodeselection.NodeFilters{}.WithCountryFilter(location.NewSet(location.Germany)), nodeselection.DefaultDownloadSelector)
		placementRules.AddPlacementRule(storj.PlacementConstraint(6), nodeselection.WithAnnotation(nodeselection.NodeFilters{}.WithCountryFilter(location.NewSet(location.Germany)), nodeselection.AutoExcludeSubnet, nodeselection.AutoExcludeSubnetOFF), nodeselection.DefaultDownloadSelector)

		cache, err := overlay.NewUploadSelectionCache(zap.NewNop(),
			db.OverlayCache(),
			lowStaleness,
			nodeSelectionConfig,
			nodeselection.NodeFilters{},
			placementRules,
		)
		require.NoError(t, err)

		cacheCtx, cacheCancel := context.WithCancel(ctx)
		defer cacheCancel()
		ctx.Go(func() error { return cache.Run(cacheCtx) })

		// add 10 nodes to the database and vet 8
		// 4 subnets   [A  A  A  B  B  B  C  C  C  D]
		// 2 countries [DE X  DE x  DE x  DE x  DE x]
		// vetted      [1  1  1  1  1  1  1  1  0  0]
		const nodeCount = 10
		nodeIds := addNodesToNodesTable(ctx, t, db.OverlayCache(), nodeCount, 8)
		require.Len(t, nodeIds, 8)

		t.Run("normal selection", func(t *testing.T) {
			t.Run("get 2", func(t *testing.T) {
				t.Parallel()
				// confirm cache.GetNodes returns the correct nodes
				selectedNodes, err := cache.GetNodes(ctx, overlay.FindStorageNodesRequest{RequestedCount: 2})
				require.NoError(t, err)
				require.Len(t, selectedNodes, 2)

				for _, node := range selectedNodes {
					require.NotEqual(t, node.ID, "")
					require.NotEqual(t, node.Address.Address, "")
					require.NotEqual(t, node.LastIPPort, "")
					require.NotEqual(t, node.LastNet, "")
					require.NotEqual(t, node.LastNet, "")
				}
			})
			t.Run("too much", func(t *testing.T) {
				t.Parallel()
				// we have 5 subnets (1 new, 4 vetted), with two nodes in each
				_, err := cache.GetNodes(ctx, overlay.FindStorageNodesRequest{RequestedCount: 6})
				require.Error(t, err)
			})

		})

		t.Run("using country filter", func(t *testing.T) {
			t.Run("normal", func(t *testing.T) {
				t.Parallel()
				selectedNodes, err := cache.GetNodes(ctx, overlay.FindStorageNodesRequest{
					RequestedCount: 3,
					Placement:      5,
				})
				require.NoError(t, err)
				require.Len(t, selectedNodes, 3)
			})
			t.Run("too much", func(t *testing.T) {
				t.Parallel()
				_, err := cache.GetNodes(ctx, overlay.FindStorageNodesRequest{
					RequestedCount: 4,
					Placement:      5,
				})
				require.Error(t, err)
			})
		})

		t.Run("using country without subnets", func(t *testing.T) {
			t.Run("normal", func(t *testing.T) {
				t.Parallel()
				// it's possible to get 5 only because we don't use subnet exclusions.
				selectedNodes, err := cache.GetNodes(ctx, overlay.FindStorageNodesRequest{
					RequestedCount: 5,
					Placement:      6,
				})
				require.NoError(t, err)
				require.Len(t, selectedNodes, 5)
			})
			t.Run("too much", func(t *testing.T) {
				t.Parallel()
				_, err := cache.GetNodes(ctx, overlay.FindStorageNodesRequest{
					RequestedCount: 6,
					Placement:      6,
				})
				require.Error(t, err)
			})
		})

		t.Run("using country without subnets and exclusions", func(t *testing.T) {
			// DE nodes: 0 (subet:A), 2 (A), 4 (B) 6(C) 8(C, but not vetted)
			// if everything works well, we can exclude 0, and got 3 (2,4,6)
			// unless somebody removes the 2 (because it's in the same subnet as 0)
			selectedNodes, err := cache.GetNodes(ctx, overlay.FindStorageNodesRequest{
				RequestedCount: 3,
				Placement:      6,
				ExcludedIDs: []storj.NodeID{
					nodeIds[0],
				},
			})
			require.NoError(t, err)
			require.Len(t, selectedNodes, 3)
		})

		t.Run("check subnet selection", func(t *testing.T) {
			for i := 0; i < 10; i++ {
				selectedNodes, err := cache.GetNodes(ctx, overlay.FindStorageNodesRequest{
					RequestedCount: 3,
					Placement:      0,
				})
				require.NoError(t, err)
				require.Len(t, selectedNodes, 3)

				// Becaus in how is setup the overlay cache for this test, there should be 0 or 1 unvetted
				// node and in that case it may be in the same subnet of a vetted node or in a new one.

				subnets := map[string]*nodeselection.SelectedNode{}
				for _, node := range selectedNodes {
					if prev, ok := subnets[node.LastNet]; ok {
						// xor between the already tracked and the one in this iteration.
						require.True(t, (prev.Vetted || node.Vetted) && !(prev.Vetted && node.Vetted))
					} else {
						subnets[node.LastNet] = node
					}
				}

				// 2 or 3 depending if an unvetted node was selected and it's the same subnet of any of the
				// other 2 vetted nodes.
				require.GreaterOrEqual(t, len(subnets), 2)
			}
		})

	})
}

func TestGetNodesExcludeCountryCodes(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 2, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		planet.StorageNodes[0].Contact.Chore.Pause(ctx)

		err := planet.Satellites[0].Overlay.Service.TestSetNodeCountryCode(ctx, planet.StorageNodes[0].ID(), "FR")
		require.NoError(t, err)

		cache := planet.Satellites[0].Overlay.Service.UploadSelectionCache

		// confirm cache.GetNodes returns the correct nodes
		selectedNodes, err := cache.GetNodes(ctx, overlay.FindStorageNodesRequest{RequestedCount: 2})
		// we only expect one node to be returned, even though we requested two, so there will be an error
		require.Error(t, err)

		require.Equal(t, 1, len(selectedNodes))
		// the node that was returned should be the one that does not have the "FR" country code
		require.Equal(t, planet.StorageNodes[1].ID(), selectedNodes[0].ID)
	})
}

func TestGetNodesConcurrent(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	reputableNodes := []*nodeselection.SelectedNode{{
		ID:         storj.NodeID{1},
		Address:    &pb.NodeAddress{Address: "127.0.0.9"},
		LastNet:    "127.0.0",
		LastIPPort: "127.0.0.9:8000",
	}}
	newNodes := []*nodeselection.SelectedNode{{
		ID:         storj.NodeID{1},
		Address:    &pb.NodeAddress{Address: "127.0.0.10"},
		LastNet:    "127.0.0",
		LastIPPort: "127.0.0.10:8000",
	}}

	// concurrent GetNodes with high staleness, where high staleness means the
	// cache should only be refreshed the first time we call cache.GetNodes
	mockDB := overlay.Mockdb{
		Reputable: reputableNodes,
		New:       newNodes,
	}
	cache, err := overlay.NewUploadSelectionCache(zap.NewNop(),
		&mockDB,
		highStaleness,
		nodeSelectionConfig,
		nodeselection.NodeFilters{},
		nodeselection.TestPlacementDefinitionsWithFraction(1),
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
	require.Equal(t, 1, mockDB.CallCount)

	// concurrent get nodes with low staleness, where low staleness means that
	// the cache will refresh each time cache.GetNodes is called
	mockDB = overlay.Mockdb{
		Reputable: reputableNodes,
		New:       newNodes,
	}
	cache, err = overlay.NewUploadSelectionCache(zap.NewNop(),
		&mockDB,
		lowStaleness,
		nodeSelectionConfig,
		nodeselection.NodeFilters{},
		nodeselection.TestPlacementDefinitionsWithFraction(1),
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
	require.True(t, 1 <= mockDB.CallCount && mockDB.CallCount <= 2, "calls %d", mockDB.CallCount)
}

func TestGetNodesDistinct(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	reputableNodes := []*nodeselection.SelectedNode{{
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

	newNodes := []*nodeselection.SelectedNode{{
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

	mockDB := overlay.Mockdb{
		Reputable: reputableNodes,
		New:       newNodes,
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
			nodeselection.NodeFilters{},
			nodeselection.TestPlacementDefinitions(),
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
		for _, nodeList := range [][]*nodeselection.SelectedNode{reputableNodes, newNodes} {
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
			nodeselection.NodeFilters{},
			nodeselection.TestPlacementDefinitions(),
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

	mockDB := overlay.Mockdb{}
	cache, err := overlay.NewUploadSelectionCache(zap.NewNop(),
		&mockDB,
		highStaleness,
		nodeSelectionConfig,
		nodeselection.NodeFilters{},
		nodeselection.TestPlacementDefinitions(),
	)
	require.NoError(t, err)

	cacheCtx, cacheCancel := context.WithCancel(ctx)
	defer cacheCancel()
	ctx.Go(func() error { return cache.Run(cacheCtx) })

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
			nodeselection.NodeFilters{},
			nodeselection.TestPlacementDefinitionsWithFraction(newNodeFraction),
		)
		require.NoError(t, err)

		cacheCtx, cacheCancel := context.WithCancel(ctx)
		defer cacheCancel()
		ctx.Go(func() error { return cache.Run(cacheCtx) })

		// the cache should have no nodes to start
		err = cache.Refresh(ctx)
		require.NoError(t, err)

		// add some nodes to the database, some are reputable and some are new nodes
		// 3 nodes per net --> we need 4 net (* 3 node) reputable + 1 net (* 3 node) new to select 5 with 0.2 percentage new
		const nodeCount = 15
		repIDs := addNodesToNodesTable(ctx, t, db.OverlayCache(), nodeCount, 12)
		require.Len(t, repIDs, 12)
		// confirm nodes are in the cache once
		err = cache.Refresh(ctx)
		require.NoError(t, err)

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

func BenchmarkGetNodes(b *testing.B) {
	newNodes := 2000
	oldNodes := 18000
	required := 110
	if testing.Short() {
		newNodes = 10
		oldNodes = 50
		required = 2
	}

	ctx, cancel := context.WithCancel(testcontext.New(b))
	defer cancel()
	log, err := zap.NewDevelopment()
	require.NoError(b, err)
	placement := nodeselection.TestPlacementDefinitions()
	placement.AddLegacyStaticRules()
	defaultFilter := nodeselection.NodeFilters{}

	db := NewMockUploadSelectionDb(
		generatedSelectedNodes(b, oldNodes),
		generatedSelectedNodes(b, newNodes),
	)
	cache, err := overlay.NewUploadSelectionCache(log, db, 10*time.Minute, overlay.NodeSelectionConfig{
		NewNodeFraction: 0.1,
	}, defaultFilter, placement)
	require.NoError(b, err)

	go func() {
		_ = cache.Run(ctx)
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := cache.GetNodes(ctx, overlay.FindStorageNodesRequest{
			RequestedCount: required,
			Placement:      storj.US,
		})
		require.NoError(b, err)
	}
}

// MockUploadSelection implements overlay.UploadSelectionDB with a static list.
type MockUploadSelectionDB struct {
	new       []*nodeselection.SelectedNode
	reputable []*nodeselection.SelectedNode
}

// NewMockUploadSelectionDb creates a MockUploadSelectionDB with the given reputable and new nodes.
func NewMockUploadSelectionDb(reputable, new []*nodeselection.SelectedNode) *MockUploadSelectionDB {
	return &MockUploadSelectionDB{
		new:       new,
		reputable: reputable,
	}

}

// SelectAllStorageNodesUpload implements overlay.UploadSelectionDB.
func (m MockUploadSelectionDB) SelectAllStorageNodesUpload(ctx context.Context, selectionCfg overlay.NodeSelectionConfig) (reputable, new []*nodeselection.SelectedNode, err error) {
	return m.reputable, m.new, nil
}

var _ overlay.UploadSelectionDB = &MockUploadSelectionDB{}

func generatedSelectedNodes(b *testing.B, nodeNo int) []*nodeselection.SelectedNode {
	nodes := make([]*nodeselection.SelectedNode, nodeNo)
	ctx := testcontext.New(b)
	for i := 0; i < nodeNo; i++ {
		node := nodeselection.SelectedNode{}
		identity, err := testidentity.NewTestIdentity(ctx)
		require.NoError(b, err)
		node.ID = identity.ID

		// with 5% percentage chance, we re-use an existing IP address.
		if rand.Intn(100) < 5 && i > 0 {
			prevParts := strings.Split(nodes[rand.Intn(i)].LastIPPort, ":")
			node.LastIPPort = fmt.Sprintf("%s:%d", prevParts[0], rand.Int31n(10000)+1000)
		} else {
			node.LastIPPort = fmt.Sprintf("%d.%d.%d.%d:%d", 10+i/256/256%256, i/256%256, i%256, 1, rand.Int31n(10000)+1000)
		}

		parts := strings.Split(node.LastIPPort, ".")
		node.LastNet = fmt.Sprintf("%s.%s.%s.0", parts[0], parts[1], parts[2])
		node.CountryCode = []location.CountryCode{location.None, location.UnitedStates, location.Germany, location.Hungary, location.Austria}[i%5]
		nodes[i] = &node
	}
	return nodes
}
