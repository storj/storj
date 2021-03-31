// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metrics_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
)

func TestCounterInlineAndRemote(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		ul := planet.Uplinks[0]
		metricsChore := satellite.Metrics.Chore
		metricsChore.Loop.Pause()

		segmentSize := 8 * memory.KiB

		// upload 2 inline files
		for i := 0; i < 2; i++ {
			testData := testrand.Bytes(segmentSize / 8)
			path := "/some/inline/path/" + strconv.Itoa(i)
			err := ul.Upload(ctx, satellite, "bucket", path, testData)
			require.NoError(t, err)
		}

		// upload 2 remote files with 1 segment
		for i := 0; i < 2; i++ {
			testData := testrand.Bytes(segmentSize)
			path := "/some/remote/path/" + strconv.Itoa(i)
			err := ul.Upload(ctx, satellite, "testbucket", path, testData)
			require.NoError(t, err)
		}

		metricsChore.Loop.TriggerWait()
		require.EqualValues(t, 2, metricsChore.Counter.InlineObjectCount())
		require.EqualValues(t, 2, metricsChore.Counter.RemoteDependent)
		require.EqualValues(t, 4, metricsChore.Counter.ObjectCount)
	})
}

func TestCounterInlineOnly(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		ul := planet.Uplinks[0]
		metricsChore := satellite.Metrics.Chore
		metricsChore.Loop.Pause()

		// upload 2 inline files
		for i := 0; i < 2; i++ {
			testData := testrand.Bytes(memory.KiB)
			path := "/some/inline/path/" + strconv.Itoa(i)
			err := ul.Upload(ctx, satellite, "bucket", path, testData)
			require.NoError(t, err)
		}

		metricsChore.Loop.TriggerWait()
		require.EqualValues(t, 2, metricsChore.Counter.InlineObjectCount())
		require.EqualValues(t, 0, metricsChore.Counter.RemoteDependent)
		require.EqualValues(t, 2, metricsChore.Counter.ObjectCount)
	})
}

func TestCounterRemoteOnly(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.MaxSegmentSize(16 * memory.KiB),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		ul := planet.Uplinks[0]
		metricsChore := satellite.Metrics.Chore
		metricsChore.Loop.Pause()

		// upload 2 remote files with multiple segments
		for i := 0; i < 2; i++ {
			testData := testrand.Bytes(32 * memory.KiB)
			path := "/some/remote/path/" + strconv.Itoa(i)
			err := ul.Upload(ctx, satellite, "testbucket", path, testData)
			require.NoError(t, err)
		}

		metricsChore.Loop.TriggerWait()
		t.Log(metricsChore.Counter.ObjectCount, metricsChore.Counter.RemoteDependent)
		require.EqualValues(t, 0, metricsChore.Counter.InlineObjectCount())
		require.EqualValues(t, 2, metricsChore.Counter.RemoteDependent)
		require.EqualValues(t, 2, metricsChore.Counter.ObjectCount)
	})
}
