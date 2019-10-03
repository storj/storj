// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/uplink"
)

func TestChore(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 8,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		exitingNode := planet.StorageNodes[1]

		satellite.GracefulExit.Chore.Loop.Pause()

		rs := &uplink.RSConfig{
			MinThreshold:     4,
			RepairThreshold:  6,
			SuccessThreshold: 8,
			MaxThreshold:     8,
		}

		err := uplinkPeer.UploadWithConfig(ctx, satellite, rs, "testbucket", "test/path1", testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)

		rs = &uplink.RSConfig{
			MinThreshold:     4,
			RepairThreshold:  6,
			SuccessThreshold: 8,
			MaxThreshold:     8,
		}

		err = uplinkPeer.UploadWithConfig(ctx, satellite, rs, "testbucket", "test/path2", testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)

		exitStatus := overlay.ExitStatusRequest{
			NodeID:          exitingNode.ID(),
			ExitInitiatedAt: time.Now().UTC(),
		}

		_, err = satellite.Overlay.DB.UpdateExitStatus(ctx, &exitStatus)
		require.NoError(t, err)

		satellite.GracefulExit.Chore.Loop.TriggerWait()

		incompleteTransfers, err := satellite.DB.GracefulExit().GetIncomplete(ctx, exitingNode.ID(), 20, 0)
		require.NoError(t, err)
		require.Len(t, incompleteTransfers, 2)

		// Test the other nodes don't have anything to transfer
		for _, sn := range planet.StorageNodes {
			if sn.ID() == exitingNode.ID() {
				continue
			}
			incompleteTransfers, err := satellite.DB.GracefulExit().GetIncomplete(ctx, sn.ID(), 20, 0)
			require.NoError(t, err)
			require.Len(t, incompleteTransfers, 0)
		}
	})
}
