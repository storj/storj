// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/storagenode/heldamount"
)

// ensures that heldamountDB implements heldamount.DB interface.
var _ heldamount.DB = (*heldamountDB)(nil)

// ErrHeldAmount represents errors from the heldamount database.
var ErrHeldAmount = errs.Class("heldamount error")

// HeldAmountDBName represents the database name.
const HeldAmountDBName = "heldamount"

// heldamountDB works with node heldamount DB
type heldamountDB struct {
	dbContainerImpl
}

// StorePayStub inserts or updates paystub data into the db.
func (db *heldamountDB) StorePayStub(ctx context.Context, paystub heldamount.PayStub) (err error) {
	defer mon.Task()(&ctx)(&err)

	query := `INSERT OR REPLACE INTO paystubs (
			period,
			satellite_id,
			created_at,
			codes,
			usage_at_rest,
			usage_get,
			usage_put,
			usage_get_repair,
			usage_put_repair,
			usage_get_audit,
			comp_at_rest,
			comp_get,
			comp_put,
			comp_get_repair,
			comp_put_repair,
			comp_get_audit,
			surge_percent,
			held,
			owed,
			disposed,
			paid
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`

	_, err = db.ExecContext(ctx, query,
		paystub.Period,
		paystub.SatelliteID,
		paystub.Created,
		paystub.Codes,
		paystub.UsageAtRest,
		paystub.UsageGet,
		paystub.UsagePut,
		paystub.UsageGetRepair,
		paystub.UsagePutRepair,
		paystub.UsageGetAudit,
		paystub.CompAtRest,
		paystub.CompGet,
		paystub.CompPut,
		paystub.CompGetRepair,
		paystub.CompPutRepair,
		paystub.CompGetAudit,
		paystub.SurgePercent,
		paystub.Held,
		paystub.Owed,
		paystub.Disposed,
		paystub.Paid,
	)

	return ErrHeldAmount.Wrap(err)
}

// GetPayStub retrieves paystub data for a specific satellite.
func (db *heldamountDB) GetPayStub(ctx context.Context, satelliteID storj.NodeID, period string) (_ *heldamount.PayStub, err error) {
	defer mon.Task()(&ctx)(&err)

	result := heldamount.PayStub{
		SatelliteID: satelliteID,
		Period:      period,
	}

	row := db.QueryRowContext(ctx,
		`SELECT created_at,
			codes,
			usage_at_rest,
			usage_get,
			usage_put,
			usage_get_repair,
			usage_put_repair,
			usage_get_audit,
			comp_at_rest,
			comp_get,
			comp_put,
			comp_get_repair,
			comp_put_repair,
			comp_get_audit,
			surge_percent,
			held,
			owed,
			disposed,
			paid
		FROM paystubs WHERE satellite_id = ? AND period = ?`,
		satelliteID, period,
	)

	err = row.Scan(
		&result.Created,
		&result.Codes,
		&result.UsageAtRest,
		&result.UsageGet,
		&result.UsagePut,
		&result.UsageGetRepair,
		&result.UsagePutRepair,
		&result.UsageGetAudit,
		&result.CompAtRest,
		&result.CompGet,
		&result.CompPut,
		&result.CompGetRepair,
		&result.CompPutRepair,
		&result.CompGetAudit,
		&result.SurgePercent,
		&result.Held,
		&result.Owed,
		&result.Disposed,
		&result.Paid,
	)
	if err != nil {
		if sql.ErrNoRows == err {
			return nil, heldamount.ErrNoPayStubForPeriod.Wrap(err)
		}
		return nil, ErrHeldAmount.Wrap(err)
	}

	return &result, nil
}

// AllPayStubs retrieves all paystub stats from DB for specific period.
func (db *heldamountDB) AllPayStubs(ctx context.Context, period string) (_ []heldamount.PayStub, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `SELECT 
			  	satellite_id,
			  	created_at,
			  	codes,
			  	usage_at_rest,
			  	usage_get,
			  	usage_put,
			  	usage_get_repair,
			  	usage_put_repair,
			  	usage_get_audit,
			  	comp_at_rest,
			  	comp_get,
			  	comp_put,
			  	comp_get_repair,
			  	comp_put_repair,
			  	comp_get_audit,
			  	surge_percent,
			  	held,
			  	owed,
			  	disposed,
			  	paid
			  FROM paystubs WHERE period = ?`

	rows, err := db.QueryContext(ctx, query, period)
	if err != nil {
		return nil, err
	}

	defer func() { err = errs.Combine(err, rows.Close()) }()

	var paystubList []heldamount.PayStub
	for rows.Next() {
		var paystub heldamount.PayStub
		paystub.Period = period

		err := rows.Scan(&paystub.SatelliteID,
			&paystub.Created,
			&paystub.Codes,
			&paystub.UsageAtRest,
			&paystub.UsageGet,
			&paystub.UsagePut,
			&paystub.UsageGetRepair,
			&paystub.UsagePutRepair,
			&paystub.UsageGetAudit,
			&paystub.CompAtRest,
			&paystub.CompGet,
			&paystub.CompPut,
			&paystub.CompGetRepair,
			&paystub.CompPutRepair,
			&paystub.CompGetAudit,
			&paystub.SurgePercent,
			&paystub.Held,
			&paystub.Owed,
			&paystub.Disposed,
			&paystub.Paid,
		)
		if err != nil {
			return nil, ErrHeldAmount.Wrap(err)
		}

		paystubList = append(paystubList, paystub)
	}
	if err = rows.Err(); err != nil {
		return nil, ErrHeldAmount.Wrap(err)
	}

	return paystubList, nil
}

// SatellitesHeldbackHistory retrieves heldback history for specific satellite.
func (db *heldamountDB) SatellitesHeldbackHistory(ctx context.Context, id storj.NodeID) (_ []heldamount.Heldback, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `SELECT 
				period,
				held
			  FROM paystubs WHERE satellite_id = ? ORDER BY period ASC`

	rows, err := db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, err
	}

	defer func() { err = errs.Combine(err, rows.Close()) }()

	var heldback []heldamount.Heldback
	for rows.Next() {
		var held heldamount.Heldback

		err := rows.Scan(&held.Period, &held.Held)
		if err != nil {
			return nil, ErrHeldAmount.Wrap(err)
		}

		heldback = append(heldback, held)
	}
	if err = rows.Err(); err != nil {
		return nil, ErrHeldAmount.Wrap(err)
	}

	return heldback, nil
}
