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
			require.Zero(t, len(usage0.BandwidthUsage))
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
			require.GreaterOrEqual(t, usage1.BandwidthUsage[0].Value, 5*memory.KiB)
		},
	)
}
