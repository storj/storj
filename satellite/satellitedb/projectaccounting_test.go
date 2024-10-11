// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

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
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/uplink/private/metaclient"
)

func TestDailyUsage(t *testing.T) {
	testplanet.Run(t, testplanet.Config{SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1, EnableSpanner: true},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
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
	testplanet.Run(t, testplanet.Config{SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1, EnableSpanner: true},
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

func TestGetProjectTotal(t *testing.T) {
	// Spanner only allows dates in the year range of [1, 9999], so a default value will fail.
	since := time.Time{}.Add(24 * 365 * time.Hour)

	testplanet.Run(t, testplanet.Config{SatelliteCount: 1, StorageNodeCount: 1, EnableSpanner: true},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
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

func TestSingleBucketTotal(t *testing.T) {
	testplanet.Run(t, testplanet.Config{SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1, EnableSpanner: true,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.UseBucketLevelObjectVersioning = true
			},
		}},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			project := planet.Uplinks[0].Projects[0]
			sat := planet.Satellites[0]
			db := sat.DB

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

			err = client.SetBucketVersioning(ctx, metaclient.SetBucketVersioningParams{
				Name:       []byte(bucketName),
				Versioning: false,
			})
			require.NoError(t, err)

			storedBucket.Placement = storj.EveryCountry
			_, err = db.Buckets().UpdateBucket(ctx, storedBucket)
			require.NoError(t, err)

			usage, err = db.ProjectAccounting().GetSingleBucketTotals(ctx, project.ID, bucketName, before)
			require.NoError(t, err)
			require.Equal(t, buckets.VersioningSuspended, usage.Versioning)
			require.Equal(t, storj.EveryCountry, usage.DefaultPlacement)
		},
	)
}

func TestGetProjectTotalByPartner(t *testing.T) {
	const (
		epsilon          = 1e-8
		usagePeriod      = time.Hour
		tallyRollupCount = 2
	)
	// Spanner only allows dates in the year range of [1, 9999], so a default value will fail.
	since := time.Time{}.Add(24 * 365 * time.Hour)
	before := since.Add(2 * usagePeriod)

	testplanet.Run(t, testplanet.Config{SatelliteCount: 1, StorageNodeCount: 1, EnableSpanner: true},
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

				_, err = sat.DB.Attribution().Insert(ctx, &attribution.Info{
					ProjectID:  project.ID,
					BucketName: []byte(bucket.Name),
					UserAgent:  bucket.UserAgent,
				})
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

func TestGetProjectObjectsSegments(t *testing.T) {
	testplanet.Run(t, testplanet.Config{SatelliteCount: 1, UplinkCount: 1, EnableSpanner: true},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			planet.Satellites[0].Accounting.Tally.Loop.Pause()

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
	testplanet.Run(t, testplanet.Config{SatelliteCount: 1, UplinkCount: 1, EnableSpanner: true},
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
		SatelliteCount: 1, UplinkCount: 1, EnableSpanner: true,
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
		}, 0)
		require.NoError(t, err)
	}, satellitedbtest.WithSpanner())
}
