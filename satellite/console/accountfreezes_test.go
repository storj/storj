// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package console_test

import (
	"database/sql"
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/uplink"
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
		Storage:    p.StorageLimit.Int64(),
		Bandwidth:  p.BandwidthLimit.Int64(),
		Segment:    *p.SegmentLimit,
		RateLimit:  p.RateLimit,
		BurstLimit: p.BurstLimit,
	}
}

func randUsageLimits() console.UsageLimits {
	return console.UsageLimits{Storage: rand.Int63(), Bandwidth: rand.Int63(), Segment: rand.Int63()}
}

func TestAccountBillingFreeze(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		usersDB := sat.DB.Console().Users()
		projectsDB := sat.DB.Console().Projects()
		service := console.NewAccountFreezeService(sat.DB.Console(), sat.API.Analytics.Service, sat.Config.Console.AccountFreeze)

		billingFreezeGracePeriod := int(sat.Config.Console.AccountFreeze.BillingFreezeGracePeriod.Hours() / 24)

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

		frozen, err := service.IsUserBillingFrozen(ctx, user.ID)
		require.NoError(t, err)
		require.False(t, frozen)

		require.NoError(t, service.ViolationFreezeUser(ctx, user.ID))
		// cannot billing freeze a violation frozen user.
		require.Error(t, service.BillingFreezeUser(ctx, user.ID))
		require.NoError(t, service.ViolationUnfreezeUser(ctx, user.ID))

		require.NoError(t, service.LegalFreezeUser(ctx, user.ID))
		// cannot billing freeze a legal-frozen user.
		require.Error(t, service.BillingFreezeUser(ctx, user.ID))
		require.NoError(t, service.LegalUnfreezeUser(ctx, user.ID))

		require.NoError(t, service.BillingFreezeUser(ctx, user.ID))

		user, err = usersDB.Get(ctx, user.ID)
		require.NoError(t, err)
		require.Zero(t, getUserLimits(user))

		proj, err = projectsDB.Get(ctx, proj.ID)
		require.NoError(t, err)
		require.Zero(t, getProjectLimits(proj))

		frozen, err = service.IsUserBillingFrozen(ctx, user.ID)
		require.NoError(t, err)
		require.True(t, frozen)

		freezes, err := service.GetAll(ctx, user.ID)
		require.NoError(t, err)
		require.NotNil(t, freezes.BillingFreeze)
		require.Equal(t, &billingFreezeGracePeriod, freezes.BillingFreeze.DaysTillEscalation)

		err = service.EscalateBillingFreeze(ctx, user.ID, *freezes.BillingFreeze)
		require.NoError(t, err)

		freezes, err = service.GetAll(ctx, user.ID)
		require.NoError(t, err)
		require.NotNil(t, freezes.BillingFreeze)
		require.Nil(t, freezes.BillingFreeze.DaysTillEscalation)

		user, err = usersDB.Get(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, console.PendingDeletion, user.Status)
	})
}

func TestAccountBillingUnFreeze(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		usersDB := sat.DB.Console().Users()
		projectsDB := sat.DB.Console().Projects()
		service := console.NewAccountFreezeService(sat.DB.Console(), sat.API.Analytics.Service, sat.Config.Console.AccountFreeze)

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

		require.NoError(t, service.BillingFreezeUser(ctx, user.ID))

		status := console.PendingDeletion
		err = usersDB.Update(ctx, user.ID, console.UpdateUserRequest{
			Status: &status,
		})
		require.NoError(t, err)
		user, err = usersDB.Get(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, status, user.Status)

		require.NoError(t, service.BillingUnfreezeUser(ctx, user.ID))
		user, err = usersDB.Get(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, console.Active, user.Status)

		user, err = usersDB.Get(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, userLimits, getUserLimits(user))

		proj, err = projectsDB.Get(ctx, proj.ID)
		require.NoError(t, err)
		require.Equal(t, projLimits, getProjectLimits(proj))

		frozen, err := service.IsUserBillingFrozen(ctx, user.ID)
		require.NoError(t, err)
		require.False(t, frozen)
	})
}

func TestAccountViolationFreeze(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		usersDB := sat.DB.Console().Users()
		projectsDB := sat.DB.Console().Projects()
		service := console.NewAccountFreezeService(sat.DB.Console(), sat.API.Analytics.Service, sat.Config.Console.AccountFreeze)

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

		checkLimits := func(testT *testing.T) {
			user, err = usersDB.Get(ctx, user.ID)
			require.NoError(t, err)
			require.Zero(t, getUserLimits(user))

			proj, err = projectsDB.Get(ctx, proj.ID)
			require.NoError(t, err)
			require.Zero(t, getProjectLimits(proj))
		}

		frozen, err := service.IsUserViolationFrozen(ctx, user.ID)
		require.NoError(t, err)
		require.False(t, frozen)

		require.NoError(t, service.ViolationFreezeUser(ctx, user.ID))
		frozen, err = service.IsUserViolationFrozen(ctx, user.ID)
		require.NoError(t, err)
		require.True(t, frozen)

		user, err = usersDB.Get(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, console.PendingDeletion, user.Status)

		checkLimits(t)

		require.NoError(t, service.ViolationUnfreezeUser(ctx, user.ID))
		frozen, err = service.IsUserViolationFrozen(ctx, user.ID)
		require.NoError(t, err)
		require.False(t, frozen)

		require.NoError(t, service.BillingWarnUser(ctx, user.ID))
		frozen, err = service.IsUserViolationFrozen(ctx, user.ID)
		require.NoError(t, err)
		require.False(t, frozen)
		// violation freezing a warned user should be possible.
		require.NoError(t, service.ViolationFreezeUser(ctx, user.ID))
		frozen, err = service.IsUserViolationFrozen(ctx, user.ID)
		require.NoError(t, err)
		require.True(t, frozen)
		require.NoError(t, service.ViolationUnfreezeUser(ctx, user.ID))

		require.NoError(t, service.BillingFreezeUser(ctx, user.ID))
		frozen, err = service.IsUserViolationFrozen(ctx, user.ID)
		require.NoError(t, err)
		require.False(t, frozen)
		// violation freezing a billing frozen user should be possible.
		require.NoError(t, service.ViolationFreezeUser(ctx, user.ID))
		frozen, err = service.IsUserViolationFrozen(ctx, user.ID)
		require.NoError(t, err)
		require.True(t, frozen)

		freezes, err := service.GetAll(ctx, user.ID)
		require.NoError(t, err)
		require.NotNil(t, freezes.ViolationFreeze)
		require.Nil(t, freezes.ViolationFreeze.DaysTillEscalation)

		checkLimits(t)
	})
}

func TestAccountLegalFreeze(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		usersDB := sat.DB.Console().Users()
		projectsDB := sat.DB.Console().Projects()
		service := console.NewAccountFreezeService(sat.DB.Console(), sat.API.Analytics.Service, sat.Config.Console.AccountFreeze)

		userLimits := randUsageLimits()
		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "user@mail.test",
		}, 2)
		require.NoError(t, err)
		require.NoError(t, usersDB.UpdateUserProjectLimits(ctx, user.ID, userLimits))

		projLimits := randUsageLimits()
		rateLimit := 20000
		projLimits.RateLimit = &rateLimit
		projLimits.BurstLimit = &rateLimit
		proj, err := sat.AddProject(ctx, user.ID, "")
		require.NoError(t, err)
		require.NoError(t, projectsDB.UpdateUsageLimits(ctx, proj.ID, projLimits))
		require.NoError(t, projectsDB.UpdateRateLimit(ctx, proj.ID, projLimits.RateLimit))
		require.NoError(t, projectsDB.UpdateBurstLimit(ctx, proj.ID, projLimits.BurstLimit))

		checkLimits := func(t *testing.T) {
			user, err = usersDB.Get(ctx, user.ID)
			require.NoError(t, err)
			require.Zero(t, getUserLimits(user))

			proj, err = projectsDB.Get(ctx, proj.ID)
			require.NoError(t, err)
			usageLimits := getProjectLimits(proj)
			require.Zero(t, usageLimits.Segment)
			require.Zero(t, usageLimits.Storage)
			require.Zero(t, usageLimits.Bandwidth)
			zeroLimit := 0
			require.Equal(t, &zeroLimit, usageLimits.RateLimit)
			require.Equal(t, &zeroLimit, usageLimits.BurstLimit)
		}

		frozen, err := service.IsUserFrozen(ctx, user.ID, console.LegalFreeze)
		require.NoError(t, err)
		require.False(t, frozen)

		require.NoError(t, service.LegalFreezeUser(ctx, user.ID))
		frozen, err = service.IsUserFrozen(ctx, user.ID, console.LegalFreeze)
		require.NoError(t, err)
		require.True(t, frozen)

		user, err = usersDB.Get(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, console.LegalHold, user.Status)

		checkLimits(t)

		require.NoError(t, service.LegalUnfreezeUser(ctx, user.ID))
		frozen, err = service.IsUserFrozen(ctx, user.ID, console.LegalFreeze)
		require.NoError(t, err)
		require.False(t, frozen)

		require.NoError(t, service.BillingWarnUser(ctx, user.ID))
		frozen, err = service.IsUserFrozen(ctx, user.ID, console.LegalFreeze)
		require.NoError(t, err)
		require.False(t, frozen)
		// legal freezing a warned user should be possible.
		require.NoError(t, service.LegalFreezeUser(ctx, user.ID))
		frozen, err = service.IsUserFrozen(ctx, user.ID, console.LegalFreeze)
		require.NoError(t, err)
		require.True(t, frozen)
		require.NoError(t, service.LegalUnfreezeUser(ctx, user.ID))

		require.NoError(t, service.BillingFreezeUser(ctx, user.ID))
		frozen, err = service.IsUserBillingFrozen(ctx, user.ID)
		require.NoError(t, err)
		require.True(t, frozen)
		// legal freezing a billing frozen user should be possible.
		require.NoError(t, service.LegalFreezeUser(ctx, user.ID))
		frozen, err = service.IsUserFrozen(ctx, user.ID, console.LegalFreeze)
		require.NoError(t, err)
		require.True(t, frozen)
		require.NoError(t, service.LegalUnfreezeUser(ctx, user.ID))

		require.NoError(t, service.TrialExpirationFreezeUser(ctx, user.ID))
		// legal freezing a trial-expiration frozen user should be possible.
		require.NoError(t, service.LegalFreezeUser(ctx, user.ID))

		freezes, err := service.GetAll(ctx, user.ID)
		require.NoError(t, err)
		require.NotNil(t, freezes.LegalFreeze)
		require.Nil(t, freezes.LegalFreeze.DaysTillEscalation)

		checkLimits(t)

		require.NoError(t, service.LegalUnfreezeUser(ctx, user.ID))

		user, err = usersDB.Get(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, userLimits, getUserLimits(user))

		proj, err = projectsDB.Get(ctx, proj.ID)
		require.NoError(t, err)
		require.Equal(t, projLimits, getProjectLimits(proj))
	})
}

func TestRemoveAccountBillingWarning(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := console.NewAccountFreezeService(sat.DB.Console(), sat.API.Analytics.Service, sat.Config.Console.AccountFreeze)

		billingWarnGracePeriod := int(sat.Config.Console.AccountFreeze.BillingWarnGracePeriod.Hours() / 24)

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "user@mail.test",
		}, 2)
		require.NoError(t, err)

		require.NoError(t, service.BillingWarnUser(ctx, user.ID))
		require.NoError(t, service.BillingUnWarnUser(ctx, user.ID))

		freezes, err := service.GetAll(ctx, user.ID)
		require.NoError(t, err)
		require.NotNil(t, freezes)
		require.Nil(t, freezes.BillingWarning)
		require.Nil(t, freezes.BillingFreeze)
		require.Nil(t, freezes.ViolationFreeze)

		require.NoError(t, service.BillingWarnUser(ctx, user.ID))

		freezes, err = service.GetAll(ctx, user.ID)
		require.NoError(t, err)
		require.NotNil(t, freezes.BillingWarning)
		require.Equal(t, &billingWarnGracePeriod, freezes.BillingWarning.DaysTillEscalation)
		require.Nil(t, freezes.BillingFreeze)
		require.Nil(t, freezes.ViolationFreeze)
		require.NoError(t, service.BillingFreezeUser(ctx, user.ID))

		freezes, err = service.GetAll(ctx, user.ID)
		require.NoError(t, err)
		require.NotNil(t, freezes.BillingFreeze)
		require.Nil(t, freezes.ViolationFreeze)
		// billing-freezing should remove prior warning events.
		require.Nil(t, freezes.BillingWarning)

		// cannot warn a billing-frozen user.
		require.Error(t, service.BillingWarnUser(ctx, user.ID))
		require.NoError(t, service.BillingUnfreezeUser(ctx, user.ID))

		require.NoError(t, service.LegalFreezeUser(ctx, user.ID))
		// cannot warn a legal-frozen user.
		require.Error(t, service.BillingWarnUser(ctx, user.ID))
		require.NoError(t, service.LegalUnfreezeUser(ctx, user.ID))

		require.NoError(t, service.BillingWarnUser(ctx, user.ID))
		require.NoError(t, service.ViolationFreezeUser(ctx, user.ID))
		// cannot warn a violation-frozen user.
		require.Error(t, service.BillingWarnUser(ctx, user.ID))

		freezes, err = service.GetAll(ctx, user.ID)
		require.NoError(t, err)
		require.NotNil(t, freezes.ViolationFreeze)
		require.Nil(t, freezes.BillingFreeze)
		// billing-freezing should remove prior warning events.
		require.Nil(t, freezes.BillingWarning)
	})
}

func TestAccountFreezeAlreadyFrozen(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		usersDB := sat.DB.Console().Users()
		projectsDB := sat.DB.Console().Projects()
		service := console.NewAccountFreezeService(sat.DB.Console(), sat.API.Analytics.Service, sat.Config.Console.AccountFreeze)

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
			require.NoError(t, service.BillingFreezeUser(ctx, user.ID))

			proj2Limits := randUsageLimits()
			proj2, err := sat.AddProject(ctx, user.ID, "project2")
			require.NoError(t, err)
			require.NoError(t, projectsDB.UpdateUsageLimits(ctx, proj2.ID, proj2Limits))

			require.NoError(t, service.BillingFreezeUser(ctx, user.ID))

			user, err := usersDB.Get(ctx, user.ID)
			require.NoError(t, err)
			require.Zero(t, getUserLimits(user))

			proj2, err = projectsDB.Get(ctx, proj2.ID)
			require.NoError(t, err)
			require.Zero(t, getProjectLimits(proj2))

			require.NoError(t, service.BillingUnfreezeUser(ctx, user.ID))

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

			require.NoError(t, service.BillingFreezeUser(ctx, user.ID))
			require.NoError(t, usersDB.UpdateUserProjectLimits(ctx, user.ID, userLimits))
			require.NoError(t, service.BillingFreezeUser(ctx, user.ID))

			user, err = usersDB.Get(ctx, user.ID)
			require.NoError(t, err)
			require.Zero(t, getUserLimits(user))

			require.NoError(t, service.BillingUnfreezeUser(ctx, user.ID))

			user, err = usersDB.Get(ctx, user.ID)
			require.NoError(t, err)
			require.Equal(t, userLimits, getUserLimits(user))
		})

		// Freezing a frozen user should not modify user limits stored by the prior freeze.
		t.Run("Frozen user limits", func(t *testing.T) {
			require.NoError(t, service.BillingFreezeUser(ctx, user.ID))
			require.NoError(t, service.BillingFreezeUser(ctx, user.ID))

			user, err = usersDB.Get(ctx, user.ID)
			require.NoError(t, err)
			require.Zero(t, getUserLimits(user))

			require.NoError(t, service.BillingUnfreezeUser(ctx, user.ID))
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
				// disable limit caching
				config.Metainfo.RateLimiter.CacheCapacity = 0
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		consoleService := sat.API.Console.Service
		freezeService := console.NewAccountFreezeService(sat.DB.Console(), sat.API.Analytics.Service, sat.Config.Console.AccountFreeze)

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

		shouldNotUploadAndDownload := func(testT *testing.T) {
			// Should not be able to upload because account is frozen.
			err = uplink1.Upload(ctx, sat, bucketName, path, expectedData)
			require.Error(testT, err)

			// Should not be able to download because account is frozen.
			_, err = uplink1.Download(ctx, sat, bucketName, path)
			require.Error(testT, err)

			// Should not be able to create bucket because account is frozen.
			err = uplink1.CreateBucket(ctx, sat, "anotherBucket")
			require.Error(testT, err)
		}

		shouldListAndDelete := func(testT *testing.T) {
			// Should be able to list even if frozen.
			_, err := uplink1.ListObjects(ctx, sat, bucketName)
			require.NoError(testT, err)

			// Should be able to delete even if frozen.
			err = uplink1.DeleteObject(ctx, sat, bucketName, path)
			require.NoError(testT, err)
		}

		shouldNotListAndDelete := func(testT *testing.T) {
			// Should not be able to list.
			_, err := uplink1.ListObjects(ctx, sat, bucketName)
			require.Error(testT, err)
			require.ErrorIs(testT, err, uplink.ErrPermissionDenied)

			// Should not be able to delete.
			err = uplink1.DeleteObject(ctx, sat, bucketName, path)
			require.Error(testT, err)
			require.ErrorIs(testT, err, uplink.ErrPermissionDenied)
		}

		t.Run("BillingFreeze effect on project owner", func(t *testing.T) {
			shouldUploadAndDownload(t)
			shouldListAndDelete(t)

			require.NoError(t, freezeService.BillingWarnUser(ctx, user1.ID))

			// Should be able to download and list because account is not frozen.
			shouldUploadAndDownload(t)
			shouldListAndDelete(t)

			require.NoError(t, freezeService.BillingFreezeUser(ctx, user1.ID))

			shouldNotUploadAndDownload(t)
			shouldListAndDelete(t)

			require.NoError(t, freezeService.BillingUnfreezeUser(ctx, user1.ID))

			shouldUploadAndDownload(t)
		})

		t.Run("ViolationFreeze effect on project owner", func(t *testing.T) {
			shouldUploadAndDownload(t)
			shouldListAndDelete(t)

			require.NoError(t, freezeService.ViolationFreezeUser(ctx, user1.ID))

			shouldNotUploadAndDownload(t)
			shouldListAndDelete(t)

			require.NoError(t, freezeService.ViolationUnfreezeUser(ctx, user1.ID))

			shouldUploadAndDownload(t)
		})

		t.Run("LegalFreeze effect on project owner", func(t *testing.T) {
			shouldUploadAndDownload(t)
			shouldListAndDelete(t)

			require.NoError(t, freezeService.LegalFreezeUser(ctx, user1.ID))

			shouldNotUploadAndDownload(t)
			shouldNotListAndDelete(t)

			require.NoError(t, freezeService.LegalUnfreezeUser(ctx, user1.ID))

			shouldListAndDelete(t)
			shouldUploadAndDownload(t)
		})
	})
}

func TestAccountBotFreezeUnfreeze(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		usersDB := sat.DB.Console().Users()
		projectsDB := sat.DB.Console().Projects()
		accFreezeDB := sat.DB.Console().AccountFreezeEvents()
		service := console.NewAccountFreezeService(sat.DB.Console(), sat.API.Analytics.Service, sat.Config.Console.AccountFreeze)

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test Bot User",
			Email:    "botuser@mail.test",
		}, 2)
		require.NoError(t, err)

		_, err = sat.AddProject(ctx, user.ID, "test")
		require.NoError(t, err)

		_, err = sat.AddProject(ctx, user.ID, "test1")
		require.NoError(t, err)

		_, err = accFreezeDB.Upsert(ctx, &console.AccountFreezeEvent{
			UserID: user.ID,
			Type:   console.DelayedBotFreeze,
		})
		require.NoError(t, err)

		user, err = usersDB.Get(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, console.Active, user.Status)
		require.NotZero(t, user.ProjectBandwidthLimit)
		require.NotZero(t, user.ProjectStorageLimit)
		require.NotZero(t, user.ProjectSegmentLimit)

		require.NoError(t, service.BotFreezeUser(ctx, user.ID))

		user, err = usersDB.Get(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, console.PendingBotVerification, user.Status)
		require.Zero(t, user.ProjectBandwidthLimit)
		require.Zero(t, user.ProjectStorageLimit)
		require.Zero(t, user.ProjectSegmentLimit)

		projects, err := projectsDB.GetOwn(ctx, user.ID)
		require.NoError(t, err)
		require.Len(t, projects, 2)

		for _, p := range projects {
			require.Zero(t, *p.BandwidthLimit)
			require.Zero(t, *p.StorageLimit)
			require.Zero(t, *p.SegmentLimit)
			require.Zero(t, *p.RateLimit)
			require.Zero(t, *p.BurstLimit)
		}

		event, err := accFreezeDB.Get(ctx, user.ID, console.DelayedBotFreeze)
		require.Error(t, err)
		require.True(t, errs.Is(err, sql.ErrNoRows))
		require.Nil(t, event)

		event, err = accFreezeDB.Get(ctx, user.ID, console.BotFreeze)
		require.NoError(t, err)
		require.NotNil(t, event)

		require.NoError(t, service.BotUnfreezeUser(ctx, user.ID))

		user, err = usersDB.Get(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, console.Active, user.Status)
		require.NotZero(t, user.ProjectBandwidthLimit)
		require.NotZero(t, user.ProjectStorageLimit)
		require.NotZero(t, user.ProjectSegmentLimit)

		projects, err = sat.DB.Console().Projects().GetOwn(ctx, user.ID)
		require.NoError(t, err)
		require.Len(t, projects, 2)

		for _, p := range projects {
			require.NotZero(t, *p.BandwidthLimit)
			require.NotZero(t, *p.StorageLimit)
			require.NotZero(t, *p.SegmentLimit)
		}

		event, err = accFreezeDB.Get(ctx, user.ID, console.BotFreeze)
		require.Error(t, err)
		require.True(t, errs.Is(err, sql.ErrNoRows))
		require.Nil(t, event)

		require.NoError(t, service.TrialExpirationFreezeUser(ctx, user.ID))
		// bot freezing a trial-expiration frozen user should be possible.
		require.NoError(t, service.BotFreezeUser(ctx, user.ID))
	})
}

func TestTrailExpirationFreeze(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		usersDB := sat.DB.Console().Users()
		projectsDB := sat.DB.Console().Projects()
		service := console.NewAccountFreezeService(sat.DB.Console(), sat.API.Analytics.Service, sat.Config.Console.AccountFreeze)

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

		frozen, err := service.IsUserFrozen(ctx, user.ID, console.TrialExpirationFreeze)
		require.NoError(t, err)
		require.False(t, frozen)

		require.NoError(t, service.ViolationFreezeUser(ctx, user.ID))
		// cannot trial-expiration freeze a violation frozen user.
		require.Error(t, service.TrialExpirationFreezeUser(ctx, user.ID))
		require.NoError(t, service.ViolationUnfreezeUser(ctx, user.ID))

		require.NoError(t, service.LegalFreezeUser(ctx, user.ID))
		// cannot trial-expiration freeze a legal-frozen user.
		require.Error(t, service.TrialExpirationFreezeUser(ctx, user.ID))
		require.NoError(t, service.LegalUnfreezeUser(ctx, user.ID))

		require.NoError(t, service.BotFreezeUser(ctx, user.ID))
		// cannot trial-expiration freeze a bot-frozen user.
		require.Error(t, service.TrialExpirationFreezeUser(ctx, user.ID))
		require.NoError(t, service.BotUnfreezeUser(ctx, user.ID))

		require.NoError(t, service.TrialExpirationFreezeUser(ctx, user.ID))

		user, err = usersDB.Get(ctx, user.ID)
		require.NoError(t, err)
		require.Zero(t, getUserLimits(user))

		proj, err = projectsDB.Get(ctx, proj.ID)
		require.NoError(t, err)
		usageLimits := getProjectLimits(proj)
		require.Zero(t, usageLimits.Segment)
		require.Zero(t, usageLimits.Storage)
		require.Zero(t, usageLimits.Bandwidth)
		zeroLimit := 0
		require.Equal(t, &zeroLimit, usageLimits.RateLimit)
		require.Equal(t, &zeroLimit, usageLimits.BurstLimit)

		frozen, err = service.IsUserFrozen(ctx, user.ID, console.TrialExpirationFreeze)
		require.NoError(t, err)
		require.True(t, frozen)

		freezes, err := service.GetAll(ctx, user.ID)
		require.NoError(t, err)
		require.NotNil(t, freezes.TrialExpirationFreeze)

		require.NoError(t, service.TrialExpirationUnfreezeUser(ctx, user.ID))

		user, err = usersDB.Get(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, userLimits, getUserLimits(user))

		proj, err = projectsDB.Get(ctx, proj.ID)
		require.NoError(t, err)
		require.Equal(t, projLimits, getProjectLimits(proj))
	})
}

func TestGetAllEvents(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		freezeDB := sat.DB.Console().AccountFreezeEvents()

		freezeTypes := []console.AccountFreezeEventType{
			console.BillingFreeze,
			console.BillingWarning,
			console.ViolationFreeze,
			console.LegalFreeze,
			console.TrialExpirationFreeze,
			console.BotFreeze,
			console.DelayedBotFreeze,
		}
		for _, freezeType := range freezeTypes {
			user, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Test User",
				Email:    fmt.Sprintf("%duser@mail.test", freezeType),
			}, 2)
			require.NoError(t, err)
			_, err = freezeDB.Upsert(ctx, &console.AccountFreezeEvent{
				UserID: user.ID,
				Type:   freezeType,
			})
			require.NoError(t, err)
		}

		cursor := console.FreezeEventsCursor{Limit: 10}
		eventPage, err := freezeDB.GetAllEvents(ctx, cursor, nil)
		require.NoError(t, err)
		require.Len(t, eventPage.Events, len(freezeTypes))

		eventPage, err = freezeDB.GetAllEvents(ctx, cursor, freezeTypes)
		require.NoError(t, err)
		require.Len(t, eventPage.Events, len(freezeTypes))

		eventPage, err = freezeDB.GetAllEvents(ctx, cursor, []console.AccountFreezeEventType{console.BillingFreeze})
		require.NoError(t, err)
		require.Len(t, eventPage.Events, 1)
		require.Equal(t, console.BillingFreeze, eventPage.Events[0].Type)

		eventPage, err = freezeDB.GetAllEvents(ctx, cursor, []console.AccountFreezeEventType{console.BillingFreeze, console.LegalFreeze})
		require.NoError(t, err)
		require.Len(t, eventPage.Events, 2)
	})
}
