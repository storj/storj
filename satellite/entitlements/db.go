// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package entitlements

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
)

var (
	// ErrNotFound is returned when an entitlement is not found.
	ErrNotFound = errs.Class("entitlement not found")
	mon         = monkit.Package()
)

// DB represents a storage interface for managing entitlements.
type DB interface {
	// GetByScope retrieves an entitlement by its scope.
	GetByScope(ctx context.Context, scope []byte) (*Entitlement, error)
	// UpsertByScope creates or updates an entitlement by its scope.
	UpsertByScope(ctx context.Context, ent *Entitlement) (*Entitlement, error)
	// DeleteByScope removes an entitlement by its scope.
	DeleteByScope(ctx context.Context, scope []byte) error
}

// Entitlement represents the structure of an entitlement.
type Entitlement struct {
	Scope     []byte
	Features  []byte
	CreatedAt time.Time
	UpdatedAt time.Time
}
