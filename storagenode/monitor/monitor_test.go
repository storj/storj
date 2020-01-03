// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package monitor_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
)

func TestMonitor(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		var freeBandwidthInit int64
		for _, storageNode := range planet.StorageNodes {
			storageNode.Storage2.Monitor.Loop.Pause()

			// assume that all storage nodes have the same initial values
			freeBandwidthInit = storageNode.Local().Capacity.FreeBandwidth
		}

		expectedData := testrand.Bytes(100 * memory.KiB)

		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path", expectedData)
		require.NoError(t, err)

		nodeAssertions := 0
		for _, storageNode := range planet.StorageNodes {
			storageNode.Storage2.Monitor.Loop.TriggerWait()

			freeBandwidthNew := storageNode.Local().Capacity.FreeBandwidth
			stats, err := storageNode.Storage2.Inspector.Stats(ctx, &pb.StatsRequest{})
			require.NoError(t, err)
			if stats.UsedSpace > 0 {
				assert.Equal(t, freeBandwidthInit-stats.UsedBandwidth, freeBandwidthNew)
				nodeAssertions++
			}
		}
		assert.NotZero(t, nodeAssertions, "No storage node were verifed")
	})
}
