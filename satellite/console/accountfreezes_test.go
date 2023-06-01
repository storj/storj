// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package console_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
)

func getUserLimits(u *console.User) console.UsageLimits {
	return console.UsageLimits{
		Storage:   u.ProjectStorageLimit,
		Bandwidth: u.ProjectBandwidthLimit,
		Segment:   u.ProjectSegmentLimit,
	}
}

func getProjectLimits(p *console.Project) console.UsageLimits {
	return console.UsageLimits{
		Storage:   p.StorageLimit.Int64(),
		Bandwidth: p.BandwidthLimit.Int64(),
		Segment:   *p.SegmentLimit,
	}
}

func randUsageLimits() console.UsageLimits {
	return console.UsageLimits{Storage: rand.Int63(), Bandwidth: rand.Int63(), Segment: rand.Int63()}
}

func TestAccountFreeze(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		usersDB := sat.DB.Console().Users()
		projectsDB := sat.DB.Console().Projects()
		service := console.NewAccountFreezeService(sat.DB.Console().AccountFreezeEvents(), usersDB, projectsDB, sat.API.Analytics.Service)

		userLimits := randUsageLimits()
		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "user@mail.test",
		}, 2)
		require.NoError(t, err)
		require.NoError(t, usersDB.UpdateUserProjectLimits(ctx, user.ID, userLimits))

		projLimits := randUsageLimits()
		proj, err := sat.AddProject(ctx, user.ID, "")
		require.NoError(t, err)
		require.NoError(t, projectsDB.UpdateUsageLimits(ctx, proj.ID, projLimits))

		frozen, err := service.IsUserFrozen(ctx, user.ID)
		require.NoError(t, err)
		require.False(t, frozen)

		require.NoError(t, service.FreezeUser(ctx, user.ID))

		user, err = usersDB.Get(ctx, user.ID)
		require.NoError(t, err)
		require.Zero(t, getUserLimits(user))

		proj, err = projectsDB.Get(ctx, proj.ID)
		require.NoError(t, err)
		require.Zero(t, getProjectLimits(proj))

		frozen, err = service.IsUserFrozen(ctx, user.ID)
		require.NoError(t, err)
		require.True(t, frozen)
	})
}

func TestAccountUnfreeze(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		usersDB := sat.DB.Console().Users()
		projectsDB := sat.DB.Console().Projects()
		service := console.NewAccountFreezeService(sat.DB.Console().AccountFreezeEvents(), usersDB, projectsDB, sat.API.Analytics.Service)

		userLimits := randUsageLimits()
		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "user@mail.test",
		}, 2)
		require.NoError(t, err)
		require.NoError(t, usersDB.UpdateUserProjectLimits(ctx, user.ID, userLimits))

		projLimits := randUsageLimits()
		proj, err := sat.AddProject(ctx, user.ID, "")
		require.NoError(t, err)
		require.NoError(t, projectsDB.UpdateUsageLimits(ctx, proj.ID, projLimits))

		require.NoError(t, service.FreezeUser(ctx, user.ID))
		require.NoError(t, service.UnfreezeUser(ctx, user.ID))

		user, err = usersDB.Get(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, userLimits, getUserLimits(user))

		proj, err = projectsDB.Get(ctx, proj.ID)
		require.NoError(t, err)
		require.Equal(t, projLimits, getProjectLimits(proj))

		frozen, err := service.IsUserFrozen(ctx, user.ID)
		require.NoError(t, err)
		require.False(t, frozen)
	})
}

func TestRemoveAccountWarning(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		usersDB := sat.DB.Console().Users()
		projectsDB := sat.DB.Console().Projects()
		service := console.NewAccountFreezeService(sat.DB.Console().AccountFreezeEvents(), usersDB, projectsDB, sat.API.Analytics.Service)

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "user@mail.test",
		}, 2)
		require.NoError(t, err)

		require.NoError(t, service.WarnUser(ctx, user.ID))
		require.NoError(t, service.UnWarnUser(ctx, user.ID))

		freeze, warning, err := service.GetAll(ctx, user.ID)
		require.NoError(t, err)
		require.Nil(t, warning)
		require.Nil(t, freeze)

		require.NoError(t, service.WarnUser(ctx, user.ID))
		require.NoError(t, service.FreezeUser(ctx, user.ID))

		freeze, warning, err = service.GetAll(ctx, user.ID)
		require.NoError(t, err)
		require.NotNil(t, freeze)
		// freezing should remove prior warning events.
		require.Nil(t, warning)
	})
}

func TestAccountFreezeAlreadyFrozen(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		usersDB := sat.DB.Console().Users()
		projectsDB := sat.DB.Console().Projects()
		service := console.NewAccountFreezeService(sat.DB.Console().AccountFreezeEvents(), usersDB, projectsDB, sat.API.Analytics.Service)

		userLimits := randUsageLimits()
		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "user@mail.test",
		}, 2)
		require.NoError(t, err)
		require.NoError(t, usersDB.UpdateUserProjectLimits(ctx, user.ID, userLimits))

		proj1Limits := randUsageLimits()
		proj1, err := sat.AddProject(ctx, user.ID, "project1")
		require.NoError(t, err)
		require.NoError(t, projectsDB.UpdateUsageLimits(ctx, proj1.ID, proj1Limits))

		// Freezing a frozen user should freeze any projects that were unable to be frozen prior.
		// The limits stored for projects frozen by the prior freeze should not be modified.
		t.Run("Project limits", func(t *testing.T) {
			require.NoError(t, service.FreezeUser(ctx, user.ID))

			proj2Limits := randUsageLimits()
			proj2, err := sat.AddProject(ctx, user.ID, "project2")
			require.NoError(t, err)
			require.NoError(t, projectsDB.UpdateUsageLimits(ctx, proj2.ID, proj2Limits))

			require.NoError(t, service.FreezeUser(ctx, user.ID))

			user, err := usersDB.Get(ctx, user.ID)
			require.NoError(t, err)
			require.Zero(t, getUserLimits(user))

			proj2, err = projectsDB.Get(ctx, proj2.ID)
			require.NoError(t, err)
			require.Zero(t, getProjectLimits(proj2))

			require.NoError(t, service.UnfreezeUser(ctx, user.ID))

			user, err = usersDB.Get(ctx, user.ID)
			require.NoError(t, err)
			require.Equal(t, userLimits, getUserLimits(user))

			proj1, err = projectsDB.Get(ctx, proj1.ID)
			require.NoError(t, err)
			require.Equal(t, proj1Limits, getProjectLimits(proj1))

			proj2, err = projectsDB.Get(ctx, proj2.ID)
			require.NoError(t, err)
			require.Equal(t, proj2Limits, getProjectLimits(proj2))
		})

		// Freezing a frozen user should freeze the user's limits if they were unable to be frozen prior.
		t.Run("Unfrozen user limits", func(t *testing.T) {
			user, err := usersDB.Get(ctx, user.ID)
			require.NoError(t, err)

			require.NoError(t, service.FreezeUser(ctx, user.ID))
			require.NoError(t, usersDB.UpdateUserProjectLimits(ctx, user.ID, userLimits))
			require.NoError(t, service.FreezeUser(ctx, user.ID))

			user, err = usersDB.Get(ctx, user.ID)
			require.NoError(t, err)
			require.Zero(t, getUserLimits(user))

			require.NoError(t, service.UnfreezeUser(ctx, user.ID))

			user, err = usersDB.Get(ctx, user.ID)
			require.NoError(t, err)
			require.Equal(t, userLimits, getUserLimits(user))
		})

		// Freezing a frozen user should not modify user limits stored by the prior freeze.
		t.Run("Frozen user limits", func(t *testing.T) {
			require.NoError(t, service.FreezeUser(ctx, user.ID))
			require.NoError(t, service.FreezeUser(ctx, user.ID))

			user, err = usersDB.Get(ctx, user.ID)
			require.NoError(t, err)
			require.Zero(t, getUserLimits(user))

			require.NoError(t, service.UnfreezeUser(ctx, user.ID))
			user, err = usersDB.Get(ctx, user.ID)
			require.NoError(t, err)
			require.Equal(t, userLimits, getUserLimits(user))
		})
	})
}

func TestFreezeEffects(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 2, UplinkCount: 2,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.AccountFreeze.Enabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		usersDB := sat.DB.Console().Users()
		projectsDB := sat.DB.Console().Projects()
		consoleService := sat.API.Console.Service
		freezeService := console.NewAccountFreezeService(sat.DB.Console().AccountFreezeEvents(), usersDB, projectsDB, sat.API.Analytics.Service)

		uplink1 := planet.Uplinks[0]
		user1, _, err := consoleService.GetUserByEmailWithUnverified(ctx, uplink1.User[sat.ID()].Email)
		require.NoError(t, err)

		bucketName := "testbucket"
		path := "test/path"

		expectedData := testrand.Bytes(50 * memory.KiB)

		shouldUploadAndDownload := func(testT *testing.T) {
			// Should be able to upload because account is not warned nor frozen.
			err = uplink1.Upload(ctx, sat, bucketName, path, expectedData)
			require.NoError(testT, err)

			// Should be able to download because account is not frozen.
			data, err := uplink1.Download(ctx, sat, bucketName, path)
			require.NoError(testT, err)
			require.Equal(testT, expectedData, data)
		}

		t.Run("Freeze effect on project owner", func(t *testing.T) {
			shouldUploadAndDownload(t)

			err = freezeService.WarnUser(ctx, user1.ID)
			require.NoError(t, err)

			// Should be able to download because account is not frozen.
			data, err := uplink1.Download(ctx, sat, bucketName, path)
			require.NoError(t, err)
			require.Equal(t, expectedData, data)

			err = freezeService.FreezeUser(ctx, user1.ID)
			require.NoError(t, err)

			// Should not be able to upload because account is frozen.
			err = uplink1.Upload(ctx, sat, bucketName, path, expectedData)
			require.Error(t, err)

			// Should not be able to download because account is frozen.
			_, err = uplink1.Download(ctx, sat, bucketName, path)
			require.Error(t, err)

			// Should not be able to create bucket because account is frozen.
			err = uplink1.CreateBucket(ctx, sat, "anotherBucket")
			require.Error(t, err)

			// Should be able to list even if frozen.
			objects, err := uplink1.ListObjects(ctx, sat, bucketName)
			require.NoError(t, err)
			require.Len(t, objects, 1)

			// Should be able to delete even if frozen.
			err = uplink1.DeleteObject(ctx, sat, bucketName, path)
			require.NoError(t, err)
		})
	})
}
