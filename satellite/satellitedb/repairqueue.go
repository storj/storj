// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"

	"github.com/zeebo/errs"

	"storj.io/common/pb"
	"storj.io/storj/private/dbutil"
	"storj.io/storj/private/dbutil/pgutil"
	"storj.io/storj/storage"
)

// RepairQueueSelectLimit defines how many items can be selected at the same time.
const RepairQueueSelectLimit = 1000

type repairQueue struct {
	db *satelliteDB
}

func (r *repairQueue) Insert(ctx context.Context, seg *pb.InjuredSegment) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO injuredsegments ( path, data ) VALUES ( ?, ? )`), seg.Path, seg)
	if err != nil {
		if pgutil.IsConstraintError(err) {
			return nil // quietly fail on reinsert
		}
		return err
	}
	return nil
}

func (r *repairQueue) Select(ctx context.Context) (seg *pb.InjuredSegment, err error) {
	defer mon.Task()(&ctx)(&err)
	switch r.db.implementation {
	case dbutil.Cockroach:
		err = r.db.QueryRowContext(ctx, `
				UPDATE injuredsegments SET attempted = now() AT TIME ZONE 'UTC' WHERE path = (
					SELECT path FROM injuredsegments
					WHERE attempted IS NULL OR attempted < now() AT TIME ZONE 'UTC' - interval '1 hour'
					ORDER BY attempted LIMIT 1
				) RETURNING data`).Scan(&seg)
	case dbutil.Postgres:
		err = r.db.QueryRowContext(ctx, `
				UPDATE injuredsegments SET attempted = now() AT TIME ZONE 'UTC' WHERE path = (
					SELECT path FROM injuredsegments
					WHERE attempted IS NULL OR attempted < now() AT TIME ZONE 'UTC' - interval '1 hour'
					ORDER BY attempted NULLS FIRST FOR UPDATE SKIP LOCKED LIMIT 1
				) RETURNING data`).Scan(&seg)
	default:
		return seg, errs.New("invalid dbType: %v", r.db.implementation)
	}
	if err == sql.ErrNoRows {
		err = storage.ErrEmptyQueue.New("")
	}
	return seg, err
}

func (r *repairQueue) Delete(ctx context.Context, seg *pb.InjuredSegment) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = r.db.ExecContext(ctx, r.db.Rebind(`DELETE FROM injuredsegments WHERE path = ?`), seg.Path)
	return Error.Wrap(err)
}

func (r *repairQueue) SelectN(ctx context.Context, limit int) (segs []pb.InjuredSegment, err error) {
	defer mon.Task()(&ctx)(&err)
	if limit <= 0 || limit > RepairQueueSelectLimit {
		limit = RepairQueueSelectLimit
	}
	//todo: strictly enforce order-by or change tests
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT data FROM injuredsegments LIMIT ?`), limit)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	for rows.Next() {
		var seg pb.InjuredSegment
		err = rows.Scan(&seg)
		if err != nil {
			return segs, Error.Wrap(err)
		}
		segs = append(segs, seg)
	}

	return segs, Error.Wrap(rows.Err())
}

func (r *repairQueue) Count(ctx context.Context) (count int, err error) {
	defer mon.Task()(&ctx)(&err)

	// Count every segment regardless of how recently repair was last attempted
	err = r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT COUNT(*) as count FROM injuredsegments`)).Scan(&count)

	return count, Error.Wrap(err)
}
