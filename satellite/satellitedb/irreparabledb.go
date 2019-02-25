// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"storj.io/storj/pkg/datarepair/irreparable"
	"storj.io/storj/pkg/utils"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type irreparableDB struct {
	db *dbx.DB
}

// IncrementRepairAttempts a db entry for to increment the repair attempts field
func (db *irreparableDB) IncrementRepairAttempts(ctx context.Context, segmentInfo *irreparable.RemoteSegmentInfo) (err error) {
	tx, err := db.db.Open(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	dbxInfo, err := tx.Get_Irreparabledb_By_Segmentpath(ctx, dbx.Irreparabledb_Segmentpath(segmentInfo.EncryptedSegmentPath))
	if err != nil {
		// no rows err, so create/insert an entry
		_, err = tx.Create_Irreparabledb(
			ctx,
			dbx.Irreparabledb_Segmentpath(segmentInfo.EncryptedSegmentPath),
			dbx.Irreparabledb_Segmentdetail(segmentInfo.EncryptedSegmentDetail),
			dbx.Irreparabledb_PiecesLostCount(segmentInfo.LostPiecesCount),
			dbx.Irreparabledb_SegDamagedUnixSec(segmentInfo.RepairUnixSec),
			dbx.Irreparabledb_RepairAttemptCount(segmentInfo.RepairAttemptCount),
		)
		if err != nil {
			return Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
		}
	} else {
		// row exits increment the attempt counter
		dbxInfo.RepairAttemptCount++
		updateFields := dbx.Irreparabledb_Update_Fields{}
		updateFields.RepairAttemptCount = dbx.Irreparabledb_RepairAttemptCount(dbxInfo.RepairAttemptCount)
		updateFields.SegDamagedUnixSec = dbx.Irreparabledb_SegDamagedUnixSec(segmentInfo.RepairUnixSec)
		_, err = tx.Update_Irreparabledb_By_Segmentpath(
			ctx,
			dbx.Irreparabledb_Segmentpath(dbxInfo.Segmentpath),
			updateFields,
		)
		if err != nil {
			return Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
		}
	}

	return Error.Wrap(tx.Commit())
}

// Get a irreparable's segment info from the db
func (db *irreparableDB) Get(ctx context.Context, segmentPath []byte) (resp *irreparable.RemoteSegmentInfo, err error) {
	dbxInfo, err := db.db.Get_Irreparabledb_By_Segmentpath(ctx, dbx.Irreparabledb_Segmentpath(segmentPath))
	if err != nil {
		return &irreparable.RemoteSegmentInfo{}, Error.Wrap(err)
	}

	return &irreparable.RemoteSegmentInfo{
		EncryptedSegmentPath:   dbxInfo.Segmentpath,
		EncryptedSegmentDetail: dbxInfo.Segmentdetail,
		LostPiecesCount:        dbxInfo.PiecesLostCount,
		RepairUnixSec:          dbxInfo.SegDamagedUnixSec,
		RepairAttemptCount:     dbxInfo.RepairAttemptCount,
	}, nil
}

// Delete a irreparable's segment info from the db
func (db *irreparableDB) Delete(ctx context.Context, segmentPath []byte) (err error) {
	_, err = db.db.Delete_Irreparabledb_By_Segmentpath(ctx, dbx.Irreparabledb_Segmentpath(segmentPath))

	return Error.Wrap(err)
}
