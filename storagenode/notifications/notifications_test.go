// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package notifications_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/identity/testidentity"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/notifications"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestNotificationsDB(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		notificationsdb := db.Notifications()

		satellite0 := testidentity.MustPregeneratedSignedIdentity(0, storj.LatestIDVersion()).ID
		satellite1 := testidentity.MustPregeneratedSignedIdentity(1, storj.LatestIDVersion()).ID
		satellite2 := testidentity.MustPregeneratedSignedIdentity(2, storj.LatestIDVersion()).ID

		expectedNotification0 := notifications.NewNotification{
			SenderID: satellite0,
			Type:     0,
			Title:    "testTitle0",
			Message:  "testMessage0",
		}
		expectedNotification1 := notifications.NewNotification{
			SenderID: satellite1,
			Type:     1,
			Title:    "testTitle1",
			Message:  "testMessage1",
		}
		expectedNotification2 := notifications.NewNotification{
			SenderID: satellite2,
			Type:     2,
			Title:    "testTitle2",
			Message:  "testMessage2",
		}

		notificationCursor := notifications.Cursor{
			Limit: 2,
			Page:  1,
		}

		notificationFromDB0, err := notificationsdb.Insert(ctx, expectedNotification0)
		require.NoError(t, err)
		require.Equal(t, expectedNotification0.SenderID, notificationFromDB0.SenderID)
		require.Equal(t, expectedNotification0.Type, notificationFromDB0.Type)
		require.Equal(t, expectedNotification0.Title, notificationFromDB0.Title)
		require.Equal(t, expectedNotification0.Message, notificationFromDB0.Message)
		// Ensure that every insert gets a different "created at" time.
		waitForTimeToChange()

		notificationFromDB1, err := notificationsdb.Insert(ctx, expectedNotification1)
		require.NoError(t, err)
		require.Equal(t, expectedNotification1.SenderID, notificationFromDB1.SenderID)
		require.Equal(t, expectedNotification1.Type, notificationFromDB1.Type)
		require.Equal(t, expectedNotification1.Title, notificationFromDB1.Title)
		require.Equal(t, expectedNotification1.Message, notificationFromDB1.Message)
		waitForTimeToChange()

		notificationFromDB2, err := notificationsdb.Insert(ctx, expectedNotification2)
		require.NoError(t, err)
		require.Equal(t, expectedNotification2.SenderID, notificationFromDB2.SenderID)
		require.Equal(t, expectedNotification2.Type, notificationFromDB2.Type)
		require.Equal(t, expectedNotification2.Title, notificationFromDB2.Title)
		require.Equal(t, expectedNotification2.Message, notificationFromDB2.Message)

		page := notifications.Page{}

		// test List method to return right form of page depending on cursor.
		t.Run("paged list", func(t *testing.T) {
			page, err = notificationsdb.List(ctx, notificationCursor)
			require.NoError(t, err)
			require.Equal(t, 2, len(page.Notifications))
			require.Equal(t, notificationFromDB1, page.Notifications[1])
			require.Equal(t, notificationFromDB2, page.Notifications[0])
			require.Equal(t, notificationCursor.Limit, page.Limit)
			require.Equal(t, uint64(0), page.Offset)
			require.Equal(t, uint(2), page.PageCount)
			require.Equal(t, uint64(3), page.TotalCount)
			require.Equal(t, uint(1), page.CurrentPage)
		})

		notificationCursor = notifications.Cursor{
			Limit: 5,
			Page:  1,
		}

		// test Read method to make specific notification's status as read.
		t.Run("notification read", func(t *testing.T) {
			err = notificationsdb.Read(ctx, notificationFromDB0.ID)
			require.NoError(t, err)

			page, err = notificationsdb.List(ctx, notificationCursor)
			require.NoError(t, err)
			require.NotEqual(t, page.Notifications[2].ReadAt, (*time.Time)(nil))

			err = notificationsdb.Read(ctx, notificationFromDB1.ID)
			require.NoError(t, err)

			page, err = notificationsdb.List(ctx, notificationCursor)
			require.NoError(t, err)
			require.NotEqual(t, page.Notifications[1].ReadAt, (*time.Time)(nil))

			require.Equal(t, page.Notifications[0].ReadAt, (*time.Time)(nil))
		})

		// test ReadAll method to make all notifications' status as read.
		t.Run("notification read all", func(t *testing.T) {
			err = notificationsdb.ReadAll(ctx)
			require.NoError(t, err)

			page, err = notificationsdb.List(ctx, notificationCursor)
			require.NoError(t, err)
			require.NotEqual(t, page.Notifications[2].ReadAt, (*time.Time)(nil))
			require.NotEqual(t, page.Notifications[1].ReadAt, (*time.Time)(nil))
			require.NotEqual(t, page.Notifications[0].ReadAt, (*time.Time)(nil))
		})
	})
}

func TestEmptyNotificationsDB(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		notificationsdb := db.Notifications()

		notificationCursor := notifications.Cursor{
			Limit: 5,
			Page:  1,
		}

		// test List method to return right form of page depending on cursor with empty database.
		t.Run("empty paged list", func(t *testing.T) {
			page, err := notificationsdb.List(ctx, notificationCursor)
			require.NoError(t, err)
			require.Equal(t, len(page.Notifications), 0)
			require.Equal(t, page.Limit, notificationCursor.Limit)
			require.Equal(t, page.Offset, uint64(0))
			require.Equal(t, page.PageCount, uint(0))
			require.Equal(t, page.TotalCount, uint64(0))
			require.Equal(t, page.CurrentPage, uint(0))
		})

		// test notification read with not existing id.
		t.Run("notification read with not existing id", func(t *testing.T) {
			err := notificationsdb.Read(ctx, testrand.UUID())
			require.Error(t, err, "no rows affected")
		})

		// test read for all notifications if they don't exist.
		t.Run("notification readAll on empty page", func(t *testing.T) {
			err := notificationsdb.ReadAll(ctx)
			require.NoError(t, err)
		})
	})
}

func waitForTimeToChange() {
	t := time.Now()
	for time.Since(t) == 0 {
		time.Sleep(100 * time.Millisecond)
	}
}
