// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/console"
)

func Test_DailyUsage(t *testing.T) {
	testplanet.Run(t, testplanet.Config{SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			const (
				bucketName = "testbucket"
				firstPath  = "path"
				secondPath = "another_path"
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

			planet.Satellites[0].Orders.Chore.Loop.Pause()
			satelliteSys.Accounting.Tally.Loop.Pause()

			usage0, err := satelliteSys.DB.ProjectAccounting().GetProjectDailyUsageByDateRange(ctx, projectID, now, inFiveMinutes, 0)
			require.NoError(t, err)
			require.Zero(t, len(usage0.AllocatedBandwidthUsage))
			require.Zero(t, len(usage0.SettledBandwidthUsage))
			require.Zero(t, len(usage0.StorageUsage))

			firstSegment := testrand.Bytes(5 * memory.KiB)
			secondSegment := testrand.Bytes(10 * memory.KiB)

			err = uplink.Upload(ctx, satelliteSys, bucketName, firstPath, firstSegment)
			require.NoError(t, err)
			err = uplink.Upload(ctx, satelliteSys, bucketName, secondPath, secondSegment)
			require.NoError(t, err)

			_, err = uplink.Download(ctx, satelliteSys, bucketName, firstPath)
			require.NoError(t, err)

			require.NoError(t, planet.WaitForStorageNodeEndpoints(ctx))
			tomorrow := time.Now().Add(24 * time.Hour)
			planet.StorageNodes[0].Storage2.Orders.SendOrders(ctx, tomorrow)

			planet.Satellites[0].Orders.Chore.Loop.TriggerWait()
			satelliteSys.Accounting.Tally.Loop.TriggerWait()

			usage1, err := satelliteSys.DB.ProjectAccounting().GetProjectDailyUsageByDateRange(ctx, projectID, now, inFiveMinutes, 0)
			require.NoError(t, err)
			require.GreaterOrEqual(t, usage1.StorageUsage[0].Value, 15*memory.KiB)
			require.GreaterOrEqual(t, usage1.AllocatedBandwidthUsage[0].Value, 5*memory.KiB)
			require.GreaterOrEqual(t, usage1.SettledBandwidthUsage[0].Value, 5*memory.KiB)
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
