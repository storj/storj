// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"strings"

	"storj.io/storj/pkg/pb"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/storage"
)

type repairQueue struct {
	db *dbx.DB
}

func (r *repairQueue) Insert(ctx context.Context, seg *pb.InjuredSegment) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO injuredsegments ( path, data ) VALUES ( ?, ? )`), seg.Path, seg)
	if strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "violates unique constraint") {
		return nil // quietly fail on reinsert
	}
	return err
}

func (r *repairQueue) Select(ctx context.Context) (seg *pb.InjuredSegment, err error) {
	//todo :  add or age > some time (1 hour?)
	err = r.db.QueryRowContext(ctx, `SELECT data FROM injuredsegments ORDER BY id LIMIT 1 WHERE attempted IS NOT NULL`).Scan(&seg)
	return
}

func (r *repairQueue) Delete(ctx context.Context, seg *pb.InjuredSegment) (err error) {
	_, err = r.db.ExecContext(ctx, r.db.Rebind(`DELETE FROM injuredsegments WHERE path = ?`), seg.Path)
	return
}

func (r *repairQueue) SelectN(ctx context.Context, limit int) (segs []pb.InjuredSegment, err error) {
	if limit <= 0 || limit > storage.LookupLimit {
		limit = storage.LookupLimit
	}
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT data FROM injuredsegments ORDER BY id LIMIT ?`), limit)
	for rows.Next() {
		var seg *pb.InjuredSegment
		err = rows.Scan(seg)
		if err != nil {
			return
		}
		segs = append(segs, *seg)
	}
	return
}
