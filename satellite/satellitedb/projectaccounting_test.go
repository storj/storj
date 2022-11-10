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
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/orders"
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
			err = satelliteSys.DB.Orders().UpdateBucketBandwidthAllocation(ctx, projectID, []byte(firstBucketName), pb.PieceAction_GET, segment, inFiveMinutes)
			require.NoError(t, err)
			err = satelliteSys.DB.Orders().UpdateBucketBandwidthSettle(ctx, projectID, []byte(firstBucketName), pb.PieceAction_GET, segment, 0, inFiveMinutes)
			require.NoError(t, err)
			err = satelliteSys.DB.Orders().UpdateBucketBandwidthAllocation(ctx, projectID, []byte(secondBucketName), pb.PieceAction_GET, segment, inFiveMinutes)
			require.NoError(t, err)
			err = planet.Satellites[0].DB.Orders().UpdateBucketBandwidthSettle(ctx, projectID, []byte(secondBucketName), pb.PieceAction_GET, segment, 0, inFiveMinutes)
			require.NoError(t, err)

			usage1, err := satelliteSys.DB.ProjectAccounting().GetProjectDailyUsageByDateRange(ctx, projectID, now, inFiveMinutes, 0)
			require.NoError(t, err)
			require.Equal(t, 2*segment, usage1.StorageUsage[0].Value)
			require.Equal(t, 2*segment, usage1.AllocatedBandwidthUsage[0].Value)
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
				tally := accounting.BucketStorageTally{
					BucketName:        bucketName,
					ProjectID:         projectID,
					IntervalStart:     time.Time{}.Add(time.Duration(i) * time.Hour),
					TotalBytes:        int64(testrand.Intn(1000)),
					ObjectCount:       int64(testrand.Intn(1000)),
					TotalSegmentCount: int64(testrand.Intn(1000)),
				}
				tallies = append(tallies, tally)
				require.NoError(t, db.ProjectAccounting().CreateStorageTally(ctx, tally))
			}

			var rollups []orders.BucketBandwidthRollup
			var expectedEgress int64
			for i := 0; i < 2; i++ {
				rollup := orders.BucketBandwidthRollup{
					ProjectID:     projectID,
					BucketName:    bucketName,
					Action:        pb.PieceAction_GET,
					IntervalStart: tallies[i].IntervalStart,
					Inline:        int64(testrand.Intn(1000)),
					Settled:       int64(testrand.Intn(1000)),
				}
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
