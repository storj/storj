// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"time"

	"storj.io/common/uuid"
)

// GetProjectSegmentCount contains arguments necessary for fetching an information
// about project segment count.
type GetProjectSegmentCount struct {
	ProjectID uuid.UUID

	AsOfSystemTime     time.Time
	AsOfSystemInterval time.Duration
}

// Verify verifies reqest fields.
func (g *GetProjectSegmentCount) Verify() error {
	if g.ProjectID.IsZero() {
		return ErrInvalidRequest.New("ProjectID missing")
	}
	return nil
}

// GetProjectSegmentCount returns number of segments that specified project has.
func (db *DB) GetProjectSegmentCount(ctx context.Context, opts GetProjectSegmentCount) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return 0, err
	}

	var segmentsCount *int64
	err = db.db.QueryRowContext(ctx, `
		SELECT
			sum(segment_count)
		FROM objects
		`+db.asOfTime(opts.AsOfSystemTime, opts.AsOfSystemInterval)+`
		WHERE
			project_id = $1
		`, opts.ProjectID).Scan(&segmentsCount)
	if err != nil {
		return 0, Error.New("unable to query project segment count: %w", err)
	}

	if segmentsCount == nil {
		return 0, Error.New("project not found: %s", opts.ProjectID)
	}

	return *segmentsCount, nil
}
