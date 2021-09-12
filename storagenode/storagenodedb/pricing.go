// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"errors"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/storagenode/pricing"
)

// ensures that pricingDB implements pricing.DB interface.
var _ pricing.DB = (*pricingDB)(nil)

// ErrPricing represents errors from the pricing database.
var ErrPricing = errs.Class("pricing")

// PricingDBName represents the database name.
const PricingDBName = "pricing"

// pricing works with node pricing DB.
//
// architecture: Database
type pricingDB struct {
	dbContainerImpl
}

// Store inserts or updates pricing model into the db.
func (db *pricingDB) Store(ctx context.Context, pricing pricing.Pricing) (err error) {
	defer mon.Task()(&ctx)(&err)

	query := `INSERT OR REPLACE INTO pricing (
			satellite_id,
			egress_bandwidth_price,
			repair_bandwidth_price,
			audit_bandwidth_price,
			disk_space_price
		) VALUES(?,?,?,?,?)`

	_, err = db.ExecContext(ctx, query,
		pricing.SatelliteID,
		pricing.EgressBandwidth,
		pricing.RepairBandwidth,
		pricing.AuditBandwidth,
		pricing.DiskSpace,
	)

	return ErrPricing.Wrap(err)
}

// Get retrieves pricing model for specific satellite.
func (db *pricingDB) Get(ctx context.Context, satelliteID storj.NodeID) (_ *pricing.Pricing, err error) {
	defer mon.Task()(&ctx)(&err)

	pricingModel := pricing.Pricing{
		SatelliteID: satelliteID,
	}

	row := db.QueryRowContext(ctx,
		`SELECT egress_bandwidth_price,
			repair_bandwidth_price,
			audit_bandwidth_price,
			disk_space_price
		FROM pricing WHERE satellite_id = ?`,
		satelliteID,
	)

	err = row.Scan(
		&pricingModel.EgressBandwidth,
		&pricingModel.RepairBandwidth,
		&pricingModel.AuditBandwidth,
		&pricingModel.DiskSpace,
	)

	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}

	return &pricingModel, ErrPricing.Wrap(err)
}
