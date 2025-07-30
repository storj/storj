// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"storj.io/common/macaroon"
	"storj.io/common/uuid"
)

// APIKeys is interface for working with api keys store.
//
// architecture: Database
type APIKeys interface {
	// GetPagedByProjectID is a method for querying API keys from the database by projectID and cursor.
	GetPagedByProjectID(ctx context.Context, projectID uuid.UUID, cursor APIKeyCursor, ignoredNamePrefix string) (akp *APIKeyPage, err error)
	// Get retrieves APIKeyInfo with given ID.
	Get(ctx context.Context, id uuid.UUID) (*APIKeyInfo, error)
	// GetByHead retrieves APIKeyInfo for given key head.
	GetByHead(ctx context.Context, head []byte) (*APIKeyInfo, error)
	// GetByNameAndProjectID retrieves APIKeyInfo for given key name and projectID.
	GetByNameAndProjectID(ctx context.Context, name string, projectID uuid.UUID) (*APIKeyInfo, error)
	// GetAllNamesByProjectID retrieves all API key names for given projectID.
	GetAllNamesByProjectID(ctx context.Context, projectID uuid.UUID) ([]string, error)
	// Create creates and stores new APIKeyInfo.
	Create(ctx context.Context, head []byte, info APIKeyInfo) (*APIKeyInfo, error)
	// Update updates APIKeyInfo in store.
	Update(ctx context.Context, key APIKeyInfo) error
	// Delete deletes APIKeyInfo from store.
	Delete(ctx context.Context, id uuid.UUID) error
	// DeleteMultiple deletes multiple APIKeyInfo from store.
	DeleteMultiple(ctx context.Context, ids []uuid.UUID) error
	// DeleteAllByProjectID deletes all APIKeyInfos from store by given projectID.
	DeleteAllByProjectID(ctx context.Context, id uuid.UUID) error
	// DeleteExpiredByNamePrefix deletes expired APIKeyInfo from store by key name prefix.
	DeleteExpiredByNamePrefix(ctx context.Context, lifetime time.Duration, prefix string, asOfSystemTimeInterval time.Duration, pageSize int) error
}

// CreateAPIKeyRequest holds create API key info.
type CreateAPIKeyRequest struct {
	ProjectID string `json:"projectID"`
	Name      string `json:"name"`
}

// CreateAPIKeyResponse holds macaroon.APIKey and APIKeyInfo.
type CreateAPIKeyResponse struct {
	Key     string      `json:"key"`
	KeyInfo *APIKeyInfo `json:"keyInfo"`
}

// APIKeyInfo describing api key model in the database.
type APIKeyInfo struct {
	ID              uuid.UUID              `json:"id"`
	ProjectID       uuid.UUID              `json:"projectId"`
	ProjectPublicID uuid.UUID              `json:"projectPublicId"`
	CreatedBy       uuid.UUID              `json:"createdBy"`
	CreatorEmail    string                 `json:"creatorEmail"`
	UserAgent       []byte                 `json:"userAgent"`
	Name            string                 `json:"name"`
	Head            []byte                 `json:"-"`
	Secret          []byte                 `json:"-"`
	CreatedAt       time.Time              `json:"createdAt"`
	Version         macaroon.APIKeyVersion `json:"version"`

	// TODO move this closer to metainfo
	ProjectRateLimit        *int `json:"-"`
	ProjectBurstLimit       *int `json:"-"`
	ProjectRateLimitHead    *int `json:"-"`
	ProjectBurstLimitHead   *int `json:"-"`
	ProjectRateLimitGet     *int `json:"-"`
	ProjectBurstLimitGet    *int `json:"-"`
	ProjectRateLimitPut     *int `json:"-"`
	ProjectBurstLimitPut    *int `json:"-"`
	ProjectRateLimitList    *int `json:"-"`
	ProjectBurstLimitList   *int `json:"-"`
	ProjectRateLimitDelete  *int `json:"-"`
	ProjectBurstLimitDelete *int `json:"-"`

	ProjectStorageLimit   *int64 `json:"-"`
	ProjectSegmentsLimit  *int64 `json:"-"`
	ProjectBandwidthLimit *int64 `json:"-"`
}

// APIKeyCursor holds info for api keys cursor pagination.
type APIKeyCursor struct {
	Search         string         `json:"search"`
	Limit          uint           `json:"limit"`
	Page           uint           `json:"page"`
	Order          APIKeyOrder    `json:"order"`
	OrderDirection OrderDirection `json:"orderDirection"`
}

// APIKeyPage represent api key page result.
type APIKeyPage struct {
	APIKeys []APIKeyInfo `json:"apiKeys"`

	Search         string         `json:"search"`
	Limit          uint           `json:"limit"`
	Order          APIKeyOrder    `json:"order"`
	OrderDirection OrderDirection `json:"orderDirection"`
	Offset         uint64         `json:"offset"`

	PageCount   uint   `json:"pageCount"`
	CurrentPage uint   `json:"currentPage"`
	TotalCount  uint64 `json:"totalCount"`
}

// APIKeyOrder is used for querying api keys in specified order.
type APIKeyOrder uint8

const (
	// KeyName indicates that we should order by key name.
	KeyName APIKeyOrder = 1
	// CreationDate indicates that we should order by creation date.
	CreationDate APIKeyOrder = 2
	// KeyCreatorEmail indicates that we should order by key creator email.
	KeyCreatorEmail APIKeyOrder = 3
)
