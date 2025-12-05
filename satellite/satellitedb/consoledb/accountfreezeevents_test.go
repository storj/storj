// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package consoledb_test

import (
	"database/sql"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestAccountFreezeEvents(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		randUsageLimits := func() console.UsageLimits {
			return console.UsageLimits{Storage: rand.Int63(), Bandwidth: rand.Int63(), Segment: rand.Int63()}
		}

		days := 60
		userID := testrand.UUID()
		event := &console.AccountFreezeEvent{
			UserID:             userID,
			Type:               console.BillingFreeze,
			DaysTillEscalation: &days,
			NotificationsCount: 1,
			Limits: &console.AccountFreezeEventLimits{
				User: randUsageLimits(),
				Projects: map[uuid.UUID]console.UsageLimits{
					testrand.UUID(): randUsageLimits(),
					testrand.UUID(): randUsageLimits(),
				},
			},
		}

		eventsDB := db.Console().AccountFreezeEvents()

		t.Run("Can't insert nil event", func(t *testing.T) {
			_, err := eventsDB.Upsert(ctx, nil)
			require.Error(t, err)
		})

		t.Run("Insert event", func(t *testing.T) {
			dbEvent, err := eventsDB.Upsert(ctx, event)
			require.NoError(t, err)
			require.NotNil(t, dbEvent)
			require.WithinDuration(t, time.Now(), dbEvent.CreatedAt, time.Minute)
			dbEvent.CreatedAt = event.CreatedAt
			require.Equal(t, event, dbEvent)
		})

		t.Run("Get event", func(t *testing.T) {
			dbEvent, err := eventsDB.Get(ctx, event.UserID, event.Type)
			require.NoError(t, err)
			require.NotNil(t, dbEvent)
			dbEvent.CreatedAt = event.CreatedAt
			require.Equal(t, event, dbEvent)
		})

		t.Run("Update event limits", func(t *testing.T) {
			event.Limits = &console.AccountFreezeEventLimits{
				User: randUsageLimits(),
				Projects: map[uuid.UUID]console.UsageLimits{
					testrand.UUID(): randUsageLimits(),
				},
			}

			_, err := eventsDB.Upsert(ctx, event)
			require.NoError(t, err)
			dbEvent, err := eventsDB.Get(ctx, event.UserID, event.Type)
			require.NoError(t, err)
			require.Equal(t, event.Limits, dbEvent.Limits)

			event.Limits = nil
			_, err = eventsDB.Upsert(ctx, event)
			require.NoError(t, err)
			dbEvent, err = eventsDB.Get(ctx, event.UserID, event.Type)
			require.NoError(t, err)
			require.Nil(t, dbEvent.Limits)
		})

		t.Run("Delete event", func(t *testing.T) {
			require.NoError(t, eventsDB.DeleteAllByUserID(ctx, event.UserID))
			_, err := eventsDB.Get(ctx, event.UserID, event.Type)
			require.ErrorIs(t, err, sql.ErrNoRows)
		})

		t.Run("GetAll must return bot freeze events", func(t *testing.T) {
			botEvent := &console.AccountFreezeEvent{
				UserID:             userID,
				Type:               console.DelayedBotFreeze,
				DaysTillEscalation: &days,
			}

			dbEvent, err := eventsDB.Upsert(ctx, botEvent)
			require.NoError(t, err)
			require.NotNil(t, dbEvent)

			botEvent.Type = console.BotFreeze

			dbEvent, err = eventsDB.Upsert(ctx, botEvent)
			require.NoError(t, err)
			require.NotNil(t, dbEvent)

			events, err := eventsDB.GetAll(ctx, userID)
			require.NoError(t, err)
			require.NotNil(t, events)
			require.Nil(t, events.BillingFreeze)
			require.Nil(t, events.BillingWarning)
			require.Nil(t, events.ViolationFreeze)
			require.Nil(t, events.LegalFreeze)
			require.NotNil(t, events.DelayedBotFreeze)
			require.NotNil(t, events.BotFreeze)
		})

		t.Run("Increment notifications count", func(t *testing.T) {
			event := &console.AccountFreezeEvent{
				UserID: userID,
				Type:   console.BillingFreeze,
			}

			dbEvent, err := eventsDB.Upsert(ctx, event)
			require.NoError(t, err)
			require.Equal(t, 0, dbEvent.NotificationsCount)

			err = eventsDB.IncrementNotificationsCount(ctx, userID, console.BillingFreeze)
			require.NoError(t, err)

			dbEvent, err = eventsDB.Get(ctx, userID, console.BillingFreeze)
			require.NoError(t, err)
			require.Equal(t, 1, dbEvent.NotificationsCount)
		})
	})
}
