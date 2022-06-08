// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestGetUnverifiedNeedingReminderCutoff(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		users := db.Console().Users()

		id := testrand.UUID()
		_, err := users.Insert(ctx, &console.User{
			ID:           id,
			FullName:     "test",
			Email:        "userone@mail.test",
			PasswordHash: []byte("testpassword"),
		})
		require.NoError(t, err)

		u, err := users.Get(ctx, id)
		require.NoError(t, err)
		require.Equal(t, console.UserStatus(0), u.Status)

		now := time.Now()
		reminders := now.Add(time.Hour)

		// to get a reminder, created_at needs be after cutoff.
		// since we don't have control over created_at, make cutoff in the future to test that
		// user doesn't get a reminder.
		cutoff := now.Add(time.Hour)

		needingReminder, err := users.GetUnverifiedNeedingReminder(ctx, reminders, reminders, cutoff)
		require.NoError(t, err)
		require.Len(t, needingReminder, 0)

		// change cutoff so user created_at is after it.
		// user should get a reminder.
		cutoff = now.Add(-time.Hour)

		needingReminder, err = users.GetUnverifiedNeedingReminder(ctx, now, now, cutoff)
		require.NoError(t, err)
		require.Len(t, needingReminder, 1)
	})
}
