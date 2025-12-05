// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/errs2"
	"storj.io/common/memory"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/gracefulexit"
)

func TestWorkerFailure_IneligibleNodeAge(t *testing.T) {
	const successThreshold = 4
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 5,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					// Set the required node age to 1 month.
					config.GracefulExit.NodeMinAgeInMonths = 1
				},
				testplanet.ReconfigureRS(2, 3, successThreshold, successThreshold),
			),

			StorageNode: func(index int, config *storagenode.Config) {
				config.GracefulExit.NumWorkers = 2
				config.GracefulExit.NumConcurrentTransfers = 2
				config.GracefulExit.MinBytesPerSecond = 128
				config.GracefulExit.MinDownloadTimeout = 2 * time.Minute
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		ul := planet.Uplinks[0]

		err := ul.Upload(ctx, satellite, "testbucket", "test/path1", testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)

		exitingNode, err := findNodeToExit(ctx, planet)
		require.NoError(t, err)
		exitingNode.GracefulExit.Chore.Loop.Pause()

		err = exitingNode.DB.Satellites().InitiateGracefulExit(ctx, satellite.ID(), time.Now(), 0)
		require.NoError(t, err)

		worker := gracefulexit.NewWorker(zaptest.NewLogger(t), exitingNode.GracefulExit.Service, exitingNode.Dialer, satellite.NodeURL(), exitingNode.Config.GracefulExit)
		err = worker.Run(ctx)
		require.Error(t, err)
		require.True(t, errs2.IsRPC(err, rpcstatus.FailedPrecondition))

		result, err := exitingNode.DB.Satellites().ListGracefulExits(ctx)
		require.NoError(t, err)
		require.Len(t, result, 0)
	})
}
