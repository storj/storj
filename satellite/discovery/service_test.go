// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package discovery_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
)

func TestCache_Refresh(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		for _, storageNode := range planet.StorageNodes {
			node, err := satellite.Overlay.Service.Get(ctx, storageNode.ID())
			if assert.NoError(t, err) {
				assert.Equal(t, storageNode.Addr(), node.Address.Address)
			}
		}
	})
}

func TestCache_Discovery(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		testnode := planet.StorageNodes[0]
		offlineID := testnode.ID()

		satellite.Kademlia.Service.RefreshBuckets.Pause()

		satellite.Discovery.Service.Refresh.Pause()
		satellite.Discovery.Service.Discovery.Pause()

		overlay := satellite.Overlay.Service

		// mark node as offline in overlay cache
		_, err := overlay.UpdateUptime(ctx, offlineID, false)
		require.NoError(t, err)

		node, err := overlay.Get(ctx, offlineID)
		assert.NoError(t, err)
		assert.False(t, overlay.IsOnline(node))

		satellite.Discovery.Service.Discovery.TriggerWait()

		found, err := overlay.Get(ctx, offlineID)
		assert.NoError(t, err)
		assert.Equal(t, offlineID, found.Id)
		assert.True(t, overlay.IsOnline(found))
	})
}
