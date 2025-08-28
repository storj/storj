// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package consoledb

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/satellitedb/dbx"
)

// Ensures that entitlementsDB implements entitlements.DB.
var _ entitlements.DB = (*entitlementsDB)(nil)

// entitlementsDB implements entitlements.DB.
type entitlementsDB struct {
	db dbx.DriverMethods
}

// GetByScope retrieves an entitlement by its scope.
func (e *entitlementsDB) GetByScope(ctx context.Context, scope []byte) (_ *entitlements.Entitlement, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(scope) == 0 {
		return nil, errs.New("scope is empty")
	}

	dbxEntitlement, err := e.db.Get_Entitlement_By_Scope(ctx, dbx.Entitlement_Scope(scope))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, entitlements.ErrNotFound.New("")
		}

		return nil, err
	}

	return fromDBXEntitlementRow(ctx, dbxEntitlement)
}

// UpsertByScope creates or updates an entitlement by its scope.
func (e *entitlementsDB) UpsertByScope(ctx context.Context, ent *entitlements.Entitlement) (_ *entitlements.Entitlement, err error) {
	defer mon.Task()(&ctx)(&err)

	if ent == nil {
		return nil, errs.New("entitlement is nil")
	}
	if len(ent.Scope) == 0 {
		return nil, errs.New("scope is empty")
	}

	if bytes.Equal(bytes.TrimSpace(ent.Features), []byte("null")) {
		ent.Features = []byte("{}")
	}

	updatedAt := ent.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}

	dbxEntitlement, err := e.db.Replace_Entitlement(
		ctx,
		dbx.Entitlement_Scope(ent.Scope),
		dbx.Entitlement_UpdatedAt(updatedAt),
		dbx.Entitlement_Create_Fields{Features: dbx.Entitlement_Features(ent.Features)},
	)
	if err != nil {
		return nil, err
	}

	return fromDBXEntitlementRow(ctx, dbxEntitlement)
}

// DeleteByScope removes an entitlement by its scope.
func (e *entitlementsDB) DeleteByScope(ctx context.Context, scope []byte) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(scope) == 0 {
		return errs.New("scope is empty")
	}

	_, err = e.db.Delete_Entitlement_By_Scope(ctx, dbx.Entitlement_Scope(scope))
	return err
}

func fromDBXEntitlementRow(ctx context.Context, row *dbx.Entitlement) (_ *entitlements.Entitlement, err error) {
	defer mon.Task()(&ctx)(&err)

	if row == nil {
		return nil, errs.New("nil entitlement row")
	}

	scope := make([]byte, len(row.Scope))
	copy(scope, row.Scope)
	feats := make([]byte, len(row.Features))
	copy(feats, row.Features)

	return &entitlements.Entitlement{
		Scope:     scope,
		Features:  feats,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}, nil
}
