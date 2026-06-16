// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package accountfreeze_test

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
)

func TestOptOutFreezeChore(t *testing.T) {
	freezeDate := time.Now().UTC().Truncate(time.Minute).Add(-time.Hour)
	const freezeGrace = 1080 * time.Hour // 45 days.

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.AccountFreeze.Enabled = true
				config.AccountFreeze.EmailsEnabled = true
				config.Console.AccountFreeze.OptOutFreezeDate = freezeDate.Format(time.RFC3339)
				config.Console.AccountFreeze.OptOutFreezeGracePeriod = freezeGrace
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		usersDB := sat.DB.Console().Users()
		service := console.NewAccountFreezeService(sat.DB.Console(), newFreezeTrackerMock(t), sat.Config.Console.AccountFreeze)
		chore := sat.Core.AccountFreeze.OptOutFreezeChore

		chore.Loop.Pause()
		chore.TestSetFreezeService(service)

		setOptInStatus := func(t *testing.T, userID uuid.UUID, status console.OptInStatus) {
			t.Helper()
			require.NoError(t, usersDB.UpsertSettings(ctx, userID, console.UpsertUserSettingsRequest{
				OptInStatus: &status,
			}))
		}

		t.Run("freeze -> escalate", func(t *testing.T) {
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			chore.TestSetNow(func() time.Time { return freezeDate.Add(time.Hour) })

			user, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "OptOut User",
				Email:    "optout@mail.test",
			}, 1)
			require.NoError(t, err)

			paidKind := console.PaidUser
			require.NoError(t, usersDB.Update(ctx, user.ID, console.UpdateUserRequest{Kind: &paidKind}))

			// Stage 1: freeze.
			chore.Loop.TriggerWait()

			freezes, err := service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes.OptOutFreeze, "user should be frozen")
			require.Equal(t, 1, freezes.OptOutFreeze.NotificationsCount, "expected freeze email to be sent on freeze")

			frozenUser, err := usersDB.Get(ctx, user.ID)
			require.NoError(t, err)
			require.EqualValues(t, 0, frozenUser.ProjectStorageLimit)
			require.EqualValues(t, 0, frozenUser.ProjectBandwidthLimit)
			require.EqualValues(t, 0, frozenUser.ProjectSegmentLimit)
			require.NotEqual(t, console.PendingDeletion, frozenUser.Status)

			// Stage 2: escalate. Advance past the freeze grace.
			chore.TestSetNow(func() time.Time { return freezeDate.Add(3 * freezeGrace) })
			chore.Loop.TriggerWait()

			escalatedUser, err := usersDB.Get(ctx, user.ID)
			require.NoError(t, err)
			require.Equal(t, console.PendingDeletion, escalatedUser.Status, "user should be marked for deletion")

			freezes, err = service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes.OptOutFreeze)
			require.Equal(t, 2, freezes.OptOutFreeze.NotificationsCount, "expected escalation email to be sent on escalate")
		})

		t.Run("no-op before freeze date", func(t *testing.T) {
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			chore.TestSetNow(func() time.Time { return freezeDate.Add(-time.Hour) })

			user, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Before Cutoff User",
				Email:    "before-cutoff@mail.test",
			}, 1)
			require.NoError(t, err)

			paidKind := console.PaidUser
			require.NoError(t, usersDB.Update(ctx, user.ID, console.UpdateUserRequest{Kind: &paidKind}))

			chore.Loop.TriggerWait()

			freezes, err := service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.Nil(t, freezes.OptOutFreeze, "no freeze should be issued before OptOutFreezeDate")
		})

		t.Run("user already in another freeze flow is skipped", func(t *testing.T) {
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			chore.TestSetNow(func() time.Time { return freezeDate.Add(time.Hour) })

			user, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Billing-Frozen User",
				Email:    "billing-frozen-optout@mail.test",
			}, 1)
			require.NoError(t, err)

			paidKind := console.PaidUser
			require.NoError(t, usersDB.Update(ctx, user.ID, console.UpdateUserRequest{Kind: &paidKind}))

			require.NoError(t, service.BillingFreezeUser(ctx, user.ID))

			chore.Loop.TriggerWait()

			freezes, err := service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes.BillingFreeze)
			require.Nil(t, freezes.OptOutFreeze, "user with BillingFreeze should not be opt-out frozen")
		})

		t.Run("opted-in user is not frozen", func(t *testing.T) {
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			chore.TestSetNow(func() time.Time { return freezeDate.Add(time.Hour) })

			user, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Opted-In User",
				Email:    "optedin@mail.test",
			}, 1)
			require.NoError(t, err)

			paidKind := console.PaidUser
			require.NoError(t, usersDB.Update(ctx, user.ID, console.UpdateUserRequest{Kind: &paidKind}))

			optedIn := console.OptedIn
			require.NoError(t, usersDB.UpsertSettings(ctx, user.ID, console.UpsertUserSettingsRequest{
				OptInStatus: &optedIn,
			}))

			chore.Loop.TriggerWait()

			freezes, err := service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.Nil(t, freezes.OptOutFreeze, "OptedIn user should not be opt-out frozen")
		})

		t.Run("non-paid user kinds are not frozen", func(t *testing.T) {
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			chore.TestSetNow(func() time.Time { return freezeDate.Add(time.Hour) })

			for _, kind := range []console.UserKind{console.FreeUser, console.NFRUser, console.MemberUser} {
				user, err := sat.AddUser(ctx, console.CreateUser{
					FullName: "Non-Paid User",
					Email:    fmt.Sprintf("%d@mail.test", kind),
				}, 1)
				require.NoError(t, err)

				require.NoError(t, usersDB.Update(ctx, user.ID, console.UpdateUserRequest{Kind: &kind}))

				chore.Loop.TriggerWait()

				freezes, err := service.GetAll(ctx, user.ID)
				require.NoError(t, err)
				require.Nil(t, freezes.OptOutFreeze, kind.String()+" user should not be opt-out frozen")
			}
		})

		t.Run("excluded user is not frozen", func(t *testing.T) {
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			chore.TestSetNow(func() time.Time { return freezeDate.Add(time.Hour) })

			user, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Excluded User",
				Email:    "excluded@mail.test",
			}, 1)
			require.NoError(t, err)

			paidKind := console.PaidUser
			require.NoError(t, usersDB.Update(ctx, user.ID, console.UpdateUserRequest{Kind: &paidKind}))

			excluded := console.Excluded
			require.NoError(t, usersDB.UpsertSettings(ctx, user.ID, console.UpsertUserSettingsRequest{
				OptInStatus: &excluded,
			}))

			chore.Loop.TriggerWait()

			freezes, err := service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.Nil(t, freezes.OptOutFreeze, "Excluded user should not be opt-out frozen")
		})

		t.Run("frozen user opts in and is unfrozen", func(t *testing.T) {
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			chore.TestSetNow(func() time.Time { return freezeDate.Add(time.Hour) })

			user, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Reopt-In Frozen User",
				Email:    "reoptin-frozen@mail.test",
			}, 1)
			require.NoError(t, err)

			paidKind := console.PaidUser
			require.NoError(t, usersDB.Update(ctx, user.ID, console.UpdateUserRequest{Kind: &paidKind}))

			origStorage := user.ProjectStorageLimit
			origBandwidth := user.ProjectBandwidthLimit
			origSegment := user.ProjectSegmentLimit
			require.Positive(t, origStorage)

			chore.Loop.TriggerWait() // freeze

			freezes, err := service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes.OptOutFreeze)
			frozenUser, err := usersDB.Get(ctx, user.ID)
			require.NoError(t, err)
			require.EqualValues(t, 0, frozenUser.ProjectStorageLimit)

			setOptInStatus(t, user.ID, console.OptedIn)
			chore.Loop.TriggerWait()

			freezes, err = service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.Nil(t, freezes.OptOutFreeze, "freeze should be cleared once user is OptedIn")

			restoredUser, err := usersDB.Get(ctx, user.ID)
			require.NoError(t, err)
			require.Equal(t, origStorage, restoredUser.ProjectStorageLimit, "user storage limit should be restored")
			require.Equal(t, origBandwidth, restoredUser.ProjectBandwidthLimit, "user bandwidth limit should be restored")
			require.Equal(t, origSegment, restoredUser.ProjectSegmentLimit, "user segment limit should be restored")
		})

		t.Run("escalated user opts in and reverts PendingDeletion", func(t *testing.T) {
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			chore.TestSetNow(func() time.Time { return freezeDate.Add(time.Hour) })

			user, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Escalated Reopt-In User",
				Email:    "escalated-reoptin@mail.test",
			}, 1)
			require.NoError(t, err)

			paidKind := console.PaidUser
			require.NoError(t, usersDB.Update(ctx, user.ID, console.UpdateUserRequest{Kind: &paidKind}))

			chore.Loop.TriggerWait() // freeze
			chore.TestSetNow(func() time.Time { return freezeDate.Add(3 * freezeGrace) })
			chore.Loop.TriggerWait() // escalate

			escalated, err := usersDB.Get(ctx, user.ID)
			require.NoError(t, err)
			require.Equal(t, console.PendingDeletion, escalated.Status)

			setOptInStatus(t, user.ID, console.OptedIn)
			chore.Loop.TriggerWait()

			restored, err := usersDB.Get(ctx, user.ID)
			require.NoError(t, err)
			require.Equal(t, console.Active, restored.Status, "PendingDeletion should be reverted to Active on unfreeze")
			freezes, err := service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.Nil(t, freezes.OptOutFreeze)
		})

		t.Run("frozen user marked excluded is unfrozen", func(t *testing.T) {
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			chore.TestSetNow(func() time.Time { return freezeDate.Add(time.Hour) })

			user, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Excluded Frozen User",
				Email:    "excluded-frozen@mail.test",
			}, 1)
			require.NoError(t, err)

			paidKind := console.PaidUser
			require.NoError(t, usersDB.Update(ctx, user.ID, console.UpdateUserRequest{Kind: &paidKind}))

			chore.Loop.TriggerWait()

			freezes, err := service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes.OptOutFreeze)

			setOptInStatus(t, user.ID, console.Excluded)
			chore.Loop.TriggerWait()

			freezes, err = service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.Nil(t, freezes.OptOutFreeze, "freeze should be cleared for Excluded user")
		})

		t.Run("pre-freeze reminder", func(t *testing.T) {
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))

			const reminderBefore = 7 * 24 * time.Hour
			reminderAt := freezeDate.Add(-reminderBefore)

			user, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Reminder User",
				Email:    "pre-freeze-reminder@mail.test",
			}, 1)
			require.NoError(t, err)

			paidKind := console.PaidUser
			require.NoError(t, usersDB.Update(ctx, user.ID, console.UpdateUserRequest{Kind: &paidKind}))

			getReminderSent := func(t *testing.T) bool {
				t.Helper()
				settings, err := usersDB.GetSettings(ctx, user.ID)
				if errors.Is(err, sql.ErrNoRows) {
					return false
				}
				require.NoError(t, err)
				return settings.NoticeDismissal.OptOutFreezeReminderSent
			}

			// Before the reminder window: no reminder sent and user not frozen.
			chore.TestSetNow(func() time.Time { return reminderAt.Add(-time.Hour) })
			chore.Loop.TriggerWait()
			require.False(t, getReminderSent(t), "reminder should not be sent before window")
			freezes, err := service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.Nil(t, freezes.OptOutFreeze, "user should not be frozen before freeze date")

			// In the reminder window: reminder is sent, user still not frozen.
			chore.TestSetNow(func() time.Time { return reminderAt.Add(time.Hour) })
			chore.Loop.TriggerWait()
			require.True(t, getReminderSent(t), "reminder should be sent in window")
			freezes, err = service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.Nil(t, freezes.OptOutFreeze, "user should not be frozen before freeze date")

			// Past freeze date: user gets frozen.
			chore.TestSetNow(func() time.Time { return freezeDate.Add(time.Hour) })
			chore.Loop.TriggerWait()
			freezes, err = service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes.OptOutFreeze, "user should be frozen after freeze date")
			require.Equal(t, 1, freezes.OptOutFreeze.NotificationsCount, "only the freeze email should have been sent")
		})

		t.Run("opted-out user gets no pre-freeze reminder but is still frozen", func(t *testing.T) {
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))

			const reminderBefore = 7 * 24 * time.Hour
			reminderAt := freezeDate.Add(-reminderBefore)

			user, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Opted-Out User",
				Email:    "opted-out-reminder@mail.test",
			}, 1)
			require.NoError(t, err)

			paidKind := console.PaidUser
			require.NoError(t, usersDB.Update(ctx, user.ID, console.UpdateUserRequest{Kind: &paidKind}))
			setOptInStatus(t, user.ID, console.OptedOut)

			// In the reminder window: opted-out user is not reminded.
			chore.TestSetNow(func() time.Time { return reminderAt.Add(time.Hour) })
			chore.Loop.TriggerWait()
			settings, err := usersDB.GetSettings(ctx, user.ID)
			require.NoError(t, err)
			require.False(t, settings.NoticeDismissal.OptOutFreezeReminderSent,
				"opted-out user should not receive the pre-freeze reminder")
			freezes, err := service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.Nil(t, freezes.OptOutFreeze, "user should not be frozen before freeze date")

			// Past freeze date: opted-out user is still frozen.
			chore.TestSetNow(func() time.Time { return freezeDate.Add(time.Hour) })
			chore.Loop.TriggerWait()
			freezes, err = service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes.OptOutFreeze, "opted-out user should still be frozen after freeze date")
		})
	})
}
