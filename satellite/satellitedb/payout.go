// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"errors"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/satellite/snopayout"
)

// paymentStubs is payment data for specific storagenode for some specific period by working with satellite.
//
// architecture: Database
type paymentStubs struct {
	db *satelliteDB
}

// GetPaystub returns payStub by nodeID and period.
func (paystubs *paymentStubs) GetPaystub(ctx context.Context, nodeID storj.NodeID, period string) (payStub snopayout.PayStub, err error) {
	query := `SELECT * FROM storagenode_paystubs WHERE node_id = $1 AND period = $2;`

	row := paystubs.db.QueryRowContext(ctx, query, nodeID, period)
	err = row.Scan(
		&payStub.Period,
		&payStub.NodeID,
		&payStub.Created,
		&payStub.Codes,
		&payStub.UsageAtRest,
		&payStub.UsageGet,
		&payStub.UsagePut,
		&payStub.UsageGetRepair,
		&payStub.UsagePutRepair,
		&payStub.UsageGetAudit,
		&payStub.CompAtRest,
		&payStub.CompGet,
		&payStub.CompPut,
		&payStub.CompGetRepair,
		&payStub.CompPutRepair,
		&payStub.CompGetAudit,
		&payStub.SurgePercent,
		&payStub.Held,
		&payStub.Owed,
		&payStub.Disposed,
		&payStub.Paid,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return snopayout.PayStub{}, snopayout.ErrNoDataForPeriod.Wrap(err)
		}

		return snopayout.PayStub{}, Error.Wrap(err)
	}

	return payStub, nil
}

// GetAllPaystubs return all payStubs by nodeID.
func (paystubs *paymentStubs) GetAllPaystubs(ctx context.Context, nodeID storj.NodeID) (payStubs []snopayout.PayStub, err error) {
	query := `SELECT * FROM storagenode_paystubs WHERE node_id = $1;`

	rows, err := paystubs.db.QueryContext(ctx, query, nodeID)
	if err != nil {
		return []snopayout.PayStub{}, Error.Wrap(err)
	}

	defer func() {
		err = errs.Combine(err, Error.Wrap(rows.Close()))
	}()

	for rows.Next() {
		paystub := snopayout.PayStub{}

		err = rows.Scan(
			&paystub.Period,
			&paystub.NodeID,
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
		if err = rows.Err(); err != nil {
			return []snopayout.PayStub{}, Error.Wrap(err)
		}

		payStubs = append(payStubs, paystub)
	}

	return payStubs, Error.Wrap(rows.Err())
}

// CreatePaystub inserts storagenode_paystub into database.
func (paystubs *paymentStubs) CreatePaystub(ctx context.Context, stub snopayout.PayStub) (err error) {
	return paystubs.db.CreateNoReturn_StoragenodePaystub(
		ctx,
		dbx.StoragenodePaystub_Period(stub.Period),
		dbx.StoragenodePaystub_NodeId(stub.NodeID[:]),
		dbx.StoragenodePaystub_Codes(stub.Codes),
		dbx.StoragenodePaystub_UsageAtRest(stub.UsageAtRest),
		dbx.StoragenodePaystub_UsageGet(stub.UsageGet),
		dbx.StoragenodePaystub_UsagePut(stub.UsagePut),
		dbx.StoragenodePaystub_UsageGetRepair(stub.UsageGetRepair),
		dbx.StoragenodePaystub_UsagePutRepair(stub.UsagePutRepair),
		dbx.StoragenodePaystub_UsageGetAudit(stub.UsageGetAudit),
		dbx.StoragenodePaystub_CompAtRest(stub.CompAtRest),
		dbx.StoragenodePaystub_CompGet(stub.CompGet),
		dbx.StoragenodePaystub_CompPut(stub.CompPut),
		dbx.StoragenodePaystub_CompGetRepair(stub.CompGetRepair),
		dbx.StoragenodePaystub_CompPutRepair(stub.CompPutRepair),
		dbx.StoragenodePaystub_CompGetAudit(stub.CompGetAudit),
		dbx.StoragenodePaystub_SurgePercent(stub.SurgePercent),
		dbx.StoragenodePaystub_Held(stub.Held),
		dbx.StoragenodePaystub_Owed(stub.Owed),
		dbx.StoragenodePaystub_Disposed(stub.Disposed),
		dbx.StoragenodePaystub_Paid(stub.Paid),
	)
}

// GetPayment returns payment by nodeID and period.
func (paystubs *paymentStubs) GetPayment(ctx context.Context, nodeID storj.NodeID, period string) (payment snopayout.StoragenodePayment, err error) {
	query := `SELECT * FROM storagenode_payments WHERE node_id = $1 AND period = $2;`

	row := paystubs.db.QueryRowContext(ctx, query, nodeID, period)
	err = row.Scan(
		&payment.ID,
		&payment.Created,
		&payment.NodeID,
		&payment.Period,
		&payment.Amount,
		&payment.Receipt,
		&payment.Notes,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return snopayout.StoragenodePayment{}, snopayout.ErrNoDataForPeriod.Wrap(err)
		}

		return snopayout.StoragenodePayment{}, Error.Wrap(err)
	}

	return payment, nil
}

// CreatePayment inserts storagenode_payment into database.
func (paystubs *paymentStubs) CreatePayment(ctx context.Context, payment snopayout.StoragenodePayment) (err error) {
	return paystubs.db.CreateNoReturn_StoragenodePayment(
		ctx,
		dbx.StoragenodePayment_NodeId(payment.NodeID[:]),
		dbx.StoragenodePayment_Period(payment.Period),
		dbx.StoragenodePayment_Amount(payment.Amount),
		dbx.StoragenodePayment_Create_Fields{
			Receipt: dbx.StoragenodePayment_Receipt(payment.Receipt),
			Notes:   dbx.StoragenodePayment_Notes(payment.Notes),
		},
	)
}

// GetAllPayments return all payments by nodeID.
func (paystubs *paymentStubs) GetAllPayments(ctx context.Context, nodeID storj.NodeID) (payments []snopayout.StoragenodePayment, err error) {
	query := `SELECT * FROM storagenode_payments WHERE node_id = $1;`

	rows, err := paystubs.db.QueryContext(ctx, query, nodeID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	defer func() {
		err = errs.Combine(err, Error.Wrap(rows.Close()))
	}()

	for rows.Next() {
		payment := snopayout.StoragenodePayment{}

		err = rows.Scan(
			&payment.ID,
			&payment.Created,
			&payment.NodeID,
			&payment.Period,
			&payment.Amount,
			&payment.Receipt,
			&payment.Notes,
		)

		if err = rows.Err(); err != nil {
			return nil, Error.Wrap(err)
		}

		payments = append(payments, payment)
	}

	return payments, Error.Wrap(rows.Err())
}
