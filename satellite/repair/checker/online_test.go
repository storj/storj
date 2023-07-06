// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package checker_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/storj"
	"storj.io/common/storj/location"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeevents"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair/checker"
)

func TestReliabilityCache_Concurrent(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	overlayCache, err := overlay.NewService(zap.NewNop(), fakeOverlayDB{}, fakeNodeEvents{}, overlay.NewPlacementRules().CreateFilters, "", "", overlay.Config{
		NodeSelectionCache: overlay.UploadSelectionCacheConfig{
			Staleness: 2 * time.Nanosecond,
		},
	})
	require.NoError(t, err)
	cacheCtx, cacheCancel := context.WithCancel(ctx)
	defer cacheCancel()
	ctx.Go(func() error { return overlayCache.Run(cacheCtx) })
	defer ctx.Check(overlayCache.Close)

	cache := checker.NewReliabilityCache(overlayCache, time.Millisecond, overlay.NewPlacementRules().CreateFilters, []string{})
	var group errgroup.Group
	for i := 0; i < 10; i++ {
		group.Go(func() error {
			for i := 0; i < 10000; i++ {
				pieces := []metabase.Piece{{StorageNode: testrand.NodeID()}}
				_, err := cache.MissingPieces(ctx, time.Now(), pieces)
				if err != nil {
					return err
				}
			}
			return nil
		})
	}
	require.NoError(t, group.Wait())
}

type fakeOverlayDB struct{ overlay.DB }
type fakeNodeEvents struct{ nodeevents.DB }

func (fakeOverlayDB) Reliable(context.Context, time.Duration, time.Duration) ([]nodeselection.SelectedNode, []nodeselection.SelectedNode, error) {
	return []nodeselection.SelectedNode{
		{ID: testrand.NodeID()},
		{ID: testrand.NodeID()},
		{ID: testrand.NodeID()},
		{ID: testrand.NodeID()},
	}, nil, nil
}

func TestReliabilityCache_OutOfPlacementPieces(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Overlay.Node.AsOfSystemTime.Enabled = false
				config.Overlay.Node.AsOfSystemTime.DefaultInterval = 0
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		overlayService := planet.Satellites[0].Overlay.Service
		config := planet.Satellites[0].Config.Checker

		rules := overlay.NewPlacementRules()
		rules.AddLegacyStaticRules()
		cache := checker.NewReliabilityCache(overlayService, config.ReliabilityCacheStaleness, rules.CreateFilters, []string{})

		nodesPlacement := func(location location.CountryCode, nodes ...*testplanet.StorageNode) {
			for _, node := range nodes {
				err := overlayService.TestNodeCountryCode(ctx, node.ID(), location.String())
				require.NoError(t, err)
			}
			require.NoError(t, cache.Refresh(ctx))
		}

		allPieces := metabase.Pieces{
			metabase.Piece{Number: 0, StorageNode: planet.StorageNodes[0].ID()},
			metabase.Piece{Number: 1, StorageNode: planet.StorageNodes[1].ID()},
			metabase.Piece{Number: 2, StorageNode: planet.StorageNodes[2].ID()},
			metabase.Piece{Number: 3, StorageNode: planet.StorageNodes[3].ID()},
		}

		pieces, err := cache.OutOfPlacementPieces(ctx, time.Now().Add(-time.Hour), metabase.Pieces{}, storj.EU)
		require.NoError(t, err)
		require.Empty(t, pieces)

		nodesPlacement(location.Poland, planet.StorageNodes...)
		pieces, err = cache.OutOfPlacementPieces(ctx, time.Now().Add(-time.Hour), allPieces, storj.EU)
		require.NoError(t, err)
		require.Empty(t, pieces)

		pieces, err = cache.OutOfPlacementPieces(ctx, time.Now().Add(-time.Hour), allPieces, storj.US)
		require.NoError(t, err)
		require.ElementsMatch(t, allPieces, pieces)

		nodesPlacement(location.UnitedStates, planet.StorageNodes[:2]...)
		pieces, err = cache.OutOfPlacementPieces(ctx, time.Now().Add(-time.Hour), allPieces, storj.EU)
		require.NoError(t, err)
		require.ElementsMatch(t, allPieces[:2], pieces)

		pieces, err = cache.OutOfPlacementPieces(ctx, time.Now().Add(-time.Hour), allPieces, storj.US)
		require.NoError(t, err)
		require.ElementsMatch(t, allPieces[2:], pieces)
	})
}
