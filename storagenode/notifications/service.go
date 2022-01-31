// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package notifications

import (
	"context"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/common/uuid"
)

var (
	mon = monkit.Package()
)

// TimesNotified is a numeric value of amount of notifications being sent to user.
type TimesNotified int

const (
	// TimesNotifiedZero haven't being notified yet.
	TimesNotifiedZero TimesNotified = 0
	// TimesNotifiedFirst sent notification one time.
	TimesNotifiedFirst TimesNotified = 1
	// TimesNotifiedSecond sent notifications twice.
	TimesNotifiedSecond TimesNotified = 2
	// TimesNotifiedLast three notification has been send.
	TimesNotifiedLast TimesNotified = 3
)

// Service is the notification service between storage nodes and satellites.
// architecture: Service
type Service struct {
	log *zap.Logger
	db  DB
}

// NewService creates a new notification service.
func NewService(log *zap.Logger, db DB) *Service {
	return &Service{
		log: log,
		db:  db,
	}
}

// Receive - receives notifications from satellite and Insert them into DB.
func (service *Service) Receive(ctx context.Context, newNotification NewNotification) (Notification, error) {
	notification, err := service.db.Insert(ctx, newNotification)
	if err != nil {
		return Notification{}, err
	}

	return notification, nil
}

// Read - change notification status to Read by ID.
func (service *Service) Read(ctx context.Context, notificationID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = service.db.Read(ctx, notificationID)
	if err != nil {
		return err
	}

	return nil
}

// ReadAll - change status of all user's notifications to Read.
func (service *Service) ReadAll(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = service.db.ReadAll(ctx)
	if err != nil {
		return err
	}

	return nil
}

// List - shows the list of paginated notifications.
func (service *Service) List(ctx context.Context, cursor Cursor) (_ Page, err error) {
	defer mon.Task()(&ctx)(&err)

	notificationPage, err := service.db.List(ctx, cursor)
	if err != nil {
		return Page{}, err
	}

	if notificationPage.Notifications == nil {
		notificationPage = Page{Notifications: []Notification{}}
	}

	return notificationPage, nil
}

// UnreadAmount - returns amount on notifications with value is_read = nil.
func (service *Service) UnreadAmount(ctx context.Context) (_ int, err error) {
	defer mon.Task()(&ctx)(&err)

	amount, err := service.db.UnreadAmount(ctx)
	if err != nil {
		return 0, err
	}

	return amount, nil
}
