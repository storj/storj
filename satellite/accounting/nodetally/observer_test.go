// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package nodetally_test

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting/nodetally"
	"storj.io/storj/satellite/metabase/rangedloop"
)

func TestSingleObjectNodeTallyRangedLoop(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				testplanet.ReconfigureRS(2, 2, 2, 2),
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.RangedLoop.Parallelism = 4
					config.RangedLoop.BatchSize = 4

					// disable repairer to not interfere with the test
					// as used RS will trigger repair for segments
					config.Repairer.Interval = -1
				},
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		timespanHours := 2

		firstNow := time.Date(2020, 8, 8, 8, 8, 8, 0, time.UTC)
		obs := planet.Satellites[0].RangedLoop.Accounting.NodeTallyObserver
		obs.SetNow(func() time.Time {
			return firstNow
		})

		// first run to zero out the database
		_, err := planet.Satellites[0].RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)

		// Setup: create 50KiB of data for the uplink to upload
		expectedData := testrand.Bytes(50 * memory.KiB)

		// Execute test: upload a file, then calculate at rest data
		expectedBucketName := "testbucket"
		require.NoError(t, planet.Uplinks[0].Upload(ctx, planet.Satellites[0], expectedBucketName, "test/path", expectedData))

		segments, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 1)

		secondNow := firstNow.Add(2 * time.Hour)
		obs.SetNow(func() time.Time {
			return secondNow
		})

		_, err = planet.Satellites[0].RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)

		// Confirm the correct number of bytes were stored on each node
		for _, actualTotalBytes := range obs.Node {
			require.EqualValues(t, segments[0].PieceSize(), actualTotalBytes)
		}

		// Confirm that tallies where saved to DB
		tallies, err := planet.Satellites[0].DB.StoragenodeAccounting().GetTalliesSince(ctx, secondNow.Add(-1*time.Second))
		require.NoError(t, err)
		require.Len(t, tallies, len(obs.Node))

		aliasMap, err := planet.Satellites[0].Metabase.DB.LatestNodesAliasMap(ctx)
		require.NoError(t, err)
		for _, tally := range tallies {
			nodeAlias, ok := aliasMap.Alias(tally.NodeID)
			require.Truef(t, ok, "could not get node alias for node %s", tally.NodeID)
			require.Equal(t, obs.Node[nodeAlias]*float64(timespanHours), tally.DataTotal)
		}

		thirdNow := secondNow.Add(2 * time.Hour)
		obs.SetNow(func() time.Time {
			return thirdNow
		})

		_, err = planet.Satellites[0].RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)

		tallies, err = planet.Satellites[0].DB.StoragenodeAccounting().GetTalliesSince(ctx, thirdNow.Add(-1*time.Second))
		require.NoError(t, err)
		require.Len(t, tallies, len(obs.Node))

		aliasMap, err = planet.Satellites[0].Metabase.DB.LatestNodesAliasMap(ctx)
		require.NoError(t, err)
		for _, tally := range tallies {
			nodeAlias, ok := aliasMap.Alias(tally.NodeID)
			require.Truef(t, ok, "could not get node alias for node %s", tally.NodeID)
			require.Equal(t, obs.Node[nodeAlias]*float64(timespanHours), tally.DataTotal)
		}
	})
}

func TestManyObjectsNodeTallyRangedLoop(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				testplanet.ReconfigureRS(2, 2, 2, 2),
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.RangedLoop.Parallelism = 4
					config.RangedLoop.BatchSize = 4

					// disable repairer to not interfere with the test
					// as used RS will trigger repair for segments
					config.Repairer.Interval = -1
				},
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		const timespanHours = 2
		numObjects := 10

		now := time.Date(2020, 8, 8, 8, 8, 8, 0, time.UTC)
		lastTally := now.Add(-timespanHours * time.Hour)
		// Set previous accounting run timestamp
		err := planet.Satellites[0].DB.StoragenodeAccounting().DeleteTalliesBefore(ctx, now.Add(1*time.Second), 5000)
		require.NoError(t, err)
		err = planet.Satellites[0].DB.StoragenodeAccounting().SaveTallies(ctx, lastTally,
			[]storj.NodeID{planet.StorageNodes[0].ID(), planet.StorageNodes[1].ID(), planet.StorageNodes[2].ID(), planet.StorageNodes[3].ID()},
			[]float64{0, 0, 0, 0},
		)
		require.NoError(t, err)

		// Setup: create 50KiB of data for the uplink to upload
		expectedData := testrand.Bytes(50 * memory.KiB)

		// Execute test: upload a file, then calculate at rest data
		expectedBucketName := "testbucket"
		for i := range numObjects {
			require.NoError(t, planet.Uplinks[0].Upload(ctx, planet.Satellites[0], expectedBucketName, fmt.Sprintf("test/path%d", i), expectedData))
		}

		segments, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, segments)

		rangedLoop := planet.Satellites[0].RangedLoop
		obs := rangedLoop.Accounting.NodeTallyObserver
		obs.SetNow(func() time.Time {
			return now
		})

		_, err = planet.Satellites[0].RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)

		// all objects have the same parameters so we can use one to calculate expected bytes
		expectedBytes := numObjects * int(segments[0].PieceSize()) * int(segments[0].Redundancy.OptimalShares)

		// Confirm the correct number of bytes were stored on each node
		totalBytes := 0
		for _, actualTotalBytes := range obs.Node {
			totalBytes += int(actualTotalBytes)
		}

		require.Equal(t, expectedBytes, totalBytes)

		// Confirm that tallies where saved to DB
		tallies, err := planet.Satellites[0].DB.StoragenodeAccounting().GetTalliesSince(ctx, now.Add(-1*time.Second))
		require.NoError(t, err)

		totalByteHours := 0
		for _, tally := range tallies {
			totalByteHours += int(tally.DataTotal)
		}

		require.Equal(t, expectedBytes*timespanHours, totalByteHours)
	})
}

func TestExpiredObjectsNotCountedInNodeTally(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				testplanet.ReconfigureRS(2, 2, 2, 2),
				func(log *zap.Logger, index int, config *satellite.Config) {
					// disable ranged loop interval execution
					// to execute it manually to have a predictable test
					config.RangedLoop.Interval = -1
					config.RangedLoop.Parallelism = 4
					config.RangedLoop.BatchSize = 4

					// disable repairer to not interfere with the test
					// as used RS will trigger repair for segments
					config.Repairer.Interval = -1
				},
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		const timespanHours = 2
		numObjects := 10

		now := time.Date(2030, 8, 8, 8, 8, 8, 0, time.UTC)
		obs := planet.Satellites[0].RangedLoop.Accounting.NodeTallyObserver
		obs.SetNow(func() time.Time {
			return now
		})

		lastTally := now.Add(-timespanHours * time.Hour)
		err := planet.Satellites[0].DB.StoragenodeAccounting().SaveTallies(ctx, lastTally,
			[]storj.NodeID{planet.StorageNodes[0].ID(), planet.StorageNodes[1].ID(), planet.StorageNodes[2].ID(), planet.StorageNodes[3].ID()},
			[]float64{0, 0, 0, 0},
		)
		require.NoError(t, err)

		// Setup: create 50KiB of data for the uplink to upload
		expectedData := testrand.Bytes(50 * memory.KiB)

		// Upload expired objects and the same number of soon-to-expire objects
		expectedBucketName := "testbucket"
		for i := range numObjects {
			require.NoError(t, planet.Uplinks[0].UploadWithExpiration(
				ctx, planet.Satellites[0], expectedBucketName, fmt.Sprint("test/pathA", i), expectedData, now.Add(-1*time.Minute),
			))

			require.NoError(t, planet.Uplinks[0].UploadWithExpiration(
				ctx, planet.Satellites[0], expectedBucketName, fmt.Sprint("test/pathB", i), expectedData, now.Add(1*time.Minute),
			))
		}

		_, err = planet.Satellites[0].RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)

		segments, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, segments)

		// all objects have the same parameters so we can use one to calculate expected bytes
		expectedBytes := numObjects * int(segments[0].PieceSize()) * int(segments[0].Redundancy.OptimalShares)

		// Confirm the correct number of bytes were stored on each node
		totalBytes := 0
		for _, actualTotalBytes := range obs.Node {
			totalBytes += int(actualTotalBytes)
		}
		require.Equal(t, expectedBytes, totalBytes)
	})
}

func BenchmarkProcess(b *testing.B) {
	testplanet.Bench(b, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(b *testing.B, ctx *testcontext.Context, planet *testplanet.Planet) {

		for i := 0; i < 10; i++ {
			err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "object"+strconv.Itoa(i), testrand.Bytes(10*memory.KiB))
			require.NoError(b, err)
		}

		observer := nodetally.NewObserver(zaptest.NewLogger(b), nil, planet.Satellites[0].Metabase.DB, planet.Satellites[0].Config.NodeTally)

		segments, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
		require.NoError(b, err)

		loopSegments := []rangedloop.Segment{}

		for _, segment := range segments {
			loopSegments = append(loopSegments, rangedloop.Segment{
				StreamID:   segment.StreamID,
				Position:   segment.Position,
				CreatedAt:  segment.CreatedAt,
				ExpiresAt:  segment.ExpiresAt,
				Redundancy: segment.Redundancy,
				Pieces:     segment.Pieces,
			})
		}

		fork, err := observer.Fork(ctx)
		require.NoError(b, err)

		b.Run("multiple segments", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = fork.Process(ctx, loopSegments)
			}
		})
	})
}
