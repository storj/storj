// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/audit"
)

func TestChoreAndWorkerIntegration(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 5, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		audits := satellite.Audit
		audits.Worker.Loop.Pause()
		audits.Chore.Loop.Pause()

		ul := planet.Uplinks[0]

		// Upload 2 remote files with 1 segment.
		for i := 0; i < 2; i++ {
			testData := testrand.Bytes(8 * memory.KiB)
			path := "/some/remote/path/" + strconv.Itoa(i)
			err := ul.Upload(ctx, satellite, "testbucket", path, testData)
			require.NoError(t, err)
		}

		audits.Chore.Loop.TriggerWait()
		require.EqualValues(t, 2, audits.Queue.Size(), "audit queue")

		uniquePaths := make(map[storj.Path]struct{})
		var err error
		var path storj.Path
		var pathCount int
		for {
			path, err = audits.Queue.Next()
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
		require.Equal(t, 0, audits.Queue.Size())

		// Repopulate the queue for the worker.
		audits.Chore.Loop.TriggerWait()
		require.EqualValues(t, 2, audits.Queue.Size(), "audit queue")

		// Make sure the worker processes all the items in the audit queue.
		audits.Worker.Loop.TriggerWait()
		require.EqualValues(t, 0, audits.Queue.Size(), "audit queue")
	})
}
