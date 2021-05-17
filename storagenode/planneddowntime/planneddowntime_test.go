// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package planneddowntime_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"storj.io/common/testcontext"
	rand "storj.io/common/testrand"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/planneddowntime"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestPlannedDowntimeDB(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		now := time.Now()
		plannedDowntimeDB := db.PlannedDowntime()

		oneWeekAgo := now.Add(-7 * 24 * time.Hour)
		oneWeekFromNow := now.Add(7 * 24 * time.Hour)
		currentID := rand.BytesInt(32)
		pastID := rand.BytesInt(32)
		futureID := rand.BytesInt(32)
		entries := []planneddowntime.Entry{
			{
				ID:          currentID,
				Start:       now.Add(-1 * time.Hour),
				End:         now.Add(1 * time.Hour),
				ScheduledAt: now,
			},
			{
				ID:          pastID,
				Start:       oneWeekAgo.Add(-1 * time.Hour),
				End:         oneWeekAgo.Add(1 * time.Hour),
				ScheduledAt: now,
			},
			{
				ID:          futureID,
				Start:       oneWeekFromNow.Add(-1 * time.Hour),
				End:         oneWeekFromNow.Add(1 * time.Hour),
				ScheduledAt: now,
			},
		}

		t.Run("insert", func(t *testing.T) {
			for _, entry := range entries {
				err := plannedDowntimeDB.Add(ctx, entry)
				require.NoError(t, err)
			}
		})

		t.Run("get scheduled", func(t *testing.T) {
			scheduled, err := plannedDowntimeDB.GetScheduled(ctx, now)
			require.NoError(t, err)
			require.Len(t, scheduled, 2)
			// earliest window should be first
			require.Equal(t, currentID, scheduled[0].ID)
			require.Equal(t, futureID, scheduled[1].ID)
		})

		t.Run("get completed", func(t *testing.T) {
			completed, err := plannedDowntimeDB.GetCompleted(ctx, now)
			require.NoError(t, err)
			require.Len(t, completed, 1)
			require.Equal(t, pastID, completed[0].ID)
		})
	})
}
