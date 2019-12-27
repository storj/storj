// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package bandwidth_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/private/teststorj"
)

// Simple test for ensuring the service runs Rollups in the Loop
func TestBandwidthService(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 3, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		testID1 := teststorj.NodeIDFromString("testId1")

		for _, storageNode := range planet.StorageNodes {
			// stop bandwidth service, so we can run it manually
			storageNode.Bandwidth.Loop.Pause()

			// Create data for an hour ago so we can rollup
			err := storageNode.DB.Bandwidth().Add(ctx, testID1, pb.PieceAction_PUT, 2, time.Now().Add(time.Hour*-2))
			require.NoError(t, err)
			err = storageNode.DB.Bandwidth().Add(ctx, testID1, pb.PieceAction_GET, 3, time.Now().Add(time.Hour*-2))
			require.NoError(t, err)
			err = storageNode.DB.Bandwidth().Add(ctx, testID1, pb.PieceAction_GET_AUDIT, 4, time.Now().Add(time.Hour*-2))
			require.NoError(t, err)

			storageNode.Bandwidth.Loop.TriggerWait()

			usage, err := storageNode.DB.Bandwidth().Summary(ctx, time.Now().Add(time.Hour*-48), time.Now())
			require.NoError(t, err)
			require.Equal(t, int64(9), usage.Total())
		}
	})
}
