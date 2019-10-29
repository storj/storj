// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testblobs"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/gracefulexit"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/uplink"
)

func TestWorkerTimeout(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 9,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			NewStorageNodeDB: func(index int, db storagenode.DB, log *zap.Logger) (storagenode.DB, error) {
				return testblobs.NewSlowDB(log.Named("slowdb"), db), nil
			},
			StorageNode: func(index int, config *storagenode.Config) {
				// This config value will create a very short timeframe allowed for receiving
				// data from storage nodes. This will cause context to cancel with timeout.
				config.GracefulExit.MinDownloadTimeout = 10 * time.Millisecond
			},
			Satellite: func(logger *zap.Logger, index int, config *satellite.Config) {
				// This config value will create a very short timeframe allowed for receiving
				// data from storage nodes. This will cause context to cancel with timeout.
				config.GracefulExit.RecvTimeout = 1 * time.Minute
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		ul := planet.Uplinks[0]

		satellite.GracefulExit.Chore.Loop.Pause()

		rs := &uplink.RSConfig{
			MinThreshold:     4,
			RepairThreshold:  6,
			SuccessThreshold: 8,
			MaxThreshold:     8,
		}

		err := ul.UploadWithConfig(ctx, satellite, rs, "testbucket", "test/path1", testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)

		exitingNode, err := findNodeToExit(ctx, planet, 1)
		require.NoError(t, err)
		exitingNode.GracefulExit.Chore.Loop.Pause()

		exitStatusReq := overlay.ExitStatusRequest{
			NodeID:          exitingNode.ID(),
			ExitInitiatedAt: time.Now(),
		}
		_, err = satellite.Overlay.DB.UpdateExitStatus(ctx, &exitStatusReq)
		require.NoError(t, err)

		// run the satellite chore to build the transfer queue.
		satellite.GracefulExit.Chore.Loop.TriggerWait()
		satellite.GracefulExit.Chore.Loop.Pause()

		// check that the satellite knows the storage node is exiting.
		exitingNodes, err := satellite.DB.OverlayCache().GetExitingNodes(ctx)
		require.NoError(t, err)
		require.Len(t, exitingNodes, 1)
		require.Equal(t, exitingNode.ID(), exitingNodes[0].NodeID)

		queueItems, err := satellite.DB.GracefulExit().GetIncomplete(ctx, exitingNode.ID(), 10, 0)
		require.NoError(t, err)
		require.Len(t, queueItems, 1)

		storageNodeDB := exitingNode.DB.(*testblobs.SlowDB)
		// make uploads on storage node slower than the timeout for transferring bytes to another node
		delay := 200 * time.Millisecond
		storageNodeDB.SetLatency(delay)
		store := pieces.NewStore(zaptest.NewLogger(t), storageNodeDB.Pieces(), nil, nil, storageNodeDB.PieceSpaceUsedDB())

		config := gracefulexit.Config{
			// defaultInterval is 15 seconds in testplanet
			ChoreInterval:      15 * time.Second,
			NumWorkers:         1,
			MinBytesPerSecond:  10 * memory.MiB,
			MinDownloadTimeout: 2 * time.Millisecond,
		}

		// run the SN chore again to start processing transfers.
		//exitingNode.GracefulExit.Chore.Loop.TriggerWait()
		worker := gracefulexit.NewWorker(zaptest.NewLogger(t), store, exitingNode.DB.Satellites(), exitingNode.Dialer, satellite.ID(), satellite.Addr(), config)
		err = worker.Run(ctx, func() {})
		require.NoError(t, err)

		progress, err := satellite.DB.GracefulExit().GetProgress(ctx, exitingNode.ID())
		require.NoError(t, err)
		require.EqualValues(t, progress.PiecesFailed, 1)
		require.EqualValues(t, progress.PiecesTransferred, 0)

		exitStatus, err := satellite.DB.OverlayCache().GetExitStatus(ctx, exitingNode.ID())
		require.NoError(t, err)
		require.NotNil(t, exitStatus.ExitFinishedAt)
		require.False(t, exitStatus.ExitSuccess)
	})
}
