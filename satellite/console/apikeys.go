// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// APIKeys is interface for working with api keys store
type APIKeys interface {
	// GetByProjectID retrieves list of APIKeys for given projectID
	GetByProjectID(ctx context.Context, projectID uuid.UUID) ([]APIKeyInfo, error)
	// Get retrieves APIKeyInfo with given ID
	Get(ctx context.Context, id uuid.UUID) (*APIKeyInfo, error)
	// GetByHead retrieves APIKeyInfo for given key head
	GetByHead(ctx context.Context, head []byte) (*APIKeyInfo, error)
	// Create creates and stores new APIKeyInfo
	Create(ctx context.Context, head []byte, info APIKeyInfo) (*APIKeyInfo, error)
	// Update updates APIKeyInfo in store
	Update(ctx context.Context, key APIKeyInfo) error
	// Delete deletes APIKeyInfo from store
	Delete(ctx context.Context, id uuid.UUID) error
}

// APIKeyInfo describing api key model in the database
type APIKeyInfo struct {
	ID        uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"projectId"`
	PartnerID uuid.UUID `json:"partnerId"`
	Name      string    `json:"name"`
	Secret    []byte    `json:"-"`
	CreatedAt time.Time `json:"createdAt"`
}
