// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package emailreminders_test

import (
	"testing"

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
