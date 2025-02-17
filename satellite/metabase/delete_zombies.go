// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"time"

	"cloud.google.com/go/spanner"
	"go.uber.org/zap"
)

// DeleteZombieObjects contains all the information necessary to delete zombie objects and segments.
type DeleteZombieObjects struct {
	DeadlineBefore     time.Time
	InactiveDeadline   time.Time
	AsOfSystemInterval time.Duration
	BatchSize          int
}

// DeleteZombieObjects deletes all objects that zombie deletion deadline passed.
// TODO will be removed when objects table will be free from pending objects.
func (db *DB) DeleteZombieObjects(ctx context.Context, opts DeleteZombieObjects) (err error) {
	defer mon.Task()(&ctx)(&err)

	for _, adapter := range db.adapters {
		err := adapter.IterateZombieObjects(ctx, opts, func(ctx context.Context, objects []ObjectStream) error {
			objectsDeleted, segmentsDeleted, err := adapter.DeleteInactiveObjectsAndSegments(ctx, objects, opts)
			if err != nil {
				return Error.Wrap(err)
			}

			mon.Meter("zombie_object_delete").Mark64(objectsDeleted)
			mon.Meter("object_delete").Mark64(objectsDeleted)
			mon.Meter("zombie_segment_delete").Mark64(segmentsDeleted)
			mon.Meter("segment_delete").Mark64(segmentsDeleted)
			return nil
		})
		if err != nil {
			db.log.Warn("delete from DB zombie objects", zap.Error(err))
		}
	}
	return nil
}

type postgresStatement struct {
	SQL    string
	Params []any
}

// IterateZombieObjects iterates over all zombie objects and calls process with at most opts.BatchSize objects.
func (p *PostgresAdapter) IterateZombieObjects(ctx context.Context, opts DeleteZombieObjects, process func(context.Context, []ObjectStream) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	return Error.Wrap(p.processObjectStreamBatches(ctx, opts.AsOfSystemInterval, opts.BatchSize, postgresStatement{
		SQL: `
			SELECT
				project_id, bucket_name, object_key, version, stream_id
			FROM objects
			WHERE
				status = ` + statusPending + `
				AND (zombie_deletion_deadline IS NULL OR zombie_deletion_deadline < $1)
		`,
		Params: []any{
			opts.DeadlineBefore,
		},
	}, process))
}

// IterateZombieObjects iterates over all zombie objects and calls process with at most opts.BatchSize objects.
func (s *SpannerAdapter) IterateZombieObjects(ctx context.Context, opts DeleteZombieObjects, process func(context.Context, []ObjectStream) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	return Error.Wrap(s.processObjectStreamBatches(ctx, opts.AsOfSystemInterval, opts.BatchSize, spanner.Statement{
		SQL: `
			SELECT
				project_id, bucket_name, object_key, version, stream_id
			FROM objects
			WHERE
				status = ` + statusPending + `
				AND (zombie_deletion_deadline IS NULL OR zombie_deletion_deadline < @deadline)
		`,
		Params: map[string]interface{}{
			"deadline": opts.DeadlineBefore,
		},
	}, process))
}
