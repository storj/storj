// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package notifications_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/notifications"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestUpsert(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		notificationsdb := db.Notifications()

		satellite := testrand.NodeID()
		id, err := uuid.New()
		require.NoError(t, err)

		all := notifications.Cursor{Limit: 50, Page: 1}

		require.NoError(t, notificationsdb.Upsert(ctx, id, notifications.NewNotification{
			SenderID: satellite,
			Type:     notifications.TypeCustom,
			Title:    "original title",
			Message:  "original message",
		}))

		page, err := notificationsdb.List(ctx, all)
		require.NoError(t, err)
		require.Len(t, page.Notifications, 1)
		createdAt := page.Notifications[0].CreatedAt

		// Repeated delivery of the same notification must not accumulate rows,
		// which is what would happen on every check-in otherwise.
		waitForTimeToChange()
		require.NoError(t, notificationsdb.Upsert(ctx, id, notifications.NewNotification{
			SenderID: satellite,
			Type:     notifications.TypeCustom,
			Title:    "corrected title",
			Message:  "corrected message",
		}))

		page, err = notificationsdb.List(ctx, all)
		require.NoError(t, err)
		require.Len(t, page.Notifications, 1)
		require.Equal(t, "corrected title", page.Notifications[0].Title)
		require.Equal(t, "corrected message", page.Notifications[0].Message)
		require.Equal(t, createdAt, page.Notifications[0].CreatedAt,
			"created_at must survive an update so the received time stays honest")

		t.Run("update does not clear read status", func(t *testing.T) {
			require.NoError(t, notificationsdb.Read(ctx, id))

			require.NoError(t, notificationsdb.Upsert(ctx, id, notifications.NewNotification{
				SenderID: satellite,
				Type:     notifications.TypeCustom,
				Title:    "second correction",
				Message:  "second correction",
			}))

			page, err := notificationsdb.List(ctx, all)
			require.NoError(t, err)
			require.Len(t, page.Notifications, 1)
			require.NotNil(t, page.Notifications[0].ReadAt,
				"editing a message must not re-alert an operator who already read it")
		})

		t.Run("distinct ids stay distinct", func(t *testing.T) {
			other, err := uuid.New()
			require.NoError(t, err)

			require.NoError(t, notificationsdb.Upsert(ctx, other, notifications.NewNotification{
				SenderID: testrand.NodeID(),
				Type:     notifications.TypeCustom,
				Title:    "from another satellite",
				Message:  "from another satellite",
			}))

			page, err := notificationsdb.List(ctx, all)
			require.NoError(t, err)
			require.Len(t, page.Notifications, 2)
		})
	})
}

func TestReceiveCustom(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		service := notifications.NewService(zaptest.NewLogger(t), db.Notifications())

		satellite0 := testrand.NodeID()
		satellite1 := testrand.NodeID()

		all := notifications.Cursor{Limit: 50, Page: 1}

		require.NoError(t, service.ReceiveCustom(ctx, satellite0, notifications.Custom{Key: "maintenance", Title: "title", Message: "message"}))
		require.NoError(t, service.ReceiveCustom(ctx, satellite0, notifications.Custom{Key: "maintenance", Title: "title", Message: "message"}))

		page, err := service.List(ctx, all)
		require.NoError(t, err)
		require.Len(t, page.Notifications, 1, "re-delivery of the same key must upsert")
		require.Equal(t, satellite0, page.Notifications[0].SenderID)
		require.Equal(t, notifications.TypeCustom, page.Notifications[0].Type)

		t.Run("same key from a different satellite is a different notification", func(t *testing.T) {
			require.NoError(t, service.ReceiveCustom(ctx, satellite1, notifications.Custom{Key: "maintenance", Title: "title", Message: "message"}))

			page, err := service.List(ctx, all)
			require.NoError(t, err)
			require.Len(t, page.Notifications, 2)
		})

		t.Run("a new key is a new notification", func(t *testing.T) {
			require.NoError(t, service.ReceiveCustom(ctx, satellite0, notifications.Custom{Key: "policy-change", Title: "title", Message: "message"}))

			page, err := service.List(ctx, all)
			require.NoError(t, err)
			require.Len(t, page.Notifications, 3)
		})

		t.Run("link is stored and updated in place", func(t *testing.T) {
			require.NoError(t, service.ReceiveCustom(ctx, satellite0, notifications.Custom{
				Key: "linked", Title: "Linked", Message: "body",
				Link: "https://forum.storj.io/t/1", LinkLabel: "Forum thread",
			}))

			stored := listByTitle(ctx, t, service, all)["Linked"]
			require.Equal(t, "https://forum.storj.io/t/1", stored.Link)
			require.Equal(t, "Forum thread", stored.LinkLabel)

			// A broken link is corrected the same way a typo is.
			require.NoError(t, service.ReceiveCustom(ctx, satellite0, notifications.Custom{
				Key: "linked", Title: "Linked", Message: "body",
				Link: "https://forum.storj.io/t/2", LinkLabel: "Corrected thread",
			}))

			stored = listByTitle(ctx, t, service, all)["Linked"]
			require.Equal(t, "https://forum.storj.io/t/2", stored.Link)
			require.Equal(t, "Corrected thread", stored.LinkLabel)
		})

		t.Run("notification without a link has empty link fields", func(t *testing.T) {
			stored := listByTitle(ctx, t, service, all)["title"]
			require.Empty(t, stored.Link)
			require.Empty(t, stored.LinkLabel)
		})

		t.Run("edited text reaches the stored notification", func(t *testing.T) {
			require.NoError(t, service.ReceiveCustom(ctx, satellite0, notifications.Custom{Key: "maintenance", Title: "fixed", Message: "fixed body"}))

			page, err := service.List(ctx, all)
			require.NoError(t, err)

			var found bool
			for _, n := range page.Notifications {
				if n.SenderID == satellite0 && n.Title == "fixed" {
					require.Equal(t, "fixed body", n.Message)
					found = true
				}
			}
			require.True(t, found)
		})
	})
}

// listByTitle indexes the stored notifications by title, to keep the assertions
// readable when a test stores more than one notification.
func listByTitle(ctx context.Context, t *testing.T, service *notifications.Service, cursor notifications.Cursor) map[string]notifications.Notification {
	t.Helper()

	page, err := service.List(ctx, cursor)
	require.NoError(t, err)

	byTitle := make(map[string]notifications.Notification, len(page.Notifications))
	for _, notification := range page.Notifications {
		byTitle[notification.Title] = notification
	}
	return byTitle
}
