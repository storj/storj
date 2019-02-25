// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/utils"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

//database implements DB
type accountingDB struct {
	db *dbx.DB
}

// LastTimestamp records the greatest last tallied time
func (db *accountingDB) LastTimestamp(ctx context.Context, timestampType string) (last time.Time, err error) {
	// todo: use WithTx https://github.com/spacemonkeygo/dbx#transactions
	tx, err := db.db.Open(ctx)
	if err != nil {
		return last, Error.Wrap(err)
	}
	defer func() {
		if err == nil {
			err = tx.Commit()
		} else {
			err = errs.Combine(err, tx.Rollback())
		}
	}()
	lastTally, err := tx.Find_AccountingTimestamps_Value_By_Name(ctx, dbx.AccountingTimestamps_Name(timestampType))
	if lastTally == nil {
		update := dbx.AccountingTimestamps_Value(time.Time{})
		_, err = tx.Create_AccountingTimestamps(ctx, dbx.AccountingTimestamps_Name(timestampType), update)
		return time.Time{}, err
	}
	return lastTally.Value, err
}

// SaveBWRaw records granular tallies (sums of bw agreement values) to the database and updates the LastTimestamp
func (db *accountingDB) SaveBWRaw(ctx context.Context, tallyEnd time.Time, created time.Time, bwTotals map[storj.NodeID][]int64) (err error) {
	// We use the latest bandwidth agreement value of a batch of records as the start of the next batch
	// todo:  consider finding the sum of bwagreements using SQL sum() direct against the bwa table
	if len(bwTotals) == 0 {
		return Error.New("In SaveBWRaw with empty bwtotals")
	}
	//insert all records in a transaction so if we fail, we don't have partial info stored
	tx, err := db.db.Open(ctx)
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() {
		if err == nil {
			err = tx.Commit()
		} else {
			err = utils.CombineErrors(err, tx.Rollback())
		}
	}()
	//create a granular record per node id
	for nodeID, totals := range bwTotals {
		for actionType, total := range totals {
			nID := dbx.AccountingRaw_NodeId(nodeID.Bytes())
			end := dbx.AccountingRaw_IntervalEndTime(tallyEnd)
			total := dbx.AccountingRaw_DataTotal(float64(total))
			dataType := dbx.AccountingRaw_DataType(actionType)
			timestamp := dbx.AccountingRaw_CreatedAt(created)
			_, err = tx.Create_AccountingRaw(ctx, nID, end, total, dataType, timestamp)
			if err != nil {
				return Error.Wrap(err)
			}
		}
	}
	//save this batch's greatest time
	update := dbx.AccountingTimestamps_Update_Fields{Value: dbx.AccountingTimestamps_Value(tallyEnd)}
	_, err = tx.Update_AccountingTimestamps_By_Name(ctx, dbx.AccountingTimestamps_Name(accounting.LastBandwidthTally), update)
	return err
}

// SaveAtRestRaw records raw tallies of at rest data to the database
func (db *accountingDB) SaveAtRestRaw(ctx context.Context, latestTally time.Time, created time.Time, nodeData map[storj.NodeID]float64) error {
	if len(nodeData) == 0 {
		return Error.New("In SaveAtRestRaw with empty nodeData")
	}
	tx, err := db.db.Open(ctx)
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() {
		if err == nil {
			err = tx.Commit()
		} else {
			err = utils.CombineErrors(err, tx.Rollback())
		}
	}()
	for k, v := range nodeData {
		nID := dbx.AccountingRaw_NodeId(k.Bytes())
		end := dbx.AccountingRaw_IntervalEndTime(latestTally)
		total := dbx.AccountingRaw_DataTotal(v)
		dataType := dbx.AccountingRaw_DataType(accounting.AtRest)
		timestamp := dbx.AccountingRaw_CreatedAt(created)
		_, err = tx.Create_AccountingRaw(ctx, nID, end, total, dataType, timestamp)
		if err != nil {
			return Error.Wrap(err)
		}
	}
	update := dbx.AccountingTimestamps_Update_Fields{Value: dbx.AccountingTimestamps_Value(latestTally)}
	_, err = tx.Update_AccountingTimestamps_By_Name(ctx, dbx.AccountingTimestamps_Name(accounting.LastAtRestTally), update)
	return Error.Wrap(err)
}

// GetRaw retrieves all raw tallies
func (db *accountingDB) GetRaw(ctx context.Context) ([]*accounting.Raw, error) {
	raws, err := db.db.All_AccountingRaw(ctx)
	out := make([]*accounting.Raw, len(raws))
	for i, r := range raws {
		nodeID, err := storj.NodeIDFromBytes(r.NodeId)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		out[i] = &accounting.Raw{
			ID:              r.Id,
			NodeID:          nodeID,
			IntervalEndTime: r.IntervalEndTime,
			DataTotal:       r.DataTotal,
			DataType:        r.DataType,
			CreatedAt:       r.CreatedAt,
		}
	}
	return out, Error.Wrap(err)
}

// GetRawSince r retrieves all raw tallies sinces
func (db *accountingDB) GetRawSince(ctx context.Context, latestRollup time.Time) ([]*accounting.Raw, error) {
	raws, err := db.db.All_AccountingRaw_By_IntervalEndTime_GreaterOrEqual(ctx, dbx.AccountingRaw_IntervalEndTime(latestRollup))
	out := make([]*accounting.Raw, len(raws))
	for i, r := range raws {
		nodeID, err := storj.NodeIDFromBytes(r.NodeId)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		out[i] = &accounting.Raw{
			ID:              r.Id,
			NodeID:          nodeID,
			IntervalEndTime: r.IntervalEndTime,
			DataTotal:       r.DataTotal,
			DataType:        r.DataType,
			CreatedAt:       r.CreatedAt,
		}
	}
	return out, Error.Wrap(err)
}

// SaveRollup records raw tallies of at rest data to the database
func (db *accountingDB) SaveRollup(ctx context.Context, latestRollup time.Time, stats accounting.RollupStats) error {
	if len(stats) == 0 {
		return Error.New("In SaveRollup with empty nodeData")
	}
	tx, err := db.db.Open(ctx)
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() {
		if err == nil {
			err = tx.Commit()
		} else {
			err = utils.CombineErrors(err, tx.Rollback())
		}
	}()
	for _, arsByDate := range stats {
		for _, ar := range arsByDate {
			nID := dbx.AccountingRollup_NodeId(ar.NodeID.Bytes())
			start := dbx.AccountingRollup_StartTime(ar.StartTime)
			put := dbx.AccountingRollup_PutTotal(ar.PutTotal)
			get := dbx.AccountingRollup_GetTotal(ar.GetTotal)
			audit := dbx.AccountingRollup_GetAuditTotal(ar.GetAuditTotal)
			getRepair := dbx.AccountingRollup_GetRepairTotal(ar.GetRepairTotal)
			putRepair := dbx.AccountingRollup_PutRepairTotal(ar.PutRepairTotal)
			atRest := dbx.AccountingRollup_AtRestTotal(ar.AtRestTotal)
			_, err = tx.Create_AccountingRollup(ctx, nID, start, put, get, audit, getRepair, putRepair, atRest)
			if err != nil {
				return Error.Wrap(err)
			}
		}
	}
	update := dbx.AccountingTimestamps_Update_Fields{Value: dbx.AccountingTimestamps_Value(latestRollup)}
	_, err = tx.Update_AccountingTimestamps_By_Name(ctx, dbx.AccountingTimestamps_Name(accounting.LastRollup), update)
	return Error.Wrap(err)
}

// QueryPaymentInfo queries StatDB, Accounting Rollup on nodeID
func (db *accountingDB) QueryPaymentInfo(ctx context.Context, start time.Time, end time.Time) ([]*accounting.CSVRow, error) {
	var sql = `SELECT n.id, n.created_at, n.audit_success_ratio, r.at_rest_total, r.get_repair_total,
	    r.put_repair_total, r.get_audit_total, r.put_total, r.get_total, o.operator_wallet
	    FROM (
			SELECT node_id, SUM(at_rest_total) AS at_rest_total, SUM(get_repair_total) AS get_repair_total, 
			SUM(put_repair_total) AS put_repair_total, SUM(get_audit_total) AS get_audit_total, 
			SUM(put_total) AS put_total, SUM(get_total) AS get_total
			FROM accounting_rollups
			WHERE start_time >= ? AND start_time < ?
			GROUP BY node_id
		) r
		LEFT JOIN nodes n ON n.id = r.node_id
		LEFT JOIN overlay_cache_nodes o ON n.id = o.node_id
	    ORDER BY n.id`
	rows, err := db.db.DB.Query(db.db.Rebind(sql), start.UTC(), end.UTC())
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()
	csv := make([]*accounting.CSVRow, 0, 0)
	for rows.Next() {
		var nodeID []byte
		r := &accounting.CSVRow{}
		err := rows.Scan(&nodeID, &r.NodeCreationDate, &r.AuditSuccessRatio, &r.AtRestTotal, &r.GetRepairTotal,
			&r.PutRepairTotal, &r.GetAuditTotal, &r.PutTotal, &r.GetTotal, &r.Wallet)
		if err != nil {
			return csv, Error.Wrap(err)
		}
		id, err := storj.NodeIDFromBytes(nodeID)
		if err != nil {
			return csv, Error.Wrap(err)
		}
		r.NodeID = id
		csv = append(csv, r)
	}
	return csv, nil
}
