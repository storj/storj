// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/projectlimitevents"
)

func TestProjectLimitThresholdEvents(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 4,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.LimitEmailNotificationsEnabled = true
				config.LiveAccounting.AsOfSystemInterval = 0
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		projectUsage := sat.Accounting.ProjectUsage
		acctDB := sat.DB.ProjectAccounting()
		eventsDB := sat.DB.ProjectLimitEvents()
		projectsDB := sat.DB.Console().Projects()

		// enableNotifications opts the project into receiving limit notification emails.
		// Must be called before the first API key use so the LRU caches the correct
		// LimitNotificationFlags value as a Redis-miss fallback.
		enableNotifications := func(t *testing.T, projectID uuid.UUID, flags accounting.ProjectUsageThreshold) {
			t.Helper()
			project, err := projectsDB.Get(ctx, projectID)
			require.NoError(t, err)
			f := int(flags)
			project.NotificationFlags = &f
			require.NoError(t, projectsDB.Update(ctx, project))
		}

		// drainEvents marks all pending events as sent so subsequent subtests start with a clean queue.
		drainEvents := func(t *testing.T) {
			t.Helper()
			for {
				batch, err := eventsDB.GetNextBatch(ctx, time.Now())
				require.NoError(t, err)
				if len(batch) == 0 {
					break
				}
				ids := make([]uuid.UUID, len(batch))
				for i, e := range batch {
					ids[i] = e.ID
				}
				require.NoError(t, eventsDB.UpdateEmailSent(ctx, ids, time.Now()))
			}
		}

		// hasEventType returns true if any event in the batch matches the given type and project.
		hasEventType := func(events []projectlimitevents.Event, projectID uuid.UUID, eventType accounting.ProjectUsageThreshold) bool {
			for _, e := range events {
				if e.ProjectID == projectID && e.Event == eventType {
					return true
				}
			}
			return false
		}

		t.Run("storage 80%", func(t *testing.T) {
			defer drainEvents(t)
			projectID := planet.Uplinks[0].Projects[0].ID

			enableNotifications(t, projectID, accounting.StorageNotificationsEnabled)

			storageLimit := 100 * memory.MiB
			require.NoError(t, acctDB.UpdateProjectUsageLimit(ctx, projectID, storageLimit))

			// Pre-fill to 1 byte below the 80% threshold so the next upload triggers it.
			below80 := storageLimit.Int64()*80/100 - 1
			require.NoError(t, projectUsage.UpdateProjectStorageAndSegmentUsage(ctx,
				accounting.ProjectLimits{ProjectID: projectID}, below80, 0))

			require.NoError(t, planet.Uplinks[0].Upload(ctx, sat, "bucket", "object", testrand.Bytes(memory.KiB)))

			events, err := eventsDB.GetNextBatch(ctx, time.Now())
			require.NoError(t, err)
			require.True(t, hasEventType(events, projectID, accounting.StorageUsage80))
			require.False(t, hasEventType(events, projectID, accounting.StorageUsage100))
		})

		t.Run("storage 100% only, skips 80%", func(t *testing.T) {
			defer drainEvents(t)
			projectID := planet.Uplinks[1].Projects[0].ID

			enableNotifications(t, projectID, accounting.StorageNotificationsEnabled)

			storageLimit := 100 * memory.MiB
			require.NoError(t, acctDB.UpdateProjectUsageLimit(ctx, projectID, storageLimit))

			// Pre-fill to 1 byte below 100%; neither 80% nor 100% flags are set.
			// The next upload should enqueue only the 100% event (highest threshold crossed first).
			below100 := storageLimit.Int64() - 1
			require.NoError(t, projectUsage.UpdateProjectStorageAndSegmentUsage(ctx,
				accounting.ProjectLimits{ProjectID: projectID}, below100, 0))

			require.NoError(t, planet.Uplinks[1].Upload(ctx, sat, "bucket", "object", testrand.Bytes(memory.KiB)))

			events, err := eventsDB.GetNextBatch(ctx, time.Now())
			require.NoError(t, err)
			require.True(t, hasEventType(events, projectID, accounting.StorageUsage100))
			require.False(t, hasEventType(events, projectID, accounting.StorageUsage80))
		})

		t.Run("bandwidth 80%", func(t *testing.T) {
			defer drainEvents(t)
			projectID := planet.Uplinks[2].Projects[0].ID

			enableNotifications(t, projectID, accounting.EgressNotificationsEnabled)

			// Set the bandwidth limit before the first API key use so the LRU cache
			// is populated with the correct limit and keyInfo.ProjectBandwidthLimit is non-nil.
			bandwidthLimit := 100 * memory.MiB
			require.NoError(t, acctDB.UpdateProjectBandwidthLimit(ctx, projectID, bandwidthLimit))

			// Upload an object so we have something to download.
			require.NoError(t, planet.Uplinks[2].Upload(ctx, sat, "bucket", "object", testrand.Bytes(memory.KiB)))

			// Pre-fill bandwidth to exactly the 80% threshold. Unlike storage, bandwidth threshold
			// detection checks current >= threshold at check-download time (before the download is
			// counted), so we need usage to already be at the threshold for the event to fire.
			bandwidthLimitBytes := bandwidthLimit.Int64()
			at80 := bandwidthLimitBytes * 80 / 100
			require.NoError(t, projectUsage.UpdateProjectBandwidthUsage(ctx,
				accounting.ProjectLimits{ProjectID: projectID, Bandwidth: &bandwidthLimitBytes}, at80))

			_, err := planet.Uplinks[2].Download(ctx, sat, "bucket", "object")
			require.NoError(t, err)

			events, err := eventsDB.GetNextBatch(ctx, time.Now())
			require.NoError(t, err)
			require.True(t, hasEventType(events, projectID, accounting.EgressUsage80))
			require.False(t, hasEventType(events, projectID, accounting.EgressUsage100))
		})

		t.Run("no events below threshold", func(t *testing.T) {
			projectID := planet.Uplinks[3].Projects[0].ID

			enableNotifications(t, projectID, accounting.StorageNotificationsEnabled|accounting.EgressNotificationsEnabled)

			storageLimit := 100 * memory.MiB
			require.NoError(t, acctDB.UpdateProjectUsageLimit(ctx, projectID, storageLimit))

			// Upload well below the 80% threshold — no event should be queued.
			require.NoError(t, planet.Uplinks[3].Upload(ctx, sat, "bucket", "object", testrand.Bytes(memory.KiB)))

			events, err := eventsDB.GetNextBatch(ctx, time.Now())
			require.NoError(t, err)
			require.Empty(t, events)
		})
	})
}
