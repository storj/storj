// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"storj.io/common/uuid"
)

// AccountFreezeEvents exposes methods to manage the account freeze events table in database.
//
// architecture: Database
type AccountFreezeEvents interface {
	// Insert is a method for inserting account freeze event into the database.
	Insert(ctx context.Context, event *AccountFreezeEvent) (*AccountFreezeEvent, error)
	// Get is a method for querying account freeze event from the database by user ID and event type.
	Get(ctx context.Context, userID uuid.UUID, eventType AccountFreezeEventType) (*AccountFreezeEvent, error)
	// UpdateLimits is a method for updating the limits of an account freeze event by user ID and event type.
	UpdateLimits(ctx context.Context, userID uuid.UUID, eventType AccountFreezeEventType, limits *AccountFreezeEventLimits) error
	// DeleteAllByUserID is a method for deleting all account freeze events from the database by user ID.
	DeleteAllByUserID(ctx context.Context, userID uuid.UUID) error
}

// AccountFreezeEvent represents an event related to account freezing.
type AccountFreezeEvent struct {
	UserID    uuid.UUID
	Type      AccountFreezeEventType
	Limits    *AccountFreezeEventLimits
	CreatedAt time.Time
}

// AccountFreezeEventLimits represents the usage limits for a user's account and projects before they were frozen.
type AccountFreezeEventLimits struct {
	User     UsageLimits               `json:"user"`
	Projects map[uuid.UUID]UsageLimits `json:"projects"`
}

// AccountFreezeEventType is used to indicate the account freeze event's type.
type AccountFreezeEventType int

const (
	// Freeze signifies that the user has been frozen.
	Freeze AccountFreezeEventType = 0
	// Warning signifies that the user has been warned that they may be frozen soon.
	Warning AccountFreezeEventType = 1
)
