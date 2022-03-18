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
	"storj.io/storj/satellite/metrics"
)

func TestCounterInlineAndRemote(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		ul := planet.Uplinks[0]

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

		counter := metrics.NewCounter()
		err := satellite.Metabase.SegmentLoop.Join(ctx, counter)
		require.NoError(t, err)

		require.EqualValues(t, 2, counter.InlineObjects)
		require.EqualValues(t, 2, counter.RemoteObjects)

		require.EqualValues(t, 2, counter.TotalInlineSegments)
		require.EqualValues(t, 2, counter.TotalRemoteSegments)
		// 2 inline segments * (1024 + encryption overhead)
		require.EqualValues(t, 2080, counter.TotalInlineBytes)
		// 2 remote segments * (8192 + encryption overhead)
		require.EqualValues(t, 29696, counter.TotalRemoteBytes)
	})
}

func TestCounterInlineOnly(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		ul := planet.Uplinks[0]

		// upload 2 inline files
		for i := 0; i < 2; i++ {
			testData := testrand.Bytes(memory.KiB)
			path := "/some/inline/path/" + strconv.Itoa(i)
			err := ul.Upload(ctx, satellite, "bucket", path, testData)
			require.NoError(t, err)
		}

		counter := metrics.NewCounter()
		err := satellite.Metabase.SegmentLoop.Join(ctx, counter)
		require.NoError(t, err)

		require.EqualValues(t, 2, counter.InlineObjects)
		require.EqualValues(t, 0, counter.RemoteObjects)
	})
}

func TestCounterRemoteOnly(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.MaxSegmentSize(150 * memory.KiB),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		ul := planet.Uplinks[0]

		// upload 2 remote files with multiple segments
		for i := 0; i < 2; i++ {
			testData := testrand.Bytes(300 * memory.KiB)
			path := "/some/remote/path/" + strconv.Itoa(i)
			err := ul.Upload(ctx, satellite, "testbucket", path, testData)
			require.NoError(t, err)
		}

		counter := metrics.NewCounter()
		err := satellite.Metabase.SegmentLoop.Join(ctx, counter)
		require.NoError(t, err)

		require.EqualValues(t, 0, counter.InlineObjects)
		require.EqualValues(t, 2, counter.RemoteObjects)
	})
}
