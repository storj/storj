// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package monitor_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/pb"
)

func TestMonitor(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		var freeBandwidth int64
		for _, storageNode := range planet.StorageNodes {
			storageNode.Storage2.Monitor.Loop.Pause()

			info, err := storageNode.Kademlia.Service.FetchInfo(ctx, storageNode.Local().Node)
			require.NoError(t, err)

			// assume that all storage nodes have the same initial values
			freeBandwidth = info.Capacity.FreeBandwidth
		}

		expectedData := testrand.Bytes(100 * memory.KiB)

		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path", expectedData)
		require.NoError(t, err)

		nodeAssertions := 0
		for _, storageNode := range planet.StorageNodes {
			storageNode.Storage2.Monitor.Loop.TriggerWait()

			info, err := storageNode.Kademlia.Service.FetchInfo(ctx, storageNode.Local().Node)
			require.NoError(t, err)

			stats, err := storageNode.Storage2.Inspector.Stats(ctx, &pb.StatsRequest{})
			require.NoError(t, err)
			if stats.UsedSpace > 0 {
				assert.Equal(t, freeBandwidth-stats.UsedBandwidth, info.Capacity.FreeBandwidth)
				nodeAssertions++
			}
		}
		assert.NotZero(t, nodeAssertions, "No storage node were verifed")
	})
}
