// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package tally_test

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
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
			expectedRaws: 3,
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
			nodeIDs := []storj.NodeID{{1}, {2}, {3}}
			nodeBWAmounts := []float64{1000, 1000, 1000}

			err := planet.Satellites[0].DB.StoragenodeAccounting().SaveTallies(ctx, time.Now(), nodeIDs, nodeBWAmounts)
			require.NoError(t, err)

			err = planet.Satellites[0].DB.StoragenodeAccounting().DeleteTalliesBefore(ctx, test.eraseBefore, 1)
			require.NoError(t, err)

			raws, err := planet.Satellites[0].DB.StoragenodeAccounting().GetTallies(ctx)
			require.NoError(t, err)
			assert.Len(t, raws, test.expectedRaws)
		})
	}
}

func TestOnlyInline(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		planet.Satellites[0].Accounting.Tally.Loop.Pause()
		up := planet.Uplinks[0]

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
				ProjectID:  up.Projects[0].ID,
				BucketName: expectedBucketName,
			},
			ObjectCount:   1,
			TotalSegments: 1,
			TotalBytes:    int64(expectedTotalBytes),
			MetadataSize:  0,
		}

		// Execute test: upload a file, then calculate at rest data
		err := up.Upload(ctx, planet.Satellites[0], expectedBucketName, "test/path", expectedData)
		assert.NoError(t, err)

		// run multiple times to ensure we add tallies
		for i := 0; i < 2; i++ {
			collector := tally.NewBucketTallyCollector(planet.Satellites[0].Log.Named("bucket tally"), time.Now(), planet.Satellites[0].Metabase.DB, planet.Satellites[0].DB.Buckets(), planet.Satellites[0].Config.Tally)
			err := collector.Run(ctx)
			require.NoError(t, err)

			now := time.Now().Add(time.Duration(i) * time.Second)
			err = planet.Satellites[0].DB.ProjectAccounting().SaveTallies(ctx, now, collector.Bucket)
			require.NoError(t, err)

			assert.Equal(t, 1, len(collector.Bucket))
			for _, actualTally := range collector.Bucket {
				// checking the exact metadata size is brittle, instead, verify that it's not zero
				assert.NotZero(t, actualTally.MetadataSize)
				actualTally.MetadataSize = expectedTally.MetadataSize
				assert.Equal(t, expectedTally, actualTally)
			}
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

		objects, err := satellite.Metabase.DB.TestingAllObjects(ctx)
		require.NoError(t, err)

		segments, err := satellite.Metabase.DB.TestingAllSegments(ctx)
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
			t.TotalSegments++
			t.TotalBytes += int64(segment.EncryptedSize)
		}
		require.Len(t, expectedTotal, 3)

		collector := tally.NewBucketTallyCollector(satellite.Log.Named("bucket tally"), time.Now(), satellite.Metabase.DB, planet.Satellites[0].DB.Buckets(), planet.Satellites[0].Config.Tally)
		err = collector.Run(ctx)
		require.NoError(t, err)
		require.Equal(t, expectedTotal, collector.Bucket)
	})
}

func TestIgnoresExpiredPointers(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		now := time.Now()
		err := planet.Uplinks[0].UploadWithExpiration(ctx, planet.Satellites[0], "bucket", "path", []byte{1}, now.Add(12*time.Hour))
		require.NoError(t, err)

		collector := tally.NewBucketTallyCollector(satellite.Log.Named("bucket tally"), now.Add(24*time.Hour), satellite.Metabase.DB, planet.Satellites[0].DB.Buckets(), planet.Satellites[0].Config.Tally)
		err = collector.Run(ctx)
		require.NoError(t, err)

		// there should be no observed buckets because all of the objects are expired
		require.Equal(t, collector.Bucket, map[metabase.BucketLocation]*accounting.BucketTally{})
	})
}

func TestLiveAccountingWithObjectsLoop(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.MaxSegmentSize(20 * memory.KiB),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		tally := planet.Satellites[0].Accounting.Tally
		projectID := planet.Uplinks[0].Projects[0].ID
		tally.Loop.Pause()

		expectedData := testrand.Bytes(19 * memory.KiB)

		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path", expectedData)
		require.NoError(t, err)

		segments, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 1)

		segmentSize := int64(segments[0].EncryptedSize)

		tally.Loop.TriggerWait()

		expectedSize := segmentSize

		total, err := planet.Satellites[0].Accounting.ProjectUsage.GetProjectStorageTotals(ctx, projectID)
		require.NoError(t, err)
		require.Equal(t, expectedSize, total)

		for i := 0; i < 3; i++ {
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

func TestLiveAccountingWithCustomSQLQuery(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Tally.UseObjectsLoop = false
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		tally := planet.Satellites[0].Accounting.Tally
		projectID := planet.Uplinks[0].Projects[0].ID
		tally.Loop.Pause()

		expectedData := testrand.Bytes(19 * memory.KiB)

		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path", expectedData)
		require.NoError(t, err)

		segments, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 1)

		segmentSize := int64(segments[0].EncryptedSize)

		tally.Loop.TriggerWait()

		expectedSize := segmentSize

		total, err := planet.Satellites[0].Accounting.ProjectUsage.GetProjectStorageTotals(ctx, projectID)
		require.NoError(t, err)
		require.Equal(t, expectedSize, total)

		for i := 0; i < 3; i++ {
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

func TestEmptyProjectUpdatesLiveAccounting(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 2,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.MaxSegmentSize(20 * memory.KiB),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		planet.Satellites[0].Accounting.Tally.Loop.Pause()

		project1 := planet.Uplinks[1].Projects[0].ID

		data := testrand.Bytes(30 * memory.KiB)

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

func TestTallyOnCopiedObject(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		planet.Satellites[0].Accounting.Tally.Loop.Pause()

		testCases := []struct {
			name                     string
			size                     memory.Size
			expectedTallyAfterCopy   accounting.BucketTally
			expectedTallyAfterDelete accounting.BucketTally
		}{
			{"inline", memory.KiB,
				accounting.BucketTally{
					ObjectCount:   2,
					TotalBytes:    2080,
					TotalSegments: 2,
				}, accounting.BucketTally{
					ObjectCount:   1,
					TotalBytes:    1040,
					TotalSegments: 1,
				},
			},
			{"remote", 8 * memory.KiB,
				accounting.BucketTally{
					ObjectCount:   2,
					TotalBytes:    29696,
					TotalSegments: 2,
				},
				accounting.BucketTally{
					ObjectCount:   1,
					TotalBytes:    14848,
					TotalSegments: 1,
				},
			},
		}

		findTally := func(bucket string, tallies []accounting.BucketTally) accounting.BucketTally {
			for _, v := range tallies {
				if v.BucketName == bucket {
					return v
				}
			}
			t.Fatalf("unable to find tally for %s", bucket)
			return accounting.BucketTally{}
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], tc.name)
				require.NoError(t, err)

				data := testrand.Bytes(tc.size)

				err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], tc.name, "ancestor", data)
				require.NoError(t, err)

				project, err := planet.Uplinks[0].GetProject(ctx, planet.Satellites[0])
				require.NoError(t, err)
				defer ctx.Check(project.Close)

				_, err = project.CopyObject(ctx, tc.name, "ancestor", tc.name, "copy", nil)
				require.NoError(t, err)

				planet.Satellites[0].Accounting.Tally.Loop.TriggerWait()
				planet.Satellites[0].Accounting.Tally.Loop.Pause()

				tallies, err := planet.Satellites[0].DB.ProjectAccounting().GetTallies(ctx)
				require.NoError(t, err)
				lastTally := findTally(tc.name, tallies)
				require.Equal(t, tc.name, lastTally.BucketName)
				require.Equal(t, tc.expectedTallyAfterCopy.ObjectCount, lastTally.ObjectCount)
				require.Equal(t, tc.expectedTallyAfterCopy.TotalBytes, lastTally.TotalBytes)
				require.Equal(t, tc.expectedTallyAfterCopy.TotalSegments, lastTally.TotalSegments)

				_, err = project.DeleteObject(ctx, tc.name, "ancestor")
				require.NoError(t, err)

				planet.Satellites[0].Accounting.Tally.Loop.TriggerWait()
				planet.Satellites[0].Accounting.Tally.Loop.Pause()

				tallies, err = planet.Satellites[0].DB.ProjectAccounting().GetTallies(ctx)
				require.NoError(t, err)
				lastTally = findTally(tc.name, tallies)
				require.Equal(t, tc.name, lastTally.BucketName)
				require.Equal(t, tc.expectedTallyAfterDelete.ObjectCount, lastTally.ObjectCount)
				require.Equal(t, tc.expectedTallyAfterDelete.TotalBytes, lastTally.TotalBytes)
				require.Equal(t, tc.expectedTallyAfterDelete.TotalSegments, lastTally.TotalSegments)
			})
		}
	})
}

func TestTallyBatchSize(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.ProjectLimits.MaxBuckets = 100
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		planet.Satellites[0].Accounting.Tally.Loop.Pause()

		projectID := planet.Uplinks[0].Projects[0].ID

		numberOfBuckets := 13
		for i := 0; i < numberOfBuckets; i++ {
			data := testrand.Bytes(1*memory.KiB + memory.Size(i))
			err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "bucket"+strconv.Itoa(i), "test", data)
			require.NoError(t, err)
		}

		objects, err := planet.Satellites[0].Metabase.DB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, numberOfBuckets)

		for _, batchSize := range []int{1, 2, 3, numberOfBuckets, 14, planet.Satellites[0].Config.Tally.ListLimit} {
			collector := tally.NewBucketTallyCollector(zaptest.NewLogger(t), time.Now(), planet.Satellites[0].Metabase.DB, planet.Satellites[0].DB.Buckets(), tally.Config{
				Interval:           1 * time.Hour,
				UseObjectsLoop:     false,
				ListLimit:          batchSize,
				AsOfSystemInterval: 1 * time.Microsecond,
			})
			err := collector.Run(ctx)
			require.NoError(t, err)

			require.Equal(t, numberOfBuckets, len(collector.Bucket))
			for _, object := range objects {
				bucket := collector.Bucket[metabase.BucketLocation{
					ProjectID:  projectID,
					BucketName: object.BucketName,
				}]
				require.Equal(t, object.TotalEncryptedSize, bucket.TotalBytes)
				require.EqualValues(t, 1, bucket.ObjectCount)
			}
		}
	})
}
