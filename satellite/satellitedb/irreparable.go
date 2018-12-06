// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"storj.io/storj/pkg/datarepair"
	"storj.io/storj/pkg/utils"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type irreparable struct {
	db *dbx.DB
}

// IncrementRepairAttempts a db entry for to increment the repair attempts field
func (db *irreparable) IncrementRepairAttempts(ctx context.Context, segmentInfo *datarepair.RemoteSegmentInfo) (err error) {
	tx, err := db.db.Begin()
	if err != nil {
		return err
	}

	dbxInfo, err := db.Get(ctx, segmentInfo.EncryptedSegmentPath)
	if err != nil {
		// no rows err, so create/insert an entry
		_, err = db.db.Create_Irreparabledb(
			ctx,
			dbx.Irreparabledb_Segmentpath(segmentInfo.EncryptedSegmentPath),
			dbx.Irreparabledb_Segmentdetail(segmentInfo.EncryptedSegmentDetail),
			dbx.Irreparabledb_PiecesLostCount(segmentInfo.LostPiecesCount),
			dbx.Irreparabledb_SegDamagedUnixSec(segmentInfo.RepairUnixSec),
			dbx.Irreparabledb_RepairAttemptCount(segmentInfo.RepairAttemptCount),
		)
		if err != nil {
			return utils.CombineErrors(err, tx.Rollback())
		}
	} else {
		// row exits increment the attempt counter
		dbxInfo.RepairAttemptCount++
		updateFields := dbx.Irreparabledb_Update_Fields{}
		updateFields.RepairAttemptCount = dbx.Irreparabledb_RepairAttemptCount(dbxInfo.RepairAttemptCount)
		_, err = db.db.Update_Irreparabledb_By_Segmentpath(
			ctx,
			dbx.Irreparabledb_Segmentpath(dbxInfo.EncryptedSegmentPath),
			updateFields,
		)
		if err != nil {
			return utils.CombineErrors(err, tx.Rollback())
		}
	}

	return tx.Commit()
}

// Get a irreparable's segment info from the db
func (db *irreparable) Get(ctx context.Context, segmentPath []byte) (resp *datarepair.RemoteSegmentInfo, err error) {
	dbxInfo, err := db.db.Get_Irreparabledb_By_Segmentpath(ctx, dbx.Irreparabledb_Segmentpath(segmentPath))
	if err != nil {
		return &datarepair.RemoteSegmentInfo{}, err
	}

	return &datarepair.RemoteSegmentInfo{
		EncryptedSegmentPath:   dbxInfo.Segmentpath,
		EncryptedSegmentDetail: dbxInfo.Segmentdetail,
		LostPiecesCount:        dbxInfo.PiecesLostCount,
		RepairUnixSec:          dbxInfo.SegDamagedUnixSec,
		RepairAttemptCount:     dbxInfo.RepairAttemptCount,
	}, nil
}

// Delete a irreparable's segment info from the db
func (db *irreparable) Delete(ctx context.Context, segmentPath []byte) (err error) {
	_, err = db.db.Delete_Irreparabledb_By_Segmentpath(ctx, dbx.Irreparabledb_Segmentpath(segmentPath))

	return err
}

// Close  close db connection
func (db *irreparable) Close() (err error) {
	return db.db.Close()
}
