// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"storj.io/common/uuid"
)

// APIKeys is interface for working with api keys store.
//
// architecture: Database
type APIKeys interface {
	// GetPagedByProjectID is a method for querying API keys from the database by projectID and cursor
	GetPagedByProjectID(ctx context.Context, projectID uuid.UUID, cursor APIKeyCursor) (akp *APIKeyPage, err error)
	// Get retrieves APIKeyInfo with given ID
	Get(ctx context.Context, id uuid.UUID) (*APIKeyInfo, error)
	// GetByHead retrieves APIKeyInfo for given key head
	GetByHead(ctx context.Context, head []byte) (*APIKeyInfo, error)
	// GetByNameAndProjectID retrieves APIKeyInfo for given key name and projectID
	GetByNameAndProjectID(ctx context.Context, name string, projectID uuid.UUID) (*APIKeyInfo, error)
	// Create creates and stores new APIKeyInfo
	Create(ctx context.Context, head []byte, info APIKeyInfo) (*APIKeyInfo, error)
	// Update updates APIKeyInfo in store
	Update(ctx context.Context, key APIKeyInfo) error
	// Delete deletes APIKeyInfo from store
	Delete(ctx context.Context, id uuid.UUID) error
}

// APIKeyInfo describing api key model in the database.
type APIKeyInfo struct {
	ID        uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"projectId"`
	PartnerID uuid.UUID `json:"partnerId"`
	UserAgent []byte    `json:"userAgent"`
	Name      string    `json:"name"`
	Head      []byte    `json:"-"`
	Secret    []byte    `json:"-"`
	CreatedAt time.Time `json:"createdAt"`
}

// APIKeyCursor holds info for api keys cursor pagination.
type APIKeyCursor struct {
	Search         string
	Limit          uint
	Page           uint
	Order          APIKeyOrder
	OrderDirection OrderDirection
}

// APIKeyPage represent api key page result.
type APIKeyPage struct {
	APIKeys []APIKeyInfo

	Search         string
	Limit          uint
	Order          APIKeyOrder
	OrderDirection OrderDirection
	Offset         uint64

	PageCount   uint
	CurrentPage uint
	TotalCount  uint64
}

// APIKeyOrder is used for querying api keys in specified order.
type APIKeyOrder uint8

const (
	// KeyName indicates that we should order by key name.
	KeyName APIKeyOrder = 1
	// CreationDate indicates that we should order by creation date.
	CreationDate APIKeyOrder = 2
)
