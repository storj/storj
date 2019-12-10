// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package notifications

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/pkg/storj"
)

// DB tells how application works with notifications database.
//
// architecture: Database
type DB interface {
	Insert(ctx context.Context, notification NewNotification) (Notification, error)
	List(ctx context.Context, cursor NotificationCursor) (NotificationPage, error)
	Read(ctx context.Context, notificationID uuid.UUID) error
	ReadAll(ctx context.Context) error
}

// NotificationType is a numeric value of specific notification type.
type NotificationType int

const (
	// NotificationTypeCustom is a common notification type which doesn't describe node's core functionality.
	// TODO: change type name when all notification types will be known
	NotificationTypeCustom NotificationType = 0
	// NotificationTypeAuditCheckFailure is a notification type which describes node's audit check failure.
	NotificationTypeAuditCheckFailure NotificationType = 1
	// NotificationTypeUptimeCheckFailure is a notification type which describes node's uptime check failure.
	NotificationTypeUptimeCheckFailure NotificationType = 2
	// NotificationTypeDisqualification is a notification type which describes node's disqualification status.
	NotificationTypeDisqualification NotificationType = 3
)

// NewNotification holds notification entity info which is being received from satellite or local client.
type NewNotification struct {
	SenderID storj.NodeID
	Type     NotificationType
	Title    string
	Message  string
}

// Notification holds notification entity info which is being retrieved from database.
type Notification struct {
	ID        uuid.UUID
	SenderID  storj.NodeID
	Type      NotificationType
	Title     string
	Message   string
	ReadAt    *time.Time
	CreatedAt time.Time
}

// NotificationCursor holds notification cursor entity which is used to create listed page from database.
type NotificationCursor struct {
	Limit uint
	Page  uint
}

// NotificationPage holds notification page entity which is used to show listed page of notifications on UI.
type NotificationPage struct {
	Notifications []Notification

	Offset      uint64
	Limit       uint
	CurrentPage uint
	PageCount   uint
	TotalCount  uint64
}
