// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/audit"
)

func TestChoreAndWorkerIntegration(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 5, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		satellite.Audit.Worker.Loop.Pause()
		satellite.Audit.Chore.Loop.Pause()

		ul := planet.Uplinks[0]

		// Upload 2 remote files with 1 segment.
		for i := 0; i < 2; i++ {
			testData := testrand.Bytes(8 * memory.KiB)
			path := "/some/remote/path/" + strconv.Itoa(i)
			err := ul.Upload(ctx, satellite, "testbucket", path, testData)
			require.NoError(t, err)
		}

		satellite.Audit.Chore.Loop.TriggerWait()
		require.EqualValues(t, 2, satellite.Audit.Queue.Size(), "audit queue")

		uniquePaths := make(map[storj.Path]struct{})
		var err error
		var path storj.Path
		var pathCount int
		for {
			path, err = satellite.Audit.Queue.Next()
			if err != nil {
				break
			}
			pathCount++
			_, ok := uniquePaths[path]
			require.False(t, ok, "expected unique path in chore queue")

			uniquePaths[path] = struct{}{}
		}
		require.True(t, audit.ErrEmptyQueue.Has(err))
		require.Equal(t, 2, pathCount)
		require.Equal(t, 0, satellite.Audit.Queue.Size())

		// Repopulate the queue for the worker.
		satellite.Audit.Chore.Loop.TriggerWait()
		require.EqualValues(t, 2, satellite.Audit.Queue.Size(), "audit queue")

		// Make sure the worker processes all the items in the audit queue.
		satellite.Audit.Worker.Loop.TriggerWait()
		require.EqualValues(t, 0, satellite.Audit.Queue.Size(), "audit queue")
	})
}
