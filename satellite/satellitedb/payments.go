// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"time"

	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

// Payments represents access to masterdb for the payments service
type payments struct {
	db *dbx.DB
}

// QueryPaymentInfo queries StatDB, Accounting Rollup on nodeID
// TODO: add satellite ID from BW allocation, wallet address from overlay cache
func (db *payments) QueryPaymentInfo(ctx context.Context, start time.Time, end time.Time) ([]*dbx.Node_Id_Node_CreatedAt_Node_AuditSuccessRatio_AccountingRollup_DataType_AccountingRollup_DataTotal_AccountingRollup_CreatedAt_Row, error) {
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
