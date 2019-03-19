// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/lib/pq"
	sqlite3 "github.com/mattn/go-sqlite3"

	"storj.io/storj/pkg/pb"
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

func (r *repairQueue) postgresDequeue(ctx context.Context) (seg pb.InjuredSegment, err error) {
	err = r.db.DB.QueryRowContext(ctx, `
	DELETE FROM injuredsegments
		WHERE id = ( SELECT id FROM injuredsegments ORDER BY id FOR UPDATE SKIP LOCKED LIMIT 1 )
		RETURNING info
	`).Scan(&seg)
	if err == sql.ErrNoRows {
		err = storage.ErrEmptyQueue.New("")
	}
	return seg, err
}

func (r *repairQueue) sqliteDequeue(ctx context.Context) (seg pb.InjuredSegment, err error) {
	err = r.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
		var id int64
		err = tx.Tx.QueryRowContext(ctx, `SELECT id, info FROM injuredsegments ORDER BY id LIMIT 1`).Scan(&id, &seg)
		if err != nil {
			return err
		}
		res, err := tx.Tx.ExecContext(ctx, r.db.Rebind(`DELETE FROM injuredsegments WHERE id = ?`), id)
		if err != nil {
			return err
		}
		count, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if count != 1 {
			return fmt.Errorf("Expected 1, got %d segments deleted", count)
		}
		return nil
	})
	if err == sql.ErrNoRows {
		err = storage.ErrEmptyQueue.New("")
	}
	return seg, err
}

func (r *repairQueue) Dequeue(ctx context.Context) (seg pb.InjuredSegment, err error) {
	switch t := r.db.DB.Driver().(type) {
	case *sqlite3.SQLiteDriver:
		return r.sqliteDequeue(ctx)
	case *pq.Driver:
		return r.postgresDequeue(ctx)
	default:
		return seg, fmt.Errorf("Unsupported database %t", t)
	}
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
