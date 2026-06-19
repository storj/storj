// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"time"

	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/dx"
	"storj.io/storj/shared/dbutil/tidbutil"
)

// BucketEvent is a row in the bucket_eventing_outbox table.
// ID and CreatedAt are zero when writing; the database assigns them on insert.
type BucketEvent struct {
	ID        int64
	EventName string
	ObjectStream
	TotalPlainSize int64
	CreatedAt      time.Time
}

// enqueueBucketEvent buffers an insert of one or more BucketEvent rows into the
// bucket eventing outbox, flushed together with the transaction's COMMIT and
// discarded if the transaction is rolled back.
func (tx *tidbTransactionAdapter) enqueueBucketEvent(events ...BucketEvent) {
	tidbEnqueueBucketEvent(tx.tx, events...)
}

// enqueueBucketEvent buffers an insert of one or more BucketEvent rows into the
// bucket eventing outbox on tx. The insert is flushed together with the
// transaction's COMMIT; if the transaction is rolled back it is discarded.
func tidbEnqueueBucketEvent(tx *tidbutil.Tx, events ...BucketEvent) {
	if len(events) == 0 {
		return
	}
	tx.EnqueueExec(
		tidbBatchInsertQuery("bucket_eventing_outbox", bucketEventColumns, len(events)),
		bucketEventArgs(events)...,
	)
}

// ReadBucketEventBatch reads up to limit rows from the bucket_eventing_outbox table
// whose id is strictly greater than afterID, ordered by id ascending.
func (t *TiDBAdapter) ReadBucketEventBatch(ctx context.Context, afterID int64, limit int) (_ []BucketEvent, err error) {
	defer mon.Task()(&ctx)(&err)

	var batch []BucketEvent
	err = dx.WithRows(t.db.QueryContext(ctx, `
		SELECT id, project_id, bucket_name, object_key, version, stream_id, total_plain_size, event_name, created_at
		FROM bucket_eventing_outbox
		WHERE id > ?
		ORDER BY id
		LIMIT ?
	`, afterID, limit))(func(rows dx.Rows) error {
		for rows.Next() {
			var r BucketEvent
			var projectIDBytes, streamIDBytes []byte
			if err := rows.Scan(&r.ID, &projectIDBytes, &r.BucketName, &r.ObjectKey, &r.Version, &streamIDBytes, &r.TotalPlainSize, &r.EventName, &r.CreatedAt); err != nil {
				return Error.New("outbox scan: %w", err)
			}
			var parseErr error
			r.ProjectID, parseErr = uuid.FromBytes(projectIDBytes)
			if parseErr != nil {
				return Error.New("outbox scan project_id: %w", parseErr)
			}
			r.StreamID, parseErr = uuid.FromBytes(streamIDBytes)
			if parseErr != nil {
				return Error.New("outbox scan stream_id: %w", parseErr)
			}

			batch = append(batch, r)
		}
		return nil
	})
	return batch, Error.Wrap(err)
}

// DeleteBucketEvents deletes rows from the bucket_eventing_outbox table by ID.
// It is a no-op when ids is empty.
func (t *TiDBAdapter) DeleteBucketEvents(ctx context.Context, ids []int64) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(ids) == 0 {
		return nil
	}

	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	_, err = t.db.ExecContext(ctx,
		"DELETE FROM bucket_eventing_outbox WHERE id IN ("+tidbPlaceholders(len(ids))+")",
		args...)
	return Error.Wrap(err)
}

// TestingInsertBucketEvent inserts a single BucketEvent into the outbox, for use in tests.
func (t *TiDBAdapter) TestingInsertBucketEvent(ctx context.Context, event BucketEvent) error {
	_, err := t.db.ExecContext(ctx,
		tidbBatchInsertQuery("bucket_eventing_outbox", bucketEventColumns, 1),
		bucketEventArgs([]BucketEvent{event})...,
	)
	return Error.Wrap(err)
}

// TestingGetAllBucketEvents returns all rows from the bucket eventing outbox, for use in tests.
func (t *TiDBAdapter) TestingGetAllBucketEvents(ctx context.Context) (_ []BucketEvent, err error) {
	var events []BucketEvent
	err = dx.WithRows(t.db.QueryContext(ctx,
		`SELECT project_id, bucket_name, object_key, version, stream_id, total_plain_size, event_name FROM bucket_eventing_outbox`,
	))(func(rows dx.Rows) error {
		for rows.Next() {
			var e BucketEvent
			if err := rows.Scan(&e.ProjectID, &e.BucketName, &e.ObjectKey, &e.Version, &e.StreamID, &e.TotalPlainSize, &e.EventName); err != nil {
				return Error.New("TestingGetAllBucketEvents scan: %w", err)
			}
			events = append(events, e)
		}
		return nil
	})
	return events, Error.Wrap(err)
}

// TestingCountBucketEvents returns the number of rows in the bucket eventing outbox, for use in tests.
func (t *TiDBAdapter) TestingCountBucketEvents(ctx context.Context) (int, error) {
	var count int
	row := t.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM bucket_eventing_outbox")
	return count, Error.Wrap(row.Scan(&count))
}

var bucketEventColumns = []string{
	"project_id",
	"bucket_name",
	"object_key",
	"version",
	"stream_id",
	"total_plain_size",
	"event_name",
}

func bucketEventArgs(events []BucketEvent) []any {
	args := make([]any, 0, len(events)*len(bucketEventColumns))
	for _, e := range events {
		args = append(args,
			e.ProjectID.Bytes(),
			[]byte(e.BucketName),
			[]byte(e.ObjectKey),
			int64(e.Version),
			e.StreamID.Bytes(),
			e.TotalPlainSize,
			e.EventName,
		)
	}
	return args
}
