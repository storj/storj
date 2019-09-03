// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/uplink"
)

func TestQueue(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 5, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		satellite.Audit.Worker.Loop.Pause()
		satellite.Audit.Chore.Loop.Pause()

		ul := planet.Uplinks[0]

		// upload 2 remote files with 1 segment
		for i := 0; i < 2; i++ {
			testData := testrand.Bytes(8 * memory.KiB)
			path := "/some/remote/path/" + string(i)
			err := ul.UploadWithConfig(ctx, satellite, &uplink.RSConfig{
				MinThreshold:     3,
				RepairThreshold:  4,
				SuccessThreshold: 5,
				MaxThreshold:     5,
			}, "testbucket", path, testData)
			require.NoError(t, err)
		}

		satellite.Audit.Chore.Loop.Restart()
		satellite.Audit.Chore.Loop.TriggerWait()

		uniquePaths := make(map[storj.Path]struct{})
		assert.Len(t, satellite.Audit.Queue.Queue, 2)

		for _, path := range satellite.Audit.Queue.Queue {
			_, ok := uniquePaths[path]
			require.False(t, ok, "expected unique path in chore queue")

			uniquePaths[path] = struct{}{}
		}
		require.Len(t, uniquePaths, 2)

		satellite.Audit.Worker.Loop.Restart()
		satellite.Audit.Worker.Loop.TriggerWait()

		require.Len(t, satellite.Audit.Queue.Queue, 0)
	})
}
