// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package consoledb

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/zeebo/errs"
	"gopkg.in/yaml.v3"

	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/dbx"
)

var _ console.TenantWhiteLabelConfigs = (*tenantWhiteLabelConfigs)(nil)

type tenantWhiteLabelConfigs struct {
	db dbx.DriverMethods
}

// Get returns the whitelabel config for the given tenant.
func (t *tenantWhiteLabelConfigs) Get(ctx context.Context, tenantID string) (_ *console.TenantWhiteLabelConfig, err error) {
	defer mon.Task()(&ctx)(&err)

	if tenantID == "" {
		return nil, errs.New("tenant_id is empty")
	}

	row, err := t.db.Get_TenantWhitelabelConfig_By_TenantId(ctx, dbx.TenantWhitelabelConfig_TenantId(tenantID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, console.ErrTenantWhiteLabelConfigNotFound
		}
		return nil, err
	}

	return fromDBXTenantWhiteLabelConfig(row)
}

// List returns all whitelabel configs.
func (t *tenantWhiteLabelConfigs) List(ctx context.Context) (_ []console.TenantWhiteLabelConfig, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := t.db.All_TenantWhitelabelConfig_OrderBy_Asc_TenantId(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]console.TenantWhiteLabelConfig, 0, len(rows))
	for _, row := range rows {
		converted, err := fromDBXTenantWhiteLabelConfig(row)
		if err != nil {
			return nil, err
		}
		out = append(out, *converted)
	}
	return out, nil
}

// Upsert creates or replaces the whitelabel config for the given tenant.
func (t *tenantWhiteLabelConfigs) Upsert(ctx context.Context, tenantID string, cfg console.WhiteLabelConfig) (_ *console.TenantWhiteLabelConfig, err error) {
	defer mon.Task()(&ctx)(&err)

	if tenantID == "" {
		return nil, errs.New("tenant_id is empty")
	}

	encoded, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	row, err := t.db.Replace_TenantWhitelabelConfig(
		ctx,
		dbx.TenantWhitelabelConfig_TenantId(tenantID),
		dbx.TenantWhitelabelConfig_UpdatedAt(time.Now()),
		dbx.TenantWhitelabelConfig_Create_Fields{
			Config: dbx.TenantWhitelabelConfig_Config(encoded),
		},
	)
	if err != nil {
		return nil, err
	}

	return fromDBXTenantWhiteLabelConfig(row)
}

// Delete removes the whitelabel config for the given tenant. Removing a
// missing tenant is not an error.
func (t *tenantWhiteLabelConfigs) Delete(ctx context.Context, tenantID string) (err error) {
	defer mon.Task()(&ctx)(&err)

	if tenantID == "" {
		return errs.New("tenant_id is empty")
	}

	_, err = t.db.Delete_TenantWhitelabelConfig_By_TenantId(ctx, dbx.TenantWhitelabelConfig_TenantId(tenantID))
	return err
}

func fromDBXTenantWhiteLabelConfig(row *dbx.TenantWhitelabelConfig) (*console.TenantWhiteLabelConfig, error) {
	if row == nil {
		return nil, errs.New("nil tenant whitelabel config row")
	}

	var cfg console.WhiteLabelConfig
	if len(row.Config) > 0 {
		if err := yaml.Unmarshal(row.Config, &cfg); err != nil {
			return nil, errs.Wrap(err)
		}
	}

	return &console.TenantWhiteLabelConfig{
		TenantID:  row.TenantId,
		Config:    cfg,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}, nil
}
