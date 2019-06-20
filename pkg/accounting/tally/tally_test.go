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
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/storj"
)

var (
	testPath1 = paths.NewUnencrypted("test/path1")
	testPath2 = paths.NewUnencrypted("test/path2")
)

func TestDeleteTalliesBefore(t *testing.T) {
	tests := []struct {
		eraseBefore  time.Time
		expectedRaws int
	}{
		{
			eraseBefore:  time.Now(),
			expectedRaws: 1,
		},
		{
			eraseBefore:  time.Now().Add(24 * time.Hour),
			expectedRaws: 0,
		},
	}

	for _, tt := range tests {
		test := tt
		testplanet.Run(t, testplanet.Config{
			SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			id := teststorj.NodeIDFromBytes([]byte{})
			nodeData := make(map[storj.NodeID]float64)
			nodeData[id] = float64(1000)

			err := planet.Satellites[0].DB.StoragenodeAccounting().SaveTallies(ctx, time.Now(), nodeData)
			require.NoError(t, err)

			err = planet.Satellites[0].DB.StoragenodeAccounting().DeleteTalliesBefore(ctx, test.eraseBefore)
			require.NoError(t, err)

			raws, err := planet.Satellites[0].DB.StoragenodeAccounting().GetTallies(ctx)
			require.NoError(t, err)
			assert.Len(t, raws, test.expectedRaws)
		})
	}
}

func TestOnlyInline(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		tallySvc := planet.Satellites[0].Accounting.Tally
		uplink := planet.Uplinks[0]

		ps, err1 := planet.Satellites[0].DB.Console().Projects().GetAll(ctx)
		if err1 != nil {
			assert.NoError(t, err1)
		}
		project := ps[0]
		projectID := []byte(project.ID.String())

		// Setup: create data for the uplink to upload
		expectedData := make([]byte, 1*memory.KiB)
		_, err := rand.Read(expectedData)
		require.NoError(t, err)

		// Setup: get the expected size of the data that will be stored in pointer
		// Since the data is small enough to be stored inline, when it is encrypted, we only
		// add 16 bytes of encryption authentication overhead.  No encryption block
		// padding will be added since we are not chunking data that we store inline.
		const encryptionAuthOverhead = 16 // bytes
		expectedTotalBytes := len(expectedData) + encryptionAuthOverhead

		// Setup: The data in this tally should match the pointer that the uplink.upload created
		expectedBucketName := "testbucket"
		expectedTally := accounting.BucketTally{
			BucketName:     []byte(expectedBucketName),
			ProjectID:      projectID,
			Segments:       1,
			InlineSegments: 1,
			Files:          1,
			InlineFiles:    1,
			Bytes:          int64(expectedTotalBytes),
			InlineBytes:    int64(expectedTotalBytes),
			MetadataSize:   111, // brittle, this is hardcoded since its too difficult to get this value progamatically
		}

		// Execute test: upload a file, then calculate at rest data
		err = uplink.Upload(ctx, planet.Satellites[0], expectedBucketName, testPath1, expectedData)
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

func TestCalculateNodeAtRestData(t *testing.T) {
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

		// Execute test: upload a file, then calculate at rest data
		expectedBucketName := "testbucket"
		err = uplink.Upload(ctx, planet.Satellites[0], expectedBucketName, testPath1, expectedData)

		assert.NoError(t, err)
		_, actualNodeData, _, err := tallySvc.CalculateAtRestData(ctx)
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
	})
}

func TestCalculateBucketAtRestData(t *testing.T) {
	t.Skip("TODO: this test is flaky")
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		tallySvc := planet.Satellites[0].Accounting.Tally
		uplink := planet.Uplinks[0]

		ps, err1 := planet.Satellites[0].DB.Console().Projects().GetAll(ctx)
		if err1 != nil {
			assert.NoError(t, err1)
		}
		project := ps[0]
		projectID := []byte(project.ID.String())

		// Setup: create 50KiB of data for the uplink to upload
		expectedData := make([]byte, 50*memory.KiB)
		_, err := rand.Read(expectedData)
		require.NoError(t, err)
		expectedData2 := make([]byte, 100*memory.KiB)
		_, err = rand.Read(expectedData)
		require.NoError(t, err)

		// Setup: get the expected size of the data that will be stored in pointer
		uplinkConfig := uplink.GetConfig(planet.Satellites[0])
		expectedTotalBytes, err := encryption.CalcEncryptedSize(int64(len(expectedData)), uplinkConfig.GetEncryptionScheme())
		require.NoError(t, err)
		expectedTotalBytes2, err := encryption.CalcEncryptedSize(int64(len(expectedData2)), uplinkConfig.GetEncryptionScheme())
		require.NoError(t, err)

		// Setup: The data in this tally should match the pointer that the uplink.upload created
		expectedBucketName1 := "testbucket1"
		expectedTally1 := accounting.BucketTally{
			BucketName:     []byte(expectedBucketName1),
			ProjectID:      projectID,
			Segments:       1,
			RemoteSegments: 1,
			Files:          1,
			RemoteFiles:    1,
			Bytes:          expectedTotalBytes,
			RemoteBytes:    expectedTotalBytes,
			MetadataSize:   112, // brittle, this is hardcoded since its too difficult to get this value progamatically
		}

		expectedBucketName2 := "testbucket2"
		expectedTally2 := accounting.BucketTally{
			BucketName:     []byte(expectedBucketName2),
			ProjectID:      projectID,
			Segments:       1,
			RemoteSegments: 1,
			Files:          1,
			RemoteFiles:    1,
			Bytes:          expectedTotalBytes2,
			RemoteBytes:    expectedTotalBytes2,
			MetadataSize:   112, // brittle, this is hardcoded since its too difficult to get this value progamatically
		}

		expectedBucketName3 := "testbucket3"
		expectedTally3 := accounting.BucketTally{
			BucketName:     []byte(expectedBucketName3),
			ProjectID:      projectID,
			Segments:       0,
			RemoteSegments: 0,
			Files:          0,
			RemoteFiles:    0,
			Bytes:          0,
			RemoteBytes:    0,
			MetadataSize:   0,
		}

		expectedBucketName4 := "testbucket4"
		expectedTally4 := accounting.BucketTally{
			BucketName:     []byte(expectedBucketName4),
			ProjectID:      projectID,
			Segments:       2,
			RemoteSegments: 2,
			Files:          2,
			RemoteFiles:    2,
			Bytes:          expectedTotalBytes + expectedTotalBytes2,
			RemoteBytes:    expectedTotalBytes + expectedTotalBytes2,
			MetadataSize:   224,
		}

		// Execute test: upload a file, then calculate at rest data

		err = uplink.Upload(ctx, planet.Satellites[0], expectedBucketName2, testPath2, expectedData2)
		assert.NoError(t, err)
		err = uplink.Upload(ctx, planet.Satellites[0], expectedBucketName1, testPath1, expectedData)
		assert.NoError(t, err)
		err = uplink.Upload(ctx, planet.Satellites[0], expectedBucketName4, testPath2, expectedData2)
		assert.NoError(t, err)
		err = uplink.Upload(ctx, planet.Satellites[0], expectedBucketName4, testPath1, expectedData)
		assert.NoError(t, err)

		_, _, actualBucketData, err := tallySvc.CalculateAtRestData(ctx)
		require.NoError(t, err)

		// Confirm the correct bucket storage tally was created
		assert.Equal(t, len(actualBucketData), 3)
		for bucketID, actualTally := range actualBucketData {
			var bucketName = string(actualTally.BucketName)
			assert.True(t, bucketName == expectedBucketName1 || bucketName == expectedBucketName2 || bucketName == expectedBucketName3 || bucketName == expectedBucketName4, "Test bucket names do not exist in results")
			switch bucketName {
			case expectedBucketName1:
				assert.Contains(t, bucketID, expectedBucketName1)
				assert.Equal(t, expectedTally1, *actualTally)
			case expectedBucketName2:
				assert.Contains(t, bucketID, expectedBucketName2)
				assert.Equal(t, expectedTally2, *actualTally)
			case expectedBucketName3:
				assert.Contains(t, bucketID, expectedBucketName3)
				assert.Equal(t, expectedTally3, *actualTally)
			case expectedBucketName4:
				assert.Contains(t, bucketID, expectedBucketName4)
				assert.Equal(t, expectedTally4, *actualTally)
			}
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
