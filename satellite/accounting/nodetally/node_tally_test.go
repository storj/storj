// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package nodetally_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/encryption"
	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/accounting/nodetally"
	"storj.io/storj/satellite/metabase/segmentloop"
)

func TestCalculateNodeAtRestData(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		tallySvc := planet.Satellites[0].Accounting.NodeTally
		tallySvc.Loop.Pause()
		uplink := planet.Uplinks[0]

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
		err = uplink.Upload(ctx, planet.Satellites[0], expectedBucketName, "test/path", expectedData)
		require.NoError(t, err)

		obs := nodetally.NewObserver(planet.Satellites[0].Log.Named("observer"), time.Now())
		err = planet.Satellites[0].Metabase.SegmentLoop.Join(ctx, obs)
		require.NoError(t, err)

		// Confirm the correct number of shares were stored
		rs := satelliteRS(t, planet.Satellites[0])
		if !correctRedundencyScheme(len(obs.Node), rs) {
			t.Fatalf("expected between: %d and %d, actual: %d", rs.RepairShares, rs.TotalShares, len(obs.Node))
		}

		// Confirm the correct number of bytes were stored on each node
		for _, actualTotalBytes := range obs.Node {
			assert.Equal(t, expectedTotalBytes, int64(actualTotalBytes))
		}
	})
}

func correctRedundencyScheme(shareCount int, uplinkRS storj.RedundancyScheme) bool {
	// The shareCount should be a value between RequiredShares and TotalShares where
	// RequiredShares is the min number of shares required to recover a segment and
	// TotalShares is the number of shares to encode
	return int(uplinkRS.RepairShares) <= shareCount && shareCount <= int(uplinkRS.TotalShares)
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

func BenchmarkRemoteSegment(b *testing.B) {
	testplanet.Bench(b, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(b *testing.B, ctx *testcontext.Context, planet *testplanet.Planet) {

		for i := 0; i < 10; i++ {
			err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "object"+strconv.Itoa(i), testrand.Bytes(10*memory.KiB))
			require.NoError(b, err)
		}

		observer := nodetally.NewObserver(zaptest.NewLogger(b), time.Now())

		segments, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
		require.NoError(b, err)

		loopSegments := []*segmentloop.Segment{}

		for _, segment := range segments {
			loopSegments = append(loopSegments, &segmentloop.Segment{
				StreamID:   segment.StreamID,
				Position:   segment.Position,
				CreatedAt:  segment.CreatedAt,
				ExpiresAt:  segment.ExpiresAt,
				Redundancy: segment.Redundancy,
				Pieces:     segment.Pieces,
			})
		}

		b.Run("multiple segments", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				for _, loopSegment := range loopSegments {
					err := observer.RemoteSegment(ctx, loopSegment)
					if err != nil {
						b.FailNow()
					}
				}
			}
		})
	})

}
