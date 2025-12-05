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
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/nodeevents"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair/checker"
)

func TestReliabilityCache_Concurrent(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	overlayCache, err := overlay.NewService(zap.NewNop(), fakeOverlayDB{}, fakeNodeEvents{}, nodeselection.TestPlacementDefinitionsWithFraction(1), "", "", overlay.Config{
		NodeSelectionCache: overlay.UploadSelectionCacheConfig{
			Staleness: 2 * time.Nanosecond,
		},
	})
	require.NoError(t, err)
	cacheCtx, cacheCancel := context.WithCancel(ctx)
	defer cacheCancel()
	ctx.Go(func() error { return overlayCache.Run(cacheCtx) })
	defer ctx.Check(overlayCache.Close)

	cache := checker.NewReliabilityCache(overlayCache, time.Millisecond, 5*time.Minute)
	var group errgroup.Group
	for i := 0; i < 10; i++ {
		group.Go(func() error {
			for i := 0; i < 10000; i++ {
				nodeIDs := []storj.NodeID{testrand.NodeID()}
				_, err := cache.GetNodes(ctx, time.Now(), nodeIDs, make([]nodeselection.SelectedNode, len(nodeIDs)))
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

func (fakeOverlayDB) GetAllParticipatingNodes(context.Context, time.Duration, time.Duration) ([]nodeselection.SelectedNode, error) {
	return []nodeselection.SelectedNode{
		{ID: testrand.NodeID(), Online: true},
		{ID: testrand.NodeID(), Online: true},
		{ID: testrand.NodeID(), Online: true},
		{ID: testrand.NodeID(), Online: true},
	}, nil
}
