// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"storj.io/private/tagsql"
	"storj.io/storj/storagenode/planneddowntime"
)

// PlannedDowntimeDBName represents the database name.
const PlannedDowntimeDBName = "planned_downtime"

// ErrPlannedDowntime represents errors from the planned downtime database.
var ErrPlannedDowntime = errs.Class("planned downtime db")

type plannedDowntimeDB struct {
	dbContainerImpl
}

// Add inserts piece information into the database.
func (db *plannedDowntimeDB) Add(ctx context.Context, planned planneddowntime.Entry) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO
			planned_downtime(id, start, end, scheduled_at)
		VALUES (?,?,?,?)
	`, planned.ID, planned.Start.UTC(), planned.End.UTC(), planned.ScheduledAt.UTC())

	return ErrPlannedDowntime.Wrap(err)
}

// Delete deletes an existing planned downtime entry.
func (db *plannedDowntimeDB) Delete(ctx context.Context, id []byte) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.ExecContext(ctx, `
		DELETE FROM planned_downtime
		WHERE id = ?
	`, id)

	return ErrPieceInfo.Wrap(err)
}

// GetScheduled gets a list of current and future planned downtimes.
func (db *plannedDowntimeDB) GetScheduled(ctx context.Context, since time.Time) (result []planneddowntime.Entry, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := db.QueryContext(ctx, `
		SELECT id, start, end, scheduled_at
		FROM planned_downtime
		WHERE end >= ?
		ORDER BY start ASC
	`, since.UTC())
	if err != nil {
		return nil, ErrPlannedDowntime.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	return parseRows(rows)
}

// GetCompleted gets a list of completed planned downtimes.
func (db *plannedDowntimeDB) GetCompleted(ctx context.Context, before time.Time) (result []planneddowntime.Entry, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := db.QueryContext(ctx, `
		SELECT id, start, end, scheduled_at
		FROM planned_downtime
		WHERE end < ?
		ORDER BY start ASC
	`, before.UTC())
	if err != nil {
		return nil, ErrPlannedDowntime.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	return parseRows(rows)
}

func parseRows(rows tagsql.Rows) (result []planneddowntime.Entry, err error) {
	for rows.Next() {
		newEntry := planneddowntime.Entry{}
		err = rows.Scan(&newEntry.ID, &newEntry.Start, &newEntry.End, &newEntry.ScheduledAt)
		if err != nil {
			return nil, ErrPlannedDowntime.Wrap(err)
		}
		result = append(result, newEntry)
	}
	return result, rows.Err()
}
