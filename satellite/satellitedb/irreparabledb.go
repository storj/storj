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

	dbxInfo, err := tx.Get_IrreparableSegment_By_Segmentpath(ctx, dbx.IrreparableSegment_Segmentpath(segmentInfo.EncryptedSegmentPath))
	if err != nil {
		// no rows err, so create/insert an entry
		_, err = tx.Create_IrreparableSegment(
			ctx,
			dbx.IrreparableSegment_Segmentpath(segmentInfo.EncryptedSegmentPath),
			dbx.IrreparableSegment_Segmentdetail(segmentInfo.EncryptedSegmentDetail),
			dbx.IrreparableSegment_PiecesLostCount(segmentInfo.LostPiecesCount),
			dbx.IrreparableSegment_SegDamagedUnixSec(segmentInfo.RepairUnixSec),
			dbx.IrreparableSegment_RepairAttemptCount(segmentInfo.RepairAttemptCount),
		)
		if err != nil {
			return Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
		}
	} else {
		// row exits increment the attempt counter
		dbxInfo.RepairAttemptCount++
		updateFields := dbx.IrreparableSegment_Update_Fields{}
		updateFields.RepairAttemptCount = dbx.IrreparableSegment_RepairAttemptCount(dbxInfo.RepairAttemptCount)
		updateFields.SegDamagedUnixSec = dbx.IrreparableSegment_SegDamagedUnixSec(segmentInfo.RepairUnixSec)
		_, err = tx.Update_IrreparableSegment_By_Segmentpath(
			ctx,
			dbx.IrreparableSegment_Segmentpath(dbxInfo.Segmentpath),
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
	dbxInfo, err := db.db.Get_IrreparableSegment_By_Segmentpath(ctx, dbx.IrreparableSegment_Segmentpath(segmentPath))
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

// Getlimited number of irreparable segments by offset
func (db *irreparableDB) GetLimited(ctx context.Context, limit int, offset int64) (resp []*irreparable.RemoteSegmentInfo, err error) {
	rows, err := db.db.Limited_IrreparableSegment(ctx, limit, offset)
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		segment := &irreparable.RemoteSegmentInfo{
			EncryptedSegmentPath:   row.Segmentpath,
			EncryptedSegmentDetail: row.Segmentdetail,
			LostPiecesCount:        row.PiecesLostCount,
			RepairUnixSec:          row.SegDamagedUnixSec,
			RepairAttemptCount:     row.RepairAttemptCount,
		}
		resp = append(resp, segment)
	}
	return resp, err
}

// Delete a irreparable's segment info from the db
func (db *irreparableDB) Delete(ctx context.Context, segmentPath []byte) (err error) {
	_, err = db.db.Delete_IrreparableSegment_By_Segmentpath(ctx, dbx.IrreparableSegment_Segmentpath(segmentPath))

	return Error.Wrap(err)
}
