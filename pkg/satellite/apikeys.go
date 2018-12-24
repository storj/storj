// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// APIKeys is interface for working with api keys store
type APIKeys interface {
	// GetByProjectID retrieves list of APIKeys for given projectID
	GetByProjectID(ctx context.Context, projectID uuid.UUID) ([]APIKey, error)
	// Get retrieves APIKey with given ID
	Get(ctx context.Context, id uuid.UUID) (*APIKey, error)
	// Create creates and stores new APIKey
	Create(ctx context.Context, key APIKey) (*APIKey, error)
	// Update updates APIKey in store
	Update(ctx context.Context, key APIKey) error
	// Delete deletes APIKey from store
	Delete(ctx context.Context, id uuid.UUID) error
}

// APIKey describing api key model in the database
type APIKey struct {
	ID uuid.UUID `json:"id"`

	// Fk on project
	ProjectID uuid.UUID `json:"projectId"`

	Key  []byte `json:"key"`
	Name string `json:"name"`

	CreatedAt time.Time `json:"createdAt"`
}
