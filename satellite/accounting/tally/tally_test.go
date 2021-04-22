// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package tally_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/encryption"
	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/private/teststorj"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/accounting/tally"
	"storj.io/storj/satellite/metabase"
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
		planet.Satellites[0].Accounting.Tally.Loop.Pause()
		uplink := planet.Uplinks[0]

		// Setup: create data for the uplink to upload
		expectedData := testrand.Bytes(1 * memory.KiB)

		// Setup: get the expected size of the data that will be stored in pointer
		// Since the data is small enough to be stored inline, when it is encrypted, we only
		// add 16 bytes of encryption authentication overhead.  No encryption block
		// padding will be added since we are not chunking data that we store inline.
		const encryptionAuthOverhead = 16 // bytes
		expectedTotalBytes := len(expectedData) + encryptionAuthOverhead

		// Setup: The data in this tally should match the pointer that the uplink.upload created
		expectedBucketName := "testbucket"
		expectedTally := &accounting.BucketTally{
			BucketLocation: metabase.BucketLocation{
				ProjectID:  uplink.Projects[0].ID,
				BucketName: expectedBucketName,
			},
			ObjectCount:    1,
			InlineSegments: 1,
			InlineBytes:    int64(expectedTotalBytes),
			MetadataSize:   0,
		}

		// Execute test: upload a file, then calculate at rest data
		err := uplink.Upload(ctx, planet.Satellites[0], expectedBucketName, "test/path", expectedData)
		assert.NoError(t, err)

		// run multiple times to ensure we add tallies
		for i := 0; i < 2; i++ {
			obs := tally.NewObserver(planet.Satellites[0].Log.Named("observer"), time.Now())
			err := planet.Satellites[0].Metainfo.Loop.Join(ctx, obs)
			require.NoError(t, err)

			now := time.Now().Add(time.Duration(i) * time.Second)
			err = planet.Satellites[0].DB.ProjectAccounting().SaveTallies(ctx, now, obs.Bucket)
			require.NoError(t, err)

			assert.Equal(t, 1, len(obs.Bucket))
			for _, actualTally := range obs.Bucket {
				// checking the exact metadata size is brittle, instead, verify that it's not zero
				assert.NotZero(t, actualTally.MetadataSize)
				actualTally.MetadataSize = expectedTally.MetadataSize
				assert.Equal(t, expectedTally, actualTally)
			}
		}
	})
}

func TestCalculateNodeAtRestData(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		tallySvc := planet.Satellites[0].Accounting.Tally
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

		obs := tally.NewObserver(planet.Satellites[0].Log.Named("observer"), time.Now())
		err = planet.Satellites[0].Metainfo.Loop.Join(ctx, obs)
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

func TestCalculateBucketAtRestData(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 2,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				testplanet.ReconfigureRS(2, 3, 4, 4),
				testplanet.MaxSegmentSize(20*memory.KiB),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		err := planet.Uplinks[0].Upload(ctx, satellite, "alpha", "inline", make([]byte, 10*memory.KiB))
		require.NoError(t, err)

		err = planet.Uplinks[0].Upload(ctx, satellite, "alpha", "remote", make([]byte, 30*memory.KiB))
		require.NoError(t, err)

		err = planet.Uplinks[0].Upload(ctx, satellite, "beta", "remote", make([]byte, 30*memory.KiB))
		require.NoError(t, err)

		err = planet.Uplinks[1].Upload(ctx, satellite, "alpha", "remote", make([]byte, 30*memory.KiB))
		require.NoError(t, err)

		objects, err := satellite.Metainfo.Metabase.TestingAllObjects(ctx)
		require.NoError(t, err)

		segments, err := satellite.Metainfo.Metabase.TestingAllSegments(ctx)
		require.NoError(t, err)

		expectedTotal := map[metabase.BucketLocation]*accounting.BucketTally{}
		ensure := func(loc metabase.BucketLocation) *accounting.BucketTally {
			if t, ok := expectedTotal[loc]; ok {
				return t
			}
			t := &accounting.BucketTally{BucketLocation: loc}
			expectedTotal[loc] = t
			return t
		}

		streamLocation := map[uuid.UUID]metabase.BucketLocation{}
		for _, object := range objects {
			loc := object.Location().Bucket()
			streamLocation[object.StreamID] = loc
			t := ensure(loc)
			t.ObjectCount++
			t.MetadataSize += int64(len(object.EncryptedMetadata))
		}
		for _, segment := range segments {
			loc := streamLocation[segment.StreamID]
			t := ensure(loc)
			if len(segment.Pieces) > 0 {
				t.RemoteSegments++
				t.RemoteBytes += int64(segment.EncryptedSize)
			} else {
				t.InlineSegments++
				t.InlineBytes += int64(segment.EncryptedSize)
			}
		}
		require.Len(t, expectedTotal, 3)

		obs := tally.NewObserver(satellite.Log.Named("observer"), time.Now())
		err = satellite.Metainfo.Loop.Join(ctx, obs)
		require.NoError(t, err)
		require.Equal(t, expectedTotal, obs.Bucket)
	})
}

func TestTallyIgnoresExpiredPointers(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		now := time.Now()
		err := planet.Uplinks[0].UploadWithExpiration(ctx, planet.Satellites[0], "bucket", "path", []byte{1}, now.Add(12*time.Hour))
		require.NoError(t, err)

		obs := tally.NewObserver(satellite.Log.Named("observer"), now.Add(24*time.Hour))
		err = satellite.Metainfo.Loop.Join(ctx, obs)
		require.NoError(t, err)

		// there should be no observed buckets because all of the pointers are expired
		require.Equal(t, obs.Bucket, map[metabase.BucketLocation]*accounting.BucketTally{})
	})
}

func TestTallyLiveAccounting(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		tally := planet.Satellites[0].Accounting.Tally
		projectID := planet.Uplinks[0].Projects[0].ID
		tally.Loop.Pause()

		expectedData := testrand.Bytes(5 * memory.MB)

		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path", expectedData)
		require.NoError(t, err)

		segments, err := planet.Satellites[0].Metainfo.Metabase.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 1)

		segmentSize := int64(segments[0].EncryptedSize)

		tally.Loop.TriggerWait()

		expectedSize := segmentSize

		total, err := planet.Satellites[0].Accounting.ProjectUsage.GetProjectStorageTotals(ctx, projectID)
		require.NoError(t, err)
		require.Equal(t, expectedSize, total)

		for i := 0; i < 5; i++ {
			err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", fmt.Sprintf("test/path/%d", i), expectedData)
			require.NoError(t, err)

			tally.Loop.TriggerWait()

			expectedSize += segmentSize

			total, err := planet.Satellites[0].Accounting.ProjectUsage.GetProjectStorageTotals(ctx, projectID)
			require.NoError(t, err)
			require.Equal(t, expectedSize, total)
		}
	})
}

func TestTallyEmptyProjectUpdatesLiveAccounting(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 2,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		planet.Satellites[0].Accounting.Tally.Loop.Pause()

		project1 := planet.Uplinks[1].Projects[0].ID

		data := testrand.Bytes(1 * memory.MB)

		// we need an extra bucket with data for this test. If no buckets are found at all,
		// the update block is skipped in tally
		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "bucket", "test", data)
		require.NoError(t, err)

		err = planet.Uplinks[1].Upload(ctx, planet.Satellites[0], "bucket", "test", data)
		require.NoError(t, err)

		planet.Satellites[0].Accounting.Tally.Loop.TriggerWait()
		planet.Satellites[0].Accounting.Tally.Loop.Pause()

		total, err := planet.Satellites[0].Accounting.ProjectUsage.GetProjectStorageTotals(ctx, project1)
		require.NoError(t, err)
		require.True(t, total >= int64(len(data)))

		err = planet.Uplinks[1].DeleteObject(ctx, planet.Satellites[0], "bucket", "test")
		require.NoError(t, err)

		planet.Satellites[0].Accounting.Tally.Loop.TriggerWait()

		p1Total, err := planet.Satellites[0].Accounting.ProjectUsage.GetProjectStorageTotals(ctx, project1)
		require.NoError(t, err)
		require.Zero(t, p1Total)
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
