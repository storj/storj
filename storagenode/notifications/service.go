// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package notifications

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"
)

var (
	mon = monkit.Package()
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

	return notificationPage, nil
}
