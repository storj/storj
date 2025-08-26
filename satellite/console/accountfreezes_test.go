// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package console_test

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/uplink"
)

var (
	zero            = 0
	zeroUsageLimits = console.UsageLimits{
		Storage:          int64(zero),
		Bandwidth:        int64(zero),
		Segment:          int64(zero),
		RateLimit:        &zero,
		RateLimitHead:    &zero,
		RateLimitGet:     &zero,
		RateLimitPut:     &zero,
		RateLimitList:    &zero,
		RateLimitDelete:  &zero,
		BurstLimit:       &zero,
		BurstLimitHead:   &zero,
		BurstLimitGet:    &zero,
		BurstLimitPut:    &zero,
		BurstLimitList:   &zero,
		BurstLimitDelete: &zero,
	}
)

func getUserLimits(u *console.User) console.UsageLimits {
	return console.UsageLimits{
		Storage:   u.ProjectStorageLimit,
		Bandwidth: u.ProjectBandwidthLimit,
		Segment:   u.ProjectSegmentLimit,
	}
}

func getProjectLimits(p *console.Project) console.UsageLimits {
	limits := console.UsageLimits{
		Storage:          p.StorageLimit.Int64(),
		Bandwidth:        p.BandwidthLimit.Int64(),
		Segment:          *p.SegmentLimit,
		RateLimit:        p.RateLimit,
		RateLimitHead:    p.RateLimitHead,
		RateLimitGet:     p.RateLimitGet,
		RateLimitList:    p.RateLimitList,
		RateLimitPut:     p.RateLimitPut,
		RateLimitDelete:  p.RateLimitDelete,
		BurstLimit:       p.BurstLimit,
		BurstLimitHead:   p.BurstLimitHead,
		BurstLimitGet:    p.BurstLimitGet,
		BurstLimitList:   p.BurstLimitList,
		BurstLimitPut:    p.BurstLimitPut,
		BurstLimitDelete: p.BurstLimitDelete,
	}
	if p.UserSpecifiedBandwidthLimit != nil {
		value := p.UserSpecifiedBandwidthLimit.Int64()
		limits.UserSetBandwidthLimit = &value
	}
	if p.UserSpecifiedStorageLimit != nil {
		value := p.UserSpecifiedStorageLimit.Int64()
		limits.UserSetStorageLimit = &value
	}
	return limits
}

func randUsageLimits(forProject bool) console.UsageLimits {
	usageLimits := console.UsageLimits{
		Storage:   rand.Int63() + 1,
		Bandwidth: rand.Int63() + 1,
		Segment:   rand.Int63() + 1,
	}
	if forProject {
		usageLimits.UserSetBandwidthLimit = &usageLimits.Bandwidth
		usageLimits.UserSetStorageLimit = &usageLimits.Storage

		rate, burst := rand.Intn(100)+1, rand.Intn(100)+1
		usageLimits.RateLimit = &rate
		usageLimits.RateLimitHead = &rate
		usageLimits.RateLimitGet = &rate
		usageLimits.RateLimitList = &rate
		usageLimits.RateLimitPut = &rate
		usageLimits.RateLimitDelete = &rate
		usageLimits.BurstLimit = &burst
		usageLimits.BurstLimitHead = &burst
		usageLimits.BurstLimitGet = &burst
		usageLimits.BurstLimitList = &burst
		usageLimits.BurstLimitPut = &burst
		usageLimits.BurstLimitDelete = &burst
	}
	return usageLimits
}

func updateProjectLimits(ctx context.Context, db console.Projects, p *console.Project, limits console.UsageLimits) error {
	limitUpdates := []console.Limit{
		{Kind: console.BandwidthLimit, Value: &limits.Bandwidth},
		{Kind: console.UserSetBandwidthLimit, Value: limits.UserSetBandwidthLimit},
		{Kind: console.StorageLimit, Value: &limits.Storage},
		{Kind: console.UserSetStorageLimit, Value: limits.UserSetStorageLimit},
		{Kind: console.SegmentLimit, Value: &limits.Segment},
	}
	if limits.RateLimit != nil && limits.BurstLimit != nil {
		toInt64Ptr := func(i *int) *int64 {
			if i == nil {
				return nil
			}
			v := int64(*i)
			return &v
		}
		limitUpdates = append(limitUpdates, console.Limit{Kind: console.RateLimit, Value: toInt64Ptr(limits.RateLimit)})
		limitUpdates = append(limitUpdates, console.Limit{Kind: console.RateLimitGet, Value: toInt64Ptr(limits.RateLimitGet)})
		limitUpdates = append(limitUpdates, console.Limit{Kind: console.RateLimitDelete, Value: toInt64Ptr(limits.RateLimitDelete)})
		limitUpdates = append(limitUpdates, console.Limit{Kind: console.RateLimitHead, Value: toInt64Ptr(limits.RateLimitHead)})
		limitUpdates = append(limitUpdates, console.Limit{Kind: console.RateLimitList, Value: toInt64Ptr(limits.RateLimitList)})
		limitUpdates = append(limitUpdates, console.Limit{Kind: console.RateLimitPut, Value: toInt64Ptr(limits.RateLimitPut)})

		limitUpdates = append(limitUpdates, console.Limit{Kind: console.BurstLimit, Value: toInt64Ptr(limits.BurstLimit)})
		limitUpdates = append(limitUpdates, console.Limit{Kind: console.BurstLimitGet, Value: toInt64Ptr(limits.BurstLimitGet)})
		limitUpdates = append(limitUpdates, console.Limit{Kind: console.BurstLimitHead, Value: toInt64Ptr(limits.BurstLimitHead)})
		limitUpdates = append(limitUpdates, console.Limit{Kind: console.BurstLimitDelete, Value: toInt64Ptr(limits.BurstLimitDelete)})
		limitUpdates = append(limitUpdates, console.Limit{Kind: console.BurstLimitPut, Value: toInt64Ptr(limits.BurstLimitPut)})
		limitUpdates = append(limitUpdates, console.Limit{Kind: console.BurstLimitList, Value: toInt64Ptr(limits.BurstLimitList)})
	}
	err := db.UpdateLimitsGeneric(ctx, p.ID, limitUpdates)
	if err != nil {
		return err
	}

	return nil
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

		userLimits := randUsageLimits(false)
		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "user@mail.test",
			Kind:     console.PaidUser,
		}, 2)
		require.NoError(t, err)
		require.NoError(t, usersDB.UpdateUserProjectLimits(ctx, user.ID, userLimits))

		projLimits := randUsageLimits(true)
		proj, err := sat.AddProject(ctx, user.ID, "test project")
		require.NoError(t, err)
		require.NoError(t, updateProjectLimits(ctx, projectsDB, proj, projLimits))

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

		// Test automatic billing freeze
		require.NoError(t, service.BillingFreezeUser(ctx, user.ID))

		user, err = usersDB.Get(ctx, user.ID)
		require.NoError(t, err)
		require.Zero(t, getUserLimits(user))

		proj, err = projectsDB.Get(ctx, proj.ID)
		require.NoError(t, err)
		// segment, bandwidth and storage limits should be zeroed out.
		require.NotNil(t, proj.BandwidthLimit)
		require.Zero(t, proj.BandwidthLimit.Int64())
		require.Nil(t, proj.UserSpecifiedBandwidthLimit)
		require.NotNil(t, proj.StorageLimit)
		require.Zero(t, proj.StorageLimit.Int64())
		require.Nil(t, proj.UserSpecifiedStorageLimit)
		require.NotNil(t, proj.SegmentLimit)
		require.Zero(t, *proj.SegmentLimit)
		// rate and burst limits should not be zeroed out.
		require.NotNil(t, proj.RateLimit)
		require.NotZero(t, *proj.RateLimit)
		require.NotNil(t, proj.BurstLimit)
		require.NotZero(t, *proj.BurstLimit)

		frozen, err = service.IsUserBillingFrozen(ctx, user.ID)
		require.NoError(t, err)
		require.True(t, frozen)

		require.NoError(t, service.BillingUnfreezeUser(ctx, user.ID))
		frozen, err = service.IsUserBillingFrozen(ctx, user.ID)
		require.NoError(t, err)
		require.False(t, frozen)

		// Test admin billing freeze
		require.NoError(t, service.AdminBillingFreezeUser(ctx, user.ID))

		freezes, err := service.GetAll(ctx, user.ID)
		require.NoError(t, err)
		require.NotNil(t, freezes.BillingFreeze)
		require.Equal(t, &billingFreezeGracePeriod, freezes.BillingFreeze.DaysTillEscalation)

		err = service.EscalateFreezeEvent(ctx, user.ID, *freezes.BillingFreeze)
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

		userLimits := randUsageLimits(false)
		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "user@mail.test",
		}, 2)
		require.NoError(t, err)
		require.NoError(t, usersDB.UpdateUserProjectLimits(ctx, user.ID, userLimits))

		projLimits := randUsageLimits(true)
		proj, err := sat.AddProject(ctx, user.ID, "test project")
		require.NoError(t, err)
		require.NoError(t, updateProjectLimits(ctx, projectsDB, proj, projLimits))

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

		userLimits := randUsageLimits(false)
		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "user@mail.test",
		}, 2)
		require.NoError(t, err)
		require.NoError(t, usersDB.UpdateUserProjectLimits(ctx, user.ID, userLimits))

		projLimits := randUsageLimits(true)
		proj, err := sat.AddProject(ctx, user.ID, "test project")
		require.NoError(t, err)
		require.NoError(t, updateProjectLimits(ctx, projectsDB, proj, projLimits))

		checkLimits := func(testT *testing.T) {
			user, err = usersDB.Get(ctx, user.ID)
			require.NoError(t, err)
			require.Zero(t, getUserLimits(user))

			proj, err = projectsDB.Get(ctx, proj.ID)
			require.NoError(t, err)
			proj, err = projectsDB.Get(ctx, proj.ID)
			require.NoError(t, err)
			// segment, bandwidth and storage limits should be zeroed out.
			require.NotNil(t, proj.BandwidthLimit)
			require.Zero(t, proj.BandwidthLimit.Int64())
			require.Nil(t, proj.UserSpecifiedBandwidthLimit)
			require.NotNil(t, proj.StorageLimit)
			require.Zero(t, proj.StorageLimit.Int64())
			require.Nil(t, proj.UserSpecifiedStorageLimit)
			require.NotNil(t, proj.SegmentLimit)
			require.Zero(t, *proj.SegmentLimit)
			// rate and burst limits should not be zeroed out.
			require.NotNil(t, proj.RateLimit)
			require.NotZero(t, *proj.RateLimit)
			require.NotNil(t, proj.BurstLimit)
			require.NotZero(t, *proj.BurstLimit)
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

		user, err = usersDB.Get(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, userLimits, getUserLimits(user))

		proj, err = projectsDB.Get(ctx, proj.ID)
		require.NoError(t, err)
		require.Equal(t, projLimits, getProjectLimits(proj))

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
		checkLimits(t)

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

		userLimits := randUsageLimits(false)
		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "user@mail.test",
		}, 2)
		require.NoError(t, err)
		require.NoError(t, usersDB.UpdateUserProjectLimits(ctx, user.ID, userLimits))

		projLimits := randUsageLimits(true)
		rateLimit := 20000
		projLimits.RateLimit = &rateLimit
		projLimits.BurstLimit = &rateLimit
		proj, err := sat.AddProject(ctx, user.ID, "test project")
		require.NoError(t, err)
		require.NoError(t, updateProjectLimits(ctx, projectsDB, proj, projLimits))

		checkLimits := func(t *testing.T) {
			user, err = usersDB.Get(ctx, user.ID)
			require.NoError(t, err)
			require.Zero(t, getUserLimits(user))

			proj, err = projectsDB.Get(ctx, proj.ID)
			require.NoError(t, err)
			require.EqualValues(t, zeroUsageLimits, getProjectLimits(proj))
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

		user, err = usersDB.Get(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, userLimits, getUserLimits(user))

		proj, err = projectsDB.Get(ctx, proj.ID)
		require.NoError(t, err)
		require.Equal(t, projLimits, getProjectLimits(proj))

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

		currentProjLimits := getProjectLimits(proj)
		expectedHeadListDeleteRateLimits := int(sat.Config.Console.AccountFreeze.TrialExpirationRateLimits)
		require.Equal(t, projLimits.RateLimit, currentProjLimits.RateLimit)
		require.Equal(t, projLimits.RateLimitPut, currentProjLimits.RateLimitPut)
		require.Equal(t, projLimits.RateLimitGet, currentProjLimits.RateLimitGet)
		require.Equal(t, expectedHeadListDeleteRateLimits, *currentProjLimits.RateLimitHead)
		require.Equal(t, expectedHeadListDeleteRateLimits, *currentProjLimits.RateLimitList)
		require.Equal(t, expectedHeadListDeleteRateLimits, *currentProjLimits.RateLimitDelete)
		require.Equal(t, projLimits.BurstLimit, currentProjLimits.BurstLimit)
		require.Equal(t, projLimits.BurstLimitPut, currentProjLimits.BurstLimitPut)
		require.Equal(t, projLimits.BurstLimitGet, currentProjLimits.BurstLimitGet)
		require.Equal(t, expectedHeadListDeleteRateLimits, *currentProjLimits.BurstLimitHead)
		require.Equal(t, expectedHeadListDeleteRateLimits, *currentProjLimits.BurstLimitList)
		require.Equal(t, expectedHeadListDeleteRateLimits, *currentProjLimits.BurstLimitDelete)
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

		userLimits := randUsageLimits(false)
		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "user@mail.test",
		}, 2)
		require.NoError(t, err)
		require.NoError(t, usersDB.UpdateUserProjectLimits(ctx, user.ID, userLimits))

		proj1Limits := randUsageLimits(true)
		proj1, err := sat.AddProject(ctx, user.ID, "project1")
		require.NoError(t, err)
		require.NoError(t, updateProjectLimits(ctx, projectsDB, proj1, proj1Limits))

		// Freezing a frozen user should freeze any projects that were unable to be frozen prior.
		// The limits stored for projects frozen by the prior freeze should not be modified.
		t.Run("Project limits", func(t *testing.T) {
			require.NoError(t, service.BillingFreezeUser(ctx, user.ID))

			proj2Limits := randUsageLimits(true)
			proj2, err := sat.AddProject(ctx, user.ID, "project2")
			require.NoError(t, err)
			require.NoError(t, updateProjectLimits(ctx, projectsDB, proj2, proj2Limits))

			require.NoError(t, service.BillingFreezeUser(ctx, user.ID))

			user, err := usersDB.Get(ctx, user.ID)
			require.NoError(t, err)
			require.Zero(t, getUserLimits(user))

			proj2, err = projectsDB.Get(ctx, proj2.ID)
			require.NoError(t, err)
			// segment, bandwidth and storage limits should be zeroed out.
			require.NotNil(t, proj2.BandwidthLimit)
			require.Zero(t, proj2.BandwidthLimit.Int64())
			require.Nil(t, proj2.UserSpecifiedBandwidthLimit)
			require.NotNil(t, proj2.StorageLimit)
			require.Zero(t, proj2.StorageLimit.Int64())
			require.Nil(t, proj2.UserSpecifiedStorageLimit)
			require.NotNil(t, proj2.SegmentLimit)
			require.Zero(t, *proj2.SegmentLimit)
			// rate and burst limits should not be zeroed out.
			require.NotNil(t, proj2.RateLimit)
			require.NotZero(t, *proj2.RateLimit)
			require.NotNil(t, proj2.BurstLimit)
			require.NotZero(t, *proj2.BurstLimit)

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
			SatelliteDBOptions: testplanet.SatelliteDBDisableCaches,
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		consoleService := sat.API.Console.Service
		freezeService := console.NewAccountFreezeService(sat.DB.Console(), sat.API.Analytics.Service, sat.Config.Console.AccountFreeze)

		uplink1 := planet.Uplinks[0]
		user1, _, err := consoleService.GetUserByEmailWithUnverified(ctx, uplink1.User[sat.ID()].Email)
		require.NoError(t, err)

		userCtx, err := sat.UserContext(ctx, user1.ID)
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

			// can not update limits when frozen.
			someSize := 1 * memory.KB
			err = consoleService.UpdateUserSpecifiedLimits(userCtx, uplink1.Projects[0].ID, console.UpdateLimitsInfo{
				StorageLimit:   &someSize,
				BandwidthLimit: &someSize,
			})
			require.Error(t, err)

			shouldNotUploadAndDownload(t)
			shouldListAndDelete(t)

			require.NoError(t, freezeService.BillingUnfreezeUser(ctx, user1.ID))

			shouldUploadAndDownload(t)
		})

		t.Run("ViolationFreeze effect on project owner", func(t *testing.T) {
			shouldUploadAndDownload(t)
			shouldListAndDelete(t)

			require.NoError(t, freezeService.ViolationFreezeUser(ctx, user1.ID))

			// can not update limits when frozen.
			someSize := 1 * memory.KB
			err = consoleService.UpdateUserSpecifiedLimits(userCtx, uplink1.Projects[0].ID, console.UpdateLimitsInfo{
				StorageLimit:   &someSize,
				BandwidthLimit: &someSize,
			})
			require.Error(t, err)

			shouldNotUploadAndDownload(t)
			shouldListAndDelete(t)

			require.NoError(t, freezeService.ViolationUnfreezeUser(ctx, user1.ID))

			shouldUploadAndDownload(t)
		})

		t.Run("LegalFreeze effect on project owner", func(t *testing.T) {
			shouldUploadAndDownload(t)
			shouldListAndDelete(t)

			require.NoError(t, freezeService.LegalFreezeUser(ctx, user1.ID))

			// can not update limits when frozen.
			someSize := 1 * memory.KB
			err = consoleService.UpdateUserSpecifiedLimits(userCtx, uplink1.Projects[0].ID, console.UpdateLimitsInfo{
				StorageLimit:   &someSize,
				BandwidthLimit: &someSize,
			})
			require.Error(t, err)

			shouldNotUploadAndDownload(t)
			shouldNotListAndDelete(t)

			require.NoError(t, freezeService.LegalUnfreezeUser(ctx, user1.ID))

			shouldListAndDelete(t)
			shouldUploadAndDownload(t)
		})
	})
}

func TestGetDaysTillEscalation(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 2, UplinkCount: 2,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.AccountFreeze.Enabled = true
				config.Console.AccountFreeze.BillingWarnGracePeriod = 384 * time.Hour
				config.Console.AccountFreeze.TrialExpirationFreezeGracePeriod = 384 * time.Hour
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		consoleService := sat.API.Console.Service
		freezeService := console.NewAccountFreezeService(sat.DB.Console(), sat.API.Analytics.Service, sat.Config.Console.AccountFreeze)

		uplink1 := planet.Uplinks[0]
		user1, _, err := consoleService.GetUserByEmailWithUnverified(ctx, uplink1.User[sat.ID()].Email)
		require.NoError(t, err)

		gracePeriod := sat.Config.Console.AccountFreeze.BillingWarnGracePeriod
		gracePeriodDays := int(gracePeriod.Hours() / 24)

		require.NoError(t, freezeService.BillingWarnUser(ctx, user1.ID))

		now := time.Now()

		freezes, err := freezeService.GetAll(ctx, user1.ID)
		require.NoError(t, err)
		require.NotNil(t, freezes.BillingWarning)
		require.Equal(t, gracePeriodDays, *freezes.BillingWarning.DaysTillEscalation)

		days := freezeService.GetDaysTillEscalation(*freezes.BillingWarning, now)
		require.NotNil(t, days)
		require.Equal(t, gracePeriodDays, *days)

		freezes.BillingWarning.DaysTillEscalation = nil
		freezes.BillingWarning, err = sat.DB.Console().AccountFreezeEvents().Upsert(ctx, freezes.BillingWarning)
		require.NoError(t, err)

		days = freezeService.GetDaysTillEscalation(*freezes.BillingWarning, now)
		require.Nil(t, days)

		gracePeriod = sat.Config.Console.AccountFreeze.TrialExpirationFreezeGracePeriod
		gracePeriodDays = int(gracePeriod.Hours() / 24)

		require.NoError(t, freezeService.TrialExpirationFreezeUser(ctx, user1.ID))

		now = time.Now()
		midFuture := now.Add(gracePeriod / 2)
		future := now.Add(gracePeriod).Add(time.Hour)

		freezes, err = freezeService.GetAll(ctx, user1.ID)
		require.NoError(t, err)
		require.NotNil(t, freezes.TrialExpirationFreeze)
		require.Equal(t, gracePeriodDays, *freezes.TrialExpirationFreeze.DaysTillEscalation)

		days = freezeService.GetDaysTillEscalation(*freezes.TrialExpirationFreeze, now)
		require.NotNil(t, days)
		require.Equal(t, gracePeriodDays, *days)

		days = freezeService.GetDaysTillEscalation(*freezes.TrialExpirationFreeze, midFuture)
		require.NotNil(t, days)
		require.InDelta(t, gracePeriodDays/2, *days, 1)

		days = freezeService.GetDaysTillEscalation(*freezes.TrialExpirationFreeze, future)
		require.NotNil(t, days)
		require.Equal(t, 0, *days)

		// Test for trial expiration frozen users with no days till escalation set.
		// This is for users frozen before this change.
		freezes.TrialExpirationFreeze.DaysTillEscalation = nil
		freezes.TrialExpirationFreeze, err = sat.DB.Console().AccountFreezeEvents().Upsert(ctx, freezes.TrialExpirationFreeze)
		require.NoError(t, err)

		days = freezeService.GetDaysTillEscalation(*freezes.TrialExpirationFreeze, now)
		require.NotNil(t, days)
		require.Equal(t, gracePeriodDays, *days)

		days = freezeService.GetDaysTillEscalation(*freezes.TrialExpirationFreeze, midFuture)
		require.NotNil(t, days)
		require.InDelta(t, gracePeriodDays/2, *days, 1)

		days = freezeService.GetDaysTillEscalation(*freezes.TrialExpirationFreeze, future)
		require.NotNil(t, days)
		require.Equal(t, 0, *days)
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

		pLimits := randUsageLimits(true)
		proj, err := sat.AddProject(ctx, user.ID, "test")
		require.NoError(t, err)
		require.NoError(t, updateProjectLimits(ctx, projectsDB, proj, pLimits))

		proj, err = sat.AddProject(ctx, user.ID, "test1")
		require.NoError(t, err)
		require.NoError(t, updateProjectLimits(ctx, projectsDB, proj, pLimits))

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
		require.Zero(t, getUserLimits(user))

		projects, err := projectsDB.GetOwn(ctx, user.ID)
		require.NoError(t, err)
		require.Len(t, projects, 2)

		for _, p := range projects {
			require.Equal(t, zeroUsageLimits, getProjectLimits(&p))
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
			require.Equal(t, pLimits, getProjectLimits(&p))
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

func TestTrialExpirationFreeze(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		usersDB := sat.DB.Console().Users()
		projectsDB := sat.DB.Console().Projects()
		service := console.NewAccountFreezeService(sat.DB.Console(), sat.API.Analytics.Service, sat.Config.Console.AccountFreeze)

		userLimits := randUsageLimits(false)
		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "user@mail.test",
		}, 2)
		require.NoError(t, err)
		require.NoError(t, usersDB.UpdateUserProjectLimits(ctx, user.ID, userLimits))

		projLimits := randUsageLimits(true)
		proj, err := sat.AddProject(ctx, user.ID, "test project")
		require.NoError(t, err)
		require.NoError(t, updateProjectLimits(ctx, projectsDB, proj, projLimits))

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

		zeroedProjLimits := getProjectLimits(proj)
		expectedHeadListDeleteRateLimits := int(sat.Config.Console.AccountFreeze.TrialExpirationRateLimits)
		require.Equal(t, zeroUsageLimits.RateLimit, zeroedProjLimits.RateLimit)
		require.Equal(t, zeroUsageLimits.RateLimitPut, zeroedProjLimits.RateLimitPut)
		require.Equal(t, zeroUsageLimits.RateLimitGet, zeroedProjLimits.RateLimitGet)
		require.Equal(t, expectedHeadListDeleteRateLimits, *zeroedProjLimits.RateLimitHead)
		require.Equal(t, expectedHeadListDeleteRateLimits, *zeroedProjLimits.RateLimitList)
		require.Equal(t, expectedHeadListDeleteRateLimits, *zeroedProjLimits.RateLimitDelete)
		require.Equal(t, zeroUsageLimits.BurstLimit, zeroedProjLimits.BurstLimit)
		require.Equal(t, zeroUsageLimits.BurstLimitPut, zeroedProjLimits.BurstLimitPut)
		require.Equal(t, zeroUsageLimits.BurstLimitGet, zeroedProjLimits.BurstLimitGet)
		require.Equal(t, expectedHeadListDeleteRateLimits, *zeroedProjLimits.BurstLimitHead)
		require.Equal(t, expectedHeadListDeleteRateLimits, *zeroedProjLimits.BurstLimitList)
		require.Equal(t, expectedHeadListDeleteRateLimits, *zeroedProjLimits.BurstLimitDelete)

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
		require.Equal(t, console.Active, user.Status)

		proj, err = projectsDB.Get(ctx, proj.ID)
		require.NoError(t, err)
		require.Equal(t, projLimits, getProjectLimits(proj))
	})
}

func TestGetTrialExpirationFreezesToEscalate(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		usersRepo := sat.DB.Console().Users()
		accountFreezeRepo := sat.DB.Console().AccountFreezeEvents()

		now := time.Now()
		expired := now.Add(-time.Hour)
		notExpired := now.Add(time.Hour)

		uuids := []string{
			"00000000-0000-0000-0000-000000000001",
			"00000000-0000-0000-0000-000000000002",
		}

		for _, id := range uuids {
			uid, err := uuid.FromString(id)
			require.NoError(t, err)

			u, err := usersRepo.Insert(ctx, &console.User{
				ID:              uid,
				FullName:        "expired",
				Email:           email + "1",
				PasswordHash:    []byte("123a123"),
				TrialExpiration: &expired,
			})
			require.NoError(t, err)
			_, err = accountFreezeRepo.Upsert(ctx, &console.AccountFreezeEvent{
				UserID: u.ID,
				Type:   console.TrialExpirationFreeze,
			})
			require.NoError(t, err)
		}

		expiredUser3, err := usersRepo.Insert(ctx, &console.User{
			ID:              testrand.UUID(),
			FullName:        "escalated",
			Email:           email + "2",
			PasswordHash:    []byte("123a123"),
			Status:          console.PendingDeletion,
			TrialExpiration: &expired,
		})
		require.NoError(t, err)

		pendingDeletion := console.PendingDeletion
		err = usersRepo.Update(ctx, expiredUser3.ID, console.UpdateUserRequest{
			Status: &pendingDeletion,
		})
		require.NoError(t, err)

		_, err = accountFreezeRepo.Upsert(ctx, &console.AccountFreezeEvent{
			UserID: expiredUser3.ID,
			Type:   console.TrialExpirationFreeze,
		})
		require.NoError(t, err)

		_, err = usersRepo.Insert(ctx, &console.User{
			ID:              testrand.UUID(),
			FullName:        "not expired",
			Email:           email + "2",
			PasswordHash:    []byte("123a123"),
			TrialExpiration: &notExpired,
		})
		require.NoError(t, err)

		limit := 1
		var next *console.FreezeEventsByEventAndUserStatusCursor
		events, next, err := accountFreezeRepo.GetTrialExpirationFreezesToEscalate(ctx, limit, next)
		require.NoError(t, err)
		require.Len(t, events, 1, "expected 1 expired user")
		require.Equal(t, uuids[0], events[0].UserID.String())
		require.NotNil(t, next, "expected next to not be nil")

		events, next, err = accountFreezeRepo.GetTrialExpirationFreezesToEscalate(ctx, limit, next)
		require.NoError(t, err)
		require.Len(t, events, 1, "expected 1 expired user")
		require.Equal(t, uuids[1], events[0].UserID.String())
		require.NotNil(t, next, "expected next to not be nil")

		events, next, err = accountFreezeRepo.GetTrialExpirationFreezesToEscalate(ctx, limit, next)
		require.NoError(t, err)
		require.Len(t, events, 0, "expected 0 expired user")
		require.Nil(t, next, "expected next to be nil")

		limit = 50
		events, _, err = accountFreezeRepo.GetTrialExpirationFreezesToEscalate(ctx, limit, next)
		require.NoError(t, err)
		require.Len(t, events, len(uuids), fmt.Sprintf("expected %d expired users", len(uuids)))
		require.Equal(t, uuids[0], events[0].UserID.String())
		require.Equal(t, uuids[1], events[1].UserID.String())

		err = usersRepo.Update(ctx, events[0].UserID, console.UpdateUserRequest{
			Status: &pendingDeletion,
		})
		require.NoError(t, err)

		events, _, err = accountFreezeRepo.GetTrialExpirationFreezesToEscalate(ctx, limit, nil)
		require.NoError(t, err)
		require.Len(t, events, 1, "expected 1 expired user")
		require.Equal(t, uuids[1], events[0].UserID.String())
	})
}

func TestGetEscalatedEventsBefore(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		usersRepo := sat.DB.Console().Users()
		accountFreezeRepo := sat.DB.Console().AccountFreezeEvents()

		now := time.Now()
		oldTime := now.Add(-2 * time.Hour)
		recentTime := now.Add(-30 * time.Minute)

		// Create users with different freeze types and escalation times
		oldBillingFrozen, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Old Billing User", Email: "oldbilling@test.com",
		}, 1)
		require.NoError(t, err)

		oldTrialFrozen, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Old Trial User", Email: "oldtrial@test.com",
		}, 1)
		require.NoError(t, err)

		recentlyEscalated, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Recent User", Email: "recent@test.com",
		}, 1)
		require.NoError(t, err)

		nonEscalatedUser, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Active User", Email: "active@test.com",
		}, 1)
		require.NoError(t, err)

		// Create freeze events
		_, err = accountFreezeRepo.Upsert(ctx, &console.AccountFreezeEvent{
			UserID: oldBillingFrozen.ID,
			Type:   console.BillingFreeze,
		})
		require.NoError(t, err)

		_, err = accountFreezeRepo.Upsert(ctx, &console.AccountFreezeEvent{
			UserID: oldTrialFrozen.ID,
			Type:   console.TrialExpirationFreeze,
		})
		require.NoError(t, err)

		_, err = accountFreezeRepo.Upsert(ctx, &console.AccountFreezeEvent{
			UserID: recentlyEscalated.ID,
			Type:   console.BillingFreeze,
		})
		require.NoError(t, err)

		_, err = accountFreezeRepo.Upsert(ctx, &console.AccountFreezeEvent{
			UserID: nonEscalatedUser.ID,
			Type:   console.BillingFreeze,
		})
		require.NoError(t, err)

		// mark users as escalated at different times
		usersRepo.TestSetNow(func() time.Time { return oldTime })
		pendingStatus := console.PendingDeletion
		err = usersRepo.Update(ctx, oldBillingFrozen.ID, console.UpdateUserRequest{Status: &pendingStatus})
		require.NoError(t, err)

		err = usersRepo.Update(ctx, oldTrialFrozen.ID, console.UpdateUserRequest{Status: &pendingStatus})
		require.NoError(t, err)

		usersRepo.TestSetNow(func() time.Time { return recentTime })
		err = usersRepo.Update(ctx, recentlyEscalated.ID, console.UpdateUserRequest{Status: &pendingStatus})
		require.NoError(t, err)

		usersRepo.TestSetNow(time.Now)

		// test getting single event type - get old billing freeze
		params := console.GetEscalatedEventsBeforeParams{
			Limit: 10,
			EventTypes: []console.EventTypeAndTime{
				{EventType: console.BillingFreeze, OlderThan: now.Add(-time.Hour)},
			},
		}
		events, err := accountFreezeRepo.GetEscalatedEventsBefore(ctx, params)
		require.NoError(t, err)
		require.Len(t, events, 1)
		require.Equal(t, oldBillingFrozen.ID, events[0].UserID)
		require.Equal(t, console.BillingFreeze, events[0].Type)

		// test get multiple event types - get old billing and trial freezes
		params = console.GetEscalatedEventsBeforeParams{
			Limit: 10,
			EventTypes: []console.EventTypeAndTime{
				{EventType: console.BillingFreeze, OlderThan: now.Add(-time.Hour)},
				{EventType: console.TrialExpirationFreeze, OlderThan: now.Add(-time.Hour)},
			},
		}
		events, err = accountFreezeRepo.GetEscalatedEventsBefore(ctx, params)
		require.NoError(t, err)
		require.Len(t, events, 2)
		for _, event := range events {
			require.True(t, event.Type == console.BillingFreeze || event.Type == console.TrialExpirationFreeze)
			require.True(t, event.UserID == oldBillingFrozen.ID || event.UserID == oldTrialFrozen.ID)
		}

		// test different time bounds for different event types
		params = console.GetEscalatedEventsBeforeParams{
			Limit: 10,
			EventTypes: []console.EventTypeAndTime{
				{EventType: console.BillingFreeze, OlderThan: now.Add(-3 * time.Hour)},
				{EventType: console.TrialExpirationFreeze, OlderThan: now.Add(-time.Hour)},
			},
		}
		events, err = accountFreezeRepo.GetEscalatedEventsBefore(ctx, params)
		require.NoError(t, err)
		require.Len(t, events, 1)
		require.Equal(t, oldTrialFrozen.ID, events[0].UserID)
		require.Equal(t, console.TrialExpirationFreeze, events[0].Type)

		// test limit
		params = console.GetEscalatedEventsBeforeParams{
			Limit: 1,
			EventTypes: []console.EventTypeAndTime{
				{EventType: console.BillingFreeze, OlderThan: now.Add(-time.Hour)},
				{EventType: console.TrialExpirationFreeze, OlderThan: now.Add(-time.Hour)},
			},
		}
		events, err = accountFreezeRepo.GetEscalatedEventsBefore(ctx, params)
		require.NoError(t, err)
		require.Len(t, events, 1)

		// test non-existent event type
		params = console.GetEscalatedEventsBeforeParams{
			Limit: 10,
			EventTypes: []console.EventTypeAndTime{
				{EventType: console.ViolationFreeze, OlderThan: now.Add(-time.Hour)},
			},
		}
		events, err = accountFreezeRepo.GetEscalatedEventsBefore(ctx, params)
		require.NoError(t, err)
		require.Len(t, events, 0)

		params = console.GetEscalatedEventsBeforeParams{
			Limit: 10,
			EventTypes: []console.EventTypeAndTime{
				{EventType: console.BillingFreeze, OlderThan: now.Add(-time.Hour)},
			},
		}
		events, err = accountFreezeRepo.GetEscalatedEventsBefore(ctx, params)
		require.NoError(t, err)
		require.Len(t, events, 1)
		require.Equal(t, oldBillingFrozen.ID, events[0].UserID)
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
