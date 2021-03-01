// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/private/tagsql"
)

const loopIteratorBatchSizeLimit = 2500

// LoopObjectEntry contains information about object needed by metainfo loop.
type LoopObjectEntry struct {
	ObjectStream                     // metrics, repair, tally
	ExpiresAt             *time.Time // tally
	SegmentCount          int32      // metrics
	EncryptedMetadataSize int        // tally
}

// LoopObjectsIterator iterates over a sequence of LoopObjectEntry items.
type LoopObjectsIterator interface {
	Next(ctx context.Context, item *LoopObjectEntry) bool
}

// IterateLoopObjects contains arguments necessary for listing objects in metabase.
type IterateLoopObjects struct {
	BatchSize int
}

// Verify verifies get object request fields.
func (opts *IterateLoopObjects) Verify() error {
	if opts.BatchSize < 0 {
		return ErrInvalidRequest.New("BatchSize is negative")
	}
	return nil
}

// IterateLoopObjects iterates through all objects in metabase.
func (db *DB) IterateLoopObjects(ctx context.Context, opts IterateLoopObjects, fn func(context.Context, LoopObjectsIterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return err
	}

	it := &loopIterator{
		db: db,

		batchSize: opts.BatchSize,

		curIndex: 0,
		cursor:   loopIterateCursor{},
	}

	// ensure batch size is reasonable
	if it.batchSize <= 0 || it.batchSize > loopIteratorBatchSizeLimit {
		it.batchSize = loopIteratorBatchSizeLimit
	}

	it.curRows, err = it.doNextQuery(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if rowsErr := it.curRows.Err(); rowsErr != nil {
			err = errs.Combine(err, rowsErr)
		}
		err = errs.Combine(err, it.curRows.Close())
	}()

	return fn(ctx, it)
}

// loopIterator enables iteration of all objects in metabase.
type loopIterator struct {
	db *DB

	batchSize int

	curIndex int
	curRows  tagsql.Rows
	cursor   loopIterateCursor
}

type loopIterateCursor struct {
	ProjectID  uuid.UUID
	BucketName string
	ObjectKey  ObjectKey
	Version    Version
}

// Next returns true if there was another item and copy it in item.
func (it *loopIterator) Next(ctx context.Context, item *LoopObjectEntry) bool {
	next := it.curRows.Next()
	if !next {
		if it.curIndex < it.batchSize {
			return false
		}

		if it.curRows.Err() != nil {
			return false
		}

		rows, err := it.doNextQuery(ctx)
		if err != nil {
			return false
		}

		if it.curRows.Close() != nil {
			_ = rows.Close()
			return false
		}

		it.curRows = rows
		it.curIndex = 0
		if !it.curRows.Next() {
			return false
		}
	}

	err := it.scanItem(item)
	if err != nil {
		return false
	}

	it.curIndex++
	it.cursor.ProjectID = item.ProjectID
	it.cursor.BucketName = item.BucketName
	it.cursor.ObjectKey = item.ObjectKey
	it.cursor.Version = item.Version

	return true
}

func (it *loopIterator) doNextQuery(ctx context.Context) (_ tagsql.Rows, err error) {
	defer mon.Task()(&ctx)(&err)

	return it.db.db.Query(ctx, `
		SELECT
			project_id, bucket_name,
			object_key, stream_id, version,
			expires_at,
			segment_count,
			LENGTH(COALESCE(encrypted_metadata,''))
		FROM objects
		WHERE (project_id, bucket_name, object_key, version) > ($1, $2, $3, $4)
		ORDER BY project_id ASC, bucket_name ASC, object_key ASC, version ASC
		LIMIT $5
		`, it.cursor.ProjectID, []byte(it.cursor.BucketName),
		[]byte(it.cursor.ObjectKey), int(it.cursor.Version),
		it.batchSize,
	)
}

// scanItem scans doNextQuery results into LoopObjectEntry.
func (it *loopIterator) scanItem(item *LoopObjectEntry) error {
	return it.curRows.Scan(
		&item.ProjectID, &item.BucketName,
		&item.ObjectKey, &item.StreamID, &item.Version,
		&item.ExpiresAt,
		&item.SegmentCount,
		&item.EncryptedMetadataSize,
	)
}
