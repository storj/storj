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

	"storj.io/common/encryption"
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
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Tally.UseRangedLoop = true
				config.RangedLoop.Parallelism = 4
				config.RangedLoop.BatchSize = 4
			},
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

		// TODO uplink currently hardcode block size so we need to use the same value in test
		encryptionParameters := storj.EncryptionParameters{
			CipherSuite: storj.EncAESGCM,
			BlockSize:   29 * 256 * memory.B.Int32(),
		}
		expectedTotalBytes, err := encryption.CalcEncryptedSize(int64(len(expectedData)), encryptionParameters)
		require.NoError(t, err)

		// Execute test: upload a file, then calculate at rest data
		expectedBucketName := "testbucket"
		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], expectedBucketName, "test/path", expectedData)
		require.NoError(t, err)

		secondNow := firstNow.Add(2 * time.Hour)
		obs.SetNow(func() time.Time {
			return secondNow
		})

		_, err = planet.Satellites[0].RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)

		// Confirm the correct number of shares were stored
		rs := satelliteRS(t, planet.Satellites[0])
		if !correctRedundencyScheme(len(obs.Node), rs) {
			t.Fatalf("expected between: %d and %d, actual: %d", rs.RepairShares, rs.TotalShares, len(obs.Node))
		}

		// Confirm the correct number of bytes were stored on each node
		for _, actualTotalBytes := range obs.Node {
			require.EqualValues(t, expectedTotalBytes, actualTotalBytes)
		}

		// Confirm that tallies where saved to DB
		tallies, err := planet.Satellites[0].DB.StoragenodeAccounting().GetTalliesSince(ctx, secondNow.Add(-1*time.Second))
		require.NoError(t, err)
		require.LessOrEqual(t, len(tallies), int(rs.TotalShares))
		require.GreaterOrEqual(t, len(tallies), int(rs.OptimalShares))

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
		require.LessOrEqual(t, len(tallies), int(rs.TotalShares))
		require.GreaterOrEqual(t, len(tallies), int(rs.OptimalShares))

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
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Tally.UseRangedLoop = true
				config.RangedLoop.Parallelism = 4
				config.RangedLoop.BatchSize = 4
			},
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

		// TODO uplink currently hardcode block size so we need to use the same value in test
		encryptionParameters := storj.EncryptionParameters{
			CipherSuite: storj.EncAESGCM,
			BlockSize:   29 * 256 * memory.B.Int32(),
		}
		expectedBytesPerPiece, err := encryption.CalcEncryptedSize(int64(len(expectedData)), encryptionParameters)
		require.NoError(t, err)

		// Execute test: upload a file, then calculate at rest data
		expectedBucketName := "testbucket"
		for i := 0; i < numObjects; i++ {
			err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], expectedBucketName, fmt.Sprintf("test/path%d", i), expectedData)
			require.NoError(t, err)
		}

		rangedLoop := planet.Satellites[0].RangedLoop
		obs := rangedLoop.Accounting.NodeTallyObserver
		obs.SetNow(func() time.Time {
			return now
		})

		_, err = planet.Satellites[0].RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)

		rs := satelliteRS(t, planet.Satellites[0])
		minExpectedBytes := numObjects * int(expectedBytesPerPiece) * int(rs.OptimalShares)
		maxExpectedBytes := numObjects * int(expectedBytesPerPiece) * int(rs.TotalShares)

		// Confirm the correct number of bytes were stored on each node
		totalBytes := 0
		for _, actualTotalBytes := range obs.Node {
			totalBytes += int(actualTotalBytes)
		}
		require.LessOrEqual(t, totalBytes, maxExpectedBytes)
		require.GreaterOrEqual(t, totalBytes, minExpectedBytes)

		// Confirm that tallies where saved to DB
		tallies, err := planet.Satellites[0].DB.StoragenodeAccounting().GetTalliesSince(ctx, now.Add(-1*time.Second))
		require.NoError(t, err)

		totalByteHours := 0
		for _, tally := range tallies {
			totalByteHours += int(tally.DataTotal)
		}

		require.LessOrEqual(t, totalByteHours, maxExpectedBytes*timespanHours)
		require.GreaterOrEqual(t, totalByteHours, minExpectedBytes*timespanHours)
	})
}

func TestExpiredObjectsNotCountedInNodeTally(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Tally.UseRangedLoop = true
				config.RangedLoop.Parallelism = 1
				config.RangedLoop.BatchSize = 4
			},
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
		for i := 0; i < numObjects; i++ {
			err = planet.Uplinks[0].UploadWithExpiration(
				ctx, planet.Satellites[0], expectedBucketName, fmt.Sprint("test/pathA", i), expectedData, now.Add(-1*time.Second),
			)
			require.NoError(t, err)
			err = planet.Uplinks[0].UploadWithExpiration(
				ctx, planet.Satellites[0], expectedBucketName, fmt.Sprint("test/pathB", i), expectedData, now.Add(1*time.Second),
			)
			require.NoError(t, err)
		}

		_, err = planet.Satellites[0].RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)

		rs := satelliteRS(t, planet.Satellites[0])
		// TODO uplink currently hardcode block size so we need to use the same value in test
		encryptionParameters := storj.EncryptionParameters{
			CipherSuite: storj.EncAESGCM,
			BlockSize:   29 * 256 * memory.B.Int32(),
		}
		expectedBytesPerPiece, err := encryption.CalcEncryptedSize(int64(len(expectedData)), encryptionParameters)
		require.NoError(t, err)
		minExpectedBytes := numObjects * int(expectedBytesPerPiece) * int(rs.OptimalShares)
		maxExpectedBytes := numObjects * int(expectedBytesPerPiece) * int(rs.TotalShares)

		// Confirm the correct number of bytes were stored on each node
		totalBytes := 0
		for _, actualTotalBytes := range obs.Node {
			totalBytes += int(actualTotalBytes)
		}
		require.LessOrEqual(t, totalBytes, maxExpectedBytes)
		require.GreaterOrEqual(t, totalBytes, minExpectedBytes)
	})
}

func satelliteRS(t *testing.T, satellite *testplanet.Satellite) storj.RedundancyScheme {
	rs := satellite.Config.Metainfo.RS

	return storj.RedundancyScheme{
		RequiredShares: int16(rs.Min),
		RepairShares:   int16(rs.Repair),
		OptimalShares:  int16(rs.Success),
		TotalShares:    int16(rs.Total),
		ShareSize:      rs.ErasureShareSize.Int32(),
	}
}

func correctRedundencyScheme(shareCount int, uplinkRS storj.RedundancyScheme) bool {
	// The shareCount should be a value between RequiredShares and TotalShares where
	// RequiredShares is the min number of shares required to recover a segment and
	// TotalShares is the number of shares to encode
	return int(uplinkRS.RepairShares) <= shareCount && shareCount <= int(uplinkRS.TotalShares)
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
