// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/private/dbutil"
	"storj.io/storj/satellite/accounting"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

// StoragenodeAccounting implements the accounting/db StoragenodeAccounting interface
type StoragenodeAccounting struct {
	db *satelliteDB
}

// SaveTallies records raw tallies of at rest data to the database
func (db *StoragenodeAccounting) SaveTallies(ctx context.Context, latestTally time.Time, nodeData map[storj.NodeID]float64) (err error) {
	defer mon.Task()(&ctx)(&err)
	if len(nodeData) == 0 {
		return Error.New("In SaveTallies with empty nodeData")
	}
	err = db.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
		for k, v := range nodeData {
			nID := dbx.StoragenodeStorageTally_NodeId(k.Bytes())
			end := dbx.StoragenodeStorageTally_IntervalEndTime(latestTally)
			total := dbx.StoragenodeStorageTally_DataTotal(v)
			err := tx.CreateNoReturn_StoragenodeStorageTally(ctx, nID, end, total)
			if err != nil {
				return err
			}
		}
		return tx.UpdateNoReturn_AccountingTimestamps_By_Name(ctx,
			dbx.AccountingTimestamps_Name(accounting.LastAtRestTally),
			dbx.AccountingTimestamps_Update_Fields{
				Value: dbx.AccountingTimestamps_Value(latestTally),
			},
		)
	})
	return Error.Wrap(err)
}

// GetTallies retrieves all raw tallies
func (db *StoragenodeAccounting) GetTallies(ctx context.Context) (_ []*accounting.StoragenodeStorageTally, err error) {
	defer mon.Task()(&ctx)(&err)
	raws, err := db.db.All_StoragenodeStorageTally(ctx)
	out := make([]*accounting.StoragenodeStorageTally, len(raws))
	for i, r := range raws {
		nodeID, err := storj.NodeIDFromBytes(r.NodeId)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		out[i] = &accounting.StoragenodeStorageTally{
			ID:              r.Id,
			NodeID:          nodeID,
			IntervalEndTime: r.IntervalEndTime,
			DataTotal:       r.DataTotal,
		}
	}
	return out, Error.Wrap(err)
}

// GetTalliesSince retrieves all raw tallies since latestRollup
func (db *StoragenodeAccounting) GetTalliesSince(ctx context.Context, latestRollup time.Time) (_ []*accounting.StoragenodeStorageTally, err error) {
	defer mon.Task()(&ctx)(&err)
	raws, err := db.db.All_StoragenodeStorageTally_By_IntervalEndTime_GreaterOrEqual(ctx, dbx.StoragenodeStorageTally_IntervalEndTime(latestRollup))
	out := make([]*accounting.StoragenodeStorageTally, len(raws))
	for i, r := range raws {
		nodeID, err := storj.NodeIDFromBytes(r.NodeId)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		out[i] = &accounting.StoragenodeStorageTally{
			ID:              r.Id,
			NodeID:          nodeID,
			IntervalEndTime: r.IntervalEndTime,
			DataTotal:       r.DataTotal,
		}
	}
	return out, Error.Wrap(err)
}

// GetBandwidthSince retrieves all storagenode_bandwidth_rollup entires since latestRollup
func (db *StoragenodeAccounting) GetBandwidthSince(ctx context.Context, latestRollup time.Time) (_ []*accounting.StoragenodeBandwidthRollup, err error) {
	defer mon.Task()(&ctx)(&err)
	rollups, err := db.db.All_StoragenodeBandwidthRollup_By_IntervalStart_GreaterOrEqual(ctx, dbx.StoragenodeBandwidthRollup_IntervalStart(latestRollup))
	out := make([]*accounting.StoragenodeBandwidthRollup, len(rollups))
	for i, r := range rollups {
		nodeID, err := storj.NodeIDFromBytes(r.StoragenodeId)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		out[i] = &accounting.StoragenodeBandwidthRollup{
			NodeID:        nodeID,
			IntervalStart: r.IntervalStart,
			Action:        r.Action,
			Settled:       r.Settled,
		}
	}
	return out, Error.Wrap(err)
}

// SaveRollup records raw tallies of at rest data to the database
func (db *StoragenodeAccounting) SaveRollup(ctx context.Context, latestRollup time.Time, stats accounting.RollupStats) (err error) {
	defer mon.Task()(&ctx)(&err)
	if len(stats) == 0 {
		return Error.New("In SaveRollup with empty nodeData")
	}
	err = db.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
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

				err := tx.CreateNoReturn_AccountingRollup(ctx, nID, start, put, get, audit, getRepair, putRepair, atRest)
				if err != nil {
					return err
				}
			}
		}

		return tx.UpdateNoReturn_AccountingTimestamps_By_Name(ctx,
			dbx.AccountingTimestamps_Name(accounting.LastRollup),
			dbx.AccountingTimestamps_Update_Fields{
				Value: dbx.AccountingTimestamps_Value(latestRollup),
			},
		)
	})
	return Error.Wrap(err)
}

// LastTimestamp records the greatest last tallied time
func (db *StoragenodeAccounting) LastTimestamp(ctx context.Context, timestampType string) (_ time.Time, err error) {
	defer mon.Task()(&ctx)(&err)
	lastTally := time.Time{}
	err = db.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
		lt, err := tx.Find_AccountingTimestamps_Value_By_Name(ctx, dbx.AccountingTimestamps_Name(timestampType))
		if lt == nil {
			return tx.CreateNoReturn_AccountingTimestamps(ctx,
				dbx.AccountingTimestamps_Name(timestampType),
				dbx.AccountingTimestamps_Value(lastTally),
			)
		}
		lastTally = lt.Value
		return err
	})
	return lastTally, err
}

// QueryPaymentInfo queries Overlay, Accounting Rollup on nodeID
func (db *StoragenodeAccounting) QueryPaymentInfo(ctx context.Context, start time.Time, end time.Time) (_ []*accounting.CSVRow, err error) {
	defer mon.Task()(&ctx)(&err)
	var sqlStmt = `SELECT n.id, n.created_at, r.at_rest_total, r.get_repair_total,
	    r.put_repair_total, r.get_audit_total, r.put_total, r.get_total, n.wallet, n.disqualified
	    FROM (
			SELECT node_id, SUM(at_rest_total) AS at_rest_total, SUM(get_repair_total) AS get_repair_total,
			SUM(put_repair_total) AS put_repair_total, SUM(get_audit_total) AS get_audit_total,
			SUM(put_total) AS put_total, SUM(get_total) AS get_total
			FROM accounting_rollups
			WHERE start_time >= ? AND start_time < ?
			GROUP BY node_id
		) r
		LEFT JOIN nodes n ON n.id = r.node_id
	    ORDER BY n.id`
	rows, err := db.db.DB.QueryContext(ctx, db.db.Rebind(sqlStmt), start.UTC(), end.UTC())
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()
	csv := []*accounting.CSVRow{}
	for rows.Next() {
		var nodeID []byte
		r := &accounting.CSVRow{}
		var wallet sql.NullString
		var disqualified *time.Time
		err := rows.Scan(&nodeID, &r.NodeCreationDate, &r.AtRestTotal, &r.GetRepairTotal,
			&r.PutRepairTotal, &r.GetAuditTotal, &r.PutTotal, &r.GetTotal, &wallet, &disqualified)
		if err != nil {
			return csv, Error.Wrap(err)
		}
		if wallet.Valid {
			r.Wallet = wallet.String
		}
		id, err := storj.NodeIDFromBytes(nodeID)
		if err != nil {
			return csv, Error.Wrap(err)
		}
		r.NodeID = id
		r.Disqualified = disqualified
		csv = append(csv, r)
	}
	return csv, nil
}

// QueryStorageNodeUsage returns slice of StorageNodeUsage for given period
func (db *StoragenodeAccounting) QueryStorageNodeUsage(ctx context.Context, nodeID storj.NodeID, start time.Time, end time.Time) (_ []accounting.StorageNodeUsage, err error) {
	defer mon.Task()(&ctx)(&err)

	lastRollup, err := db.db.Find_AccountingTimestamps_Value_By_Name(ctx, dbx.AccountingTimestamps_Name(accounting.LastRollup))
	if err != nil {
		return nil, Error.Wrap(err)
	}
	if lastRollup == nil {
		return nil, nil
	}

	start, end = start.UTC(), end.UTC()

	query := `
		SELECT at_rest_total, start_time::date
		FROM accounting_rollups
		WHERE node_id = $1
		AND $2 <= start_time AND start_time <= $3
		UNION
		SELECT SUM(data_total) AS at_rest_total, interval_end_time::date AS start_time
				FROM storagenode_storage_tallies
				WHERE node_id = $1
				AND NOT EXISTS (
					SELECT 1 FROM accounting_rollups
					WHERE node_id = $1
					AND $2 <= start_time AND start_time <= $3
					AND start_time::date = interval_end_time::date
				)
				AND (SELECT value FROM accounting_timestamps WHERE name = $4) < interval_end_time AND interval_end_time <= $3
				GROUP BY interval_end_time::date
		ORDER BY start_time;
	`

	rows, err := db.db.QueryContext(ctx, db.db.Rebind(query),
		nodeID, start, end, accounting.LastRollup,
	)

	if err != nil {
		return nil, Error.Wrap(err)
	}

	defer func() {
		err = errs.Combine(err, rows.Close())
	}()

	var nodeStorageUsages []accounting.StorageNodeUsage
	for rows.Next() {
		var atRestTotal float64
		var startTime dbutil.NullTime

		err = rows.Scan(&atRestTotal, &startTime)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		nodeStorageUsages = append(nodeStorageUsages, accounting.StorageNodeUsage{
			NodeID:      nodeID,
			StorageUsed: atRestTotal,
			Timestamp:   startTime.Time,
		})
	}

	return nodeStorageUsages, nil
}

// DeleteTalliesBefore deletes all raw tallies prior to some time
func (db *StoragenodeAccounting) DeleteTalliesBefore(ctx context.Context, latestRollup time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)
	var deleteRawSQL = `DELETE FROM storagenode_storage_tallies WHERE interval_end_time < ?`
	_, err = db.db.DB.ExecContext(ctx, db.db.Rebind(deleteRawSQL), latestRollup)
	return err
}
