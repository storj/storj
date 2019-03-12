// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/lib/pq"
	"github.com/mattn/go-sqlite3"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/utils"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/storage"
)

type repairQueue struct {
	db *dbx.DB
}

func (r *repairQueue) Enqueue(ctx context.Context, seg *pb.InjuredSegment) error {
	val, err := proto.Marshal(seg)
	if err != nil {
		return err
	}

	_, err = r.db.Create_Injuredsegment(
		ctx,
		dbx.Injuredsegment_Info(val),
	)
	return err
}

func (r *repairQueue) Dequeue(ctx context.Context) (seg pb.InjuredSegment, err error) {
	// note: BeginTx(ctx, &sql.TxOptions{Isolation: ...) didn't work
	// so we're using SQL 'FOR UPDATE' below instead
	tx, err := r.db.DB.Begin()
	if err != nil {
		return seg, Error.Wrap(err)
	}
	defer func() {
		if err == nil {
			err = Error.Wrap(tx.Commit())
		} else {
			err = Error.Wrap(utils.CombineErrors(tx.Rollback(), err))
		}
	}()
	//get top
	selectSQL := ""
	switch t := r.db.DB.Driver().(type) {
	case *sqlite3.SQLiteDriver:
		selectSQL = `SELECT id, info FROM injuredsegments LIMIT 1`
	case *pq.Driver:
		selectSQL = `SELECT id, info FROM injuredsegments LIMIT 1 FOR UPDATE`
	default:
		return seg, fmt.Errorf("Unsupported driver %t", t)
	}
	rows, err := tx.Query(selectSQL)
	if err != nil {
		return seg, err
	}
	//defer func() { err = errs.Combine(err, rows.Close()) }()
	if !rows.Next() {
		rows.Close()
		return seg, rows.Err()
	}
	var id int64
	err = rows.Scan(&id, &seg)
	if err != nil {
		rows.Close()
		return seg, err
	}
	rows.Close()
	//delete
	res, err := tx.Exec(r.db.Rebind(`DELETE FROM injuredsegments WHERE id = ?`), id)
	if err != nil {
		return seg, err
	}
	count, err := res.RowsAffected()
	if count != 1 {
		return seg, fmt.Errorf("Injured segment not deleted")
	}
	return seg, err
}

func (r *repairQueue) Peekqueue(ctx context.Context, limit int) ([]pb.InjuredSegment, error) {
	if limit <= 0 || limit > storage.LookupLimit {
		limit = storage.LookupLimit
	}
	rows, err := r.db.Limited_Injuredsegment(ctx, limit, 0)
	if err != nil {
		return nil, err
	}

	segments := make([]pb.InjuredSegment, 0)
	for _, entry := range rows {
		seg := &pb.InjuredSegment{}
		if err = proto.Unmarshal(entry.Info, seg); err != nil {
			return nil, err
		}
		segments = append(segments, *seg)
	}
	return segments, nil
}
