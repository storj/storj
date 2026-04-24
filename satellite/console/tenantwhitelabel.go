// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"github.com/zeebo/errs"
)

// ErrTenantWhiteLabelConfigNotFound is returned when a tenant has no
// whitelabel config row in the database.
var ErrTenantWhiteLabelConfigNotFound = errs.New("tenant whitelabel config not found")

// TenantWhiteLabelConfig is a per-tenant override of the static
// SingleWhiteLabel YAML configuration.
//
// Non-zero fields in Config override the corresponding fields from the YAML
// SingleWhiteLabel config when the satellite-console is started.
type TenantWhiteLabelConfig struct {
	TenantID  string
	Config    WhiteLabelConfig
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TenantWhiteLabelConfigs is the persistence interface for per-tenant whitelabel configs.
type TenantWhiteLabelConfigs interface {
	// Get returns the whitelabel config for the given tenant, or ErrTenantWhiteLabelConfigNotFound.
	Get(ctx context.Context, tenantID string) (*TenantWhiteLabelConfig, error)
	// List returns all whitelabel configs ordered by tenant ID.
	List(ctx context.Context) ([]TenantWhiteLabelConfig, error)
	// Upsert creates or replaces the whitelabel config for the given tenant.
	Upsert(ctx context.Context, tenantID string, cfg WhiteLabelConfig) (*TenantWhiteLabelConfig, error)
	// Delete removes the whitelabel config for the given tenant. Removing a
	// missing tenant is not an error.
	Delete(ctx context.Context, tenantID string) error
}
