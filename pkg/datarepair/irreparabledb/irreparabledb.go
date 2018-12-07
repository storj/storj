// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package irreparabledb

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/storj/internal/migrate"
	dbx "storj.io/storj/pkg/datarepair/irreparabledb/dbx"
	"storj.io/storj/pkg/utils"
)

// Error is the default irreparabledb errs class
var Error = errs.Class("irreparabledb error")

// Database implements the irreparable RPC service
type Database struct {
	db *dbx.DB
}

// RemoteSegmentInfo is info about a single entry stored in the irreparable db
type RemoteSegmentInfo struct {
	EncryptedSegmentPath   []byte
	EncryptedSegmentDetail []byte //contains marshaled info of pb.Pointer
	LostPiecesCount        int64
	RepairUnixSec          int64
	RepairAttemptCount     int64
}

// New creates instance of Server
func New(source string) (*Database, error) {
	u, err := utils.ParseURL(source)
	if err != nil {
		return nil, err
	}

	db, err := dbx.Open(u.Scheme, u.Path)
	if err != nil {
		return nil, Error.New("failed opening database %q, %q: %v",
			u.Scheme, u.Path, err)
	}

	err = migrate.Create("irreparabledb", db)
	if err != nil {
		return nil, err
	}

	return &Database{
		db: db,
	}, nil
}

// IncrementRepairAttempts a db entry for to increment the repair attempts field
func (db *Database) IncrementRepairAttempts(ctx context.Context, segmentInfo *RemoteSegmentInfo) (err error) {
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
func (db *Database) Get(ctx context.Context, segmentPath []byte) (resp *RemoteSegmentInfo, err error) {
	dbxInfo, err := db.db.Get_Irreparabledb_By_Segmentpath(ctx, dbx.Irreparabledb_Segmentpath(segmentPath))
	if err != nil {
		return &RemoteSegmentInfo{}, err
	}

	return &RemoteSegmentInfo{
		EncryptedSegmentPath:   dbxInfo.Segmentpath,
		EncryptedSegmentDetail: dbxInfo.Segmentdetail,
		LostPiecesCount:        dbxInfo.PiecesLostCount,
		RepairUnixSec:          dbxInfo.SegDamagedUnixSec,
		RepairAttemptCount:     dbxInfo.RepairAttemptCount,
	}, nil
}

// Delete a irreparable's segment info from the db
func (db *Database) Delete(ctx context.Context, segmentPath []byte) (err error) {
	_, err = db.db.Delete_Irreparabledb_By_Segmentpath(ctx, dbx.Irreparabledb_Segmentpath(segmentPath))

	return err
}

// Close close db connection
func (db *Database) Close() (err error) {
	return db.db.Close()
}
