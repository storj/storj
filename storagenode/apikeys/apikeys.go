// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package apikeys

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/private/multinodeauth"
)

// ErrNoAPIKey represents no api key error.
var ErrNoAPIKey = errs.Class("no api key")

// DB is interface for working with api keys.
//
// architecture: Database
type DB interface {
	// Store stores api key into db.
	Store(ctx context.Context, apiKey APIKey) error

	// Check checks if api key exists in db by secret.
	Check(ctx context.Context, secret multinodeauth.Secret) error

	// Revoke removes api key from db.
	Revoke(ctx context.Context, secret multinodeauth.Secret) error
}

// APIKey describing api key in the database.
type APIKey struct {
	// APIKeys is PK of the table and keeps unique value sno api key.
	Secret multinodeauth.Secret

	CreatedAt time.Time `json:"createdAt"`
}
