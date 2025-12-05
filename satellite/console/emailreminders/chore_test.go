// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package emailreminders_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
)

func TestEmailChoreUpdatesVerificationReminders(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.EmailReminders.FirstVerificationReminder = 0
				config.EmailReminders.SecondVerificationReminder = 0
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		users := planet.Satellites[0].DB.Console().Users()
		chore := planet.Satellites[0].Core.Mail.EmailReminders
		chore.Loop.Pause()

		// Overwrite link address in chore so the links don't work
		// and we can test that the correct number of reminders are sent.
		chore.TestSetLinkAddress("")

		id1 := testrand.UUID()
		_, err := users.Insert(ctx, &console.User{
			ID:           id1,
			FullName:     "test",
			Email:        "userone@mail.test",
			PasswordHash: []byte("password"),
		})
		require.NoError(t, err)

		id2 := testrand.UUID()
		_, err = users.Insert(ctx, &console.User{
			ID:           id2,
			FullName:     "test",
			Email:        "usertwo@mail.test",
			PasswordHash: []byte("password"),
		})
		require.NoError(t, err)

		id3 := testrand.UUID()
		_, err = users.Insert(ctx, &console.User{
			ID:           id3,
			FullName:     "test",
			Email:        "userthree@mail.test",
			PasswordHash: []byte("password"),
		})
		require.NoError(t, err)

		// This user will verify immediately and should not get reminders.
		user1, err := users.Get(ctx, id1)
		require.NoError(t, err)
		require.Zero(t, user1.VerificationReminders)

		// This user will get one reminder and then verify and should not get a second.
		user2, err := users.Get(ctx, id2)
		require.NoError(t, err)
		require.Zero(t, user2.VerificationReminders)

		// This user will not verify at all and should get 2 reminders and no more.
		user3, err := users.Get(ctx, id3)
		require.NoError(t, err)
		require.Zero(t, user3.VerificationReminders)

		user1.Status = 1
		err = users.Update(ctx, user1.ID, console.UpdateUserRequest{
			Status: &user1.Status,
		})
		require.NoError(t, err)

		chore.Loop.TriggerWait()

		user1, err = users.Get(ctx, id1)
		require.NoError(t, err)
		require.Zero(t, user1.VerificationReminders)

		user2, err = users.Get(ctx, id2)
		require.NoError(t, err)
		require.Equal(t, 1, user2.VerificationReminders)

		user3, err = users.Get(ctx, id3)
		require.NoError(t, err)
		require.Equal(t, 1, user3.VerificationReminders)

		user2.Status = 1
		err = users.Update(ctx, user2.ID, console.UpdateUserRequest{
			Status: &user2.Status,
		})
		require.NoError(t, err)

		chore.Loop.TriggerWait()

		user1, err = users.Get(ctx, id1)
		require.NoError(t, err)
		require.Zero(t, user1.VerificationReminders)

		user2, err = users.Get(ctx, id2)
		require.NoError(t, err)
		require.Equal(t, 1, user2.VerificationReminders)

		user3, err = users.Get(ctx, id3)
		require.NoError(t, err)
		require.Equal(t, 2, user3.VerificationReminders)

		// Check user is not reminded again after 2
		chore.Loop.TriggerWait()

		user1, err = users.Get(ctx, id1)
		require.NoError(t, err)
		require.Zero(t, user1.VerificationReminders)

		user2, err = users.Get(ctx, id2)
		require.NoError(t, err)
		require.Equal(t, 1, user2.VerificationReminders)

		user3, err = users.Get(ctx, id3)
		require.NoError(t, err)
		require.Equal(t, 2, user3.VerificationReminders)
	})
}

func TestEmailChoreLinkActivatesAccount(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.EmailReminders.FirstVerificationReminder = 0
				config.EmailReminders.SecondVerificationReminder = 0
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		users := planet.Satellites[0].DB.Console().Users()
		chore := planet.Satellites[0].Core.Mail.EmailReminders
		chore.Loop.Pause()
		chore.TestUseBlockingSend()

		id := testrand.UUID()
		_, err := users.Insert(ctx, &console.User{
			ID:           id,
			FullName:     "test",
			Email:        "userone@mail.test",
			PasswordHash: []byte("password"),
		})
		require.NoError(t, err)

		u, err := users.Get(ctx, id)
		require.NoError(t, err)
		require.Equal(t, console.UserStatus(0), u.Status)

		chore.Loop.TriggerWait()

		u, err = users.Get(ctx, id)
		require.NoError(t, err)

		require.Equal(t, console.UserStatus(1), u.Status)
	})
}

func TestEmailChoreUpdatesTrialNotifications(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.EmailReminders.EnableTrialExpirationReminders = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		users := planet.Satellites[0].DB.Console().Users()
		chore := planet.Satellites[0].Core.Mail.EmailReminders
		chore.Loop.Pause()

		// control group: paid tier user
		id1 := testrand.UUID()
		_, err := users.Insert(ctx, &console.User{
			ID:           id1,
			FullName:     "test",
			Email:        "userone@mail.test",
			PasswordHash: []byte("password"),
		})
		require.NoError(t, err)
		kind := console.PaidUser
		require.NoError(t, users.Update(ctx, id1, console.UpdateUserRequest{Kind: &kind}))

		now := time.Now()
		tomorrow := now.Add(24 * time.Hour)
		yesterday := now.Add(-24 * time.Hour)

		// one expiring user
		id2 := testrand.UUID()
		_, err = users.Insert(ctx, &console.User{
			ID:              id2,
			FullName:        "test",
			Email:           "usertwo@mail.test",
			PasswordHash:    []byte("password"),
			TrialExpiration: &tomorrow,
		})
		require.NoError(t, err)

		// one expired user who was already reminded
		id3 := testrand.UUID()
		_, err = users.Insert(ctx, &console.User{
			ID:                 id3,
			FullName:           "test",
			Email:              "usertwo@mail.test",
			PasswordHash:       []byte("password"),
			TrialExpiration:    &yesterday,
			TrialNotifications: int(console.TrialExpirationReminder),
		})
		require.NoError(t, err)

		reminded := console.TrialExpirationReminder

		require.NoError(t, users.Update(ctx, id3, console.UpdateUserRequest{
			TrialNotifications: &reminded,
		}))

		user1, err := users.Get(ctx, id1)
		require.NoError(t, err)
		require.Equal(t, console.PaidUser, user1.Kind)
		require.Nil(t, user1.TrialExpiration)
		require.Equal(t, int(console.NoTrialNotification), user1.TrialNotifications)

		user2, err := users.Get(ctx, id2)
		require.NoError(t, err)
		require.Equal(t, console.FreeUser, user2.Kind)
		require.Zero(t, cmp.Diff(user2.TrialExpiration.Truncate(time.Millisecond), tomorrow.Truncate(time.Millisecond), cmpopts.EquateApproxTime(0)))
		require.Equal(t, int(console.NoTrialNotification), user2.TrialNotifications)

		user3, err := users.Get(ctx, id3)
		require.NoError(t, err)
		require.Equal(t, console.FreeUser, user3.Kind)
		require.Zero(t, cmp.Diff(user3.TrialExpiration.Truncate(time.Millisecond), yesterday.Truncate(time.Millisecond), cmpopts.EquateApproxTime(0)))
		require.Equal(t, int(console.TrialExpirationReminder), user3.TrialNotifications)

		chore.Loop.TriggerWait()

		user1, err = users.Get(ctx, id1)
		require.NoError(t, err)
		require.Equal(t, int(console.NoTrialNotification), user1.TrialNotifications)

		user2, err = users.Get(ctx, id2)
		require.NoError(t, err)
		require.Equal(t, int(console.TrialExpirationReminder), user2.TrialNotifications)

		user3, err = users.Get(ctx, id3)
		require.NoError(t, err)
		require.Equal(t, int(console.TrialExpired), user3.TrialNotifications)

		// run again to make sure values don't change.
		chore.Loop.TriggerWait()

		user1, err = users.Get(ctx, id1)
		require.NoError(t, err)
		require.Equal(t, int(console.NoTrialNotification), user1.TrialNotifications)

		user2, err = users.Get(ctx, id2)
		require.NoError(t, err)
		require.Equal(t, int(console.TrialExpirationReminder), user2.TrialNotifications)

		user3, err = users.Get(ctx, id3)
		require.NoError(t, err)
		require.Equal(t, int(console.TrialExpired), user3.TrialNotifications)
	})
}
