// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package projectlimitevents_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/private/post"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/mailservice"
)

var errMailFailure = errors.New("smtp unavailable")

// captureSender records all rendered emails without actually sending them.
type captureSender struct {
	mu   sync.Mutex
	sent []mailservice.Message
	err  error
}

func (s *captureSender) SendRendered(_ context.Context, _ []post.Address, msg mailservice.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.sent = append(s.sent, msg)
	return nil
}

func (s *captureSender) templates() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, m := range s.sent {
		out = append(out, m.Template())
	}
	return out
}

func (s *captureSender) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = nil
	s.err = nil
}

func TestProjectLimitEventsChore(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.ProjectLimitEvents.Enabled = true
				config.ProjectLimitEvents.EmailTimeBuffer = 0
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		chore := sat.ProjectLimitEvents.Chore
		eventsDB := sat.ProjectLimitEvents.DB
		projectsDB := sat.DB.Console().Projects()
		liveCache := sat.LiveAccounting.Cache

		chore.Loop.Pause()

		sender := &captureSender{}
		chore.TestSetMailSender(sender)
		// Advance now so all inserted events are older than EmailTimeBuffer=0.
		chore.TestSetNow(func() time.Time { return time.Now().Add(time.Minute) })

		projectID := planet.Uplinks[0].Projects[0].ID
		storageLimit := 100 * memory.MiB
		bandwidthLimit := 100 * memory.MiB

		// Set limits on the project.
		require.NoError(t, sat.DB.ProjectAccounting().UpdateProjectUsageLimit(ctx, projectID, storageLimit))
		require.NoError(t, sat.DB.ProjectAccounting().UpdateProjectBandwidthLimit(ctx, projectID, bandwidthLimit))

		requireDBFlags := func(t *testing.T, expected accounting.ProjectUsageThreshold) {
			t.Helper()
			p, err := projectsDB.Get(ctx, projectID)
			require.NoError(t, err)
			require.NotNil(t, p.NotificationFlags)
			require.Equal(t, int(expected), *p.NotificationFlags)
		}

		requireRedisFlags := func(t *testing.T, expected accounting.ProjectUsageThreshold) {
			t.Helper()
			flags, err := liveCache.GetProjectNotificationFlags(ctx, projectID)
			require.NoError(t, err)
			require.Equal(t, int(expected), flags)
		}

		requireEventProcessed := func(t *testing.T, id uuid.UUID) {
			t.Helper()
			e, err := eventsDB.GetByID(ctx, id)
			require.NoError(t, err)
			require.NotNil(t, e.EmailSent)
		}

		// Enable both storage and egress notifications for the project.
		enableNotifications := func(flags accounting.ProjectUsageThreshold) {
			project, err := projectsDB.Get(ctx, projectID)
			require.NoError(t, err)
			f := int(flags)
			project.NotificationFlags = &f
			require.NoError(t, projectsDB.Update(ctx, project))
		}

		triggerAndDrain := func(t *testing.T) {
			t.Helper()
			chore.Loop.TriggerWait()
		}

		t.Run("storage 80% sends email and sets flag", func(t *testing.T) {
			defer sender.reset()
			enableNotifications(accounting.StorageNotificationsEnabled)

			event, err := eventsDB.Insert(ctx, projectID, accounting.StorageUsage80, false)
			require.NoError(t, err)

			triggerAndDrain(t)

			require.Equal(t, []string{"ProjectStorageUsage80"}, sender.templates())
			expectedFlags := accounting.StorageNotificationsEnabled | accounting.StorageUsage80
			requireDBFlags(t, expectedFlags)
			requireRedisFlags(t, expectedFlags)
			requireEventProcessed(t, event.ID)
		})

		t.Run("dedup: two events for same threshold → one email", func(t *testing.T) {
			defer sender.reset()
			enableNotifications(accounting.StorageNotificationsEnabled)

			_, err := eventsDB.Insert(ctx, projectID, accounting.StorageUsage80, false)
			require.NoError(t, err)
			_, err = eventsDB.Insert(ctx, projectID, accounting.StorageUsage80, false)
			require.NoError(t, err)

			triggerAndDrain(t)

			require.Equal(t, []string{"ProjectStorageUsage80"}, sender.templates())
		})

		t.Run("both 80% and 100% in batch → only 100% email, both bits set", func(t *testing.T) {
			defer sender.reset()
			enableNotifications(accounting.StorageNotificationsEnabled)

			_, err := eventsDB.Insert(ctx, projectID, accounting.StorageUsage80, false)
			require.NoError(t, err)
			_, err = eventsDB.Insert(ctx, projectID, accounting.StorageUsage100, false)
			require.NoError(t, err)

			triggerAndDrain(t)

			require.Equal(t, []string{"ProjectStorageUsage100"}, sender.templates())

			p, err := projectsDB.Get(ctx, projectID)
			require.NoError(t, err)
			expected := int(accounting.StorageNotificationsEnabled | accounting.StorageUsage80 | accounting.StorageUsage100)
			require.Equal(t, expected, *p.NotificationFlags)
		})

		t.Run("feature flag off → no email, event marked processed", func(t *testing.T) {
			defer sender.reset()
			enableNotifications(0) // notifications disabled

			event, err := eventsDB.Insert(ctx, projectID, accounting.StorageUsage80, false)
			require.NoError(t, err)

			triggerAndDrain(t)

			require.Empty(t, sender.templates())

			e, err := eventsDB.GetByID(ctx, event.ID)
			require.NoError(t, err)
			require.NotNil(t, e.EmailSent)
		})

		t.Run("email sent bit already set → dedup across chore runs", func(t *testing.T) {
			defer sender.reset()
			enableNotifications(accounting.StorageNotificationsEnabled | accounting.StorageUsage80)

			event, err := eventsDB.Insert(ctx, projectID, accounting.StorageUsage80, false)
			require.NoError(t, err)

			triggerAndDrain(t)

			require.Empty(t, sender.templates())

			e, err := eventsDB.GetByID(ctx, event.ID)
			require.NoError(t, err)
			require.NotNil(t, e.EmailSent)
		})

		t.Run("reset event clears flag, re-notification works", func(t *testing.T) {
			defer sender.reset()
			// Start with storage 80% email already sent.
			enableNotifications(accounting.StorageNotificationsEnabled | accounting.StorageUsage80)

			// Storage drops below 80% — enqueue reset.
			_, err := eventsDB.Insert(ctx, projectID, accounting.StorageUsage80, true)
			require.NoError(t, err)

			triggerAndDrain(t)

			require.Empty(t, sender.templates())

			p, err := projectsDB.Get(ctx, projectID)
			require.NoError(t, err)
			require.Equal(t, int(accounting.StorageNotificationsEnabled), *p.NotificationFlags)

			// Storage crosses 80% again — new email must be sent.
			_, err = eventsDB.Insert(ctx, projectID, accounting.StorageUsage80, false)
			require.NoError(t, err)

			triggerAndDrain(t)

			require.Equal(t, []string{"ProjectStorageUsage80"}, sender.templates())
		})

		t.Run("mail failure updates last_attempted, event stays in queue", func(t *testing.T) {
			defer sender.reset()
			enableNotifications(accounting.StorageNotificationsEnabled)
			sender.err = errMailFailure

			event, err := eventsDB.Insert(ctx, projectID, accounting.StorageUsage80, false)
			require.NoError(t, err)

			triggerAndDrain(t)

			e, err := eventsDB.GetByID(ctx, event.ID)
			require.NoError(t, err)
			require.Nil(t, e.EmailSent)
			require.NotNil(t, e.LastAttempted)
		})

		t.Run("reset event last_attempted not set on mail failure", func(t *testing.T) {
			defer sender.reset()
			enableNotifications(accounting.StorageNotificationsEnabled | accounting.EgressUsage80)
			sender.err = errMailFailure

			thresholdEvent, err := eventsDB.Insert(ctx, projectID, accounting.StorageUsage80, false)
			require.NoError(t, err)
			resetEvent, err := eventsDB.Insert(ctx, projectID, accounting.EgressUsage80, true)
			require.NoError(t, err)

			triggerAndDrain(t)

			te, err := eventsDB.GetByID(ctx, thresholdEvent.ID)
			require.NoError(t, err)
			require.NotNil(t, te.LastAttempted)

			re, err := eventsDB.GetByID(ctx, resetEvent.ID)
			require.NoError(t, err)
			require.Nil(t, re.LastAttempted)
		})
	})
}
