// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"errors"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/storagenode/payouts"
)

// ensures that payoutDB implements payouts.DB interface.
var _ payouts.DB = (*payoutDB)(nil)

// ErrPayout represents errors from the payouts database.
var ErrPayout = errs.Class("payouts")

// HeldAmountDBName represents the database name.
const HeldAmountDBName = "heldamount"

// payoutDB works with node payouts DB.
type payoutDB struct {
	dbContainerImpl
}

// StorePayStub inserts or updates paystub data into the db.
func (db *payoutDB) StorePayStub(ctx context.Context, paystub payouts.PayStub) (err error) {
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
			paid,
			distributed
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`

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
		paystub.Distributed,
	)

	return ErrPayout.Wrap(err)
}

// GetPayStub retrieves paystub data for a specific satellite and period.
func (db *payoutDB) GetPayStub(ctx context.Context, satelliteID storj.NodeID, period string) (_ *payouts.PayStub, err error) {
	defer mon.Task()(&ctx)(&err)

	result := payouts.PayStub{
		SatelliteID: satelliteID,
		Period:      period,
	}

	rowStub := db.QueryRowContext(ctx,
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
			paid,
			distributed
		FROM paystubs WHERE satellite_id = ? AND period = ?`,
		satelliteID, period,
	)

	err = rowStub.Scan(
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
		&result.Distributed,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, payouts.ErrNoPayStubForPeriod.Wrap(err)
		}
		return nil, ErrPayout.Wrap(err)
	}

	return &result, nil
}

// AllPayStubs retrieves all paystub stats from DB for specific period.
func (db *payoutDB) AllPayStubs(ctx context.Context, period string) (_ []payouts.PayStub, err error) {
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
			paid,
			distributed
		FROM paystubs WHERE period = ?`

	rows, err := db.QueryContext(ctx, query, period)
	if err != nil {
		return nil, err
	}

	defer func() { err = errs.Combine(err, rows.Close()) }()

	var paystubList []payouts.PayStub
	for rows.Next() {
		var paystub payouts.PayStub
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
			&paystub.Distributed,
		)
		if err != nil {
			return nil, ErrPayout.Wrap(err)
		}

		paystubList = append(paystubList, paystub)
	}
	if err = rows.Err(); err != nil {
		return nil, ErrPayout.Wrap(err)
	}

	return paystubList, nil
}

// SatellitesHeldbackHistory retrieves heldback history for specific satellite.
func (db *payoutDB) SatellitesHeldbackHistory(ctx context.Context, id storj.NodeID) (_ []payouts.HoldForPeriod, err error) {
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

	var heldback []payouts.HoldForPeriod
	for rows.Next() {
		var held payouts.HoldForPeriod

		err := rows.Scan(&held.Period, &held.Amount)
		if err != nil {
			return nil, ErrPayout.Wrap(err)
		}

		heldback = append(heldback, held)
	}
	if err = rows.Err(); err != nil {
		return nil, ErrPayout.Wrap(err)
	}

	return heldback, nil
}

// SatellitePeriods retrieves all periods for concrete satellite in which we have some payouts data.
func (db *payoutDB) SatellitePeriods(ctx context.Context, satelliteID storj.NodeID) (_ []string, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `SELECT distinct period FROM paystubs WHERE satellite_id = ? ORDER BY created_at`

	rows, err := db.QueryContext(ctx, query, satelliteID[:])
	if err != nil {
		return nil, ErrPayout.Wrap(err)
	}

	defer func() { err = errs.Combine(err, rows.Close()) }()

	var periodList []string
	for rows.Next() {
		var period string
		err := rows.Scan(&period)
		if err != nil {
			return nil, ErrPayout.Wrap(err)
		}

		periodList = append(periodList, period)
	}
	if err = rows.Err(); err != nil {
		return nil, ErrPayout.Wrap(err)
	}

	return periodList, nil
}

// AllPeriods retrieves all periods in which we have some payouts data.
func (db *payoutDB) AllPeriods(ctx context.Context) (_ []string, err error) {
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
			return nil, ErrPayout.Wrap(err)
		}

		periodList = append(periodList, period)
	}
	if err = rows.Err(); err != nil {
		return nil, ErrPayout.Wrap(err)
	}

	return periodList, nil
}

// StorePayment inserts or updates payment data into the db.
func (db *payoutDB) StorePayment(ctx context.Context, payment payouts.Payment) (err error) {
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

	return ErrPayout.Wrap(err)
}

// SatellitesDisposedHistory returns all disposed amount for specific satellite from DB.
func (db *payoutDB) SatellitesDisposedHistory(ctx context.Context, satelliteID storj.NodeID) (_ int64, err error) {
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
			return 0, ErrPayout.Wrap(err)
		}

		totalDisposed += disposed
	}
	if err = rows.Err(); err != nil {
		return 0, ErrPayout.Wrap(err)
	}

	return totalDisposed, nil
}

// GetReceipt retrieves receipt data for a specific satellite and period.
func (db *payoutDB) GetReceipt(ctx context.Context, satelliteID storj.NodeID, period string) (receipt string, err error) {
	defer mon.Task()(&ctx)(&err)

	rowPayment := db.QueryRowContext(ctx,
		`SELECT receipt FROM payments WHERE satellite_id = ? AND period = ?`,
		satelliteID, period,
	)

	err = rowPayment.Scan(&receipt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", payouts.ErrNoPayStubForPeriod.Wrap(err)
		}
		return "", ErrPayout.Wrap(err)
	}

	return receipt, nil
}

// GetTotalEarned returns total earned value for node from all paystubs.
func (db *payoutDB) GetTotalEarned(ctx context.Context) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `SELECT comp_at_rest, comp_get, comp_get_repair, comp_get_audit FROM paystubs`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return 0, err
	}

	defer func() { err = errs.Combine(err, rows.Close()) }()

	var totalEarned int64
	for rows.Next() {
		var compAtRest, compGet, compGetRepair, compGetAudit int64

		err := rows.Scan(&compAtRest, &compGet, &compGetRepair, &compGetAudit)
		if err != nil {
			return 0, ErrPayout.Wrap(err)
		}

		totalEarned += compGetAudit + compGet + compGetRepair + compAtRest
	}
	if err = rows.Err(); err != nil {
		return 0, ErrPayout.Wrap(err)
	}

	return totalEarned, nil
}

// GetEarnedAtSatellite returns total earned value for node from specific satellite.
func (db *payoutDB) GetEarnedAtSatellite(ctx context.Context, id storj.NodeID) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)
	query := `SELECT comp_at_rest, comp_get, comp_get_repair, comp_get_audit FROM paystubs WHERE satellite_id = ?`
	rows, err := db.QueryContext(ctx, query, id)
	if err != nil {
		return 0, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()
	var totalEarned int64
	for rows.Next() {
		var compAtRest, compGet, compGetRepair, compGetAudit int64
		err := rows.Scan(&compAtRest, &compGet, &compGetRepair, &compGetAudit)
		if err != nil {
			return 0, ErrPayout.Wrap(err)
		}
		totalEarned += compGetAudit + compGet + compGetRepair + compAtRest
	}
	if err = rows.Err(); err != nil {
		return 0, ErrPayout.Wrap(err)
	}
	return totalEarned, nil
}

// GetPayingSatellitesIDs returns list of satellite ID's that ever paid to storagenode.
func (db *payoutDB) GetPayingSatellitesIDs(ctx context.Context) (_ []storj.NodeID, err error) {
	defer mon.Task()(&ctx)(&err)
	query := `SELECT DISTINCT (satellite_id) FROM paystubs`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()
	var satelliteIDs []storj.NodeID
	for rows.Next() {
		var satelliteID storj.NodeID
		err := rows.Scan(&satelliteID)
		if err != nil {
			return nil, ErrPayout.Wrap(err)
		}
		satelliteIDs = append(satelliteIDs, satelliteID)
	}
	if err = rows.Err(); err != nil {
		return nil, ErrPayout.Wrap(err)
	}
	return satelliteIDs, nil
}
