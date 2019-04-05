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
	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/storj"
)

func TestDeleteRawBefore(t *testing.T) {
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

			err := planet.Satellites[0].DB.Accounting().SaveAtRestRaw(ctx, tt.createdAt, tt.createdAt, nodeData)
			require.NoError(t, err)

			err = planet.Satellites[0].DB.Accounting().DeleteRawBefore(ctx, tt.eraseBefore)
			require.NoError(t, err)

			raws, err := planet.Satellites[0].DB.Accounting().GetRaw(ctx)
			require.NoError(t, err)
			assert.Len(t, raws, tt.expectedRaws)
		})
	}
}

func TestCalculateAtRestData(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		tallySvc := planet.Satellites[0].Accounting.Tally
		uplink := planet.Uplinks[0]
		uplinkConfig := uplink.GetConfig(planet.Satellites[0])
		blockSize := uplinkConfig.Enc.BlockSize.Int()
		uplinkRS := uplinkConfig.GetRedundancyScheme()

		// Setup: create 50KiB of data for the uplink to upload
		expectedData := make([]byte, 50*memory.KiB)
		_, err := rand.Read(expectedData)
		require.NoError(t, err)

		// Setup: when the uploaded data gets encrypted, this much padding is added to the size
		padSize := eestream.MakePadding(int64(len(expectedData)), blockSize)
		expectedTotalBytes := len(expectedData) + len(padSize)

		// Setup: The data in this tally should match the pointer that the uplink.upload created
		expectedTally := accounting.BucketTally{
			Segments:       1,
			RemoteSegments: 1,
			Files:          1,
			RemoteFiles:    1,
			Bytes:          int64(expectedTotalBytes),
			RemoteBytes:    int64(expectedTotalBytes),
			MetadataSize:   112,
		}

		// Execute test: upload a file, then calculate at rest data
		expectedBucketName := "testbucket"
		err = uplink.Upload(ctx, planet.Satellites[0], expectedBucketName, "test/path", expectedData)
		assert.NoError(t, err)
		_, actualNodeData, actualBucketData, err := tallySvc.CalculateAtRestData(ctx)
		require.NoError(t, err)

		// Confirm the correct number of shares were stored
		if !correctRedundencyScheme(len(actualNodeData), uplinkRS) {
			t.Fatalf("expected between: %d and %d, actual: %d", uplinkRS.RepairShares, uplinkRS.TotalShares, len(actualNodeData))
		}

		// Confirm the correct number of bytes were stored on each node
		for _, actualTotalBytes := range actualNodeData {
			assert.Equal(t, int(actualTotalBytes), expectedTotalBytes)
		}

		// Confirm the correct bucket storage tally was created
		assert.Equal(t, len(actualBucketData), 1)
		for bucketID, actualTally := range actualBucketData {
			assert.Contains(t, bucketID, expectedBucketName)
			assert.Equal(t, *actualTally, expectedTally)
		}
	})
}

func correctRedundencyScheme(count int, uplinkRS storj.RedundancyScheme) bool {

	// RequiredShares are the min number of shares required to recover a segment
	// TotalShares is the number of shares to encode
	if int(uplinkRS.RequiredShares) <= count && count <= int(uplinkRS.TotalShares) {
		return true
	}

	return false
}
