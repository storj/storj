// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package accountfreeze_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
)

func TestBotFreezeChore(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.AccountFreeze.Enabled = true
				config.Console.Captcha.FlagBotsEnabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		usersDB := sat.DB.Console().Users()
		accFreezeDB := sat.DB.Console().AccountFreezeEvents()
		service := console.NewAccountFreezeService(sat.DB.Console(), newFreezeTrackerMock(t), sat.Config.Console.AccountFreeze)
		chore := sat.Core.AccountFreeze.BotFreezeChore

		chore.Loop.Pause()
		chore.TestSetFreezeService(service)

		now := time.Now().UTC().Truncate(time.Minute)
		chore.TestSetNow(func() time.Time { return now })

		t.Run("Bot user is frozen with delay", func(t *testing.T) {
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			// reset chore clock
			chore.TestSetNow(func() time.Time { return now })

			botUser, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Test Bot User",
				Email:    "botuser@mail.test",
			}, 2)
			require.NoError(t, err)

			_, err = sat.AddProject(ctx, botUser.ID, "test")
			require.NoError(t, err)

			_, err = sat.AddProject(ctx, botUser.ID, "test1")
			require.NoError(t, err)

			two := 2
			_, err = accFreezeDB.Upsert(ctx, &console.AccountFreezeEvent{
				UserID:             botUser.ID,
				Type:               console.DelayedBotFreeze,
				DaysTillEscalation: &two,
			})
			require.NoError(t, err)

			chore.Loop.TriggerWait()

			// User is not bot frozen yet.
			botUser, err = usersDB.Get(ctx, botUser.ID)
			require.NoError(t, err)
			require.Equal(t, console.Active, botUser.Status)

			// Delayed event still exists.
			event, err := accFreezeDB.Get(ctx, botUser.ID, console.DelayedBotFreeze)
			require.NoError(t, err)
			require.NotNil(t, event)
			require.Equal(t, console.DelayedBotFreeze, event.Type)

			// Current date is now 3 days later.
			chore.TestSetNow(func() time.Time {
				return now.Add(25 * 3 * time.Hour)
			})
			chore.Loop.TriggerWait()

			// User is bot frozen and has zero limits.
			botUser, err = usersDB.Get(ctx, botUser.ID)
			require.NoError(t, err)
			require.Equal(t, console.PendingBotVerification, botUser.Status)
			require.Zero(t, botUser.ProjectBandwidthLimit)
			require.Zero(t, botUser.ProjectStorageLimit)
			require.Zero(t, botUser.ProjectSegmentLimit)

			// Users projects have zero limits.
			projects, err := sat.DB.Console().Projects().GetOwn(ctx, botUser.ID)
			require.NoError(t, err)
			require.Len(t, projects, 2)

			for _, p := range projects {
				require.Zero(t, *p.StorageLimit)
				require.Zero(t, *p.BandwidthLimit)
				require.Zero(t, *p.SegmentLimit)
				require.Zero(t, *p.RateLimit)
			}

			// BotFreeze event was inserted.
			event, err = accFreezeDB.Get(ctx, botUser.ID, console.BotFreeze)
			require.NoError(t, err)
			require.NotNil(t, event)
			require.NotNil(t, event.Limits)

			// Delayed event doesn't exist anymore.
			event, err = accFreezeDB.Get(ctx, botUser.ID, console.DelayedBotFreeze)
			require.Error(t, err)
			require.Nil(t, event)
		})
	})
}
