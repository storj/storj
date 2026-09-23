// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package notifications

import (
	"context"
	"crypto/sha256"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/common/storj"
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

// Custom is a notification defined in the configuration of a satellite.
type Custom struct {
	// Key identifies the notification within the sending satellite.
	Key       string
	Title     string
	Message   string
	Link      string
	LinkLabel string
}

// ReceiveCustom stores a custom notification defined in the configuration of the
// sending satellite. Satellites re-send their notifications on every check-in, so
// the notification is stored under an ID derived from the sender and the key,
// making repeated delivery update the existing notification instead of creating
// a new one.
func (service *Service) ReceiveCustom(ctx context.Context, senderID storj.NodeID, custom Custom) (err error) {
	defer mon.Task()(&ctx)(&err)

	return service.db.Upsert(ctx, notificationID(senderID, custom.Key), NewNotification{
		SenderID:  senderID,
		Type:      TypeCustom,
		Title:     custom.Title,
		Message:   custom.Message,
		Link:      custom.Link,
		LinkLabel: custom.LinkLabel,
	})
}

// notificationID derives a stable UUID from the sender and the key. The sender is
// a fixed width prefix, so the key can't shift into it and collide with a
// notification from a different satellite.
func notificationID(senderID storj.NodeID, key string) uuid.UUID {
	hash := sha256.New()
	_, _ = hash.Write(senderID[:])
	_, _ = hash.Write([]byte(key))

	var id uuid.UUID
	copy(id[:], hash.Sum(nil))
	id[6] = (id[6] & 0x0f) | 0x50 // version 5
	id[8] = (id[8] & 0x3f) | 0x80 // RFC 4122 variant
	return id
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
