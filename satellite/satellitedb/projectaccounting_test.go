// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func Test_DailyUsage(t *testing.T) {
	testplanet.Run(t, testplanet.Config{SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			const (
				firstBucketName  = "testbucket0"
				secondBucketName = "testbucket1"
			)

			now := time.Now()
			inFiveMinutes := time.Now().Add(5 * time.Minute)

			var (
				satelliteSys = planet.Satellites[0]
				uplink       = planet.Uplinks[0]
				projectID    = uplink.Projects[0].ID
			)

			newUser := console.CreateUser{
				FullName:  "Project Daily Usage Test",
				ShortName: "",
				Email:     "du@test.test",
			}

			user, err := satelliteSys.AddUser(ctx, newUser, 3)
			require.NoError(t, err)

			_, err = satelliteSys.DB.Console().ProjectMembers().Insert(ctx, user.ID, projectID)
			require.NoError(t, err)

			usage0, err := satelliteSys.DB.ProjectAccounting().GetProjectDailyUsageByDateRange(ctx, projectID, now, inFiveMinutes, 0)
			require.NoError(t, err)
			require.Zero(t, len(usage0.AllocatedBandwidthUsage))
			require.Zero(t, len(usage0.SettledBandwidthUsage))
			require.Zero(t, len(usage0.StorageUsage))

			segment := int64(15000)

			firstBucketLocation := metabase.BucketLocation{
				ProjectID:  projectID,
				BucketName: firstBucketName,
			}
			secondBucketLocation := metabase.BucketLocation{
				ProjectID:  projectID,
				BucketName: secondBucketName,
			}
			tallies := map[metabase.BucketLocation]*accounting.BucketTally{
				firstBucketLocation: {
					BucketLocation: firstBucketLocation,
					TotalBytes:     segment,
				},
				secondBucketLocation: {
					BucketLocation: secondBucketLocation,
					TotalBytes:     segment,
				},
			}

			err = satelliteSys.DB.ProjectAccounting().SaveTallies(ctx, now, tallies)
			require.NoError(t, err)
			err = satelliteSys.DB.Orders().UpdateBucketBandwidthSettle(ctx, projectID, []byte(firstBucketName), pb.PieceAction_GET, segment, 0, inFiveMinutes)
			require.NoError(t, err)
			err = planet.Satellites[0].DB.Orders().UpdateBucketBandwidthSettle(ctx, projectID, []byte(secondBucketName), pb.PieceAction_GET, segment, 0, inFiveMinutes)
			require.NoError(t, err)

			usage1, err := satelliteSys.DB.ProjectAccounting().GetProjectDailyUsageByDateRange(ctx, projectID, now, inFiveMinutes, 0)
			require.NoError(t, err)
			require.Equal(t, 2*segment, usage1.StorageUsage[0].Value)
			require.Equal(t, 2*segment, usage1.SettledBandwidthUsage[0].Value)
		},
	)
}

func Test_GetSingleBucketRollup(t *testing.T) {
	testplanet.Run(t, testplanet.Config{SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			const (
				bucketName = "testbucket"
				firstPath  = "path"
				secondPath = "another_path"
			)

			now := time.Now().UTC()
			inFiveMinutes := time.Now().Add(5 * time.Minute).UTC()

			var (
				satelliteSys = planet.Satellites[0]
				upl          = planet.Uplinks[0]
				projectID    = upl.Projects[0].ID
			)

			newUser := console.CreateUser{
				FullName:  "Project Single Bucket Rollup",
				ShortName: "",
				Email:     "sbur@test.test",
			}

			user, err := satelliteSys.AddUser(ctx, newUser, 3)
			require.NoError(t, err)

			_, err = satelliteSys.DB.Console().ProjectMembers().Insert(ctx, user.ID, projectID)
			require.NoError(t, err)

			planet.Satellites[0].Orders.Chore.Loop.Pause()
			satelliteSys.Accounting.Tally.Loop.Pause()

			timeTruncateDown := func(t time.Time) time.Time {
				return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
			}

			usage0, err := satelliteSys.DB.ProjectAccounting().GetSingleBucketUsageRollup(ctx, projectID, bucketName, now, inFiveMinutes)
			require.NoError(t, err)
			require.Equal(t, bucketName, usage0.BucketName)
			require.Equal(t, projectID, usage0.ProjectID)
			require.Equal(t, timeTruncateDown(now), usage0.Since)
			require.Equal(t, inFiveMinutes, usage0.Before)
			require.Zero(t, usage0.GetEgress)
			require.Zero(t, usage0.ObjectCount)
			require.Zero(t, usage0.AuditEgress)
			require.Zero(t, usage0.RepairEgress)
			require.Zero(t, usage0.MetadataSize)
			require.Zero(t, usage0.TotalSegments)
			require.Zero(t, usage0.TotalStoredData)

			firstSegment := testrand.Bytes(100 * memory.KiB)
			secondSegment := testrand.Bytes(200 * memory.KiB)

			err = upl.Upload(ctx, satelliteSys, bucketName, firstPath, firstSegment)
			require.NoError(t, err)

			err = upl.Upload(ctx, satelliteSys, bucketName, secondPath, secondSegment)
			require.NoError(t, err)

			_, err = upl.Download(ctx, satelliteSys, bucketName, firstPath)
			require.NoError(t, err)

			require.NoError(t, planet.WaitForStorageNodeEndpoints(ctx))
			tomorrow := time.Now().Add(24 * time.Hour)
			planet.StorageNodes[0].Storage2.Orders.SendOrders(ctx, tomorrow)

			planet.Satellites[0].Orders.Chore.Loop.TriggerWait()
			satelliteSys.Accounting.Tally.Loop.TriggerWait()
			// We trigger tally one more time because the most recent tally is skipped in service method.
			satelliteSys.Accounting.Tally.Loop.TriggerWait()

			usage1, err := satelliteSys.DB.ProjectAccounting().GetSingleBucketUsageRollup(ctx, projectID, bucketName, now, inFiveMinutes)
			require.NoError(t, err)
			require.Greater(t, usage1.GetEgress, 0.0)
			require.Greater(t, usage1.ObjectCount, 0.0)
			require.Greater(t, usage1.MetadataSize, 0.0)
			require.Greater(t, usage1.TotalSegments, 0.0)
			require.Greater(t, usage1.TotalStoredData, 0.0)
		},
	)
}

func Test_GetProjectTotal(t *testing.T) {
	testplanet.Run(t, testplanet.Config{SatelliteCount: 1, StorageNodeCount: 1},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			bucketName := testrand.BucketName()
			projectID := testrand.UUID()

			db := planet.Satellites[0].DB

			// The 3rd tally is only present to prevent CreateStorageTally from skipping the 2nd.
			var tallies []accounting.BucketStorageTally
			for i := 0; i < 3; i++ {
				tally := randTally(bucketName, projectID, time.Time{}.Add(time.Duration(i)*time.Hour))
				tallies = append(tallies, tally)
				require.NoError(t, db.ProjectAccounting().CreateStorageTally(ctx, tally))
			}

			var rollups []orders.BucketBandwidthRollup
			var expectedEgress int64
			for i := 0; i < 2; i++ {
				rollup := randRollup(bucketName, projectID, tallies[i].IntervalStart)
				rollups = append(rollups, rollup)
				expectedEgress += rollup.Inline + rollup.Settled
			}
			require.NoError(t, db.Orders().UpdateBandwidthBatch(ctx, rollups))

			usage, err := db.ProjectAccounting().GetProjectTotal(ctx, projectID, tallies[0].IntervalStart, tallies[2].IntervalStart.Add(time.Minute))
			require.NoError(t, err)

			const epsilon = 1e-8
			require.InDelta(t, float64(tallies[0].Bytes()+tallies[1].Bytes()), usage.Storage, epsilon)
			require.InDelta(t, float64(tallies[0].TotalSegmentCount+tallies[1].TotalSegmentCount), usage.SegmentCount, epsilon)
			require.InDelta(t, float64(tallies[0].ObjectCount+tallies[1].ObjectCount), usage.ObjectCount, epsilon)
			require.Equal(t, expectedEgress, usage.Egress)
			require.Equal(t, tallies[0].IntervalStart, usage.Since)
			require.Equal(t, tallies[2].IntervalStart.Add(time.Minute), usage.Before)

			// Ensure that GetProjectTotal treats the 'before' arg as exclusive
			usage, err = db.ProjectAccounting().GetProjectTotal(ctx, projectID, tallies[0].IntervalStart, tallies[2].IntervalStart)
			require.NoError(t, err)
			require.InDelta(t, float64(tallies[0].Bytes()), usage.Storage, epsilon)
			require.InDelta(t, float64(tallies[0].TotalSegmentCount), usage.SegmentCount, epsilon)
			require.InDelta(t, float64(tallies[0].ObjectCount), usage.ObjectCount, epsilon)
			require.Equal(t, expectedEgress, usage.Egress)
			require.Equal(t, tallies[0].IntervalStart, usage.Since)
			require.Equal(t, tallies[2].IntervalStart, usage.Before)

			usage, err = db.ProjectAccounting().GetProjectTotal(ctx, projectID, rollups[0].IntervalStart, rollups[1].IntervalStart)
			require.NoError(t, err)
			require.Zero(t, usage.Storage)
			require.Zero(t, usage.SegmentCount)
			require.Zero(t, usage.ObjectCount)
			require.Equal(t, rollups[0].Inline+rollups[0].Settled, usage.Egress)
			require.Equal(t, rollups[0].IntervalStart, usage.Since)
			require.Equal(t, rollups[1].IntervalStart, usage.Before)
		},
	)
}

func Test_GetProjectTotalByPartner(t *testing.T) {
	const (
		epsilon          = 1e-8
		usagePeriod      = time.Hour
		tallyRollupCount = 2
	)
	since := time.Time{}
	before := since.Add(2 * usagePeriod)

	testplanet.Run(t, testplanet.Config{SatelliteCount: 1, StorageNodeCount: 1},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			sat := planet.Satellites[0]

			user, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Test User",
				Email:    "user@mail.test",
			}, 1)
			require.NoError(t, err)

			project, err := sat.AddProject(ctx, user.ID, "testproject")
			require.NoError(t, err)

			type expectedTotal struct {
				storage  float64
				segments float64
				objects  float64
				egress   int64
			}
			expectedTotals := make(map[string]expectedTotal)
			var beforeTotal expectedTotal

			requireTotal := func(t *testing.T, expected expectedTotal, expectedSince, expectedBefore time.Time, actual accounting.ProjectUsage) {
				require.InDelta(t, expected.storage, actual.Storage, epsilon)
				require.InDelta(t, expected.segments, actual.SegmentCount, epsilon)
				require.InDelta(t, expected.objects, actual.ObjectCount, epsilon)
				require.Equal(t, expected.egress, actual.Egress)
				require.Equal(t, expectedSince, actual.Since)
				require.Equal(t, expectedBefore, actual.Before)
			}

			partnerNames := []string{"", "partner1", "partner2"}
			for _, name := range partnerNames {
				total := expectedTotal{}

				bucket := buckets.Bucket{
					ID:        testrand.UUID(),
					Name:      testrand.BucketName(),
					ProjectID: project.ID,
				}
				if name != "" {
					bucket.UserAgent = []byte(name)
				}
				_, err := sat.DB.Buckets().CreateBucket(ctx, bucket)
				require.NoError(t, err)

				// We use multiple tallies and rollups to ensure that
				// GetProjectTotalByPartner is capable of summing them.
				for i := 0; i <= tallyRollupCount; i++ {
					tally := randTally(bucket.Name, project.ID, since.Add(time.Duration(i)*usagePeriod/tallyRollupCount))
					require.NoError(t, sat.DB.ProjectAccounting().CreateStorageTally(ctx, tally))

					// The last tally's usage data is unused.
					usageHours := (usagePeriod / tallyRollupCount).Hours()
					if i < tallyRollupCount {
						total.storage += float64(tally.Bytes()) * usageHours
						total.objects += float64(tally.ObjectCount) * usageHours
						total.segments += float64(tally.TotalSegmentCount) * usageHours
					}

					if i < tallyRollupCount-1 {
						beforeTotal.storage += float64(tally.Bytes()) * usageHours
						beforeTotal.objects += float64(tally.ObjectCount) * usageHours
						beforeTotal.segments += float64(tally.TotalSegmentCount) * usageHours
					}
				}

				var rollups []orders.BucketBandwidthRollup
				for i := 0; i < tallyRollupCount; i++ {
					rollup := randRollup(bucket.Name, project.ID, since.Add(time.Duration(i)*usagePeriod/tallyRollupCount))
					rollups = append(rollups, rollup)
					total.egress += rollup.Inline + rollup.Settled

					if i < tallyRollupCount {
						beforeTotal.egress += rollup.Inline + rollup.Settled
					}
				}
				require.NoError(t, sat.DB.Orders().UpdateBandwidthBatch(ctx, rollups))

				expectedTotals[name] = total
			}

			t.Run("sum all partner usages", func(t *testing.T) {
				ctx := testcontext.New(t)
				usages, err := sat.DB.ProjectAccounting().GetProjectTotalByPartner(ctx, project.ID, nil, since, before)
				require.NoError(t, err)
				require.Len(t, usages, 1)
				require.Contains(t, usages, "")

				var summedTotal expectedTotal
				for _, total := range expectedTotals {
					summedTotal.storage += total.storage
					summedTotal.segments += total.segments
					summedTotal.objects += total.objects
					summedTotal.egress += total.egress
				}

				requireTotal(t, summedTotal, since, before, usages[""])
			})

			t.Run("individual partner usages", func(t *testing.T) {
				ctx := testcontext.New(t)
				usages, err := sat.DB.ProjectAccounting().GetProjectTotalByPartner(ctx, project.ID, partnerNames, since, before)
				require.NoError(t, err)
				require.Len(t, usages, len(expectedTotals))
				for _, name := range partnerNames {
					require.Contains(t, usages, name)
				}

				for partner, usage := range usages {
					requireTotal(t, expectedTotals[partner], since, before, usage)
				}
			})

			t.Run("select one partner usage and sum remaining usages", func(t *testing.T) {
				ctx := testcontext.New(t)
				partner := partnerNames[len(partnerNames)-1]
				usages, err := sat.DB.ProjectAccounting().GetProjectTotalByPartner(ctx, project.ID, []string{partner}, since, before)
				require.NoError(t, err)
				require.Len(t, usages, 2)
				require.Contains(t, usages, "")
				require.Contains(t, usages, partner)

				var summedTotal expectedTotal
				for _, partner := range partnerNames[:len(partnerNames)-1] {
					summedTotal.storage += expectedTotals[partner].storage
					summedTotal.segments += expectedTotals[partner].segments
					summedTotal.objects += expectedTotals[partner].objects
					summedTotal.egress += expectedTotals[partner].egress
				}

				requireTotal(t, expectedTotals[partner], since, before, usages[partner])
				requireTotal(t, summedTotal, since, before, usages[""])
			})

			t.Run("ensure the 'before' arg is exclusive", func(t *testing.T) {
				ctx := testcontext.New(t)
				before := since.Add(usagePeriod)
				usages, err := sat.DB.ProjectAccounting().GetProjectTotalByPartner(ctx, project.ID, nil, since, before)
				require.NoError(t, err)
				require.Len(t, usages, 1)
				require.Contains(t, usages, "")
				requireTotal(t, beforeTotal, since, before, usages[""])
			})
		},
	)
}

func randTally(bucketName string, projectID uuid.UUID, intervalStart time.Time) accounting.BucketStorageTally {
	return accounting.BucketStorageTally{
		BucketName:        bucketName,
		ProjectID:         projectID,
		IntervalStart:     intervalStart,
		TotalBytes:        int64(testrand.Intn(1000)),
		ObjectCount:       int64(testrand.Intn(1000)),
		TotalSegmentCount: int64(testrand.Intn(1000)),
	}
}

func randRollup(bucketName string, projectID uuid.UUID, intervalStart time.Time) orders.BucketBandwidthRollup {
	return orders.BucketBandwidthRollup{
		ProjectID:     projectID,
		BucketName:    bucketName,
		IntervalStart: intervalStart,
		Action:        pb.PieceAction_GET,
		Inline:        int64(testrand.Intn(1000)),
		Settled:       int64(testrand.Intn(1000)),
	}
}

func Test_GetProjectObjectsSegments(t *testing.T) {
	testplanet.Run(t, testplanet.Config{SatelliteCount: 1, UplinkCount: 1},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			projectID := planet.Uplinks[0].Projects[0].ID

			projectStats, err := planet.Satellites[0].DB.ProjectAccounting().GetProjectObjectsSegments(ctx, projectID)
			require.NoError(t, err)
			require.Zero(t, projectStats.ObjectCount)
			require.Zero(t, projectStats.SegmentCount)

			err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "object", []byte("testdata"))
			require.NoError(t, err)

			// bucket exists but no entry in bucket tallies yet
			projectStats, err = planet.Satellites[0].DB.ProjectAccounting().GetProjectObjectsSegments(ctx, projectID)
			require.NoError(t, err)
			require.Zero(t, projectStats.ObjectCount)
			require.Zero(t, projectStats.SegmentCount)

			planet.Satellites[0].Accounting.Tally.Loop.TriggerWait()

			projectStats, err = planet.Satellites[0].DB.ProjectAccounting().GetProjectObjectsSegments(ctx, projectID)
			require.NoError(t, err)
			require.EqualValues(t, 1, projectStats.ObjectCount)
			require.EqualValues(t, 1, projectStats.SegmentCount)

			// delete object and bucket to see if projects stats will show zero
			err = planet.Uplinks[0].DeleteObject(ctx, planet.Satellites[0], "testbucket", "object")
			require.NoError(t, err)

			err = planet.Uplinks[0].DeleteBucket(ctx, planet.Satellites[0], "testbucket")
			require.NoError(t, err)

			projectStats, err = planet.Satellites[0].DB.ProjectAccounting().GetProjectObjectsSegments(ctx, projectID)
			require.NoError(t, err)
			require.Zero(t, projectStats.ObjectCount)
			require.Zero(t, projectStats.SegmentCount)
		})
}

func Test_GetProjectSettledBandwidth(t *testing.T) {
	testplanet.Run(t, testplanet.Config{SatelliteCount: 1, UplinkCount: 1},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			projectID := planet.Uplinks[0].Projects[0].ID
			sat := planet.Satellites[0]

			now := time.Now().UTC()

			egress, err := sat.DB.ProjectAccounting().GetProjectSettledBandwidth(ctx, projectID, now.Year(), now.Month(), 0)
			require.NoError(t, err)
			require.Zero(t, egress)

			bucket := "testbucket"
			err = planet.Uplinks[0].CreateBucket(ctx, sat, bucket)
			require.NoError(t, err)

			bucket1 := "testbucket1"
			err = planet.Uplinks[0].CreateBucket(ctx, sat, bucket1)
			require.NoError(t, err)

			amount := int64(1000)
			bucketBytes := []byte(bucket)
			bucket1Bytes := []byte(bucket1)
			startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
			err = sat.DB.Orders().UpdateBucketBandwidthAllocation(ctx, projectID, bucketBytes, pb.PieceAction_GET, amount, startOfMonth)
			require.NoError(t, err)
			err = sat.DB.Orders().UpdateBucketBandwidthAllocation(ctx, projectID, bucket1Bytes, pb.PieceAction_GET, 2*amount, startOfMonth)
			require.NoError(t, err)

			egress, err = sat.DB.ProjectAccounting().GetProjectSettledBandwidth(ctx, projectID, now.Year(), now.Month(), 0)
			require.NoError(t, err)
			require.Zero(t, egress)

			err = sat.DB.Orders().UpdateBucketBandwidthSettle(ctx, projectID, bucketBytes, pb.PieceAction_GET, amount, 0, startOfMonth)
			require.NoError(t, err)
			err = sat.DB.Orders().UpdateBucketBandwidthSettle(ctx, projectID, bucket1Bytes, pb.PieceAction_GET, 2*amount, 0, startOfMonth)
			require.NoError(t, err)

			egress, err = sat.DB.ProjectAccounting().GetProjectSettledBandwidth(ctx, projectID, now.Year(), now.Month(), 0)
			require.NoError(t, err)
			require.Equal(t, 3*amount, egress)
		})
}

func TestProjectUsageGap(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		uplink := planet.Uplinks[0]
		tally := sat.Accounting.Tally

		tally.Loop.Pause()

		now := time.Time{}
		tally.SetNow(func() time.Time {
			return now
		})

		const (
			bucketName = "testbucket"
			objectPath = "test/path"
		)

		data := testrand.Bytes(10)
		require.NoError(t, uplink.Upload(ctx, sat, bucketName, objectPath, data))
		tally.Loop.TriggerWait()

		objs, err := sat.Metabase.DB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objs, 1)
		expectedStorage := objs[0].TotalEncryptedSize

		now = now.Add(time.Hour)
		require.NoError(t, uplink.DeleteObject(ctx, sat, bucketName, objectPath))
		tally.Loop.TriggerWait()

		// This object is only uploaded and tallied so that the usage calculator knows
		// how long it's been since the previous tally.
		now = now.Add(time.Hour)
		require.NoError(t, uplink.Upload(ctx, sat, bucketName, objectPath, data))
		tally.Loop.TriggerWait()

		// The bucket was full for only 1 hour, so expect `expectedStorage` byte-hours of storage usage.
		usage, err := sat.DB.ProjectAccounting().GetProjectTotal(ctx, uplink.Projects[0].ID, time.Time{}, now.Add(time.Second))
		require.NoError(t, err)
		require.EqualValues(t, expectedStorage, usage.Storage)
	})
}

func TestProjectaccounting_GetNonEmptyTallyBucketsInRange(t *testing.T) {
	// test if invalid bucket name will be handled correctly
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		_, err := db.ProjectAccounting().GetNonEmptyTallyBucketsInRange(ctx, metabase.BucketLocation{
			ProjectID:  testrand.UUID(),
			BucketName: "a\\",
		}, metabase.BucketLocation{
			ProjectID:  testrand.UUID(),
			BucketName: "b\\",
		})
		require.NoError(t, err)
	})
}
