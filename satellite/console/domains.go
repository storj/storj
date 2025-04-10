// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"storj.io/common/uuid"
)

// Domains is an interface for working with domains store.
//
// architecture: Database
type Domains interface {
	// Create creates and stores new Domain.
	Create(ctx context.Context, domain Domain) (*Domain, error)
	// Delete deletes Domain from store.
	Delete(ctx context.Context, projectID uuid.UUID, subdomain string) error
	// DeleteAllByProjectID deletes all Domains for the given project.
	DeleteAllByProjectID(ctx context.Context, projectID uuid.UUID) error
}

// Domain describing domain model in the database.
type Domain struct {
	ProjectID       uuid.UUID `json:"-"`
	ProjectPublicID uuid.UUID `json:"projectPublicID"`
	CreatedBy       uuid.UUID `json:"createdBy"`

	Subdomain string `json:"subdomain"`
	Prefix    string `json:"prefix"`
	AccessID  string `json:"accessID"`

	CreatedAt time.Time `json:"createdAt"`
}
