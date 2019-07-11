// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lib/pq"
	sqlite3 "github.com/mattn/go-sqlite3"

	"storj.io/storj/internal/dbutil/pgutil"
	"storj.io/storj/internal/dbutil/sqliteutil"
	"storj.io/storj/pkg/pb"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/storage"
)

type repairQueue struct {
	db *dbx.DB
}

func (r *repairQueue) Insert(ctx context.Context, seg *pb.InjuredSegment) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO injuredsegments ( path, data ) VALUES ( ?, ? )`), seg.Path, seg)
	if err != nil {
		if pgutil.IsConstraintError(err) || sqliteutil.IsConstraintError(err) {
			return nil // quietly fail on reinsert
		}
		return err
	}
	return nil
}

func (r *repairQueue) postgresSelect(ctx context.Context) (seg *pb.InjuredSegment, err error) {
	defer mon.Task()(&ctx)(&err)
	err = r.db.QueryRowContext(ctx, `
	UPDATE injuredsegments SET attempted = timezone('utc', now()) WHERE path = (
		SELECT path FROM injuredsegments
		WHERE attempted IS NULL OR attempted < timezone('utc', now()) - interval '1 hour'
		ORDER BY path FOR UPDATE SKIP LOCKED LIMIT 1
	) RETURNING data`).Scan(&seg)
	if err == sql.ErrNoRows {
		err = storage.ErrEmptyQueue.New("")
	}
	return
}

func (r *repairQueue) sqliteSelect(ctx context.Context) (seg *pb.InjuredSegment, err error) {
	defer mon.Task()(&ctx)(&err)
	err = r.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
		var path []byte
		err = tx.Tx.QueryRowContext(ctx, r.db.Rebind(`
			SELECT path, data FROM injuredsegments
			WHERE attempted IS NULL
			OR attempted < datetime('now','-1 hours')
			ORDER BY path LIMIT 1`)).Scan(&path, &seg)
		if err != nil {
			return err
		}
		res, err := tx.Tx.ExecContext(ctx, r.db.Rebind(`UPDATE injuredsegments SET attempted = datetime('now') WHERE path = ?`), path)
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

func (r *repairQueue) Select(ctx context.Context) (seg *pb.InjuredSegment, err error) {
	defer mon.Task()(&ctx)(&err)
	switch t := r.db.DB.Driver().(type) {
	case *sqlite3.SQLiteDriver:
		return r.sqliteSelect(ctx)
	case *pq.Driver:
		return r.postgresSelect(ctx)
	default:
		return seg, fmt.Errorf("Unsupported database %t", t)
	}
}

func (r *repairQueue) Delete(ctx context.Context, seg *pb.InjuredSegment) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = r.db.ExecContext(ctx, r.db.Rebind(`DELETE FROM injuredsegments WHERE path = ?`), seg.Path)
	return
}

func (r *repairQueue) SelectN(ctx context.Context, limit int) (segs []pb.InjuredSegment, err error) {
	defer mon.Task()(&ctx)(&err)
	if limit <= 0 || limit > storage.LookupLimit {
		limit = storage.LookupLimit
	}
	//todo: strictly enforce order-by or change tests
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT data FROM injuredsegments LIMIT ?`), limit)
	for rows.Next() {
		var seg pb.InjuredSegment
		err = rows.Scan(&seg)
		if err != nil {
			return
		}
		segs = append(segs, seg)
	}
	return segs, rows.Err()
}
