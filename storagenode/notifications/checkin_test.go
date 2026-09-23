// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package notifications_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/notifications"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestReceiveAll(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		log := zaptest.NewLogger(t)
		service := notifications.NewService(log, db.Notifications())

		satellite := testrand.NodeID()
		all := notifications.Cursor{Limit: 50, Page: 1}

		received := []*pb.Notification{
			{
				Key: "maintenance", Title: "Scheduled maintenance", Message: "back in 2 hours",
				Link: "https://forum.storj.io/t/1", LinkLabel: "Forum thread",
			},
			{Key: "policy", Title: "Policy update", Message: "see the forum"},
		}

		require.NoError(t, notifications.ReceiveAll(ctx, log, service, satellite, received))

		page, err := service.List(ctx, all)
		require.NoError(t, err)
		require.Len(t, page.Notifications, 2)
		for _, n := range page.Notifications {
			require.Equal(t, satellite, n.SenderID)
			require.Equal(t, notifications.TypeCustom, n.Type)
		}

		byTitle := listByTitle(ctx, t, service, all)
		require.Equal(t, "https://forum.storj.io/t/1", byTitle["Scheduled maintenance"].Link)
		require.Equal(t, "Forum thread", byTitle["Scheduled maintenance"].LinkLabel)
		require.Empty(t, byTitle["Policy update"].Link, "a notification without a link stays empty")

		// Check-in repeats hourly, so the same payload arrives over and over.
		t.Run("redelivery does not duplicate", func(t *testing.T) {
			require.NoError(t, notifications.ReceiveAll(ctx, log, service, satellite, received))

			page, err := service.List(ctx, all)
			require.NoError(t, err)
			require.Len(t, page.Notifications, 2)
		})

		t.Run("notification without a key is ignored", func(t *testing.T) {
			require.NoError(t, notifications.ReceiveAll(ctx, log, service, satellite, []*pb.Notification{
				{Title: "no key", Message: "dropped"},
			}))

			page, err := service.List(ctx, all)
			require.NoError(t, err)
			require.Len(t, page.Notifications, 2)
		})

		t.Run("a keyless notification does not hide the usable ones", func(t *testing.T) {
			require.NoError(t, notifications.ReceiveAll(ctx, log, service, satellite, []*pb.Notification{
				{Title: "no key", Message: "dropped"},
				{Key: "third", Title: "Third", Message: "stored"},
			}))

			page, err := service.List(ctx, all)
			require.NoError(t, err)
			require.Len(t, page.Notifications, 3)
		})

		t.Run("empty payload is not an error", func(t *testing.T) {
			require.NoError(t, notifications.ReceiveAll(ctx, log, service, satellite, nil))
		})
	})
}
