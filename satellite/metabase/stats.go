// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/errs2"
)

// GetTableStats contains arguments necessary for getting table statistics.
type GetTableStats struct {
	AsOfSystemInterval time.Duration
}

// TableStats contains information about the metabase status.
type TableStats struct {
	ObjectCount  int64
	SegmentCount int64
}

// GetTableStats gathers information about the metabase tables.
func (db *DB) GetTableStats(ctx context.Context, opts GetTableStats) (result TableStats, err error) {
	defer mon.Task()(&ctx)(&err)

	var group errs2.Group
	group.Go(func() error {
		row := db.db.QueryRowContext(ctx, `SELECT count(*) FROM objects `+db.impl.AsOfSystemInterval(opts.AsOfSystemInterval))
		return Error.Wrap(row.Scan(&result.ObjectCount))
	})
	group.Go(func() error {
		row := db.db.QueryRowContext(ctx, `SELECT count(*) FROM segments `+db.impl.AsOfSystemInterval(opts.AsOfSystemInterval))
		return Error.Wrap(row.Scan(&result.SegmentCount))
	})
	err = errs.Combine(group.Wait()...)
	return result, err
}
