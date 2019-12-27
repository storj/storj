// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metrics_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/uplink"
)

func TestCounterInlineAndRemote(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 5, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		ul := planet.Uplinks[0]
		metricsChore := satellite.Metrics.Chore
		metricsChore.Loop.Pause()

		segmentSize := 8 * memory.KiB

		// upload 2 inline files
		for i := 0; i < 2; i++ {
			testData := testrand.Bytes(segmentSize / 8)
			path := "/some/inline/path/" + string(i)
			err := ul.Upload(ctx, satellite, "bucket", path, testData)
			require.NoError(t, err)
		}

		// upload 2 remote files with 1 segment
		for i := 0; i < 2; i++ {
			testData := testrand.Bytes(segmentSize)
			path := "/some/remote/path/" + string(i)
			err := ul.UploadWithConfig(ctx, satellite, &uplink.RSConfig{
				MinThreshold:     3,
				RepairThreshold:  4,
				SuccessThreshold: 5,
				MaxThreshold:     5,
			}, "testbucket", path, testData)
			require.NoError(t, err)
		}

		metricsChore.Loop.TriggerWait()
		require.EqualValues(t, 2, metricsChore.Counter.Inline)
		require.EqualValues(t, 2, metricsChore.Counter.RemoteDependent)
		require.EqualValues(t, 4, metricsChore.Counter.Total)
	})
}

func TestCounterInlineOnly(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 5, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		ul := planet.Uplinks[0]
		metricsChore := satellite.Metrics.Chore
		metricsChore.Loop.Pause()

		// upload 2 inline files
		for i := 0; i < 2; i++ {
			testData := testrand.Bytes(memory.KiB)
			path := "/some/inline/path/" + string(i)
			err := ul.Upload(ctx, satellite, "bucket", path, testData)
			require.NoError(t, err)
		}

		metricsChore.Loop.TriggerWait()
		require.EqualValues(t, 2, metricsChore.Counter.Inline)
		require.EqualValues(t, 0, metricsChore.Counter.RemoteDependent)
		require.EqualValues(t, 2, metricsChore.Counter.Total)
	})
}

func TestCounterRemoteOnly(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 5, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		ul := planet.Uplinks[0]
		metricsChore := satellite.Metrics.Chore
		metricsChore.Loop.Pause()

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

		metricsChore.Loop.TriggerWait()
		require.EqualValues(t, 0, metricsChore.Counter.Inline)
		require.EqualValues(t, 2, metricsChore.Counter.RemoteDependent)
		require.EqualValues(t, 2, metricsChore.Counter.Total)
	})
}
