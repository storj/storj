// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package restapikeys

import (
	"context"
	"time"

	"storj.io/common/uuid"
)

// DB defines a set of operations that ca be performed against REST API keys db.
type DB interface {
	// Get retrieves the RestAPIKey for the given ID.
	Get(ctx context.Context, id uuid.UUID) (Key, error)
	// GetByToken retrieves the RestAPIKey by the given Token.
	GetByToken(ctx context.Context, token string) (Key, error)

	// GetAll gets a list of REST API keys for the provided user.
	GetAll(ctx context.Context, userID uuid.UUID) ([]Key, error)

	// Create creates a new RestAPIKey.
	Create(ctx context.Context, key Key) (*Key, error)

	// Revoke revokes a REST API key by deleting it.
	Revoke(ctx context.Context, id uuid.UUID) error
}

// Service is an interface for rest key operations.
type Service interface {
	// GetAll gets a list of REST keys for the user in context.
	GetAll(ctx context.Context) (_ []Key, err error)
	// CreateNoAuth creates and inserts a rest key into the db for a user.
	CreateNoAuth(ctx context.Context, userID uuid.UUID, expiration *time.Duration) (apiKey string, expiresAt *time.Time, err error)
	// Create creates and inserts a rest key into the db.
	Create(ctx context.Context, name string, expiration *time.Duration) (apiKey string, expiresAt *time.Time, err error)
	// GetUserAndExpirationFromKey gets the userID and expiration date attached to an account management api key.
	GetUserAndExpirationFromKey(ctx context.Context, apiKey string) (userID uuid.UUID, exp time.Time, err error)
	// RevokeByKeyNoAuth revokes an account management api key
	// this is meant for Admin use.
	RevokeByKeyNoAuth(ctx context.Context, apiKey string) (err error)
	// RevokeByIDs revokes an account management api key by ID.
	RevokeByIDs(ctx context.Context, ids []uuid.UUID) (err error)
}

// Key represents a REST API key.
type Key struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"-"`
	Name      string     `json:"name"`
	Token     string     `json:"-"`
	CreatedAt time.Time  `json:"createdAt"`
	ExpiresAt *time.Time `json:"expiresAt"`
}
