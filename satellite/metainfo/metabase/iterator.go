// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/storj/private/tagsql"
)

// objectIterator enables iteration on objects in a bucket.
type objectsIterator struct {
	opts      *IterateObjects
	db        *DB
	batchSize int
	curIndex  int
	curRows   tagsql.Rows
	status    ObjectStatus
	cursor    IterateCursor
}

func iterateAllVersions(ctx context.Context, db *DB, opts IterateObjects, fn func(context.Context, ObjectsIterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	it := &objectsIterator{
		db:        db,
		opts:      &opts,
		batchSize: opts.BatchSize,
		curIndex:  0,
		status:    Committed,
		cursor:    opts.Cursor,
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

// Next returns true if there was another item and copy it in item.
func (it *objectsIterator) Next(ctx context.Context, item *ObjectEntry) bool {
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

	item.ProjectID = it.opts.ProjectID
	item.BucketName = it.opts.BucketName

	it.curIndex++
	it.cursor.Key = item.ObjectKey
	it.cursor.Version = item.Version

	return true
}

func (it *objectsIterator) doNextQuery(ctx context.Context) (_ tagsql.Rows, err error) {
	defer mon.Task()(&ctx)(&err)

	return it.db.db.Query(ctx, `
		SELECT
			object_key, stream_id, version, status,
			created_at, expires_at,
			segment_count,
			encrypted_metadata_nonce, encrypted_metadata,
			total_encrypted_size, fixed_segment_size,
			encryption
		FROM objects
		WHERE
			project_id = $1 AND bucket_name = $2
			AND status = $3
			AND (object_key, version) > ($4, $5)
			ORDER BY object_key ASC, version ASC
		 LIMIT $6
		`, it.opts.ProjectID, it.opts.BucketName, it.status, []byte(it.cursor.Key), int(it.cursor.Version), it.opts.BatchSize)
}

func (it *objectsIterator) scanItem(item *ObjectEntry) error {
	return it.curRows.Scan(
		&item.ObjectKey, &item.StreamID, &item.Version, &item.Status,
		&item.CreatedAt, &item.ExpiresAt,
		&item.SegmentCount,
		&item.EncryptedMetadataNonce, &item.EncryptedMetadata,
		&item.TotalEncryptedSize, &item.FixedSegmentSize,
		encryptionParameters{&item.Encryption},
	)
}
