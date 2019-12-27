// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testblobs"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/gracefulexit"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/uplink"
)

func TestWorkerSuccess(t *testing.T) {
	successThreshold := 4
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: successThreshold + 1,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		ul := planet.Uplinks[0]

		satellite.GracefulExit.Chore.Loop.Pause()

		rs := &uplink.RSConfig{
			MinThreshold:     2,
			RepairThreshold:  3,
			SuccessThreshold: successThreshold,
			MaxThreshold:     successThreshold,
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

		// run the SN chore again to start processing transfers.
		worker := gracefulexit.NewWorker(zaptest.NewLogger(t), exitingNode.Storage2.Store, exitingNode.DB.Satellites(), exitingNode.Dialer, satellite.ID(), satellite.Addr(),
			gracefulexit.Config{
				ChoreInterval:          0,
				NumWorkers:             2,
				NumConcurrentTransfers: 2,
				MinBytesPerSecond:      128,
				MinDownloadTimeout:     2 * time.Minute,
			})
		defer ctx.Check(worker.Close)

		err = worker.Run(ctx, func() {})
		require.NoError(t, err)

		progress, err := satellite.DB.GracefulExit().GetProgress(ctx, exitingNode.ID())
		require.NoError(t, err)
		require.EqualValues(t, progress.PiecesFailed, 0)
		require.EqualValues(t, progress.PiecesTransferred, 1)

		exitStatus, err := satellite.DB.OverlayCache().GetExitStatus(ctx, exitingNode.ID())
		require.NoError(t, err)
		require.NotNil(t, exitStatus.ExitFinishedAt)
		require.True(t, exitStatus.ExitSuccess)
	})
}

func TestWorkerTimeout(t *testing.T) {
	successThreshold := 4
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: successThreshold + 1,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			NewStorageNodeDB: func(index int, db storagenode.DB, log *zap.Logger) (storagenode.DB, error) {
				return testblobs.NewSlowDB(log.Named("slowdb"), db), nil
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		ul := planet.Uplinks[0]

		satellite.GracefulExit.Chore.Loop.Pause()

		rs := &uplink.RSConfig{
			MinThreshold:     2,
			RepairThreshold:  3,
			SuccessThreshold: successThreshold,
			MaxThreshold:     successThreshold,
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

		// run the SN chore again to start processing transfers.
		worker := gracefulexit.NewWorker(zaptest.NewLogger(t), store, exitingNode.DB.Satellites(), exitingNode.Dialer, satellite.ID(), satellite.Addr(),
			gracefulexit.Config{
				ChoreInterval:          0,
				NumWorkers:             2,
				NumConcurrentTransfers: 2,
				// This config value will create a very short timeframe allowed for receiving
				// data from storage nodes. This will cause context to cancel with timeout.
				MinBytesPerSecond:  10 * memory.MiB,
				MinDownloadTimeout: 2 * time.Millisecond,
			})
		defer ctx.Check(worker.Close)

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
