// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
)

var (
	// ErrSubdomainAlreadyExists is used to indicate that subdomain already exists.
	ErrSubdomainAlreadyExists = errs.Class("subdomain already exists")

	// ErrNoSubdomain is used to indicate that subdomain doesn't exist.
	ErrNoSubdomain = errs.Class("subdomain doesn't exist")
)

// Domains is an interface for working with domains store.
//
// architecture: Database
type Domains interface {
	// Create creates and stores new Domain.
	Create(ctx context.Context, domain Domain) (*Domain, error)
	// GetPagedByProjectID is a method for querying domains from the database by projectID and cursor.
	GetPagedByProjectID(ctx context.Context, projectID uuid.UUID, cursor DomainCursor) (*DomainPage, error)
	// GetByProjectIDAndSubdomain returns Domain by projectID and subdomain.
	GetByProjectIDAndSubdomain(ctx context.Context, projectID uuid.UUID, subdomain string) (*Domain, error)
	// GetAllDomainNamesByProjectID returns all domain names (subdomains) for the given project ID.
	GetAllDomainNamesByProjectID(ctx context.Context, projectID uuid.UUID) ([]string, error)
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

// DomainCursor holds info for domains cursor pagination.
type DomainCursor struct {
	Search         string         `json:"search"`
	Limit          uint           `json:"limit"`
	Page           uint           `json:"page"`
	Order          DomainOrder    `json:"order"`
	OrderDirection OrderDirection `json:"orderDirection"`
}

// DomainPage represent domain page result.
type DomainPage struct {
	Domains []Domain `json:"domains"`

	Search         string         `json:"search"`
	Limit          uint           `json:"limit"`
	Order          DomainOrder    `json:"order"`
	OrderDirection OrderDirection `json:"orderDirection"`
	Offset         uint64         `json:"offset"`

	PageCount   uint   `json:"pageCount"`
	CurrentPage uint   `json:"currentPage"`
	TotalCount  uint64 `json:"totalCount"`
}

// DomainOrder is used for querying domain in specified order.
type DomainOrder uint8

const (
	// SubdomainOrder indicates that we should order by subdomain.
	SubdomainOrder DomainOrder = 1
	// CreationDateOrder indicates that we should order by creation date.
	CreationDateOrder DomainOrder = 2
)
