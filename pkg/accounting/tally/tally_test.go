// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package tally_test

import (
	"crypto/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/storj"
)

func TestDeleteTalliesBefore(t *testing.T) {
	tests := []struct {
		createdAt    time.Time
		eraseBefore  time.Time
		expectedRaws int
	}{
		{
			createdAt:    time.Now(),
			eraseBefore:  time.Now(),
			expectedRaws: 1,
		},
		{
			createdAt:    time.Now(),
			eraseBefore:  time.Now().Add(24 * time.Hour),
			expectedRaws: 0,
		},
	}

	for _, tt := range tests {
		testplanet.Run(t, testplanet.Config{
			SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			id := teststorj.NodeIDFromBytes([]byte{})
			nodeData := make(map[storj.NodeID]float64)
			nodeData[id] = float64(1000)

			err := planet.Satellites[0].DB.StoragenodeAccounting().SaveTallies(ctx, tt.createdAt, tt.createdAt, nodeData)
			require.NoError(t, err)

			err = planet.Satellites[0].DB.StoragenodeAccounting().DeleteTalliesBefore(ctx, tt.eraseBefore)
			require.NoError(t, err)

			raws, err := planet.Satellites[0].DB.StoragenodeAccounting().GetTallies(ctx)
			require.NoError(t, err)
			assert.Len(t, raws, tt.expectedRaws)
		})
	}
}

func TestOnlyInline(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		tallySvc := planet.Satellites[0].Accounting.Tally
		uplink := planet.Uplinks[0]

		// Setup: create data for the uplink to upload
		expectedData := make([]byte, 1*memory.KiB)
		_, err := rand.Read(expectedData)
		require.NoError(t, err)

		// Setup: get the expected size of the data that will be stored in pointer
		uplinkConfig := uplink.GetConfig(planet.Satellites[0])
		expectedTotalBytes, err := encryption.CalcEncryptedSize(int64(len(expectedData)), uplinkConfig.GetEncryptionScheme())
		require.NoError(t, err)

		// Setup: The data in this tally should match the pointer that the uplink.upload created
		expectedTally := accounting.BucketTally{
			Segments:       1,
			InlineSegments: 1,
			Files:          1,
			InlineFiles:    1,
			Bytes:          expectedTotalBytes,
			InlineBytes:    expectedTotalBytes,
			MetadataSize:   111, // brittle, this is hardcoded since its too difficult to get this value progamatically
		}

		// Execute test: upload a file, then calculate at rest data
		expectedBucketName := "testbucket"
		err = uplink.Upload(ctx, planet.Satellites[0], expectedBucketName, "test/path", expectedData)
		assert.NoError(t, err)

		// Run calculate twice to test unique constraint issue
		for i := 0; i < 2; i++ {
			latestTally, actualNodeData, actualBucketData, err := tallySvc.CalculateAtRestData(ctx)
			require.NoError(t, err)
			assert.Len(t, actualNodeData, 0)

			_, err = planet.Satellites[0].DB.ProjectAccounting().SaveTallies(ctx, latestTally, actualBucketData)
			require.NoError(t, err)

			// Confirm the correct bucket storage tally was created
			assert.Equal(t, len(actualBucketData), 1)
			for bucketID, actualTally := range actualBucketData {
				assert.Contains(t, bucketID, expectedBucketName)
				assert.Equal(t, expectedTally, *actualTally)
			}
		}
	})
}

func TestCalculateAtRestData(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		tallySvc := planet.Satellites[0].Accounting.Tally
		uplink := planet.Uplinks[0]

		// Setup: create 50KiB of data for the uplink to upload
		expectedData := make([]byte, 50*memory.KiB)
		_, err := rand.Read(expectedData)
		require.NoError(t, err)

		// Setup: get the expected size of the data that will be stored in pointer
		uplinkConfig := uplink.GetConfig(planet.Satellites[0])
		expectedTotalBytes, err := encryption.CalcEncryptedSize(int64(len(expectedData)), uplinkConfig.GetEncryptionScheme())
		require.NoError(t, err)

		// Setup: The data in this tally should match the pointer that the uplink.upload created
		expectedTally := accounting.BucketTally{
			Segments:       1,
			RemoteSegments: 1,
			Files:          1,
			RemoteFiles:    1,
			Bytes:          expectedTotalBytes,
			RemoteBytes:    expectedTotalBytes,
			MetadataSize:   112, // brittle, this is hardcoded since its too difficult to get this value progamatically
		}

		// Execute test: upload a file, then calculate at rest data
		expectedBucketName := "testbucket"
		err = uplink.Upload(ctx, planet.Satellites[0], expectedBucketName, "test/path", expectedData)
		assert.NoError(t, err)
		_, actualNodeData, actualBucketData, err := tallySvc.CalculateAtRestData(ctx)
		require.NoError(t, err)

		// Confirm the correct number of shares were stored
		uplinkRS := uplinkConfig.GetRedundancyScheme()
		if !correctRedundencyScheme(len(actualNodeData), uplinkRS) {
			t.Fatalf("expected between: %d and %d, actual: %d", uplinkRS.RepairShares, uplinkRS.TotalShares, len(actualNodeData))
		}

		// Confirm the correct number of bytes were stored on each node
		for _, actualTotalBytes := range actualNodeData {
			assert.Equal(t, int64(actualTotalBytes), expectedTotalBytes)
		}

		// Confirm the correct bucket storage tally was created
		assert.Equal(t, len(actualBucketData), 1)
		for bucketID, actualTally := range actualBucketData {
			assert.Contains(t, bucketID, expectedBucketName)
			assert.Equal(t, *actualTally, expectedTally)
		}
	})
}

func correctRedundencyScheme(shareCount int, uplinkRS storj.RedundancyScheme) bool {

	// The shareCount should be a value between RequiredShares and TotalShares where
	// RequiredShares is the min number of shares required to recover a segment and
	// TotalShares is the number of shares to encode
	if int(uplinkRS.RepairShares) <= shareCount && shareCount <= int(uplinkRS.TotalShares) {
		return true
	}

	return false
}
