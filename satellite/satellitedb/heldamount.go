// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/heldamount"
	"storj.io/storj/satellite/satellitedb/dbx"
)

// paymentStubs is payment data for specific storagenode for some specific period by working with satellite.
//
// architecture: Database
type paymentStubs struct {
	db *satelliteDB
}

// GetPaystub returns payStub by nodeID and period.
func (paystubs *paymentStubs) GetPaystub(ctx context.Context, nodeID storj.NodeID, period string) (payStub heldamount.PayStub, err error) {
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
		if sql.ErrNoRows == err {
			return heldamount.PayStub{}, heldamount.ErrNoDataForPeriod.Wrap(err)
		}

		return heldamount.PayStub{}, Error.Wrap(err)
	}

	return payStub, nil
}

// GetPayment returns payment by nodeID and period.
func (paystubs *paymentStubs) GetPayment(ctx context.Context, nodeID storj.NodeID, period string) (payment heldamount.StoragenodePayment, err error) {
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
		if sql.ErrNoRows == err {
			return heldamount.StoragenodePayment{}, heldamount.ErrNoDataForPeriod.Wrap(err)
		}

		return heldamount.StoragenodePayment{}, Error.Wrap(err)
	}

	return payment, nil
}

// CreatePaystub inserts storagenode_paystub into database.
func (paystubs *paymentStubs) CreatePaystub(ctx context.Context, stub heldamount.PayStub) (err error) {
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

// CreatePayment inserts storagenode_payment into database.
func (paystubs *paymentStubs) CreatePayment(ctx context.Context, payment heldamount.StoragenodePayment) (err error) {
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
