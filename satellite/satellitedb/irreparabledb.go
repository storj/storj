// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"errors"

	"storj.io/common/pb"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/metainfo/metabase"
	"storj.io/storj/satellite/satellitedb/dbx"
)

type irreparableDB struct {
	db *satelliteDB
}

// IncrementRepairAttempts a db entry for to increment the repair attempts field.
func (db *irreparableDB) IncrementRepairAttempts(ctx context.Context, segmentInfo *internalpb.IrreparableSegment) (err error) {
	defer mon.Task()(&ctx)(&err)
	err = db.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) (err error) {
		bytes, err := pb.Marshal(segmentInfo.SegmentDetail)
		if err != nil {
			return err
		}

		dbxInfo, err := tx.Get_Irreparabledb_By_Segmentpath(ctx, dbx.Irreparabledb_Segmentpath(segmentInfo.Path))
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				// no rows err, so create/insert an entry
				return tx.CreateNoReturn_Irreparabledb(
					ctx,
					dbx.Irreparabledb_Segmentpath(segmentInfo.Path),
					dbx.Irreparabledb_Segmentdetail(bytes),
					dbx.Irreparabledb_PiecesLostCount(int64(segmentInfo.LostPieces)),
					dbx.Irreparabledb_SegDamagedUnixSec(segmentInfo.LastRepairAttempt),
					dbx.Irreparabledb_RepairAttemptCount(segmentInfo.RepairAttemptCount),
				)
			}
			return err
		}

		// row exits increment the attempt counter
		dbxInfo.RepairAttemptCount++
		updateFields := dbx.Irreparabledb_Update_Fields{}
		updateFields.RepairAttemptCount = dbx.Irreparabledb_RepairAttemptCount(dbxInfo.RepairAttemptCount)
		updateFields.SegDamagedUnixSec = dbx.Irreparabledb_SegDamagedUnixSec(segmentInfo.LastRepairAttempt)
		err = tx.UpdateNoReturn_Irreparabledb_By_Segmentpath(
			ctx,
			dbx.Irreparabledb_Segmentpath(dbxInfo.Segmentpath),
			updateFields,
		)
		return err
	})
	return Error.Wrap(err)
}

// Get a irreparable's segment info from the db.
func (db *irreparableDB) Get(ctx context.Context, segmentKey metabase.SegmentKey) (resp *internalpb.IrreparableSegment, err error) {
	defer mon.Task()(&ctx)(&err)
	dbxInfo, err := db.db.Get_Irreparabledb_By_Segmentpath(ctx, dbx.Irreparabledb_Segmentpath(segmentKey))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	p := &pb.Pointer{}

	err = pb.Unmarshal(dbxInfo.Segmentdetail, p)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &internalpb.IrreparableSegment{
		Path:               dbxInfo.Segmentpath,
		SegmentDetail:      p,
		LostPieces:         int32(dbxInfo.PiecesLostCount),
		LastRepairAttempt:  dbxInfo.SegDamagedUnixSec,
		RepairAttemptCount: dbxInfo.RepairAttemptCount,
	}, nil
}

// GetLimited returns a list of irreparable segment info starting after the last segment info we retrieved.
func (db *irreparableDB) GetLimited(ctx context.Context, limit int, lastSeenSegmentKey metabase.SegmentKey) (resp []*internalpb.IrreparableSegment, err error) {
	defer mon.Task()(&ctx)(&err)
	// the offset is hardcoded to 0 since we are using the lastSeenSegmentPath to
	// indicate the item we last listed instead. In a perfect world this db query would
	// not take an offset as an argument, but currently dbx only supports `limitoffset`
	const offset = 0
	rows, err := db.db.Limited_Irreparabledb_By_Segmentpath_Greater_OrderBy_Asc_Segmentpath(ctx,
		dbx.Irreparabledb_Segmentpath(lastSeenSegmentKey),
		limit, offset,
	)
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		p := &pb.Pointer{}
		err = pb.Unmarshal(row.Segmentdetail, p)
		if err != nil {
			return nil, err
		}
		segment := &internalpb.IrreparableSegment{
			Path:               row.Segmentpath,
			SegmentDetail:      p,
			LostPieces:         int32(row.PiecesLostCount),
			LastRepairAttempt:  row.SegDamagedUnixSec,
			RepairAttemptCount: row.RepairAttemptCount,
		}
		resp = append(resp, segment)
	}
	return resp, err
}

// Delete a irreparable's segment info from the db.
func (db *irreparableDB) Delete(ctx context.Context, segmentKey metabase.SegmentKey) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = db.db.Delete_Irreparabledb_By_Segmentpath(ctx, dbx.Irreparabledb_Segmentpath(segmentKey))

	return Error.Wrap(err)
}
