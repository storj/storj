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
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		nodeIDs := []storj.NodeID{{1}, {2}, {3}}
		nodeBWAmounts := []float64{1000, 1000, 1000}

		tests := []struct {
			name         string
			tallyTimes   []time.Duration // relative to base time
			eraseBefore  time.Duration   // relative to base time
			expectedRaws int
		}{
			{
				name:         "delete nothing when before is earlier than all tallies",
				tallyTimes:   []time.Duration{0, 12 * time.Hour, 36 * time.Hour},
				eraseBefore:  -24 * time.Hour,
				expectedRaws: 9, // 3 nodes * 3 tallies
			},
			{
				name:         "delete first 24h chunk",
				tallyTimes:   []time.Duration{0, 12 * time.Hour, 36 * time.Hour},
				eraseBefore:  24 * time.Hour,
				expectedRaws: 3, // only tallies at 36h remain
			},
			{
				name:         "delete across multiple 24h chunks",
				tallyTimes:   []time.Duration{0, 25 * time.Hour, 50 * time.Hour, 75 * time.Hour},
				eraseBefore:  72 * time.Hour,
				expectedRaws: 3, // only tallies at 75h remain
			},
			{
				name:         "delete all tallies",
				tallyTimes:   []time.Duration{0, 12 * time.Hour, 36 * time.Hour},
				eraseBefore:  100 * time.Hour,
				expectedRaws: 0,
			},
			{
				name:         "delete at chunk boundary",
				tallyTimes:   []time.Duration{0, 24 * time.Hour, 48 * time.Hour},
				eraseBefore:  48 * time.Hour,
				expectedRaws: 3, // tallies at exactly 48h remain
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				baseTime := time.Now()

				// Create tallies at different times spanning multiple 24h periods
				for _, tallyTime := range test.tallyTimes {
					err := planet.Satellites[0].DB.StoragenodeAccounting().SaveTallies(ctx, baseTime.Add(tallyTime), nodeIDs, nodeBWAmounts)
					require.NoError(t, err)
				}

				// Delete tallies using 24h chunk deletion
				err := planet.Satellites[0].DB.StoragenodeAccounting().DeleteTalliesBefore(ctx, baseTime.Add(test.eraseBefore), 1)
				require.NoError(t, err)

				raws, err := planet.Satellites[0].DB.StoragenodeAccounting().GetTallies(ctx)
				require.NoError(t, err)
				assert.Len(t, raws, test.expectedRaws)

				// cleanup state for next test if needed
				if len(raws) > 0 {
					err = planet.Satellites[0].DB.StoragenodeAccounting().DeleteTalliesBefore(ctx, baseTime.Add(7*24*time.Hour), 1)
					require.NoError(t, err)
				}
			})
		}
	})
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
				BucketName: metabase.BucketName(expectedBucketName),
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
			collector := tally.NewBucketTallyCollector(
				planet.Satellites[0].Log.Named("bucket tally"),
				time.Now(),
				planet.Satellites[0].Metabase.DB,
				planet.Satellites[0].DB.Buckets(),
				planet.Satellites[0].DB.ProjectAccounting(),
				nil, // productPrices
				nil, // globalPlacementMap
				planet.Satellites[0].Config.Tally,
			)
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

		collector := tally.NewBucketTallyCollector(
			satellite.Log.Named("bucket tally"),
			time.Now(),
			satellite.Metabase.DB,
			planet.Satellites[0].DB.Buckets(),
			planet.Satellites[0].DB.ProjectAccounting(),
			nil, // productPrices
			nil, // globalPlacementMap
			planet.Satellites[0].Config.Tally,
		)
		err = collector.Run(ctx)
		require.NoError(t, err)
		require.Equal(t, expectedTotal, collector.Bucket)
	})
}

func TestIgnoresExpiredSegments(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		const bucketName = "bucket"

		now := time.Now()
		require.NoError(t, planet.Uplinks[0].UploadWithExpiration(ctx, planet.Satellites[0], "bucket", "path", []byte{1}, now.Add(12*time.Hour)))

		collector := tally.NewBucketTallyCollector(
			satellite.Log.Named("bucket tally"),
			now.Add(24*time.Hour),
			satellite.Metabase.DB,
			planet.Satellites[0].DB.Buckets(),
			planet.Satellites[0].DB.ProjectAccounting(),
			nil, // productPrices
			nil, // globalPlacementMap
			planet.Satellites[0].Config.Tally,
		)
		require.NoError(t, collector.Run(ctx))

		// there should be a single empty tally (or no tally) because all of the
		// objects are expired
		loc := metabase.BucketLocation{
			ProjectID:  planet.Uplinks[0].Projects[0].ID,
			BucketName: bucketName,
		}
		switch len(collector.Bucket) {
		case 0:
		// great
		case 1:
			require.EqualValues(t, collector.Bucket[loc].ObjectCount, 0)
		default:
			require.Fail(t, "an unexpected amount of buckets", len(collector.Bucket))
		}
	})
}

func TestLiveAccountingWithCustomSQLQuery(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
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

		findTally := func(t *testing.T, bucket string, tallies []accounting.BucketTally) accounting.BucketTally {
			for _, v := range tallies {
				if v.BucketName == metabase.BucketName(bucket) {
					return v
				}
			}
			t.Fatalf("unable to find tally for %s", bucket)
			return accounting.BucketTally{}
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				planet.Satellites[0].Accounting.Tally.Loop.Pause()

				err := planet.Uplinks[0].TestingCreateBucket(ctx, planet.Satellites[0], tc.name)
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

				tallies, err := planet.Satellites[0].DB.ProjectAccounting().GetTallies(ctx)
				require.NoError(t, err)
				lastTally := findTally(t, tc.name, tallies)
				require.Equal(t, metabase.BucketName(tc.name), lastTally.BucketName)
				require.Equal(t, tc.expectedTallyAfterCopy.ObjectCount, lastTally.ObjectCount)
				require.Equal(t, tc.expectedTallyAfterCopy.TotalBytes, lastTally.TotalBytes)
				require.Equal(t, tc.expectedTallyAfterCopy.TotalSegments, lastTally.TotalSegments)

				_, err = project.DeleteObject(ctx, tc.name, "ancestor")
				require.NoError(t, err)

				planet.Satellites[0].Accounting.Tally.Loop.TriggerWait()

				tallies, err = planet.Satellites[0].DB.ProjectAccounting().GetTallies(ctx)
				require.NoError(t, err)
				lastTally = findTally(t, tc.name, tallies)
				require.Equal(t, metabase.BucketName(tc.name), lastTally.BucketName)
				require.Equal(t, tc.expectedTallyAfterDelete.ObjectCount, lastTally.ObjectCount)
				require.Equal(t, tc.expectedTallyAfterDelete.TotalBytes, lastTally.TotalBytes)
				require.Equal(t, tc.expectedTallyAfterDelete.TotalSegments, lastTally.TotalSegments)
			})
		}
	})
}

func TestBucketTallyCollectorListLimit(t *testing.T) {
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
			collector := tally.NewBucketTallyCollector(
				zaptest.NewLogger(t),
				time.Now(),
				planet.Satellites[0].Metabase.DB,
				planet.Satellites[0].DB.Buckets(),
				planet.Satellites[0].DB.ProjectAccounting(),
				nil, // productPrices
				nil, // globalPlacementMap
				tally.Config{
					Interval:           1 * time.Hour,
					ListLimit:          batchSize,
					AsOfSystemInterval: 1 * time.Microsecond,
				},
			)
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

func TestTallySaveTalliesBatchSize(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,

		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.ProjectLimits.MaxBuckets = 23
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		planet.Satellites[0].Accounting.Tally.Loop.Pause()

		projectID := planet.Uplinks[0].Projects[0].ID

		numberOfBuckets := 23
		expectedBucketLocations := []metabase.BucketLocation{}
		for i := 0; i < numberOfBuckets; i++ {
			data := testrand.Bytes(1*memory.KiB + memory.Size(i))
			err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "bucket"+strconv.Itoa(i), "test", data)
			require.NoError(t, err)

			expectedBucketLocations = append(expectedBucketLocations, metabase.BucketLocation{
				ProjectID:  projectID,
				BucketName: metabase.BucketName("bucket" + strconv.Itoa(i)),
			})
		}

		satellite := planet.Satellites[0]
		for _, batchSize := range []int{1, 2, 3, numberOfBuckets, 29, planet.Satellites[0].Config.Tally.SaveTalliesBatchSize} {
			config := satellite.Config.Tally
			config.SaveTalliesBatchSize = batchSize

			tally := tally.New(zaptest.NewLogger(t), satellite.DB.StoragenodeAccounting(), satellite.DB.ProjectAccounting(),
				satellite.LiveAccounting.Cache, satellite.Metabase.DB, satellite.DB.Buckets(), config,
				nil, // productPrices
				nil, // globalPlacementMap
			)

			// collect and store tallies in DB
			err := tally.Tally(ctx)
			require.NoError(t, err)

			// verify we have in DB expected list of tallies
			tallies, err := satellite.DB.ProjectAccounting().GetTallies(ctx)
			require.NoError(t, err)

			_, err = satellite.DB.Testing().RawDB().ExecContext(ctx, "DELETE FROM bucket_storage_tallies WHERE TRUE")
			require.NoError(t, err)

			bucketLocations := []metabase.BucketLocation{}
			for _, tally := range tallies {
				bucketLocations = append(bucketLocations, tally.BucketLocation)
			}

			require.ElementsMatch(t, expectedBucketLocations, bucketLocations)
		}
	})
}

func TestTallyPurge(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		projectID := planet.Uplinks[0].Projects[0].ID

		tally := planet.Satellites[0].Accounting.Tally
		tally.Loop.Pause()

		var (
			now        = time.Now().Truncate(time.Hour).UTC()
			timeBefore = now.AddDate(0, 0, -366)
			timeAt     = now.AddDate(0, 0, -365)
			timeAfter  = now.AddDate(0, 0, -364)

			tallyBefore = accounting.BucketTally{BucketLocation: metabase.BucketLocation{ProjectID: projectID, BucketName: "before"}}
			tallyAt     = accounting.BucketTally{BucketLocation: metabase.BucketLocation{ProjectID: projectID, BucketName: "at"}}
			tallyAfter  = accounting.BucketTally{BucketLocation: metabase.BucketLocation{ProjectID: projectID, BucketName: "after"}}
		)

		err := satellite.DB.ProjectAccounting().SaveTallies(ctx, timeBefore, map[metabase.BucketLocation]*accounting.BucketTally{
			tallyBefore.BucketLocation: &tallyBefore,
		})
		require.NoError(t, err)

		err = satellite.DB.ProjectAccounting().SaveTallies(ctx, timeAt, map[metabase.BucketLocation]*accounting.BucketTally{
			tallyAt.BucketLocation: &tallyAt,
		})
		require.NoError(t, err)

		err = satellite.DB.ProjectAccounting().SaveTallies(ctx, timeAfter, map[metabase.BucketLocation]*accounting.BucketTally{
			tallyAfter.BucketLocation: &tallyAfter,
		})
		require.NoError(t, err)

		// Capture the pre-purge state.
		prePurge, err := satellite.DB.ProjectAccounting().GetTallies(ctx)
		require.NoError(t, err)
		require.ElementsMatch(t, []accounting.BucketTally{tallyBefore, tallyAt, tallyAfter}, prePurge)

		// Inject now as the time and run the loop to initiate the purge.
		tally.SetNow(func() time.Time { return now })
		tally.Loop.TriggerWait()

		// Capture the post-purge state and assert that the "before" tally
		// has deleted.
		postPurge, err := satellite.DB.ProjectAccounting().GetTallies(ctx)
		require.NoError(t, err)
		require.ElementsMatch(t, []accounting.BucketTally{tallyAt, tallyAfter}, postPurge)
	})
}

func TestBucketTallyCollectorWithStorageRemainder(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 3,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Tally.SmallObjectRemainder = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		sat.Accounting.Tally.Loop.Pause()

		t.Run("single remainder", func(t *testing.T) {
			projectID := planet.Uplinks[0].Projects[0].ID

			// Upload objects of various sizes to test remainder logic.
			err := planet.Uplinks[0].Upload(ctx, sat, "bucket-small", "object1", testrand.Bytes(5*memory.KiB))
			require.NoError(t, err)

			err = planet.Uplinks[0].Upload(ctx, sat, "bucket-medium", "object2", testrand.Bytes(30*memory.KiB))
			require.NoError(t, err)

			err = planet.Uplinks[0].Upload(ctx, sat, "bucket-large", "object3", testrand.Bytes(100*memory.KiB))
			require.NoError(t, err)

			// Set remainder to 50KB - objects smaller than this should be counted as 50KB.
			remainder := int64(50 * memory.KiB)
			productPrices := map[int32]tally.ProductUsagePriceModel{
				0: {
					ProductID:             0,
					StorageRemainderBytes: remainder,
				},
			}
			globalPlacementMap := tally.PlacementProductMap{0: 0}

			collector := tally.NewBucketTallyCollector(
				sat.Log.Named("bucket tally remainder"),
				time.Now(),
				sat.Metabase.DB,
				sat.DB.Buckets(),
				sat.DB.ProjectAccounting(),
				productPrices,
				globalPlacementMap,
				sat.Config.Tally,
			)
			err = collector.Run(ctx)
			require.NoError(t, err)

			// Verify all buckets were collected.
			require.GreaterOrEqual(t, len(collector.Bucket), 3, "should have at least 3 buckets")

			bucketSmall := collector.Bucket[metabase.BucketLocation{
				ProjectID:  projectID,
				BucketName: "bucket-small",
			}]
			bucketMedium := collector.Bucket[metabase.BucketLocation{
				ProjectID:  projectID,
				BucketName: "bucket-medium",
			}]
			bucketLarge := collector.Bucket[metabase.BucketLocation{
				ProjectID:  projectID,
				BucketName: "bucket-large",
			}]

			require.NotNil(t, bucketSmall, "bucket-small should exist")
			require.NotNil(t, bucketMedium, "bucket-medium should exist")
			require.NotNil(t, bucketLarge, "bucket-large should exist")

			// Verify remainder is applied correctly:
			// Small bucket (5KB) should be counted as 50KB (remainder applies).
			require.Equal(t, bucketSmall.TotalBytes, remainder,
				"Small object (5KB) size should be equal to remainder size (50KB)")
			require.EqualValues(t, 1, bucketSmall.ObjectCount, "should have 1 object")

			// Medium bucket (30KB) should be counted as 50KB (remainder applies).
			require.Equal(t, bucketMedium.TotalBytes, remainder,
				"Medium object (30KB) size should be equal to remainder size (50KB)")
			require.EqualValues(t, 1, bucketMedium.ObjectCount, "should have 1 object")

			// Large bucket (100KB) should be counted at actual size (already larger than remainder).
			// The actual bytes will be > 100KB due to encoding overhead, so just verify it's reasonable.
			require.Greater(t, bucketLarge.TotalBytes, remainder,
				"Large object (100KB) should be larger than remainder (50KB)")
			require.Greater(t, bucketLarge.TotalBytes, int64(100*memory.KiB),
				"Large object should be at least 100KB")
			require.EqualValues(t, 1, bucketLarge.ObjectCount, "should have 1 object")

			// Verify size ordering: small = medium < large (since small and medium both get remainder).
			require.Equal(t, bucketSmall.TotalBytes, bucketMedium.TotalBytes,
				"Small and medium buckets should have equal sizes (both at remainder)")
			require.Greater(t, bucketLarge.TotalBytes, bucketMedium.TotalBytes,
				"Large bucket should have more bytes than medium bucket")
		})

		t.Run("multiple remainders", func(t *testing.T) {
			projectID := planet.Uplinks[1].Projects[0].ID

			// Upload objects to different buckets.
			err := planet.Uplinks[1].Upload(ctx, sat, "bucket-no-remainder", "object", testrand.Bytes(30*memory.KiB))
			require.NoError(t, err)

			// Configure different remainders for different product IDs.
			// In reality, different buckets would have different product IDs via entitlements.
			productPrices := map[int32]tally.ProductUsagePriceModel{
				0: {ProductID: 0, StorageRemainderBytes: 0},          // No remainder
				1: {ProductID: 1, StorageRemainderBytes: 50 * 1024},  // 50KB
				2: {ProductID: 2, StorageRemainderBytes: 100 * 1024}, // 100KB
			}

			globalPlacementMap := tally.PlacementProductMap{
				0: 0, // Default placement → product 0 (no remainder).
			}

			collector := tally.NewBucketTallyCollector(
				sat.Log.Named("bucket tally"),
				time.Now(),
				sat.Metabase.DB,
				sat.DB.Buckets(),
				sat.DB.ProjectAccounting(),
				productPrices,
				globalPlacementMap,
				sat.Config.Tally,
			)
			err = collector.Run(ctx)
			require.NoError(t, err)

			require.GreaterOrEqual(t, len(collector.Bucket), 1)

			// Verify the bucket exists.
			bucket := collector.Bucket[metabase.BucketLocation{
				ProjectID:  projectID,
				BucketName: "bucket-no-remainder",
			}]
			require.NotNil(t, bucket, "bucket-no-remainder should exist")
			require.EqualValues(t, 1, bucket.ObjectCount, "should have 1 object")
		})

		t.Run("empty buckets", func(t *testing.T) {
			projectID := planet.Uplinks[2].Projects[0].ID

			// Test that emptied buckets (objects deleted but bucket still exists) get empty tallies.
			err := planet.Uplinks[2].Upload(ctx, sat, "bucket-to-empty", "object", testrand.Bytes(30*memory.KiB))
			require.NoError(t, err)

			remainder := int64(50 * memory.KiB)
			productPrices := map[int32]tally.ProductUsagePriceModel{
				0: {ProductID: 0, StorageRemainderBytes: remainder},
			}
			globalPlacementMap := tally.PlacementProductMap{0: 0}

			// Run collector and save tally (this creates a previous non-empty tally).
			collector := tally.NewBucketTallyCollector(
				sat.Log.Named("bucket tally"),
				time.Now(),
				sat.Metabase.DB,
				sat.DB.Buckets(),
				sat.DB.ProjectAccounting(),
				productPrices,
				globalPlacementMap,
				sat.Config.Tally,
			)
			err = collector.Run(ctx)
			require.NoError(t, err)

			bucketLoc := metabase.BucketLocation{
				ProjectID:  projectID,
				BucketName: "bucket-to-empty",
			}

			// Verify the bucket has data.
			bucket := collector.Bucket[bucketLoc]
			require.NotNil(t, bucket, "bucket should exist")
			require.Greater(t, bucket.TotalBytes, int64(0), "bucket should have data")
			require.EqualValues(t, 1, bucket.ObjectCount, "should have 1 object")

			// Save this tally so it becomes a "previous tally".
			err = sat.DB.ProjectAccounting().SaveTallies(ctx, time.Now(), collector.Bucket)
			require.NoError(t, err)

			// Delete the object from the bucket (bucket still exists in bucket_metainfos).
			err = planet.Uplinks[2].DeleteObject(ctx, sat, "bucket-to-empty", "object")
			require.NoError(t, err)

			// Run collector again - emptied bucket should still be present with empty tally.
			collector2 := tally.NewBucketTallyCollector(
				sat.Log.Named("bucket tally"),
				time.Now(),
				sat.Metabase.DB,
				sat.DB.Buckets(),
				sat.DB.ProjectAccounting(),
				productPrices,
				globalPlacementMap,
				sat.Config.Tally,
			)
			err = collector2.Run(ctx)
			require.NoError(t, err)

			// The emptied bucket should appear with zero tally (to mark it as now empty).
			emptiedBucket := collector2.Bucket[bucketLoc]
			require.NotNil(t, emptiedBucket, "emptied bucket should still appear in tally")
			require.EqualValues(t, 0, emptiedBucket.TotalBytes, "emptied bucket should have zero bytes")
			require.EqualValues(t, 0, emptiedBucket.ObjectCount, "emptied bucket should have zero objects")
			require.EqualValues(t, 0, emptiedBucket.TotalSegments, "emptied bucket should have zero segments")
		})
	})
}
