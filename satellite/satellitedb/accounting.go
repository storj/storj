// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"time"

	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/utils"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

//database implements DB
type accountingDB struct {
	db *dbx.DB
}

// LastRawTime records the greatest last tallied time
func (db *accountingDB) LastRawTime(ctx context.Context, timestampType string) (time.Time, bool, error) {
	lastTally, err := db.db.Find_AccountingTimestamps_Value_By_Name(ctx, dbx.AccountingTimestamps_Name(timestampType))
	if lastTally == nil {
		return time.Time{}, true, err
	}
	return lastTally.Value, false, err
}

// SaveBWRaw records granular tallies (sums of bw agreement values) to the database
// and updates the LastRawTime
func (db *accountingDB) SaveBWRaw(ctx context.Context, latestBwa time.Time, bwTotals accounting.BWTally) (err error) {
	// We use the latest bandwidth agreement value of a batch of records as the start of the next batch
	// This enables us to not use:
	// 1) local time (which may deviate from DB time)
	// 2) absolute time intervals (where in processing time could exceed the interval, causing issues)
	// 3) per-node latest times (which simply would require a lot more work, albeit more precise)
	// Any change in these assumptions would result in a change to this function
	// in particular, we should consider finding the sum of bwagreements using SQL sum() direct against the bwa table
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
	for actionType, bwActionTotals := range bwTotals {
		for k, v := range bwActionTotals {
			nID := dbx.AccountingRaw_NodeId(k.Bytes())
			end := dbx.AccountingRaw_IntervalEndTime(latestBwa)
			total := dbx.AccountingRaw_DataTotal(v)
			dataType := dbx.AccountingRaw_DataType(actionType)
			_, err = tx.Create_AccountingRaw(ctx, nID, end, total, dataType)
			if err != nil {
				return Error.Wrap(err)
			}
		}
	}
	//save this batch's greatest time
	update := dbx.AccountingTimestamps_Update_Fields{Value: dbx.AccountingTimestamps_Value(latestBwa)}
	_, err = tx.Update_AccountingTimestamps_By_Name(ctx, dbx.AccountingTimestamps_Name(accounting.LastBandwidthTally), update)
	return err
}

// SaveAtRestRaw records raw tallies of at rest data to the database
func (db *accountingDB) SaveAtRestRaw(ctx context.Context, latestTally time.Time, nodeData map[storj.NodeID]int64) error {
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
		_, err = tx.Create_AccountingRaw(ctx, nID, end, total, dataType)
		if err != nil {
			return Error.Wrap(err)
		}
	}
	update := dbx.AccountingTimestamps_Update_Fields{Value: dbx.AccountingTimestamps_Value(latestTally)}
	_, err = tx.Update_AccountingTimestamps_By_Name(ctx, dbx.AccountingTimestamps_Name(accounting.LastAtRestTally), update)
	return Error.Wrap(err)
}

// QueryPaymentInfo queries StatDB, Accounting Rollup on nodeID
func (db *accountingDB) QueryPaymentInfo(ctx context.Context, start time.Time, end time.Time) ([]*dbx.Node_Id_Node_CreatedAt_Node_AuditSuccessRatio_AccountingRollup_DataType_AccountingRollup_DataTotal_AccountingRollup_CreatedAt_Row, error) {
	tx, err := db.db.Open(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	s := dbx.AccountingRollup_CreatedAt(start)
	e := dbx.AccountingRollup_CreatedAt(end)
	rows, err := tx.All_Node_Id_Node_CreatedAt_Node_AuditSuccessRatio_AccountingRollup_DataType_AccountingRollup_DataTotal_AccountingRollup_CreatedAt_By_AccountingRollup_CreatedAt_GreaterOrEqual_And_AccountingRollup_CreatedAt_Less_OrderBy_Asc_Node_Id(ctx, s, e)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return rows, nil
}

