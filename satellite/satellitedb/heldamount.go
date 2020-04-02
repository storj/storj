// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"

	"github.com/zeebo/errs"

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

// GetAllPaystubs return all payStubs by nodeID.
func (paystubs *paymentStubs) GetAllPaystubs(ctx context.Context, nodeID storj.NodeID) (payStubs []heldamount.PayStub, err error) {
	query := `SELECT * FROM storagenode_paystubs WHERE node_id = $1;`

	rows, err := paystubs.db.QueryContext(ctx, query, nodeID)
	if err != nil {
		return []heldamount.PayStub{}, Error.Wrap(err)
	}

	defer func() {
		err = errs.Combine(err, Error.Wrap(rows.Close()))
	}()

	for rows.Next() {
		paystub := heldamount.PayStub{}

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
			return []heldamount.PayStub{}, Error.Wrap(err)
		}

		payStubs = append(payStubs, paystub)
	}

	return payStubs, Error.Wrap(rows.Err())
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
