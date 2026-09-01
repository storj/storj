// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"slices"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/private/currency"
	"storj.io/storj/satellite/compensation"
	"storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/pgutil"
)

type compensationDB struct {
	db *satelliteDB
}

// QueryTotalAmounts returns withheld data for the given node. When genesis is
// non-nil, only paystubs with period >= genesis are aggregated. The period
// column is YYYY-MM text, so a lexicographic comparison is a chronological one.
func (comp *compensationDB) QueryTotalAmounts(ctx context.Context, nodeID storj.NodeID, genesis *compensation.Period) (_ compensation.TotalAmounts, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `
		SELECT
			coalesce(SUM(held), 0) AS total_held,
			coalesce(SUM(disposed), 0) AS total_disposed,
			coalesce(SUM(paid), 0) AS total_paid,
			coalesce(SUM(distributed), 0) AS total_distributed
		FROM
			storagenode_paystubs
		WHERE
			node_id = ?
	`
	args := []interface{}{nodeID}
	if genesis != nil {
		query += ` AND period >= ?`
		args = append(args, genesis.String())
	}

	var totalHeld, totalDisposed, totalPaid, totalDistributed int64
	if err := comp.db.DB.QueryRowContext(ctx, comp.db.Rebind(query), args...).Scan(&totalHeld, &totalDisposed, &totalPaid, &totalDistributed); err != nil {
		return compensation.TotalAmounts{}, Error.Wrap(err)
	}

	return compensation.TotalAmounts{
		TotalHeld:        currency.NewMicroUnit(totalHeld),
		TotalDisposed:    currency.NewMicroUnit(totalDisposed),
		TotalPaid:        currency.NewMicroUnit(totalPaid),
		TotalDistributed: currency.NewMicroUnit(totalDistributed),
	}, nil
}

// QueryAllTotalAmounts returns withheld data for every node with at least one
// paystub row, in a single aggregate query. When genesis is non-nil, only
// paystubs with period >= genesis are aggregated.
func (comp *compensationDB) QueryAllTotalAmounts(ctx context.Context, genesis *compensation.Period) (_ map[storj.NodeID]compensation.TotalAmounts, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `
		SELECT
			node_id,
			SUM(held)        AS total_held,
			SUM(disposed)    AS total_disposed,
			SUM(paid)        AS total_paid,
			SUM(distributed) AS total_distributed
		FROM
			storagenode_paystubs
	`
	var args []interface{}
	if genesis != nil {
		query += ` WHERE period >= ? `
		args = append(args, genesis.String())
	}
	query += ` GROUP BY node_id`

	rows, err := comp.db.DB.QueryContext(ctx, comp.db.Rebind(query), args...)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	result := make(map[storj.NodeID]compensation.TotalAmounts)
	for rows.Next() {
		var nodeID storj.NodeID
		var totalHeld, totalDisposed, totalPaid, totalDistributed int64
		if err := rows.Scan(&nodeID, &totalHeld, &totalDisposed, &totalPaid, &totalDistributed); err != nil {
			return nil, Error.Wrap(err)
		}
		result[nodeID] = compensation.TotalAmounts{
			TotalHeld:        currency.NewMicroUnit(totalHeld),
			TotalDisposed:    currency.NewMicroUnit(totalDisposed),
			TotalPaid:        currency.NewMicroUnit(totalPaid),
			TotalDistributed: currency.NewMicroUnit(totalDistributed),
		}
	}
	return result, Error.Wrap(rows.Err())
}

func (comp *compensationDB) RecordPeriod(ctx context.Context, paystubs []compensation.Paystub, payments []compensation.Payment) (err error) {
	defer mon.Task()(&ctx)(&err)
	if err := comp.RecordPaystubs(ctx, paystubs); err != nil {
		return err
	}
	if err := comp.RecordPayments(ctx, payments); err != nil {
		return err
	}
	return nil
}

func stringPointersEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func (comp *compensationDB) RecordPayments(ctx context.Context, payments []compensation.Payment) (err error) {
	defer mon.Task()(&ctx)(&err)

	for _, payment := range payments {
		payment := payment // to satisfy linting

		err := comp.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
			existingPayments, err := tx.All_StoragenodePayment_By_NodeId_And_Period(ctx,
				dbx.StoragenodePayment_NodeId(payment.NodeID.Bytes()),
				dbx.StoragenodePayment_Period(payment.Period.String()))
			if err != nil {
				return Error.Wrap(err)
			}

			// check if the payment already exists. we know period and node id already match.
			for _, existingPayment := range existingPayments {
				if existingPayment.Amount == payment.Amount.Value() &&
					stringPointersEqual(existingPayment.Receipt, payment.Receipt) &&
					stringPointersEqual(existingPayment.Notes, payment.Notes) {
					return nil
				}
			}

			return Error.Wrap(tx.CreateNoReturn_StoragenodePayment(ctx,
				dbx.StoragenodePayment_NodeId(payment.NodeID.Bytes()),
				dbx.StoragenodePayment_Period(payment.Period.String()),
				dbx.StoragenodePayment_Amount(payment.Amount.Value()),
				dbx.StoragenodePayment_Create_Fields{
					Receipt: dbx.StoragenodePayment_Receipt_Raw(payment.Receipt),
					Notes:   dbx.StoragenodePayment_Notes_Raw(payment.Notes),
				},
			))
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// QueryPaystubConflicts returns the already recorded paystubs that recording
// the given ones would replace.
//
// The rows are collected per period rather than per (period, node), because the
// caller is a one-off admin command that hands over a whole payout file: one
// scan of the periods it touches costs far less than the per-paystub round trip
// RecordPaystubs makes anyway, and it keeps the query free of a node ID list,
// which has no portable binding across the supported backends.
func (comp *compensationDB) QueryPaystubConflicts(ctx context.Context, paystubs []compensation.Paystub) (_ []compensation.PaystubConflict, err error) {
	defer mon.Task()(&ctx)(&err)

	type key struct {
		period string
		nodeID storj.NodeID
	}

	wanted := make(map[key]struct{}, len(paystubs))
	var periods []string
	for _, paystub := range paystubs {
		period := paystub.Period.String()
		if _, ok := wanted[key{period, storj.NodeID(paystub.NodeID)}]; ok {
			continue
		}
		if !slices.Contains(periods, period) {
			periods = append(periods, period)
		}
		wanted[key{period, storj.NodeID(paystub.NodeID)}] = struct{}{}
	}

	var conflicts []compensation.PaystubConflict
	for _, period := range periods {
		parsed, err := compensation.PeriodFromString(period)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		payments, err := comp.queryPaymentCounts(ctx, period)
		if err != nil {
			return nil, err
		}

		rows, err := comp.db.DB.QueryContext(ctx, comp.db.Rebind(`
			SELECT node_id, distributed FROM storagenode_paystubs WHERE period = ?
		`), period)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		err = func() (err error) {
			defer func() { err = errs.Combine(err, rows.Close()) }()
			for rows.Next() {
				var nodeID storj.NodeID
				var distributed int64
				if err := rows.Scan(&nodeID, &distributed); err != nil {
					return Error.Wrap(err)
				}
				if _, ok := wanted[key{period, nodeID}]; !ok {
					continue
				}
				conflicts = append(conflicts, compensation.PaystubConflict{
					Period:      parsed,
					NodeID:      nodeID,
					Distributed: currency.NewMicroUnit(distributed),
					Payments:    payments[nodeID],
				})
			}
			return Error.Wrap(rows.Err())
		}()
		if err != nil {
			return nil, err
		}
	}

	return conflicts, nil
}

// queryPaymentCounts returns the number of storagenode_payments rows recorded
// per node for the given period.
func (comp *compensationDB) queryPaymentCounts(ctx context.Context, period string) (_ map[storj.NodeID]int, err error) {
	rows, err := comp.db.DB.QueryContext(ctx, comp.db.Rebind(`
		SELECT node_id, count(*) FROM storagenode_payments WHERE period = ? GROUP BY node_id
	`), period)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	counts := make(map[storj.NodeID]int)
	for rows.Next() {
		var nodeID storj.NodeID
		var count int64
		if err := rows.Scan(&nodeID, &count); err != nil {
			return nil, Error.Wrap(err)
		}
		counts[nodeID] = int(count)
	}
	return counts, Error.Wrap(rows.Err())
}

// paystubBatchSize is the number of paystubs sent per statement. The rows travel
// as parallel arrays, so this is not bounded by the statement parameter limit;
// it keeps the memory of a single statement, and the intents a batch adds to the
// transaction, from scaling with the number of nodes on the satellite.
const paystubBatchSize = 1000

// RecordPaystubs records the paystubs, upserting them in batches inside a
// single transaction on the backends that support it. A paystub replaces the
// existing one for its (period, node_id).
func (comp *compensationDB) RecordPaystubs(ctx context.Context, paystubs []compensation.Paystub) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(paystubs) == 0 {
		return nil
	}

	// A batch cannot carry the same (period, node_id) twice: the upsert would
	// try to touch the same row a second time and the whole statement fails.
	// Rejecting the input outright is the honest answer, since a paystubs file
	// listing a node twice for a period does not say what the node is owed.
	seen := make(map[compensation.Period]map[compensation.NodeID]struct{}, 1)
	for _, paystub := range paystubs {
		nodes, ok := seen[paystub.Period]
		if !ok {
			nodes = make(map[compensation.NodeID]struct{}, len(paystubs))
			seen[paystub.Period] = nodes
		}
		if _, ok := nodes[paystub.NodeID]; ok {
			return Error.New("duplicate paystub for node %q in period %q", paystub.NodeID, paystub.Period)
		}
		nodes[paystub.NodeID] = struct{}{}
	}

	// Only postgres and cockroach take the rows as one array per column; the
	// other backends keep the statement-per-paystub path, which is slower but
	// portable.
	switch comp.db.impl {
	case dbutil.Postgres, dbutil.Cockroach:
	default:
		return comp.recordPaystubsIndividually(ctx, paystubs)
	}

	return Error.Wrap(comp.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
		for start := 0; start < len(paystubs); start += paystubBatchSize {
			end := min(start+paystubBatchSize, len(paystubs))
			if err := comp.recordPaystubBatch(ctx, tx, paystubs[start:end]); err != nil {
				return err
			}
		}
		return nil
	}))
}

// recordPaystubsIndividually upserts the paystubs one statement at a time, for
// the backends the array-based batch does not support.
func (comp *compensationDB) recordPaystubsIndividually(ctx context.Context, paystubs []compensation.Paystub) (err error) {
	defer mon.Task()(&ctx)(&err)

	for _, paystub := range paystubs {
		err := comp.db.ReplaceNoReturn_StoragenodePaystub(ctx,
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

// recordPaystubBatch upserts one batch of paystubs with a single statement,
// passing the rows as one array per column.
func (comp *compensationDB) recordPaystubBatch(ctx context.Context, tx *dbx.Tx, paystubs []compensation.Paystub) (err error) {
	defer mon.Task()(&ctx)(&err)

	periods := make([]string, len(paystubs))
	nodeIDs := make([]storj.NodeID, len(paystubs))
	codes := make([]string, len(paystubs))
	usageAtRest := make([]float64, len(paystubs))
	usageGet := make([]int64, len(paystubs))
	usagePut := make([]int64, len(paystubs))
	usageGetRepair := make([]int64, len(paystubs))
	usagePutRepair := make([]int64, len(paystubs))
	usageGetAudit := make([]int64, len(paystubs))
	compAtRest := make([]int64, len(paystubs))
	compGet := make([]int64, len(paystubs))
	compPut := make([]int64, len(paystubs))
	compGetRepair := make([]int64, len(paystubs))
	compPutRepair := make([]int64, len(paystubs))
	compGetAudit := make([]int64, len(paystubs))
	surgePercent := make([]int64, len(paystubs))
	held := make([]int64, len(paystubs))
	owed := make([]int64, len(paystubs))
	disposed := make([]int64, len(paystubs))
	paid := make([]int64, len(paystubs))
	distributed := make([]int64, len(paystubs))

	for i, paystub := range paystubs {
		periods[i] = paystub.Period.String()
		nodeIDs[i] = storj.NodeID(paystub.NodeID)
		codes[i] = paystub.Codes.String()
		usageAtRest[i] = paystub.UsageAtRest
		usageGet[i] = paystub.UsageGet
		usagePut[i] = paystub.UsagePut
		usageGetRepair[i] = paystub.UsageGetRepair
		usagePutRepair[i] = paystub.UsagePutRepair
		usageGetAudit[i] = paystub.UsageGetAudit
		compAtRest[i] = paystub.CompAtRest.Value()
		compGet[i] = paystub.CompGet.Value()
		compPut[i] = paystub.CompPut.Value()
		compGetRepair[i] = paystub.CompGetRepair.Value()
		compPutRepair[i] = paystub.CompPutRepair.Value()
		compGetAudit[i] = paystub.CompGetAudit.Value()
		surgePercent[i] = paystub.SurgePercent
		held[i] = paystub.Held.Value()
		owed[i] = paystub.Owed.Value()
		disposed[i] = paystub.Disposed.Value()
		paid[i] = paystub.Paid.Value()
		distributed[i] = paystub.Distributed.Value()
	}

	_, err = tx.Tx.ExecContext(ctx, comp.db.Rebind(`
		INSERT INTO storagenode_paystubs (
			period, node_id, created_at, codes,
			usage_at_rest, usage_get, usage_put,
			usage_get_repair, usage_put_repair, usage_get_audit,
			comp_at_rest, comp_get, comp_put,
			comp_get_repair, comp_put_repair, comp_get_audit,
			surge_percent, held, owed, disposed, paid, distributed
		) SELECT
			unnest($1::text[]), unnest($2::bytea[]), $3, unnest($4::text[]),
			unnest($5::float8[]), unnest($6::int8[]), unnest($7::int8[]),
			unnest($8::int8[]), unnest($9::int8[]), unnest($10::int8[]),
			unnest($11::int8[]), unnest($12::int8[]), unnest($13::int8[]),
			unnest($14::int8[]), unnest($15::int8[]), unnest($16::int8[]),
			unnest($17::int8[]), unnest($18::int8[]), unnest($19::int8[]),
			unnest($20::int8[]), unnest($21::int8[]), unnest($22::int8[])
		ON CONFLICT ( period, node_id ) DO UPDATE SET
			created_at = EXCLUDED.created_at,
			codes = EXCLUDED.codes,
			usage_at_rest = EXCLUDED.usage_at_rest,
			usage_get = EXCLUDED.usage_get,
			usage_put = EXCLUDED.usage_put,
			usage_get_repair = EXCLUDED.usage_get_repair,
			usage_put_repair = EXCLUDED.usage_put_repair,
			usage_get_audit = EXCLUDED.usage_get_audit,
			comp_at_rest = EXCLUDED.comp_at_rest,
			comp_get = EXCLUDED.comp_get,
			comp_put = EXCLUDED.comp_put,
			comp_get_repair = EXCLUDED.comp_get_repair,
			comp_put_repair = EXCLUDED.comp_put_repair,
			comp_get_audit = EXCLUDED.comp_get_audit,
			surge_percent = EXCLUDED.surge_percent,
			held = EXCLUDED.held,
			owed = EXCLUDED.owed,
			disposed = EXCLUDED.disposed,
			paid = EXCLUDED.paid,
			distributed = EXCLUDED.distributed`),
		pgutil.TextArray(periods),
		pgutil.NodeIDArray(nodeIDs),
		time.Now().UTC(),
		pgutil.TextArray(codes),
		pgutil.Float8Array(usageAtRest),
		pgutil.Int8Array(usageGet),
		pgutil.Int8Array(usagePut),
		pgutil.Int8Array(usageGetRepair),
		pgutil.Int8Array(usagePutRepair),
		pgutil.Int8Array(usageGetAudit),
		pgutil.Int8Array(compAtRest),
		pgutil.Int8Array(compGet),
		pgutil.Int8Array(compPut),
		pgutil.Int8Array(compGetRepair),
		pgutil.Int8Array(compPutRepair),
		pgutil.Int8Array(compGetAudit),
		pgutil.Int8Array(surgePercent),
		pgutil.Int8Array(held),
		pgutil.Int8Array(owed),
		pgutil.Int8Array(disposed),
		pgutil.Int8Array(paid),
		pgutil.Int8Array(distributed),
	)
	return Error.Wrap(err)
}
