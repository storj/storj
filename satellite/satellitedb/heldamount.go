// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/heldamount"
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
		&payStub.Paid,
		&payStub.Disposed,
		&payStub.Owed,
		&payStub.Held,
		&payStub.SurgePercent,
		&payStub.CompGetAudit,
		&payStub.CompPutRepair,
		&payStub.CompGetRepair,
		&payStub.CompAtRest,
		&payStub.UsageGetAudit,
		&payStub.UsagePutRepair,
		&payStub.UsageGetRepair,
		&payStub.UsageAtRest,
		&payStub.Codes,
		&payStub.NodeID,
		&payStub.Created,
	)
	if err != nil {
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
		return heldamount.StoragenodePayment{}, Error.Wrap(err)
	}

	return payment, nil
}
