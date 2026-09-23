// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/storagenode/notifications"
)

// ensures that notificationDB implements notifications.Notifications interface.
var _ notifications.DB = (*notificationDB)(nil)

// NotificationsDBName represents the database name.
const NotificationsDBName = "notifications"

// ErrNotificationsDB represents errors from the notifications database.
var ErrNotificationsDB = errs.Class("notificationsdb")

// notificationDB is an implementation of notifications.Notifications.
//
// architecture: Database
type notificationDB struct {
	dbContainerImpl
}

// Insert puts new notification to database.
func (db *notificationDB) Insert(ctx context.Context, notification notifications.NewNotification) (_ notifications.Notification, err error) {
	defer mon.Task()(&ctx, notification)(&err)

	id, err := uuid.New()
	if err != nil {
		return notifications.Notification{}, ErrNotificationsDB.Wrap(err)
	}

	createdAt := time.Now().UTC()

	query := `
		INSERT INTO
			notifications (id, sender_id, type, title, message, link, link_label, created_at)
		VALUES
			(?, ?, ?, ?, ?, ?, ?, ?);
	`

	_, err = db.ExecContext(ctx, query, id[:], notification.SenderID[:], notification.Type, notification.Title, notification.Message, notification.Link, notification.LinkLabel, createdAt)
	if err != nil {
		return notifications.Notification{}, ErrNotificationsDB.Wrap(err)
	}

	return notifications.Notification{
		ID:        id,
		SenderID:  notification.SenderID,
		Type:      notification.Type,
		Title:     notification.Title,
		Message:   notification.Message,
		Link:      notification.Link,
		LinkLabel: notification.LinkLabel,
		ReadAt:    nil,
		CreatedAt: createdAt,
	}, nil
}

// Upsert stores a notification under a caller provided, stable ID. When a
// notification with that ID already exists, only the content is updated: read_at
// and created_at are left untouched, so correcting a message or a broken link
// doesn't re-alert an operator who already read it.
func (db *notificationDB) Upsert(ctx context.Context, id uuid.UUID, notification notifications.NewNotification) (err error) {
	defer mon.Task()(&ctx, notification)(&err)

	query := `
		INSERT INTO
			notifications (id, sender_id, type, title, message, link, link_label, created_at)
		VALUES
			(?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			message = excluded.message,
			link = excluded.link,
			link_label = excluded.link_label;
	`

	_, err = db.ExecContext(ctx, query, id[:], notification.SenderID[:], notification.Type, notification.Title, notification.Message, notification.Link, notification.LinkLabel, time.Now().UTC())
	return ErrNotificationsDB.Wrap(err)
}

// List returns listed page of notifications from database.
func (db *notificationDB) List(ctx context.Context, cursor notifications.Cursor) (_ notifications.Page, err error) {
	defer mon.Task()(&ctx, cursor)(&err)

	if cursor.Limit > 50 {
		cursor.Limit = 50
	}

	if cursor.Page == 0 {
		return notifications.Page{}, ErrNotificationsDB.Wrap(errs.New("page can not be 0"))
	}

	page := notifications.Page{
		Limit:  cursor.Limit,
		Offset: uint64((cursor.Page - 1) * cursor.Limit),
	}

	countQuery := `
		SELECT 
			COUNT(id)
		FROM 
			notifications
	`

	err = db.QueryRowContext(ctx, countQuery).Scan(&page.TotalCount)
	if err != nil {
		return notifications.Page{}, ErrNotificationsDB.Wrap(err)
	}
	if page.TotalCount == 0 {
		return page, nil
	}
	if page.Offset > page.TotalCount-1 {
		return notifications.Page{}, ErrNotificationsDB.Wrap(errs.New("page is out of range"))
	}

	// The columns are listed explicitly: SELECT * returns them in table order, which
	// puts columns added by a migration at the end, breaking the positional scan below.
	query := `
		SELECT
			id, sender_id, type, title, message, link, link_label, read_at, created_at
		FROM
			notifications
		ORDER BY
			created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := db.QueryContext(ctx, query, page.Limit, page.Offset)
	if err != nil {
		return notifications.Page{}, ErrNotificationsDB.Wrap(err)
	}

	defer func() {
		err = errs.Combine(err, ErrNotificationsDB.Wrap(rows.Close()))
	}()

	for rows.Next() {
		notification := notifications.Notification{}

		err = rows.Scan(
			&notification.ID,
			&notification.SenderID,
			&notification.Type,
			&notification.Title,
			&notification.Message,
			&notification.Link,
			&notification.LinkLabel,
			&notification.ReadAt,
			&notification.CreatedAt,
		)
		if err = rows.Err(); err != nil {
			return notifications.Page{}, ErrNotificationsDB.Wrap(err)
		}

		page.Notifications = append(page.Notifications, notification)
	}

	page.PageCount = uint(page.TotalCount / uint64(cursor.Limit))
	if page.TotalCount%uint64(cursor.Limit) != 0 {
		page.PageCount++
	}

	page.CurrentPage = cursor.Page

	return page, ErrNotificationsDB.Wrap(rows.Err())
}

// Read updates specific notification in database as read.
func (db *notificationDB) Read(ctx context.Context, notificationID uuid.UUID) (err error) {
	defer mon.Task()(&ctx, notificationID)(&err)

	query := `
		UPDATE
			notifications
		SET
			read_at = ?
		WHERE
			id = ?;
	`
	result, err := db.ExecContext(ctx, query, time.Now().UTC(), notificationID[:])
	if err != nil {
		return ErrNotificationsDB.Wrap(err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return ErrNotificationsDB.Wrap(err)
	}
	if rowsAffected != 1 {
		return ErrNotificationsDB.Wrap(ErrNoRows)
	}

	return nil
}

// ReadAll updates all notifications in database as read.
func (db *notificationDB) ReadAll(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	query := `
		UPDATE
			notifications
		SET
			read_at = ?
		WHERE 
			read_at IS NULL;
	`

	_, err = db.ExecContext(ctx, query, time.Now().UTC())

	return ErrNotificationsDB.Wrap(err)
}

// UnreadAmount returns amount on unread notifications.
func (db *notificationDB) UnreadAmount(ctx context.Context) (_ int, err error) {
	defer mon.Task()(&ctx)(&err)
	var amount int

	query := `
		SELECT
			COUNT(id)
		FROM 
			notifications
		WHERE 
			read_at IS NULL
	`

	err = db.QueryRowContext(ctx, query).Scan(&amount)
	if err != nil {
		return 0, ErrNotificationsDB.Wrap(err)
	}

	return amount, nil
}
