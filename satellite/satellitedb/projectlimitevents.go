// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/projectlimitevents"
	"storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/tagsql"
)

var _ projectlimitevents.DB = (*projectLimitEvents)(nil)

type projectLimitEvents struct {
	db *satelliteDB
}

// Insert adds a new project limit event to the queue.
func (p *projectLimitEvents) Insert(ctx context.Context, projectID uuid.UUID, event accounting.ProjectUsageThreshold, isReset bool) (_ projectlimitevents.Event, err error) {
	defer mon.Task()(&ctx)(&err)

	id, err := uuid.New()
	if err != nil {
		return projectlimitevents.Event{}, err
	}

	var optional dbx.ProjectLimitEvent_Create_Fields
	optional.IsReset = dbx.ProjectLimitEvent_IsReset(isReset)

	entry, err := p.db.Create_ProjectLimitEvent(ctx,
		dbx.ProjectLimitEvent_Id(id.Bytes()),
		dbx.ProjectLimitEvent_ProjectId(projectID.Bytes()),
		dbx.ProjectLimitEvent_Event(int(event)),
		optional,
	)
	if err != nil {
		return projectlimitevents.Event{}, Error.Wrap(err)
	}

	return fromDBXProjectLimitEvent(entry)
}

// GetByID returns a single event by ID.
func (p *projectLimitEvents) GetByID(ctx context.Context, id uuid.UUID) (_ projectlimitevents.Event, err error) {
	defer mon.Task()(&ctx)(&err)

	entry, err := p.db.Get_ProjectLimitEvent_By_Id(ctx, dbx.ProjectLimitEvent_Id(id.Bytes()))
	if err != nil {
		return projectlimitevents.Event{}, Error.Wrap(err)
	}

	return fromDBXProjectLimitEvent(entry)
}

// GetNextBatch returns all unprocessed events for the project that has the
// oldest unprocessed event created before firstSeenBefore.
func (p *projectLimitEvents) GetNextBatch(ctx context.Context, firstSeenBefore time.Time) (_ []projectlimitevents.Event, err error) {
	defer mon.Task()(&ctx)(&err)

	var rows tagsql.Rows
	switch p.db.impl {
	case dbutil.Cockroach, dbutil.Postgres:
		rows, err = p.db.QueryContext(ctx, `
			SELECT ple.id, ple.project_id, ple.event, ple.is_reset, ple.created_at
			FROM project_limit_events ple
			INNER JOIN (
				SELECT project_id
				FROM project_limit_events
				WHERE created_at < $1
					AND email_sent IS NULL
				ORDER BY last_attempted ASC NULLS FIRST, created_at ASC
				LIMIT 1
			) AS t ON ple.project_id = t.project_id
			WHERE ple.email_sent IS NULL
		`, firstSeenBefore)
	case dbutil.Spanner:
		// Spanner does not support "NULLS FIRST" but ASC ordering is nulls-first by default.
		rows, err = p.db.QueryContext(ctx, `
			SELECT ple.id, ple.project_id, ple.event, ple.is_reset, ple.created_at
			FROM project_limit_events ple
			INNER JOIN (
				SELECT project_id
				FROM project_limit_events
				WHERE created_at < ?
					AND email_sent IS NULL
				ORDER BY last_attempted ASC, created_at ASC
				LIMIT 1
			) AS t ON ple.project_id = t.project_id
			WHERE ple.email_sent IS NULL
		`, firstSeenBefore)
	default:
		return nil, Error.New("unsupported implementation")
	}
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	var events []projectlimitevents.Event
	for rows.Next() {
		var idBytes []byte
		var projectIDBytes []byte
		var event int
		var isReset bool
		var createdAt time.Time

		err = rows.Scan(&idBytes, &projectIDBytes, &event, &isReset, &createdAt)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		id, err := uuid.FromBytes(idBytes)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		projectID, err := uuid.FromBytes(projectIDBytes)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		events = append(events, projectlimitevents.Event{
			ID:        id,
			ProjectID: projectID,
			Event:     accounting.ProjectUsageThreshold(event),
			IsReset:   isReset,
			CreatedAt: createdAt,
		})
	}

	return events, Error.Wrap(rows.Err())
}

// UpdateEmailSent marks a group of events as processed.
func (p *projectLimitEvents) UpdateEmailSent(ctx context.Context, ids []uuid.UUID, timestamp time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	switch p.db.impl {
	case dbutil.Cockroach, dbutil.Postgres:
		_, err = p.db.ExecContext(ctx, `
			UPDATE project_limit_events SET email_sent = $1
			WHERE id = ANY($2::bytea[])
		`, timestamp, pgutil.UUIDArray(ids))
	case dbutil.Spanner:
		_, err = p.db.ExecContext(ctx, `
			UPDATE project_limit_events SET email_sent = ?
			WHERE id IN UNNEST(?)
		`, timestamp, uuidsToBytesArray(ids))
	default:
		return Error.New("unsupported implementation")
	}
	return Error.Wrap(err)
}

// UpdateLastAttempted updates last_attempted for a group of events.
func (p *projectLimitEvents) UpdateLastAttempted(ctx context.Context, ids []uuid.UUID, timestamp time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	switch p.db.impl {
	case dbutil.Cockroach, dbutil.Postgres:
		_, err = p.db.ExecContext(ctx, `
			UPDATE project_limit_events SET last_attempted = $1
			WHERE id = ANY($2::bytea[])
		`, timestamp, pgutil.UUIDArray(ids))
	case dbutil.Spanner:
		_, err = p.db.ExecContext(ctx, `
			UPDATE project_limit_events SET last_attempted = ?
			WHERE id IN UNNEST(?)
		`, timestamp, uuidsToBytesArray(ids))
	default:
		return Error.New("unsupported implementation")
	}
	return Error.Wrap(err)
}

func fromDBXProjectLimitEvent(entry *dbx.ProjectLimitEvent) (projectlimitevents.Event, error) {
	id, err := uuid.FromBytes(entry.Id)
	if err != nil {
		return projectlimitevents.Event{}, Error.Wrap(err)
	}
	projectID, err := uuid.FromBytes(entry.ProjectId)
	if err != nil {
		return projectlimitevents.Event{}, Error.Wrap(err)
	}
	return projectlimitevents.Event{
		ID:            id,
		ProjectID:     projectID,
		Event:         accounting.ProjectUsageThreshold(entry.Event),
		IsReset:       entry.IsReset,
		CreatedAt:     entry.CreatedAt,
		LastAttempted: entry.LastAttempted,
		EmailSent:     entry.EmailSent,
	}, nil
}
