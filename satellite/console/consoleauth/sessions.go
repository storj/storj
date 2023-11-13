// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleauth

import (
	"context"
	"time"

	"storj.io/common/uuid"
)

// WebappSessions is the repository for webapp sessions.
type WebappSessions interface {
	// Create creates a webapp session and returns the session info.
	Create(ctx context.Context, id, userID uuid.UUID, ip, userAgent string, expires time.Time) (WebappSession, error)
	// GetBySessionID gets the session info from the session ID.
	GetBySessionID(ctx context.Context, sessionID uuid.UUID) (WebappSession, error)
	// GetAllByUserID gets all webapp sessions with userID.
	GetAllByUserID(ctx context.Context, userID uuid.UUID) ([]WebappSession, error)
	// DeleteBySessionID deletes a webapp session by ID.
	DeleteBySessionID(ctx context.Context, sessionID uuid.UUID) error
	// DeleteAllByUserID deletes all webapp sessions by user ID.
	DeleteAllByUserID(ctx context.Context, userID uuid.UUID) (int64, error)
	// UpdateExpiration updates the expiration time of the session.
	UpdateExpiration(ctx context.Context, sessionID uuid.UUID, expiresAt time.Time) error
	// DeleteExpired deletes all sessions that have expired before the provided timestamp.
	DeleteExpired(ctx context.Context, now time.Time, asOfSystemTimeInterval time.Duration, pageSize int) error
}

// WebappSession represents a session on the satellite web app.
type WebappSession struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Address   string
	UserAgent string
	Status    int
	ExpiresAt time.Time
}
