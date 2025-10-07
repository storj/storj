// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/uplink/private/metaclient"
)

func pauseAccountingChores(planet *testplanet.Planet) {
	for _, satellite := range planet.Satellites {
		satellite.Accounting.Tally.Loop.Pause()
		satellite.Accounting.Rollup.Loop.Pause()
		satellite.Accounting.RollupArchive.Loop.Pause()
		satellite.Accounting.ProjectBWCleanup.Loop.Pause()
	}
}

func TestDailyUsage(t *testing.T) {
	testplanet.Run(t, testplanet.Config{SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			pauseAccountingChores(planet)

			const (
				firstBucketName  = "testbucket0"
				secondBucketName = "testbucket1"
			)

			now := time.Now().UTC()
			// set time to middle of day to make sure we don't cross the day boundary during tally creation
			twelveToday := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, time.UTC)
			fivePastTwelve := twelveToday.Add(5 * time.Minute)
			yesterday := twelveToday.Add(-24 * time.Hour)
			twoDaysAgo := yesterday.Add(-24 * time.Hour)

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

			_, err = satelliteSys.DB.Console().ProjectMembers().Insert(ctx, user.ID, projectID, console.RoleAdmin)
			require.NoError(t, err)

			usage0, err := satelliteSys.DB.ProjectAccounting().GetProjectDailyUsageByDateRange(ctx, projectID, twoDaysAgo, fivePastTwelve, 0)
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

			// test multiple rows existing each day
			createTallies := func(interval time.Time, tallies map[metabase.BucketLocation]*accounting.BucketTally) {
				for i := 0; i < 3; i++ {
					if i != 0 {
						interval = interval.Add(1 * time.Hour)
					}
					err = satelliteSys.DB.ProjectAccounting().SaveTallies(ctx, interval, tallies)
					require.NoError(t, err)
				}
			}
			createTallies(twoDaysAgo, tallies)
			createTallies(yesterday, tallies)
			createTallies(twelveToday, tallies)

			err = satelliteSys.DB.Orders().UpdateBucketBandwidthSettle(ctx, projectID, []byte(firstBucketName), pb.PieceAction_GET, segment, 0, fivePastTwelve)
			require.NoError(t, err)
			err = satelliteSys.DB.Orders().UpdateBucketBandwidthSettle(ctx, projectID, []byte(secondBucketName), pb.PieceAction_GET, segment, 0, fivePastTwelve)
			require.NoError(t, err)
			err = satelliteSys.DB.Orders().UpdateBucketBandwidthAllocation(ctx, projectID, []byte(firstBucketName), pb.PieceAction_GET, segment, fivePastTwelve)
			require.NoError(t, err)
			err = satelliteSys.DB.Orders().UpdateBucketBandwidthAllocation(ctx, projectID, []byte(secondBucketName), pb.PieceAction_GET, segment, fivePastTwelve)
			require.NoError(t, err)

			usage1, err := satelliteSys.DB.ProjectAccounting().GetProjectDailyUsageByDateRange(ctx, projectID, twoDaysAgo, fivePastTwelve, 0)
			require.NoError(t, err)

			require.Len(t, usage1.StorageUsage, 3)
			for _, u := range usage1.StorageUsage {
				require.Equal(t, 2*segment, u.Value)
			}
			require.Equal(t, 2*segment, usage1.AllocatedBandwidthUsage[0].Value)
			require.Equal(t, 2*segment, usage1.SettledBandwidthUsage[0].Value)
		},
	)
}

func TestGetSingleBucketRollup(t *testing.T) {
	testplanet.Run(t, testplanet.Config{SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			pauseAccountingChores(planet)

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

			_, err = satelliteSys.DB.Console().ProjectMembers().Insert(ctx, user.ID, projectID, console.RoleAdmin)
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

// TestGetProjectTotal tests the GetProjectTotal function of the ProjectAccounting interface.
// It creates mock storage tallies and bandwidth rollups for a project, then verifies that:
//
// 1. Storage and bandwidth metrics are correctly summed within the specified time range
//
// 2. The 'before' parameter is treated as exclusive for storage tallies
//
//  3. When querying with time ranges containing only bandwidth data (no storage tallies), storage
//     metrics are zero while bandwidth is correctly reported
//
// Storage tallies are calculated between intervals of 2 entries, causing the 3rd case to result
// in 0 storage. See documentation of ProjectAccounting.GetProjectTotalByPartner.
func TestGetProjectTotal(t *testing.T) {
	// Spanner only allows dates in the year range of [1, 9999], so a default value will fail.
	since := time.Time{}.Add(24 * 365 * time.Hour)

	testplanet.Run(t, testplanet.Config{SatelliteCount: 1, StorageNodeCount: 1},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			pauseAccountingChores(planet)

			bucketName := testrand.BucketName()
			projectID := testrand.UUID()

			db := planet.Satellites[0].DB

			// The 3rd tally is only present to prevent CreateStorageTally from skipping the 2nd.
			var tallies []accounting.BucketStorageTally
			for i := 0; i < 3; i++ {
				tally := randTally(bucketName, projectID, since.Add(time.Duration(i)*time.Hour))
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
			require.InDelta(t, usage.Storage, float64(tallies[0].Bytes()+tallies[1].Bytes()), epsilon)
			require.InDelta(t, usage.SegmentCount, float64(tallies[0].TotalSegmentCount+tallies[1].TotalSegmentCount), epsilon)
			require.InDelta(t, usage.ObjectCount, float64(tallies[0].ObjectCount+tallies[1].ObjectCount), epsilon)
			require.Equal(t, usage.Egress, expectedEgress)
			require.Equal(t, usage.Since, tallies[0].IntervalStart)
			require.Equal(t, usage.Before, tallies[2].IntervalStart.Add(time.Minute))

			// Ensure that GetProjectTotal treats the 'before' arg as exclusive
			usage, err = db.ProjectAccounting().GetProjectTotal(ctx, projectID, tallies[0].IntervalStart, tallies[2].IntervalStart)
			require.NoError(t, err)
			require.InDelta(t, usage.Storage, float64(tallies[0].Bytes()), epsilon)
			require.InDelta(t, usage.SegmentCount, float64(tallies[0].TotalSegmentCount), epsilon)
			require.InDelta(t, usage.ObjectCount, float64(tallies[0].ObjectCount), epsilon)
			require.Equal(t, usage.Egress, expectedEgress)
			require.Equal(t, usage.Since, tallies[0].IntervalStart)
			require.Equal(t, usage.Before, tallies[2].IntervalStart)

			usage, err = db.ProjectAccounting().GetProjectTotal(ctx, projectID, rollups[0].IntervalStart, rollups[1].IntervalStart)
			require.NoError(t, err)
			require.Zero(t, usage.Storage)
			require.Zero(t, usage.SegmentCount)
			require.Zero(t, usage.ObjectCount)
			require.Equal(t, usage.Egress, rollups[0].Inline+rollups[0].Settled)
			require.Equal(t, usage.Since, rollups[0].IntervalStart)
			require.Equal(t, usage.Before, rollups[1].IntervalStart)
		},
	)
}

// TestGetProjectTotalTallies_monthly_storage tests the GetProjectTotal method of ProjectAccounting
// for calculating monthly storage usage. It verifies that:
//
// 1. Storage byte-hours are correctly calculated across month boundaries
//
// 2. Object counts and segment counts are properly time-weighted
//
// 3. The method accurately handles tallies that span across different months
//
// The test creates storage tallies spanning multiple months (Dec-Apr) and verifies
// calculation correctness for January, February, and March individually.
// Each monthly calculation should include the appropriate time-weighted values
// from the last tally of the previous month and all tallies within the target month
// except the last tally of the month and the end of the month because the upper
// bound tally falls on the next month.
//
// NOTE this test doesn't make any verification about bandwidth usage.
func TestGetProjectTotalTallies_monthly_storage(t *testing.T) {
	testplanet.Run(t, testplanet.Config{SatelliteCount: 1, StorageNodeCount: 0},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			pauseAccountingChores(planet)

			bucketName := testrand.BucketName()
			projectID := testrand.UUID()
			db := planet.Satellites[0].DB

			// Create tallies spanning December 31st to April 2nd
			var (
				december31 = time.Date(2024, time.December, 31, 23, 0, 0, 0, time.UTC)
				dec31Tally = accounting.BucketStorageTally{
					BucketName:        bucketName,
					ProjectID:         projectID,
					IntervalStart:     december31,
					TotalBytes:        200,
					ObjectCount:       2,
					TotalSegmentCount: 2,
				}

				january20  = time.Date(2025, time.January, 20, 12, 0, 0, 0, time.UTC)
				jan20Tally = accounting.BucketStorageTally{
					BucketName:        bucketName,
					ProjectID:         projectID,
					IntervalStart:     january20,
					TotalBytes:        500,
					ObjectCount:       5,
					TotalSegmentCount: 5,
				}

				january31  = time.Date(2025, time.January, 31, 23, 0, 0, 0, time.UTC)
				jan31Tally = accounting.BucketStorageTally{
					BucketName:        bucketName,
					ProjectID:         projectID,
					IntervalStart:     january31,
					TotalBytes:        1000,
					ObjectCount:       10,
					TotalSegmentCount: 10,
				}

				february1 = time.Date(2025, time.February, 1, 0, 0, 0, 0, time.UTC)
				feb1Tally = accounting.BucketStorageTally{
					BucketName:        bucketName,
					ProjectID:         projectID,
					IntervalStart:     february1,
					TotalBytes:        2000,
					ObjectCount:       20,
					TotalSegmentCount: 20,
				}

				february15 = time.Date(2025, time.February, 15, 12, 0, 0, 0, time.UTC)
				feb15Tally = accounting.BucketStorageTally{
					BucketName:        bucketName,
					ProjectID:         projectID,
					IntervalStart:     february15,
					TotalBytes:        3000,
					ObjectCount:       30,
					TotalSegmentCount: 30,
				}

				march1      = time.Date(2025, time.March, 1, 1, 0, 0, 0, time.UTC)
				march1Tally = accounting.BucketStorageTally{
					BucketName:        bucketName,
					ProjectID:         projectID,
					IntervalStart:     march1,
					TotalBytes:        4000,
					ObjectCount:       40,
					TotalSegmentCount: 40,
				}

				april2      = time.Date(2025, time.April, 2, 0, 0, 0, 0, time.UTC)
				april2Tally = accounting.BucketStorageTally{
					BucketName:        bucketName,
					ProjectID:         projectID,
					IntervalStart:     april2,
					TotalBytes:        5000,
					ObjectCount:       50,
					TotalSegmentCount: 50,
				}
			)

			// Save all tallies
			require.NoError(t, db.ProjectAccounting().CreateStorageTally(ctx, dec31Tally))
			require.NoError(t, db.ProjectAccounting().CreateStorageTally(ctx, jan20Tally))
			require.NoError(t, db.ProjectAccounting().CreateStorageTally(ctx, jan31Tally))
			require.NoError(t, db.ProjectAccounting().CreateStorageTally(ctx, feb1Tally))
			require.NoError(t, db.ProjectAccounting().CreateStorageTally(ctx, feb15Tally))
			require.NoError(t, db.ProjectAccounting().CreateStorageTally(ctx, march1Tally))
			require.NoError(t, db.ProjectAccounting().CreateStorageTally(ctx, april2Tally))

			// epsilon is the acceptance tolerance when comparing expected and obtained storage totals
			const epsilon = 1e-8

			t.Run("storage january", func(t *testing.T) {
				var (
					janStart = time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
					febStart = time.Date(2025, time.February, 1, 0, 0, 0, 0, time.UTC)
				)

				usage, err := db.ProjectAccounting().GetProjectTotal(ctx, projectID, janStart, febStart)
				require.NoError(t, err)

				// Calculate expected storage based on the tallies
				// - Dec 31 to Jan 20. It accounts the last entry of the previous month with the first one of
				//   this month
				// - Jan 20 to Jan 31. It accounts periods in between this month
				// - Jan 31 to Feb 1. It isn't accounted because Feb 1 is excluded because febStart is designated
				//   as before
				decToJanHours := january20.Sub(december31).Hours()
				jan20ToJan31Hours := january31.Sub(january20).Hours()
				expectedStorage := float64(dec31Tally.TotalBytes)*decToJanHours +
					float64(jan20Tally.TotalBytes)*jan20ToJan31Hours
				expectedSegments := float64(dec31Tally.TotalSegmentCount)*decToJanHours +
					float64(jan20Tally.TotalSegmentCount)*jan20ToJan31Hours
				expectedObjects := float64(dec31Tally.ObjectCount)*decToJanHours +
					float64(jan20Tally.ObjectCount)*jan20ToJan31Hours

				require.InDelta(t, expectedStorage, usage.Storage, epsilon)
				require.InDelta(t, expectedSegments, usage.SegmentCount, epsilon)
				require.InDelta(t, expectedObjects, usage.ObjectCount, epsilon)
			})

			t.Run("storage february", func(t *testing.T) {
				var (
					febStart   = time.Date(2025, time.February, 1, 0, 0, 0, 0, time.UTC)
					marchStart = time.Date(2025, time.March, 1, 0, 0, 0, 0, time.UTC)
				)

				usage, err := db.ProjectAccounting().GetProjectTotal(ctx, projectID, febStart, marchStart)
				require.NoError(t, err)

				// Calculate expected storage based on the tallies
				// - Jan 31 to Feb 1. It accounts the last entry of the previous month with the first one of
				//   this month
				// - Feb 1 to Feb 15. It accounts periods in between this month
				// - Feb 15 to March 1. It isn't accounted because Feb 1 is excluded because marchStart is
				//   designated as before
				janToFebHours := february1.Sub(january31).Hours()
				feb1ToFeb15Hours := february15.Sub(february1).Hours()
				expectedStorage := float64(jan31Tally.TotalBytes)*janToFebHours +
					float64(feb1Tally.TotalBytes)*feb1ToFeb15Hours
				expectedSegments := float64(jan31Tally.TotalSegmentCount)*janToFebHours +
					float64(feb1Tally.TotalSegmentCount)*feb1ToFeb15Hours
				expectedObjects := float64(jan31Tally.ObjectCount)*janToFebHours +
					float64(feb1Tally.ObjectCount)*feb1ToFeb15Hours

				require.InDelta(t, expectedStorage, usage.Storage, epsilon)
				require.InDelta(t, expectedSegments, usage.SegmentCount, epsilon)
				require.InDelta(t, expectedObjects, usage.ObjectCount, epsilon)
			})

			t.Run("storage march", func(t *testing.T) {
				var (
					marchStart = time.Date(2025, time.March, 1, 0, 0, 0, 0, time.UTC)
					aprilStart = time.Date(2025, time.April, 1, 0, 0, 0, 0, time.UTC)
				)

				usage, err := db.ProjectAccounting().GetProjectTotal(ctx, projectID, marchStart, aprilStart)
				require.NoError(t, err)

				// Calculate expected storage based on the tallies
				// -  Feb 15 to March 1. It accounts the last entry of the previous month with the first one of
				//   this month
				// - March 1 to April 2. It isn't accounted because Feb 1 is excluded because aprilStart is
				//   designated as before
				febToMarchbHours := march1.Sub(february15).Hours()
				expectedStorage := float64(feb15Tally.TotalBytes) * febToMarchbHours
				expectedSegments := float64(feb15Tally.TotalSegmentCount) * febToMarchbHours
				expectedObjects := float64(feb15Tally.ObjectCount) * febToMarchbHours

				require.InDelta(t, expectedStorage, usage.Storage, epsilon)
				require.InDelta(t, expectedSegments, usage.SegmentCount, epsilon)
				require.InDelta(t, expectedObjects, usage.ObjectCount, epsilon)
			})
		},
	)
}

func TestGetSingleBucketTotal(t *testing.T) {
	testplanet.Run(t, testplanet.Config{SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.UseBucketLevelObjectVersioning = true
				config.Metainfo.ObjectLockEnabled = true
			},
		}},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			pauseAccountingChores(planet)

			project := planet.Uplinks[0].Projects[0]
			sat := planet.Satellites[0]
			db := sat.DB
			endpoint := sat.API.Metainfo.Endpoint

			bucketName := testrand.BucketName()
			now := time.Now()
			before := now.Add(time.Hour * 10)

			err := planet.Uplinks[0].CreateBucket(ctx, sat, bucketName)
			require.NoError(t, err)

			client, err := planet.Uplinks[0].Projects[0].DialMetainfo(ctx)
			require.NoError(t, err)
			defer func() {
				require.NoError(t, client.Close())
			}()

			err = client.SetBucketVersioning(ctx, metaclient.SetBucketVersioningParams{
				Name:       []byte(bucketName),
				Versioning: true,
			})
			require.NoError(t, err)

			storedBucket, err := db.Buckets().GetBucket(ctx, []byte(bucketName), project.ID)
			require.NoError(t, err)
			require.Equal(t, buckets.VersioningEnabled, storedBucket.Versioning)
			require.Equal(t, storj.NoRetention, storedBucket.ObjectLock.DefaultRetentionMode)
			require.Zero(t, storedBucket.ObjectLock.DefaultRetentionDays)
			require.Zero(t, storedBucket.ObjectLock.DefaultRetentionYears)

			userCtx, err := sat.UserContext(ctx, project.Owner.ID)
			require.NoError(t, err)

			_, apiKey, err := sat.API.Console.Service.CreateAPIKey(userCtx, project.ID, "test key", macaroon.APIKeyVersionObjectLock)
			require.NoError(t, err)

			setObjectLockConfigReq := &pb.SetBucketObjectLockConfigurationRequest{
				Header: &pb.RequestHeader{
					ApiKey: apiKey.SerializeRaw(),
				},
				Name: []byte(bucketName),
				Configuration: &pb.ObjectLockConfiguration{
					Enabled: true,
					DefaultRetention: &pb.DefaultRetention{
						Mode: pb.Retention_Mode(storj.GovernanceMode),
						Duration: &pb.DefaultRetention_Years{
							Years: 1,
						},
					},
				},
			}

			resp, err := endpoint.SetBucketObjectLockConfiguration(ctx, setObjectLockConfigReq)
			require.NoError(t, err)
			require.NotNil(t, resp)

			storedBucket, err = db.Buckets().GetBucket(ctx, []byte(bucketName), project.ID)
			require.NoError(t, err)
			require.True(t, storedBucket.ObjectLock.Enabled)
			require.Equal(t, storj.GovernanceMode, storedBucket.ObjectLock.DefaultRetentionMode)
			require.Zero(t, storedBucket.ObjectLock.DefaultRetentionDays)
			require.Equal(t, 1, storedBucket.ObjectLock.DefaultRetentionYears)

			storedBucket.Placement = storj.EU
			_, err = db.Buckets().UpdateBucket(ctx, storedBucket)
			require.NoError(t, err)

			// The 3rd tally is only present to prevent CreateStorageTally from skipping the 2nd.
			var tallies []accounting.BucketStorageTally
			for i := 0; i < 3; i++ {
				interval := now.Add(time.Hour * time.Duration(i+1))
				tally := randTally(bucketName, project.ID, interval)
				tallies = append(tallies, tally)
				require.NoError(t, db.ProjectAccounting().CreateStorageTally(ctx, tally))
			}

			var rollups []orders.BucketBandwidthRollup
			var expectedEgress int64
			for i := 0; i < 2; i++ {
				rollup := randRollup(bucketName, project.ID, tallies[i].IntervalStart)
				rollups = append(rollups, rollup)
				expectedEgress += rollup.Inline + rollup.Settled
			}
			require.NoError(t, db.Orders().UpdateBandwidthBatch(ctx, rollups))

			usage, err := db.ProjectAccounting().GetSingleBucketTotals(ctx, project.ID, bucketName, before)
			require.NoError(t, err)
			require.Equal(t, memory.Size(tallies[2].Bytes()).GB(), usage.Storage)
			require.Equal(t, tallies[2].TotalSegmentCount, usage.SegmentCount)
			require.Equal(t, tallies[2].ObjectCount, usage.ObjectCount)
			require.Equal(t, memory.Size(expectedEgress).GB(), usage.Egress)
			require.Equal(t, buckets.VersioningEnabled, usage.Versioning)
			require.Equal(t, storj.EU, usage.DefaultPlacement)
			require.True(t, usage.ObjectLockEnabled)
			require.Equal(t, storj.GovernanceMode, usage.DefaultRetentionMode)
			require.Nil(t, usage.DefaultRetentionDays)
			require.NotNil(t, usage.DefaultRetentionYears)
			require.Equal(t, 1, *usage.DefaultRetentionYears)

			storedBucket.Placement = storj.EveryCountry
			_, err = db.Buckets().UpdateBucket(ctx, storedBucket)
			require.NoError(t, err)

			usage, err = db.ProjectAccounting().GetSingleBucketTotals(ctx, project.ID, bucketName, before)
			require.NoError(t, err)
			require.Equal(t, storj.EveryCountry, usage.DefaultPlacement)

			bucketName1 := testrand.BucketName()

			err = planet.Uplinks[0].CreateBucket(ctx, sat, bucketName1)
			require.NoError(t, err)

			err = client.SetBucketVersioning(ctx, metaclient.SetBucketVersioningParams{
				Name:       []byte(bucketName1),
				Versioning: true,
			})
			require.NoError(t, err)
			err = client.SetBucketVersioning(ctx, metaclient.SetBucketVersioningParams{
				Name:       []byte(bucketName1),
				Versioning: false,
			})
			require.NoError(t, err)

			usage, err = db.ProjectAccounting().GetSingleBucketTotals(ctx, project.ID, bucketName1, before)
			require.NoError(t, err)
			require.Equal(t, buckets.VersioningSuspended, usage.Versioning)
		},
	)
}

func TestGetProjectTotalByPartnerAndPlacement(t *testing.T) {
	const (
		epsilon          = 1e-8
		usagePeriod      = time.Hour
		tallyRollupCount = 2
	)
	// Spanner only allows dates in the year range of [1, 9999], so a default value will fail.
	since := time.Time{}.Add(24 * 365 * time.Hour)
	before := since.Add(2 * usagePeriod)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1,
	},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			pauseAccountingChores(planet)
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

			// Keys are in the format "partner|placement"
			expectedTotals := make(map[string]expectedTotal)

			partnerNames := []string{"", "partner1", "partner2"}
			placements := []storj.PlacementConstraint{storj.DefaultPlacement, storj.PlacementConstraint(1), storj.PlacementConstraint(2)}

			// Create buckets for all combinations of partner and placement
			for _, name := range partnerNames {
				for _, placement := range placements {
					key := fmt.Sprintf("%s|%d", name, placement)
					expectedTotals[key] = expectedTotal{}

					bucket := buckets.Bucket{
						ID:        testrand.UUID(),
						Name:      testrand.BucketName(),
						ProjectID: project.ID,
						Placement: placement,
					}
					if name != "" {
						bucket.UserAgent = []byte(name)
					}
					_, err := sat.DB.Buckets().CreateBucket(ctx, bucket)
					require.NoError(t, err)

					placementPtr := placement
					_, err = sat.DB.Attribution().Insert(ctx, &attribution.Info{
						ProjectID:  project.ID,
						BucketName: []byte(bucket.Name),
						UserAgent:  bucket.UserAgent,
						Placement:  &placementPtr,
					})
					require.NoError(t, err)

					// We use multiple tallies and rollups to ensure that
					// GetProjectTotalByPartnerAndPlacement is capable of summing them.
					total := expectedTotals[key]
					for i := 0; i <= tallyRollupCount; i++ {
						// Create storage tallies with non-zero values that we can track
						tally := accounting.BucketStorageTally{
							BucketName:        bucket.Name,
							ProjectID:         project.ID,
							IntervalStart:     since.Add(time.Duration(i) * usagePeriod / tallyRollupCount),
							TotalBytes:        100 + int64(i*10),
							ObjectCount:       50 + int64(i*5),
							TotalSegmentCount: 25 + int64(i*2),
						}
						require.NoError(t, sat.DB.ProjectAccounting().CreateStorageTally(ctx, tally))

						// The last tally's usage data is unused.
						usageHours := (usagePeriod / tallyRollupCount).Hours()
						if i < tallyRollupCount {
							total.storage += float64(tally.TotalBytes) * usageHours
							total.objects += float64(tally.ObjectCount) * usageHours
							total.segments += float64(tally.TotalSegmentCount) * usageHours
						}
					}

					var rollups []orders.BucketBandwidthRollup
					for i := 0; i < tallyRollupCount; i++ {
						// Create rollups with predictable non-zero values
						rollup := orders.BucketBandwidthRollup{
							ProjectID:     project.ID,
							BucketName:    bucket.Name,
							IntervalStart: since.Add(time.Duration(i) * usagePeriod / tallyRollupCount),
							Action:        pb.PieceAction_GET,
							Inline:        100,
							Settled:       100 + int64(i*10),
						}
						rollups = append(rollups, rollup)
						total.egress += rollup.Inline + rollup.Settled
					}
					require.NoError(t, sat.DB.Orders().UpdateBandwidthBatch(ctx, rollups))

					expectedTotals[key] = total
				}
			}

			requireTotal := func(t *testing.T, expected expectedTotal, expectedSince, expectedBefore time.Time, actual accounting.ProjectUsage) {
				t.Logf("Expected: storage=%.2f, segments=%.2f, objects=%.2f, egress=%d",
					expected.storage, expected.segments, expected.objects, expected.egress)
				t.Logf("Actual:   storage=%.2f, segments=%.2f, objects=%.2f, egress=%d",
					actual.Storage, actual.SegmentCount, actual.ObjectCount, actual.Egress)

				require.InDelta(t, expected.storage, actual.Storage, epsilon)
				require.InDelta(t, expected.segments, actual.SegmentCount, epsilon)
				require.InDelta(t, expected.objects, actual.ObjectCount, epsilon)
				require.Equal(t, expected.egress, actual.Egress)
				require.Equal(t, expectedSince, actual.Since)
				require.Equal(t, expectedBefore, actual.Before)
			}

			t.Run("get usages by partner and placement", func(t *testing.T) {
				ctx := testcontext.New(t)

				usages, err := sat.DB.ProjectAccounting().GetProjectTotalByPartnerAndPlacement(ctx, project.ID, partnerNames, since, before, false)
				require.NoError(t, err)

				// Verify that entries exist and match expected values for all the keys
				for key, expectedTotal := range expectedTotals {
					require.Contains(t, usages, key, "Key %s should be in the results", key)
					requireTotal(t, expectedTotal, since, before, usages[key])
				}

				// Every result key should be in our expected totals
				for key := range usages {
					require.Contains(t, expectedTotals, key, "Unexpected key %s in results", key)
				}
			})

			t.Run("with specific partner subset", func(t *testing.T) {
				ctx := testcontext.New(t)
				// Only include one partner in the filter
				filteredPartners := []string{partnerNames[1]}

				// Debug first how GetProjectTotalByPartner works
				usagesByPartner, err := sat.DB.ProjectAccounting().GetProjectTotalByPartnerAndPlacement(ctx, project.ID, filteredPartners, since, before, false)
				require.NoError(t, err)
				for key, usage := range usagesByPartner {
					t.Logf("Partner '%s': storage=%.2f, segments=%.2f, objects=%.2f, egress=%d",
						key, usage.Storage, usage.SegmentCount, usage.ObjectCount, usage.Egress)
				}

				// Now test our new function
				usages, err := sat.DB.ProjectAccounting().GetProjectTotalByPartnerAndPlacement(ctx, project.ID, filteredPartners, since, before, false)
				require.NoError(t, err)

				// Check specific entries
				for _, placement := range placements {
					// Selected partner should be in the results
					partnerKey := fmt.Sprintf("%s|%d", filteredPartners[0], placement)
					require.Contains(t, usages, partnerKey, "Selected partner key %s should be in results", partnerKey)
					requireTotal(t, expectedTotals[partnerKey], since, before, usages[partnerKey])

					// Empty partner should be in the results
					emptyKey := fmt.Sprintf("|%d", placement)
					require.Contains(t, usages, emptyKey, "Empty partner key %s should be in results", emptyKey)
				}

				// Partner not in the filter should not be in the results
				for _, placement := range placements {
					unwantedKey := fmt.Sprintf("%s|%d", partnerNames[2], placement)
					require.NotContains(t, usages, unwantedKey, "Unwanted key %s should not be in results", unwantedKey)
				}

				// Verify that all keys in the results are expected
				for key := range usages {
					// The key should either be for the filtered partner or for empty partner
					isFilteredPartner := strings.HasPrefix(key, filteredPartners[0]+"|")
					isEmptyPartner := strings.HasPrefix(key, "|")
					require.True(t, isFilteredPartner || isEmptyPartner, "Unexpected key %s in results", key)
				}
			})

			t.Run("aggregated", func(t *testing.T) {
				ctx := testcontext.New(t)

				expected := expectedTotal{}
				for _, usage := range expectedTotals {
					expected.storage += usage.storage
					expected.objects += usage.objects
					expected.segments += usage.segments
					expected.egress += usage.egress
				}

				aggregatedUsages, err := sat.DB.ProjectAccounting().GetProjectTotalByPartnerAndPlacement(ctx, project.ID, partnerNames, since, before, true)
				require.NoError(t, err)
				require.Len(t, aggregatedUsages, 1)

				result, ok := aggregatedUsages[""]
				require.True(t, ok)

				requireTotal(t, expected, since, before, result)
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

func TestGetProjectObjectsSegments(t *testing.T) {
	testplanet.Run(t, testplanet.Config{SatelliteCount: 1, UplinkCount: 1},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			pauseAccountingChores(planet)

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

func TestGetProjectSettledBandwidth(t *testing.T) {
	testplanet.Run(t, testplanet.Config{SatelliteCount: 1, UplinkCount: 1},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			pauseAccountingChores(planet)

			projectID := planet.Uplinks[0].Projects[0].ID
			sat := planet.Satellites[0]
			now := time.Now().UTC()

			egress, err := sat.DB.ProjectAccounting().GetProjectSettledBandwidth(ctx, projectID, now.Year(), now.Month(), 0)
			require.NoError(t, err)
			require.Zero(t, egress)

			bucket := "testbucket"
			err = planet.Uplinks[0].TestingCreateBucket(ctx, sat, bucket)
			require.NoError(t, err)

			bucket1 := "testbucket1"
			err = planet.Uplinks[0].TestingCreateBucket(ctx, sat, bucket1)
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
		pauseAccountingChores(planet)

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

func TestProjectAccounting_GetPreviouslyNonEmptyTallyBucketsInRange(t *testing.T) {
	// test if invalid bucket name will be handled correctly
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		_, err := db.ProjectAccounting().GetPreviouslyNonEmptyTallyBucketsInRange(ctx, metabase.BucketLocation{
			ProjectID:  testrand.UUID(),
			BucketName: "a\\",
		}, metabase.BucketLocation{
			ProjectID:  testrand.UUID(),
			BucketName: "b\\",
		}, 0)
		require.NoError(t, err)
	})
}

// TestGetPreviouslyNonEmptyTallyBucketsInRange_DeletedBucket verifies that
// GetPreviouslyNonEmptyTallyBucketsInRange properly accounts for completely deleted
// buckets.
func TestGetPreviouslyNonEmptyTallyBucketsInRange_DeletedBucket(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 1,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		pauseAccountingChores(planet)

		satellite := planet.Satellites[0]
		uplink := planet.Uplinks[0]
		projectID := uplink.Projects[0].ID

		// Pause tally to control when it runs
		satellite.Accounting.Tally.Loop.Pause()

		// Create a bucket and upload data
		bucketName := testrand.BucketName()
		err := uplink.TestingCreateBucket(ctx, satellite, bucketName)
		require.NoError(t, err)

		data := testrand.Bytes(5 * memory.KiB)
		err = uplink.Upload(ctx, satellite, bucketName, "file", data)
		require.NoError(t, err)

		// Run tally to create storage tallies for the bucket
		satellite.Accounting.Tally.Loop.TriggerWait()

		// Check that the bucket appears in the non-empty buckets range
		from := metabase.BucketLocation{
			ProjectID:  projectID,
			BucketName: "",
		}
		to := metabase.BucketLocation{
			ProjectID:  projectID,
			BucketName: "\xff\xff\xff\xff",
		}

		buckets, err := satellite.DB.ProjectAccounting().GetPreviouslyNonEmptyTallyBucketsInRange(ctx, from, to, 0)
		require.NoError(t, err)

		// The bucket should be found as non-empty
		require.True(t,
			slices.ContainsFunc(buckets, func(loc metabase.BucketLocation) bool {
				return string(loc.BucketName) == bucketName
			}),
			"bucket should be found in non-empty buckets")

		// Now delete all objects and the bucket itself
		err = uplink.DeleteObject(ctx, satellite, bucketName, "file")
		require.NoError(t, err)

		err = uplink.DeleteBucket(ctx, satellite, bucketName)
		require.NoError(t, err)

		// Check that the bucket still appears in the previously non-empty buckets
		// range, even though we just deleted it!
		buckets, err = satellite.DB.ProjectAccounting().GetPreviouslyNonEmptyTallyBucketsInRange(ctx, from, to, 0)
		require.NoError(t, err)

		// The bucket should still be found as previously non-empty
		require.True(t,
			slices.ContainsFunc(buckets, func(loc metabase.BucketLocation) bool {
				return string(loc.BucketName) == bucketName
			}),
			"bucket should be found still until zero tally")

		// Run tally again to create a final zero tally for the bucket
		satellite.Accounting.Tally.Loop.TriggerWait()

		// Check that the bucket no longer appears in the non-empty buckets range
		// This was previously failing because it depended on bucket_metainfos, which
		// is deleted when the bucket is deleted
		buckets, err = satellite.DB.ProjectAccounting().GetPreviouslyNonEmptyTallyBucketsInRange(ctx, from, to, 0)
		require.NoError(t, err)

		// The bucket should no longer be found as non-empty
		require.False(t,
			slices.ContainsFunc(buckets, func(loc metabase.BucketLocation) bool {
				return string(loc.BucketName) == bucketName
			}),
			"bucket should not be found in non-empty buckets after deletion")
	})
}

func TestGetBucketsWithEntitlementsInRange(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		pauseAccountingChores(planet)

		sat := planet.Satellites[0]

		t.Run("with entitlements and different placements", func(t *testing.T) {
			user, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "test user",
				Email:    "test@example.com",
				Password: "password",
			}, 1)
			require.NoError(t, err)
			project, err := sat.AddProject(ctx, user.ID, "testproject1")
			require.NoError(t, err)

			newMapping := entitlements.PlacementProductMappings{
				storj.DefaultPlacement: 1,
				1:                      2,
			}
			err = sat.API.Entitlements.Service.Projects().SetPlacementProductMappingsByPublicID(ctx, project.PublicID, newMapping)
			require.NoError(t, err)

			bucket1Name := "test1-bucket-placement-0"
			bucket2Name := "test1-bucket-placement-1"

			_, err = sat.DB.Buckets().CreateBucket(ctx, buckets.Bucket{
				ID:        testrand.UUID(),
				Name:      bucket1Name,
				ProjectID: project.ID,
				Placement: storj.DefaultPlacement,
			})
			require.NoError(t, err)
			_, err = sat.DB.Buckets().CreateBucket(ctx, buckets.Bucket{
				ID:        testrand.UUID(),
				Name:      bucket2Name,
				ProjectID: project.ID,
				Placement: 1,
			})
			require.NoError(t, err)

			err = sat.DB.ProjectAccounting().SaveTallies(ctx, time.Now(), map[metabase.BucketLocation]*accounting.BucketTally{
				{ProjectID: project.ID, BucketName: metabase.BucketName(bucket1Name)}: {
					BucketLocation: metabase.BucketLocation{ProjectID: project.ID, BucketName: metabase.BucketName(bucket1Name)},
					ObjectCount:    1,
					TotalBytes:     5 * memory.KiB.Int64(),
				},
				{ProjectID: project.ID, BucketName: metabase.BucketName(bucket2Name)}: {
					BucketLocation: metabase.BucketLocation{ProjectID: project.ID, BucketName: metabase.BucketName(bucket2Name)},
					ObjectCount:    1,
					TotalBytes:     5 * memory.KiB.Int64(),
				},
			})
			require.NoError(t, err)

			from := metabase.BucketLocation{
				ProjectID:  project.ID,
				BucketName: metabase.BucketName(bucket1Name),
			}
			to := metabase.BucketLocation{
				ProjectID:  project.ID,
				BucketName: metabase.BucketName(bucket2Name),
			}

			locs, err := sat.DB.ProjectAccounting().GetBucketsWithEntitlementsInRange(ctx, from, to, entitlements.ProjectScopePrefix)
			require.NoError(t, err)
			require.Len(t, locs, 2)

			var bucket1 *accounting.BucketLocationWithEntitlements
			for i := range locs {
				if string(locs[i].Location.BucketName) == bucket1Name {
					bucket1 = &locs[i]
					break
				}
			}
			require.NotNil(t, bucket1)
			require.Equal(t, project.ID, bucket1.Location.ProjectID)
			require.Equal(t, storj.PlacementConstraint(0), bucket1.Placement)
			require.NotEmpty(t, bucket1.ProjectFeatures)
			require.NotNil(t, bucket1.ProjectFeatures.PlacementProductMappings)
			require.Equal(t, newMapping, bucket1.ProjectFeatures.PlacementProductMappings)
			require.True(t, bucket1.HasPreviousTally)

			var bucket2 *accounting.BucketLocationWithEntitlements
			for i := range locs {
				if string(locs[i].Location.BucketName) == bucket2Name {
					bucket2 = &locs[i]
					break
				}
			}
			require.NotNil(t, bucket2)
			require.Equal(t, project.ID, bucket2.Location.ProjectID)
			require.Equal(t, storj.PlacementConstraint(1), bucket2.Placement)
			require.NotEmpty(t, bucket1.ProjectFeatures)
			require.NotNil(t, bucket1.ProjectFeatures.PlacementProductMappings)
			require.Equal(t, newMapping, bucket1.ProjectFeatures.PlacementProductMappings)
		})

		t.Run("without entitlements", func(t *testing.T) {
			user, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "test user 2",
				Email:    "test2@example.com",
				Password: "password",
			}, 1)
			require.NoError(t, err)
			project, err := sat.AddProject(ctx, user.ID, "testproject2")
			require.NoError(t, err)

			bucketName := "bucket-no-entitlements"

			_, err = sat.DB.Buckets().CreateBucket(ctx, buckets.Bucket{
				ID:        testrand.UUID(),
				Name:      bucketName,
				ProjectID: project.ID,
				Placement: storj.DefaultPlacement,
			})
			require.NoError(t, err)
			err = sat.DB.ProjectAccounting().SaveTallies(ctx, time.Now(), map[metabase.BucketLocation]*accounting.BucketTally{
				{ProjectID: project.ID, BucketName: metabase.BucketName(bucketName)}: {
					BucketLocation: metabase.BucketLocation{ProjectID: project.ID, BucketName: metabase.BucketName(bucketName)},
					ObjectCount:    1,
					TotalBytes:     5 * memory.KiB.Int64(),
				},
			})
			require.NoError(t, err)

			from := metabase.BucketLocation{
				ProjectID:  project.ID,
				BucketName: metabase.BucketName(bucketName),
			}
			to := metabase.BucketLocation{
				ProjectID:  project.ID,
				BucketName: metabase.BucketName(bucketName),
			}

			locs, err := sat.DB.ProjectAccounting().GetBucketsWithEntitlementsInRange(ctx, from, to, entitlements.ProjectScopePrefix)
			require.NoError(t, err)
			require.Len(t, locs, 1)
			require.Equal(t, project.ID, locs[0].Location.ProjectID)
			require.Equal(t, bucketName, string(locs[0].Location.BucketName))
			require.Empty(t, locs[0].ProjectFeatures)
			require.True(t, locs[0].HasPreviousTally)
		})

		t.Run("empty buckets with previous tally", func(t *testing.T) {
			user, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "test user 3",
				Email:    "test3@example.com",
				Password: "password",
			}, 1)
			require.NoError(t, err)
			project, err := sat.AddProject(ctx, user.ID, "testproject3")
			require.NoError(t, err)

			bucketName := "bucket-will-be-empty"

			_, err = sat.DB.Buckets().CreateBucket(ctx, buckets.Bucket{
				ID:        testrand.UUID(),
				Name:      bucketName,
				ProjectID: project.ID,
				Placement: storj.DefaultPlacement,
			})
			require.NoError(t, err)
			err = sat.DB.ProjectAccounting().SaveTallies(ctx, time.Now(), map[metabase.BucketLocation]*accounting.BucketTally{
				{ProjectID: project.ID, BucketName: metabase.BucketName(bucketName)}: {
					BucketLocation: metabase.BucketLocation{ProjectID: project.ID, BucketName: metabase.BucketName(bucketName)},
					ObjectCount:    1,
					TotalBytes:     5 * memory.KiB.Int64(),
				},
			})
			require.NoError(t, err)

			from := metabase.BucketLocation{
				ProjectID:  project.ID,
				BucketName: metabase.BucketName(bucketName),
			}
			to := metabase.BucketLocation{
				ProjectID:  project.ID,
				BucketName: metabase.BucketName(bucketName),
			}

			locs, err := sat.DB.ProjectAccounting().GetBucketsWithEntitlementsInRange(ctx, from, to, entitlements.ProjectScopePrefix)
			require.NoError(t, err)
			require.Len(t, locs, 1)
			require.True(t, locs[0].HasPreviousTally)

			// Now insert tally data with zero content (empty bucket)
			err = sat.DB.ProjectAccounting().SaveTallies(ctx, time.Now().Add(time.Minute), map[metabase.BucketLocation]*accounting.BucketTally{
				{ProjectID: project.ID, BucketName: metabase.BucketName(bucketName)}: {
					BucketLocation: metabase.BucketLocation{ProjectID: project.ID, BucketName: metabase.BucketName(bucketName)},
					ObjectCount:    0,
					TotalBytes:     0,
				},
			})
			require.NoError(t, err)

			locs, err = sat.DB.ProjectAccounting().GetBucketsWithEntitlementsInRange(ctx, from, to, entitlements.ProjectScopePrefix)
			require.NoError(t, err)
			require.Len(t, locs, 1)
			require.False(t, locs[0].HasPreviousTally)
		})

		t.Run("multiple projects filtered correctly", func(t *testing.T) {
			user, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "test user 4",
				Email:    "test4@example.com",
				Password: "password",
			}, 2)
			require.NoError(t, err)
			project1, err := sat.AddProject(ctx, user.ID, "testproject4")
			require.NoError(t, err)
			project2, err := sat.AddProject(ctx, user.ID, "testproject5")
			require.NoError(t, err)

			bucket1Name := "project1-bucket"
			bucket2Name := "project2-bucket"

			_, err = sat.DB.Buckets().CreateBucket(ctx, buckets.Bucket{
				ID:        testrand.UUID(),
				Name:      bucket1Name,
				ProjectID: project1.ID,
				Placement: storj.DefaultPlacement,
			})
			require.NoError(t, err)
			_, err = sat.DB.Buckets().CreateBucket(ctx, buckets.Bucket{
				ID:        testrand.UUID(),
				Name:      bucket2Name,
				ProjectID: project2.ID,
				Placement: storj.DefaultPlacement,
			})
			require.NoError(t, err)

			err = sat.DB.ProjectAccounting().SaveTallies(ctx, time.Now(), map[metabase.BucketLocation]*accounting.BucketTally{
				{ProjectID: project1.ID, BucketName: metabase.BucketName(bucket1Name)}: {
					BucketLocation: metabase.BucketLocation{ProjectID: project1.ID, BucketName: metabase.BucketName(bucket1Name)},
					ObjectCount:    1,
					TotalBytes:     5 * memory.KiB.Int64(),
				},
				{ProjectID: project2.ID, BucketName: metabase.BucketName(bucket2Name)}: {
					BucketLocation: metabase.BucketLocation{ProjectID: project2.ID, BucketName: metabase.BucketName(bucket2Name)},
					ObjectCount:    1,
					TotalBytes:     5 * memory.KiB.Int64(),
				},
			})
			require.NoError(t, err)

			from := metabase.BucketLocation{
				ProjectID:  project1.ID,
				BucketName: metabase.BucketName(bucket1Name),
			}
			to := metabase.BucketLocation{
				ProjectID:  project1.ID,
				BucketName: metabase.BucketName(bucket1Name),
			}

			locs, err := sat.DB.ProjectAccounting().GetBucketsWithEntitlementsInRange(ctx, from, to, entitlements.ProjectScopePrefix)
			require.NoError(t, err)
			require.Len(t, locs, 1)
			require.Equal(t, project1.ID, locs[0].Location.ProjectID)
			require.Equal(t, bucket1Name, string(locs[0].Location.BucketName))
		})
	})
}
