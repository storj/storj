// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/storj"
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

		satellite.Audit.Chore.Loop.Restart()
		satellite.Audit.Chore.Loop.TriggerWait()

		assert.Len(t, satellite.Audit.Queue.Queue, 2)

		uniquePaths := make(map[storj.Path]struct{})
		for _, path := range satellite.Audit.Queue.Queue {
			_, ok := uniquePaths[path]
			require.False(t, ok, "expected unique path in chore queue")

			uniquePaths[path] = struct{}{}
		}

		satellite.Audit.Worker.Loop.Restart()
		satellite.Audit.Worker.Loop.TriggerWait()

		require.Len(t, satellite.Audit.Queue.Queue, 0, "audit queue")
	})
}
