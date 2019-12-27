// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package notifications_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"storj.io/common/identity/testidentity"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/notifications"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestNotificationsDB(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

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
		assert.NoError(t, err)
		assert.Equal(t, expectedNotification0.SenderID, notificationFromDB0.SenderID)
		assert.Equal(t, expectedNotification0.Type, notificationFromDB0.Type)
		assert.Equal(t, expectedNotification0.Title, notificationFromDB0.Title)
		assert.Equal(t, expectedNotification0.Message, notificationFromDB0.Message)

		notificationFromDB1, err := notificationsdb.Insert(ctx, expectedNotification1)
		assert.NoError(t, err)
		assert.Equal(t, expectedNotification1.SenderID, notificationFromDB1.SenderID)
		assert.Equal(t, expectedNotification1.Type, notificationFromDB1.Type)
		assert.Equal(t, expectedNotification1.Title, notificationFromDB1.Title)
		assert.Equal(t, expectedNotification1.Message, notificationFromDB1.Message)

		notificationFromDB2, err := notificationsdb.Insert(ctx, expectedNotification2)
		assert.NoError(t, err)
		assert.Equal(t, expectedNotification2.SenderID, notificationFromDB2.SenderID)
		assert.Equal(t, expectedNotification2.Type, notificationFromDB2.Type)
		assert.Equal(t, expectedNotification2.Title, notificationFromDB2.Title)
		assert.Equal(t, expectedNotification2.Message, notificationFromDB2.Message)

		page := notifications.Page{}

		// test List method to return right form of page depending on cursor.
		t.Run("test paged list", func(t *testing.T) {
			page, err = notificationsdb.List(ctx, notificationCursor)
			assert.NoError(t, err)
			assert.Equal(t, 2, len(page.Notifications))
			assert.Equal(t, notificationFromDB0, page.Notifications[0])
			assert.Equal(t, notificationFromDB1, page.Notifications[1])
			assert.Equal(t, notificationCursor.Limit, page.Limit)
			assert.Equal(t, uint64(0), page.Offset)
			assert.Equal(t, uint(2), page.PageCount)
			assert.Equal(t, uint64(3), page.TotalCount)
			assert.Equal(t, uint(1), page.CurrentPage)
		})

		notificationCursor = notifications.Cursor{
			Limit: 5,
			Page:  1,
		}

		// test Read method to make specific notification's status as read.
		t.Run("test notification read", func(t *testing.T) {
			err = notificationsdb.Read(ctx, notificationFromDB0.ID)
			assert.NoError(t, err)

			page, err = notificationsdb.List(ctx, notificationCursor)
			assert.NoError(t, err)
			assert.NotEqual(t, page.Notifications[0].ReadAt, (*time.Time)(nil))

			err = notificationsdb.Read(ctx, notificationFromDB1.ID)
			assert.NoError(t, err)

			page, err = notificationsdb.List(ctx, notificationCursor)
			assert.NoError(t, err)
			assert.NotEqual(t, page.Notifications[1].ReadAt, (*time.Time)(nil))

			assert.Equal(t, page.Notifications[2].ReadAt, (*time.Time)(nil))
		})

		// test ReadAll method to make all notifications' status as read.
		t.Run("test notification read all", func(t *testing.T) {
			err = notificationsdb.ReadAll(ctx)
			assert.NoError(t, err)

			page, err = notificationsdb.List(ctx, notificationCursor)
			assert.NoError(t, err)
			assert.NotEqual(t, page.Notifications[0].ReadAt, (*time.Time)(nil))
			assert.NotEqual(t, page.Notifications[1].ReadAt, (*time.Time)(nil))
			assert.NotEqual(t, page.Notifications[2].ReadAt, (*time.Time)(nil))
		})
	})
}

func TestEmptyNotificationsDB(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		notificationsdb := db.Notifications()

		notificationCursor := notifications.Cursor{
			Limit: 5,
			Page:  1,
		}

		// test List method to return right form of page depending on cursor with empty database.
		t.Run("test empty paged list", func(t *testing.T) {
			page, err := notificationsdb.List(ctx, notificationCursor)
			assert.NoError(t, err)
			assert.Equal(t, len(page.Notifications), 0)
			assert.Equal(t, page.Limit, notificationCursor.Limit)
			assert.Equal(t, page.Offset, uint64(0))
			assert.Equal(t, page.PageCount, uint(0))
			assert.Equal(t, page.TotalCount, uint64(0))
			assert.Equal(t, page.CurrentPage, uint(0))
		})

		// test notification read with not existing id.
		t.Run("test notification read with not existing id", func(t *testing.T) {
			err := notificationsdb.Read(ctx, testrand.UUID())
			assert.Error(t, err, "no rows affected")
		})

		// test read for all notifications if they don't exist.
		t.Run("test notification readAll on empty page", func(t *testing.T) {
			err := notificationsdb.ReadAll(ctx)
			assert.NoError(t, err)
		})
	})
}
