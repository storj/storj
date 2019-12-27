// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package notifications

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/common/storj"
)

// DB tells how application works with notifications database.
//
// architecture: Database
type DB interface {
	Insert(ctx context.Context, notification NewNotification) (Notification, error)
	List(ctx context.Context, cursor Cursor) (Page, error)
	Read(ctx context.Context, notificationID uuid.UUID) error
	ReadAll(ctx context.Context) error
}

// Type is a numeric value of specific notification type.
type Type int

const (
	// TypeCustom is a common notification type which doesn't describe node's core functionality.
	// TODO: change type name when all notification types will be known
	TypeCustom Type = 0
	// TypeAuditCheckFailure is a notification type which describes node's audit check failure.
	TypeAuditCheckFailure Type = 1
	// TypeUptimeCheckFailure is a notification type which describes node's uptime check failure.
	TypeUptimeCheckFailure Type = 2
	// TypeDisqualification is a notification type which describes node's disqualification status.
	TypeDisqualification Type = 3
)

// NewNotification holds notification entity info which is being received from satellite or local client.
type NewNotification struct {
	SenderID storj.NodeID
	Type     Type
	Title    string
	Message  string
}

// Notification holds notification entity info which is being retrieved from database.
type Notification struct {
	ID        uuid.UUID    `json:"id"`
	SenderID  storj.NodeID `json:"senderID"`
	Type      Type         `json:"type"`
	Title     string       `json:"title"`
	Message   string       `json:"message"`
	ReadAt    *time.Time   `json:"readAt"`
	CreatedAt time.Time    `json:"createdAt"`
}

// Cursor holds notification cursor entity which is used to create listed page from database.
type Cursor struct {
	Limit uint
	Page  uint
}

// Page holds notification page entity which is used to show listed page of notifications on UI.
type Page struct {
	Notifications []Notification `json:"notifications"`

	Offset      uint64 `json:"offset"`
	Limit       uint   `json:"limit"`
	CurrentPage uint   `json:"currentPage"`
	PageCount   uint   `json:"pageCount"`
	TotalCount  uint64 `json:"totalCount"`
}
