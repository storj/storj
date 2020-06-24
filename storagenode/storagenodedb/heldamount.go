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
func (db *heldamountDB) SatellitesHeldbackHistory(ctx context.Context, id storj.NodeID) (_ []heldamount.AmountPeriod, err error) {
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

	var heldback []heldamount.AmountPeriod
	for rows.Next() {
		var held heldamount.AmountPeriod

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

// SatellitePeriods retrieves all periods for concrete satellite in which we have some heldamount data.
func (db *heldamountDB) SatellitePeriods(ctx context.Context, satelliteID storj.NodeID) (_ []string, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `SELECT distinct period FROM paystubs WHERE satellite_id = ? ORDER BY created_at`

	rows, err := db.QueryContext(ctx, query, satelliteID[:])
	if err != nil {
		return nil, ErrHeldAmount.Wrap(err)
	}

	defer func() { err = errs.Combine(err, rows.Close()) }()

	var periodList []string
	for rows.Next() {
		var period string
		err := rows.Scan(&period)
		if err != nil {
			return nil, ErrHeldAmount.Wrap(err)
		}

		periodList = append(periodList, period)
	}
	if err = rows.Err(); err != nil {
		return nil, ErrHeldAmount.Wrap(err)
	}

	return periodList, nil
}

// AllPeriods retrieves all periods in which we have some heldamount data.
func (db *heldamountDB) AllPeriods(ctx context.Context) (_ []string, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `SELECT distinct period FROM paystubs ORDER BY created_at`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}

	defer func() { err = errs.Combine(err, rows.Close()) }()

	var periodList []string
	for rows.Next() {
		var period string
		err := rows.Scan(&period)
		if err != nil {
			return nil, ErrHeldAmount.Wrap(err)
		}

		periodList = append(periodList, period)
	}
	if err = rows.Err(); err != nil {
		return nil, ErrHeldAmount.Wrap(err)
	}

	return periodList, nil
}

// StorePayment inserts or updates payment data into the db.
func (db *heldamountDB) StorePayment(ctx context.Context, payment heldamount.Payment) (err error) {
	defer mon.Task()(&ctx)(&err)

	query := `INSERT OR REPLACE INTO payments (
			id,
			created_at,
			satellite_id,
			period,
			amount,
			receipt,
			notes
		) VALUES(?,?,?,?,?,?,?)`

	_, err = db.ExecContext(ctx, query,
		payment.ID,
		payment.Created,
		payment.SatelliteID,
		payment.Period,
		payment.Amount,
		payment.Receipt,
		payment.Notes,
	)

	return ErrHeldAmount.Wrap(err)
}

// GetPayment retrieves payment data for a specific satellite.
func (db *heldamountDB) GetPayment(ctx context.Context, satelliteID storj.NodeID, period string) (_ *heldamount.Payment, err error) {
	defer mon.Task()(&ctx)(&err)

	result := heldamount.Payment{
		SatelliteID: satelliteID,
		Period:      period,
	}

	row := db.QueryRowContext(ctx,
		`SELECT id,
			created_at,
			amount,
			receipt,
			notes
		FROM payments WHERE satellite_id = ? AND period = ?`,
		satelliteID, period,
	)

	err = row.Scan(
		&result.ID,
		&result.Created,
		&result.Amount,
		&result.Receipt,
		&result.Notes,
	)
	if err != nil {
		if sql.ErrNoRows == err {
			return nil, heldamount.ErrNoPayStubForPeriod.Wrap(err)
		}
		return nil, ErrHeldAmount.Wrap(err)
	}

	return &result, nil
}

// AllPayments retrieves all payment stats from DB for specific period.
func (db *heldamountDB) AllPayments(ctx context.Context, period string) (_ []heldamount.Payment, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `SELECT 
			satellite_id,
			id,
			created_at,
			amount,
			receipt,
			notes
		FROM payments WHERE period = ?`

	rows, err := db.QueryContext(ctx, query, period)
	if err != nil {
		return nil, err
	}

	defer func() { err = errs.Combine(err, rows.Close()) }()

	var paymentList []heldamount.Payment
	for rows.Next() {
		var payment heldamount.Payment
		payment.Period = period

		err := rows.Scan(&payment.SatelliteID,
			&payment.ID,
			&payment.Created,
			&payment.Amount,
			&payment.Receipt,
			&payment.Notes,
		)

		if err != nil {
			return nil, ErrHeldAmount.Wrap(err)
		}

		paymentList = append(paymentList, payment)
	}
	if err = rows.Err(); err != nil {
		return nil, ErrHeldAmount.Wrap(err)
	}

	return paymentList, nil
}

// SatellitesDisposedHistory returns all disposed amount for specific satellite from DB.
func (db *heldamountDB) SatellitesDisposedHistory(ctx context.Context, satelliteID storj.NodeID) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `SELECT 
				disposed
			  FROM paystubs WHERE satellite_id = ? ORDER BY period ASC`

	rows, err := db.QueryContext(ctx, query, satelliteID)
	if err != nil {
		return 0, err
	}

	defer func() { err = errs.Combine(err, rows.Close()) }()

	var totalDisposed int64
	for rows.Next() {
		var disposed int64

		err := rows.Scan(&disposed)
		if err != nil {
			return 0, ErrHeldAmount.Wrap(err)
		}

		totalDisposed += disposed
	}
	if err = rows.Err(); err != nil {
		return 0, ErrHeldAmount.Wrap(err)
	}

	return totalDisposed, nil
}
