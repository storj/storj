// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"github.com/golang/protobuf/proto"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type irreparableDB struct {
	db *dbx.DB
}

// IncrementRepairAttempts a db entry for to increment the repair attempts field
func (db *irreparableDB) IncrementRepairAttempts(ctx context.Context, segmentInfo *pb.IrreparableSegment) (err error) {
	tx, err := db.db.Open(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	bytes, err := proto.Marshal(segmentInfo.SegmentDetail)
	if err != nil {
		return err
	}

	dbxInfo, err := tx.Get_Irreparabledb_By_Segmentpath(ctx, dbx.Irreparabledb_Segmentpath(segmentInfo.Path))
	if err != nil {
		// no rows err, so create/insert an entry
		_, err = tx.Create_Irreparabledb(
			ctx,
			dbx.Irreparabledb_Segmentpath(segmentInfo.Path),
			dbx.Irreparabledb_Segmentdetail(bytes),
			dbx.Irreparabledb_PiecesLostCount(int64(segmentInfo.LostPieces)),
			dbx.Irreparabledb_SegDamagedUnixSec(segmentInfo.LastRepairAttempt),
			dbx.Irreparabledb_RepairAttemptCount(segmentInfo.RepairAttemptCount),
		)
		if err != nil {
			return Error.Wrap(errs.Combine(err, tx.Rollback()))
		}
	} else {
		// row exits increment the attempt counter
		dbxInfo.RepairAttemptCount++
		updateFields := dbx.Irreparabledb_Update_Fields{}
		updateFields.RepairAttemptCount = dbx.Irreparabledb_RepairAttemptCount(dbxInfo.RepairAttemptCount)
		updateFields.SegDamagedUnixSec = dbx.Irreparabledb_SegDamagedUnixSec(segmentInfo.LastRepairAttempt)
		_, err = tx.Update_Irreparabledb_By_Segmentpath(
			ctx,
			dbx.Irreparabledb_Segmentpath(dbxInfo.Segmentpath),
			updateFields,
		)
		if err != nil {
			return Error.Wrap(errs.Combine(err, tx.Rollback()))
		}
	}

	return Error.Wrap(tx.Commit())
}

// Get a irreparable's segment info from the db
func (db *irreparableDB) Get(ctx context.Context, segmentPath []byte) (resp *pb.IrreparableSegment, err error) {
	dbxInfo, err := db.db.Get_Irreparabledb_By_Segmentpath(ctx, dbx.Irreparabledb_Segmentpath(segmentPath))
	if err != nil {
		return &pb.IrreparableSegment{}, Error.Wrap(err)
	}

	p := &pb.Pointer{}

	err = proto.Unmarshal(dbxInfo.Segmentdetail, p)
	if err != nil {
		return &pb.IrreparableSegment{}, err
	}

	return &pb.IrreparableSegment{
		Path:               dbxInfo.Segmentpath,
		SegmentDetail:      p,
		LostPieces:         int32(dbxInfo.PiecesLostCount),
		LastRepairAttempt:  dbxInfo.SegDamagedUnixSec,
		RepairAttemptCount: dbxInfo.RepairAttemptCount,
	}, nil
}

// Getlimited number of irreparable segments by offset
func (db *irreparableDB) GetLimited(ctx context.Context, limit int, offset int64) (resp []*pb.IrreparableSegment, err error) {
	rows, err := db.db.Limited_Irreparabledb_OrderBy_Asc_Segmentpath(ctx, limit, offset)
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		p := &pb.Pointer{}
		err = proto.Unmarshal(row.Segmentdetail, p)
		if err != nil {
			return nil, err
		}
		segment := &pb.IrreparableSegment{
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

// Delete a irreparable's segment info from the db
func (db *irreparableDB) Delete(ctx context.Context, segmentPath []byte) (err error) {
	_, err = db.db.Delete_Irreparabledb_By_Segmentpath(ctx, dbx.Irreparabledb_Segmentpath(segmentPath))

	return Error.Wrap(err)
}
