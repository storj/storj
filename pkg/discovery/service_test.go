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

func TestCache_Graveyard(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		testnode := planet.StorageNodes[0]
		offlineID := testnode.ID()

		satellite.Discovery.Service.Refresh.Pause()
		satellite.Discovery.Service.Graveyard.Pause()
		satellite.Discovery.Service.Discovery.Pause()

		// mark node as offline in overlay cache
		_, err := satellite.Overlay.Service.UpdateUptime(ctx, offlineID, false)
		require.NoError(t, err)

		node, err := satellite.Overlay.Service.Get(ctx, offlineID)
		assert.NoError(t, err)
		assert.False(t, node.Online())

		satellite.Discovery.Service.Graveyard.TriggerWait()

		found, err := satellite.Overlay.Service.Get(ctx, offlineID)
		assert.NoError(t, err)
		assert.Equal(t, offlineID, found.Id)
		assert.True(t, found.Online())
	})
}
