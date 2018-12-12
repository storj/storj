// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/pkg/accounting"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

//database implements DB
type accountingDB struct {
	db *dbx.DB
}

// LastGranularTime records the greatest last tallied bandwidth agreement time
func (db *accountingDB) LastGranularTime(ctx context.Context) (time.Time, bool, error) {
	lastBwTally, err := db.db.Find_Timestamps_Value_By_Name(ctx, dbx.Timestamps_Name("LastBandwidthTally"))
	if lastBwTally == nil {
		return time.Time{}, true, err
	}
	return lastBwTally.Value, false, err
}

// SaveGranulars records granular tallies (sums of bw agreement values) to the database
// and updates the LastGranularTime
func (db *accountingDB) SaveGranulars(ctx context.Context, logger *zap.Logger, latestBwa time.Time, bwTotals map[string]int64) error {
	// We use the latest bandwidth agreement value of a batch of records as the start of the next batch
	// This enables us to not use:
	// 1) local time (which may deviate from DB time)
	// 2) absolute time intervals (where in processing time could exceed the interval, causing issues)
	// 3) per-node latest times (which simply would require a lot more work, albeit more precise)
	// Any change in these assumptions would result in a change to this function
	// in particular, we should consider finding the sum of bwagreements using SQL sum() direct against the bwa table
	if len(bwTotals) == 0 {
		logger.Warn("In SaveGranulars with empty bwtotals")
		return nil
	}
	//insert all records in a transaction so if we fail, we don't have partial info stored
	tx, err := db.db.Open(ctx)
	if err != nil {
		logger.DPanic("Failed to create DB txn in tally query")
		return err
	}
	defer func() {
		if err == nil {
			err = tx.Commit()
		} else {
			logger.Warn("DB txn was rolled back in tally query")
			err = tx.Rollback()
		}
	}()
	//create a granular record per node id
	for k, v := range bwTotals {
		nID := dbx.Raw_NodeId(k)
		end := dbx.Raw_IntervalEndTime(latestBwa)
		total := dbx.Raw_DataTotal(v)
		dataType := dbx.Raw_DataType(accounting.Bandwith)
		_, err = tx.Create_Raw(ctx, nID, end, total, dataType)
		if err != nil {
			logger.DPanic("Create granular SQL failed in tally query")
			return err
		}
	}
	//save this batch's greatest time
	update := dbx.Timestamps_Update_Fields{Value: dbx.Timestamps_Value(latestBwa)}
	_, err = tx.Update_Timestamps_By_Name(ctx, dbx.Timestamps_Name("LastBandwidthTally"), update)
	return err
}
