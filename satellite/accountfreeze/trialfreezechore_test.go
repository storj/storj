// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package accountfreeze_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
)

func TestTrialFreezeChore(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.AccountFreeze.Enabled = true
				config.AccountFreeze.EmailsEnabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		usersDB := sat.DB.Console().Users()
		projectMembersDB := sat.DB.Console().ProjectMembers()
		accFreezeDB := sat.DB.Console().AccountFreezeEvents()
		service := console.NewAccountFreezeService(sat.DB.Console(), newFreezeTrackerMock(t), sat.Config.Console.AccountFreeze)
		chore := sat.Core.AccountFreeze.TrialFreezeChore

		chore.Loop.Pause()
		chore.TestSetFreezeService(service)

		now := time.Now().UTC().Truncate(time.Minute)
		chore.TestSetNow(func() time.Time { return now })

		t.Run("Free trial expiration freeze", func(t *testing.T) {
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			chore.TestSetNow(func() time.Time { return now })

			freeUser, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Free User",
				Email:    "free@mail.test",
			}, 1)
			require.NoError(t, err)
			require.Nil(t, freeUser.TrialExpiration)

			chore.Loop.TriggerWait()

			// user with nil trial expiration should not be frozen.
			frozen, err := service.IsUserFrozen(ctx, freeUser.ID, console.TrialExpirationFreeze)
			require.NoError(t, err)
			require.False(t, frozen)

			newTime := now.Add(120 * time.Hour)
			newTimePtr := &newTime
			err = usersDB.Update(ctx, freeUser.ID, console.UpdateUserRequest{
				TrialExpiration: &newTimePtr,
			})
			require.NoError(t, err)

			chore.Loop.TriggerWait()

			// user with future trial expiration should not be frozen.
			frozen, err = service.IsUserFrozen(ctx, freeUser.ID, console.TrialExpirationFreeze)
			require.NoError(t, err)
			require.False(t, frozen)

			// forward date to after newTime
			chore.TestSetNow(func() time.Time {
				return newTime.Add(200 * time.Hour)
			})
			chore.Loop.TriggerWait()

			// user with past trial expiration should be frozen.
			frozen, err = service.IsUserFrozen(ctx, freeUser.ID, console.TrialExpirationFreeze)
			require.NoError(t, err)
			require.True(t, frozen)

			// reset chore time
			chore.TestSetNow(func() time.Time { return now })

			err = service.TrialExpirationUnfreezeUser(ctx, freeUser.ID)
			require.NoError(t, err)

			// set past expiry and paid tier
			newTime = now.Add(-120 * time.Hour)
			newTimePtr = &newTime
			kind := console.PaidUser
			err = usersDB.Update(ctx, freeUser.ID, console.UpdateUserRequest{
				TrialExpiration: &newTimePtr,
				Kind:            &kind,
			})
			require.NoError(t, err)

			chore.Loop.TriggerWait()

			// user with past trial expiration but in paid tier should not be frozen.
			frozen, err = service.IsUserFrozen(ctx, freeUser.ID, console.TrialExpirationFreeze)
			require.NoError(t, err)
			require.False(t, frozen)

			kind = console.FreeUser
			err = usersDB.Update(ctx, freeUser.ID, console.UpdateUserRequest{
				Kind: &kind,
			})
			require.NoError(t, err)

			chore.Loop.TriggerWait()

			frozen, err = service.IsUserFrozen(ctx, freeUser.ID, console.TrialExpirationFreeze)
			require.NoError(t, err)
			require.True(t, frozen)

			chore.TestSetNow(func() time.Time {
				return now.Add(50 * 24 * time.Hour)
			})

			chore.Loop.TriggerWait()

			// user should be marked for deletion after the grace period
			// (trial freeze event escalated).
			userPD, err := usersDB.Get(ctx, freeUser.ID)
			require.NoError(t, err)
			require.Equal(t, console.PendingDeletion, userPD.Status)

			// test disabled trial expiration freeze escalation.
			service.TestSetTrialExpirationFreezeGracePeriod(0)

			status := console.Active
			err = usersDB.Update(ctx, freeUser.ID, console.UpdateUserRequest{
				Status: &status,
			})
			require.NoError(t, err)

			chore.Loop.TriggerWait()

			// event not escalated because grace period is 0
			// (escalation disabled).
			userPD, err = usersDB.Get(ctx, freeUser.ID)
			require.NoError(t, err)
			require.Equal(t, status, userPD.Status)

			// enable trial freeze escalation.
			service.TestSetTrialExpirationFreezeGracePeriod(24 * time.Hour)

			// test that trial frozen users that are part of
			// projects will not be marked for deletion.
			uplinkProject := planet.Uplinks[0].Projects[0]
			_, err = projectMembersDB.Insert(ctx, freeUser.ID, uplinkProject.ID, console.RoleMember)
			require.NoError(t, err)

			chore.Loop.TriggerWait()

			userPD, err = usersDB.Get(ctx, freeUser.ID)
			require.NoError(t, err)
			require.Equal(t, status, userPD.Status)
		})

		t.Run("No trial expiration excalation for paid and NFR users", func(t *testing.T) {
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			// reset chore clock
			chore.TestSetNow(func() time.Time { return now })

			paidUser, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Paid User",
				Email:    "paid@test.test",
				Kind:     console.PaidUser,
			}, 1)
			require.NoError(t, err)

			nfrUser, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "NFR User",
				Email:    "nfr@test.test",
				Kind:     console.NFRUser,
			}, 1)
			require.NoError(t, err)

			err = service.TrialExpirationFreezeUser(ctx, paidUser.ID)
			require.NoError(t, err)
			err = service.TrialExpirationFreezeUser(ctx, nfrUser.ID)
			require.NoError(t, err)

			// forward date to after the grace period
			chore.TestSetNow(func() time.Time {
				return now.Add(sat.Config.Console.AccountFreeze.TrialExpirationFreezeGracePeriod).Add(25 * time.Hour)
			})

			// run the chore
			chore.Loop.TriggerWait()

			// verify freeze events are removed for both users
			_, err = accFreezeDB.Get(ctx, paidUser.ID, console.TrialExpirationFreeze)
			require.ErrorIs(t, err, sql.ErrNoRows)

			_, err = accFreezeDB.Get(ctx, nfrUser.ID, console.TrialExpirationFreeze)
			require.ErrorIs(t, err, sql.ErrNoRows)
		})

		t.Run("No trial expiration escalation for non-active users", func(t *testing.T) {
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			// reset chore clock
			chore.TestSetNow(func() time.Time { return now })

			inactiveUser, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Inactive User",
				Email:    "inactive@example.test",
			}, 1)
			require.NoError(t, err)

			// freeze the user for trial expiration
			err = service.TrialExpirationFreezeUser(ctx, inactiveUser.ID)
			require.NoError(t, err)

			// set user status to Deleted (non-active)
			status := console.Deleted
			err = usersDB.Update(ctx, inactiveUser.ID, console.UpdateUserRequest{
				Status: &status,
			})
			require.NoError(t, err)

			// forward date to after the grace period
			chore.TestSetNow(func() time.Time {
				return now.Add(sat.Config.Console.AccountFreeze.TrialExpirationFreezeGracePeriod).Add(25 * time.Hour)
			})

			// run the chore
			chore.Loop.TriggerWait()

			// verify user status hasn't changed (still Deleted, not escalated)
			updatedUser, err := usersDB.Get(ctx, inactiveUser.ID)
			require.NoError(t, err)
			require.Equal(t, console.Deleted, updatedUser.Status)

			// verify freeze event still exists (not escalated)
			event, err := accFreezeDB.Get(ctx, inactiveUser.ID, console.TrialExpirationFreeze)
			require.NoError(t, err)
			require.NotEmpty(t, event.DaysTillEscalation)
		})
	})
}
