// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package accountfreeze_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
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
	})
}
