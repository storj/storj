// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"storj.io/common/storj"
	"storj.io/storj/private/currency"
	"storj.io/storj/satellite/compensation"
	"storj.io/storj/satellite/satellitedb/dbx"
)

type compensationDB struct {
	db *satelliteDB
}

// QueryTotalAmounts returns withheld data for the given node.
func (comp *compensationDB) QueryTotalAmounts(ctx context.Context, nodeID storj.NodeID) (_ compensation.TotalAmounts, err error) {
	defer mon.Task()(&ctx)(&err)

	stmt := comp.db.Rebind(`
		SELECT
			coalesce(SUM(held), 0) AS total_held,
			coalesce(SUM(disposed), 0) AS total_disposed,
			coalesce(SUM(paid), 0) AS total_paid,
			coalesce(SUM(distributed), 0) AS total_distributed
		FROM
			storagenode_paystubs
		WHERE
			node_id = ?
	`)

	var totalHeld, totalDisposed, totalPaid, totalDistributed int64
	if err := comp.db.DB.QueryRow(ctx, stmt, nodeID).Scan(&totalHeld, &totalDisposed, &totalPaid, &totalDistributed); err != nil {
		return compensation.TotalAmounts{}, Error.Wrap(err)
	}

	return compensation.TotalAmounts{
		TotalHeld:        currency.NewMicroUnit(totalHeld),
		TotalDisposed:    currency.NewMicroUnit(totalDisposed),
		TotalPaid:        currency.NewMicroUnit(totalPaid),
		TotalDistributed: currency.NewMicroUnit(totalDistributed),
	}, nil
}

func (comp *compensationDB) RecordPeriod(ctx context.Context, paystubs []compensation.Paystub, payments []compensation.Payment) (err error) {
	defer mon.Task()(&ctx)(&err)
	return Error.Wrap(comp.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
		if err := recordPaystubs(ctx, tx, paystubs); err != nil {
			return err
		}
		if err := recordPayments(ctx, tx, payments); err != nil {
			return err
		}
		return nil
	}))
}

func (comp *compensationDB) RecordPayments(ctx context.Context, payments []compensation.Payment) (err error) {
	defer mon.Task()(&ctx)(&err)
	return Error.Wrap(comp.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
		return recordPayments(ctx, tx, payments)
	}))
}

func recordPaystubs(ctx context.Context, tx *dbx.Tx, paystubs []compensation.Paystub) error {
	for _, paystub := range paystubs {
		err := tx.CreateNoReturn_StoragenodePaystub(ctx,
			dbx.StoragenodePaystub_Period(paystub.Period.String()),
			dbx.StoragenodePaystub_NodeId(paystub.NodeID.Bytes()),
			dbx.StoragenodePaystub_Codes(paystub.Codes.String()),
			dbx.StoragenodePaystub_UsageAtRest(paystub.UsageAtRest),
			dbx.StoragenodePaystub_UsageGet(paystub.UsageGet),
			dbx.StoragenodePaystub_UsagePut(paystub.UsagePut),
			dbx.StoragenodePaystub_UsageGetRepair(paystub.UsageGetRepair),
			dbx.StoragenodePaystub_UsagePutRepair(paystub.UsagePutRepair),
			dbx.StoragenodePaystub_UsageGetAudit(paystub.UsageGetAudit),
			dbx.StoragenodePaystub_CompAtRest(paystub.CompAtRest.Value()),
			dbx.StoragenodePaystub_CompGet(paystub.CompGet.Value()),
			dbx.StoragenodePaystub_CompPut(paystub.CompPut.Value()),
			dbx.StoragenodePaystub_CompGetRepair(paystub.CompGetRepair.Value()),
			dbx.StoragenodePaystub_CompPutRepair(paystub.CompPutRepair.Value()),
			dbx.StoragenodePaystub_CompGetAudit(paystub.CompGetAudit.Value()),
			dbx.StoragenodePaystub_SurgePercent(paystub.SurgePercent),
			dbx.StoragenodePaystub_Held(paystub.Held.Value()),
			dbx.StoragenodePaystub_Owed(paystub.Owed.Value()),
			dbx.StoragenodePaystub_Disposed(paystub.Disposed.Value()),
			dbx.StoragenodePaystub_Paid(paystub.Paid.Value()),
			dbx.StoragenodePaystub_Distributed(paystub.Distributed.Value()),
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func recordPayments(ctx context.Context, tx *dbx.Tx, payments []compensation.Payment) error {
	for _, payment := range payments {
		opts := dbx.StoragenodePayment_Create_Fields{}
		if payment.Receipt != nil {
			opts.Receipt = dbx.StoragenodePayment_Receipt(*payment.Receipt)
		}
		if payment.Notes != nil {
			opts.Notes = dbx.StoragenodePayment_Notes(*payment.Notes)
		}
		err := tx.CreateNoReturn_StoragenodePayment(ctx,
			dbx.StoragenodePayment_NodeId(payment.NodeID.Bytes()),
			dbx.StoragenodePayment_Period(payment.Period.String()),
			dbx.StoragenodePayment_Amount(payment.Amount.Value()),
			opts,
		)
		if err != nil {
			return err
		}
	}
	return nil
}
