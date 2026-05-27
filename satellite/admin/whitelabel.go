// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/zeebo/errs"
	"gopkg.in/yaml.v3"

	"storj.io/storj/private/api"
	"storj.io/storj/satellite/console"
)

// TenantWhiteLabelConfig is the admin API representation of a per-tenant
// whitelabel config row. ConfigYAML is the same YAML shape the satellite
// reads from the SingleWhiteLabel YAML config, so admins can copy/paste
// between the two.
type TenantWhiteLabelConfig struct {
	TenantID   string    `json:"tenantID"`
	ConfigYAML string    `json:"configYAML"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// UpdateTenantWhiteLabelConfigRequest is the admin API request body for
// upserting a tenant whitelabel config. ConfigYAML is parsed server-side
// into a console.WhiteLabelConfig before being persisted.
type UpdateTenantWhiteLabelConfigRequest struct {
	ConfigYAML string `json:"configYAML"`
}

// GetTenantWhiteLabelConfig returns the persisted whitelabel config for the
// given tenant. In a tenant-scoped admin only the configured tenant ID may
// be requested.
func (s *Service) GetTenantWhiteLabelConfig(ctx context.Context, tenantID string) (_ *TenantWhiteLabelConfig, _ api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	apiError := func(status int, err error) (*TenantWhiteLabelConfig, api.HTTPError) {
		return nil, api.HTTPError{Status: status, Err: Error.Wrap(err)}
	}

	if tenantID == "" {
		return apiError(http.StatusBadRequest, errs.New("tenantID is required"))
	}
	if !s.tenantMatchesScope(tenantID) {
		return apiError(http.StatusForbidden, errs.New("tenant-scoped admin cannot access other tenants"))
	}

	row, err := s.consoleDB.TenantWhiteLabelConfigs().Get(ctx, tenantID)
	if err != nil {
		if errors.Is(err, console.ErrTenantWhiteLabelConfigNotFound) {
			return apiError(http.StatusNotFound, errs.New("whitelabel config not found for tenant %q", tenantID))
		}
		return apiError(http.StatusInternalServerError, err)
	}

	return toAPIConfig(row)
}

// ListTenantWhiteLabelConfigs returns all whitelabel configs. Only available
// in a non-tenant-scoped admin; tenant-scoped admins should use the per-tenant
// GET endpoint instead.
func (s *Service) ListTenantWhiteLabelConfigs(ctx context.Context) (_ []TenantWhiteLabelConfig, _ api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	apiError := func(status int, err error) ([]TenantWhiteLabelConfig, api.HTTPError) {
		return nil, api.HTTPError{Status: status, Err: Error.Wrap(err)}
	}

	if s.tenantID != nil {
		return apiError(http.StatusForbidden, errs.New("listing whitelabel configs is not available in a tenant-scoped admin"))
	}

	rows, err := s.consoleDB.TenantWhiteLabelConfigs().List(ctx)
	if err != nil {
		return apiError(http.StatusInternalServerError, err)
	}

	out := make([]TenantWhiteLabelConfig, 0, len(rows))
	for _, row := range rows {
		converted, httpErr := toAPIConfig(&row)
		if httpErr != (api.HTTPError{}) {
			return nil, httpErr
		}
		out = append(out, *converted)
	}
	return out, api.HTTPError{}
}

// UpdateTenantWhiteLabelConfig creates or replaces the whitelabel config for
// the given tenant. In a tenant-scoped admin only the configured tenant ID
// may be modified.
func (s *Service) UpdateTenantWhiteLabelConfig(ctx context.Context, tenantID string, request UpdateTenantWhiteLabelConfigRequest) (_ *TenantWhiteLabelConfig, _ api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	apiError := func(status int, err error) (*TenantWhiteLabelConfig, api.HTTPError) {
		return nil, api.HTTPError{Status: status, Err: Error.Wrap(err)}
	}

	if tenantID == "" {
		return apiError(http.StatusBadRequest, errs.New("tenantID is required"))
	}
	if !s.tenantMatchesScope(tenantID) {
		return apiError(http.StatusForbidden, errs.New("tenant-scoped admin cannot modify other tenants"))
	}

	var cfg console.WhiteLabelConfig
	if request.ConfigYAML != "" {
		if err := yaml.Unmarshal([]byte(request.ConfigYAML), &cfg); err != nil {
			return apiError(http.StatusBadRequest, errs.New("invalid YAML: %v", err))
		}
	}
	// SMTP credentials must not be persisted to the database since the
	// tenant_whitelabel_configs table is not encrypted at rest. SMTP
	// settings stay in the YAML SingleWhiteLabel config.
	if (cfg.SMTP != console.SMTPConfig{}) {
		return apiError(http.StatusBadRequest, errs.New("smtp settings cannot be stored per-tenant in the database; configure them in the YAML SingleWhiteLabel config"))
	}

	row, err := s.consoleDB.TenantWhiteLabelConfigs().Upsert(ctx, tenantID, cfg)
	if err != nil {
		return apiError(http.StatusInternalServerError, err)
	}

	return toAPIConfig(row)
}

// tenantMatchesScope reports whether the given tenantID is accessible from
// the admin's current tenant scope.
func (s *Service) tenantMatchesScope(tenantID string) bool {
	if s.tenantID == nil {
		return true
	}
	return tenantID == *s.tenantID
}

// toAPIConfig serializes a stored config row into the API response shape,
// re-emitting the WhiteLabelConfig as YAML.
func toAPIConfig(row *console.TenantWhiteLabelConfig) (*TenantWhiteLabelConfig, api.HTTPError) {
	encoded, err := yaml.Marshal(row.Config)
	if err != nil {
		return nil, api.HTTPError{Status: http.StatusInternalServerError, Err: Error.Wrap(err)}
	}
	return &TenantWhiteLabelConfig{
		TenantID:   row.TenantID,
		ConfigYAML: string(encoded),
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
	}, api.HTTPError{}
}
