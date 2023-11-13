// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"bytes"
	"context"
	"strings"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/private/tagsql"
)

// pendingObjectsIterator enables iteration on pending objects in a bucket.
type pendingObjectsIterator struct {
	db *DB

	projectID             uuid.UUID
	bucketName            []byte
	prefix                ObjectKey
	prefixLimit           ObjectKey
	batchSize             int
	recursive             bool
	includeCustomMetadata bool
	includeSystemMetadata bool

	curIndex int
	curRows  tagsql.Rows
	cursor   pendingObjectIterateCursor // not relative to prefix

	skipPrefix  ObjectKey // relative to prefix
	doNextQuery func(context.Context, *pendingObjectsIterator) (_ tagsql.Rows, err error)

	// failErr is set when either scan or next query fails during iteration.
	failErr error
}

type pendingObjectIterateCursor struct {
	Key       ObjectKey
	StreamID  uuid.UUID
	Inclusive bool
}

func iterateAllPendingObjects(ctx context.Context, db *DB, opts IteratePendingObjects, fn func(context.Context, PendingObjectsIterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	it := &pendingObjectsIterator{
		db: db,

		projectID:             opts.ProjectID,
		bucketName:            []byte(opts.BucketName),
		prefix:                opts.Prefix,
		prefixLimit:           prefixLimit(opts.Prefix),
		batchSize:             opts.BatchSize,
		recursive:             opts.Recursive,
		includeCustomMetadata: opts.IncludeCustomMetadata,
		includeSystemMetadata: opts.IncludeSystemMetadata,

		curIndex: 0,
		cursor:   firstPendingObjectIterateCursor(opts.Recursive, opts.Cursor, opts.Prefix),

		doNextQuery: doNextQueryAllPendingObjects,
	}

	// start from either the cursor or prefix, depending on which is larger
	if lessKey(it.cursor.Key, opts.Prefix) {
		it.cursor.Key = opts.Prefix
		it.cursor.Inclusive = true
	}

	return iteratePendingObjects(ctx, it, fn)
}

func iteratePendingObjects(ctx context.Context, it *pendingObjectsIterator, fn func(context.Context, PendingObjectsIterator) error) (err error) {
	batchsizeLimit.Ensure(&it.batchSize)

	it.curRows, err = it.doNextQuery(ctx, it)
	if err != nil {
		return err
	}
	it.cursor.Inclusive = false

	defer func() {
		if rowsErr := it.curRows.Err(); rowsErr != nil {
			err = errs.Combine(err, rowsErr)
		}
		err = errs.Combine(err, it.failErr, it.curRows.Close())
	}()

	return fn(ctx, it)
}

// Next returns true if there was another item and copy it in item.
func (it *pendingObjectsIterator) Next(ctx context.Context, item *PendingObjectEntry) bool {
	if it.recursive {
		return it.next(ctx, item)
	}

	// TODO: implement this on the database side

	// skip until we are past the prefix we returned before.
	if it.skipPrefix != "" {
		for strings.HasPrefix(string(item.ObjectKey), string(it.skipPrefix)) {
			if !it.next(ctx, item) {
				return false
			}
		}
		it.skipPrefix = ""
	} else {
		ok := it.next(ctx, item)
		if !ok {
			return false
		}
	}

	// should this be treated as a prefix?
	p := strings.IndexByte(string(item.ObjectKey), Delimiter)
	if p >= 0 {
		it.skipPrefix = item.ObjectKey[:p+1]
		*item = PendingObjectEntry{
			IsPrefix:  true,
			ObjectKey: item.ObjectKey[:p+1],
		}
	}

	return true
}

// next returns true if there was another item and copy it in item.
func (it *pendingObjectsIterator) next(ctx context.Context, item *PendingObjectEntry) bool {
	next := it.curRows.Next()
	if !next {
		if it.curIndex < it.batchSize {
			return false
		}

		if it.curRows.Err() != nil {
			return false
		}

		if !it.recursive {
			afterPrefix := it.cursor.Key[len(it.prefix):]
			p := bytes.IndexByte([]byte(afterPrefix), Delimiter)
			if p >= 0 {
				it.cursor.Key = it.prefix + prefixLimit(afterPrefix[:p+1])
				it.cursor.StreamID = uuid.UUID{}
			}
		}

		rows, err := it.doNextQuery(ctx, it)
		if err != nil {
			it.failErr = errs.Combine(it.failErr, err)
			return false
		}

		if closeErr := it.curRows.Close(); closeErr != nil {
			it.failErr = errs.Combine(it.failErr, closeErr, rows.Close())
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
		it.failErr = errs.Combine(it.failErr, err)
		return false
	}

	it.curIndex++
	it.cursor.Key = it.prefix + item.ObjectKey
	it.cursor.StreamID = item.StreamID

	return true
}

func doNextQueryAllPendingObjects(ctx context.Context, it *pendingObjectsIterator) (_ tagsql.Rows, err error) {
	defer mon.Task()(&ctx)(&err)

	cursorCompare := ">"
	if it.cursor.Inclusive {
		cursorCompare = ">="
	}

	if it.prefixLimit == "" {
		querySelectFields := pendingObjectsQuerySelectorFields("object_key", it)
		return it.db.db.QueryContext(ctx, `
			SELECT
				`+querySelectFields+`
			FROM pending_objects
			WHERE
				(project_id, bucket_name, object_key, stream_id) `+cursorCompare+` ($1, $2, $3, $4)
				AND (project_id, bucket_name) < ($1, $6)
				AND (expires_at IS NULL OR expires_at > now())
				ORDER BY (project_id, bucket_name, object_key, stream_id) ASC
			LIMIT $5
			`, it.projectID, it.bucketName,
			[]byte(it.cursor.Key), it.cursor.StreamID,
			it.batchSize,
			nextBucket(it.bucketName),
		)
	}

	fromSubstring := 1
	if it.prefix != "" {
		fromSubstring = len(it.prefix) + 1
	}

	querySelectFields := pendingObjectsQuerySelectorFields("SUBSTRING(object_key FROM $7)", it)
	return it.db.db.QueryContext(ctx, `
		SELECT
			`+querySelectFields+`
		FROM pending_objects
		WHERE
			(project_id, bucket_name, object_key, stream_id) `+cursorCompare+` ($1, $2, $3, $4)
			AND (project_id, bucket_name, object_key) < ($1, $2, $5)
			AND (expires_at IS NULL OR expires_at > now())
			ORDER BY (project_id, bucket_name, object_key, stream_id) ASC
		LIMIT $6
		`, it.projectID, it.bucketName,
		[]byte(it.cursor.Key), it.cursor.StreamID,
		[]byte(it.prefixLimit),
		it.batchSize,
		fromSubstring,
	)
}

func pendingObjectsQuerySelectorFields(objectKeyColumn string, it *pendingObjectsIterator) string {
	querySelectFields := objectKeyColumn + `
		,stream_id
		,encryption`

	if it.includeSystemMetadata {
		querySelectFields += `
			,created_at
			,expires_at`
	}

	if it.includeCustomMetadata {
		querySelectFields += `
			,encrypted_metadata_nonce
			,encrypted_metadata
			,encrypted_metadata_encrypted_key`
	}

	return querySelectFields
}

// scanItem scans doNextQuery results into PendingObjectEntry.
func (it *pendingObjectsIterator) scanItem(item *PendingObjectEntry) (err error) {
	item.IsPrefix = false

	fields := []interface{}{
		&item.ObjectKey,
		&item.StreamID,
		encryptionParameters{&item.Encryption},
	}

	if it.includeSystemMetadata {
		fields = append(fields,
			&item.CreatedAt,
			&item.ExpiresAt,
		)
	}

	if it.includeCustomMetadata {
		fields = append(fields,
			&item.EncryptedMetadataNonce,
			&item.EncryptedMetadata,
			&item.EncryptedMetadataEncryptedKey,
		)
	}

	return it.curRows.Scan(fields...)
}

// firstPendingObjectIterateCursor adjust the cursor for a non-recursive iteration.
// The cursor is non-inclusive and we need to adjust to handle prefix as cursor properly.
// We return the next possible key from the prefix.
func firstPendingObjectIterateCursor(recursive bool, cursor PendingObjectsCursor, prefix ObjectKey) pendingObjectIterateCursor {
	if recursive {
		return pendingObjectIterateCursor{
			Key:      cursor.Key,
			StreamID: cursor.StreamID,
		}
	}

	// when the cursor does not match the prefix, we'll return the original cursor.
	if !strings.HasPrefix(string(cursor.Key), string(prefix)) {
		return pendingObjectIterateCursor{
			Key:      cursor.Key,
			StreamID: cursor.StreamID,
		}
	}

	// handle case where:
	//   prefix: x/y/
	//   cursor: x/y/z/w
	// In this case, we want the skip prefix to be `x/y/z` + string('/' + 1).

	cursorWithoutPrefix := cursor.Key[len(prefix):]
	p := strings.IndexByte(string(cursorWithoutPrefix), Delimiter)
	if p < 0 {
		// The cursor is not a prefix, but instead a path inside the prefix,
		// so we can use it directly.
		return pendingObjectIterateCursor{
			Key:      cursor.Key,
			StreamID: cursor.StreamID,
		}
	}

	// return the next prefix given a scoped path
	return pendingObjectIterateCursor{
		Key:       cursor.Key[:len(prefix)+p] + ObjectKey(Delimiter+1),
		StreamID:  cursor.StreamID,
		Inclusive: true,
	}
}

func iteratePendingObjectsByKeyNew(ctx context.Context, db *DB, opts IteratePendingObjectsByKey, fn func(context.Context, PendingObjectsIterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	cursor := opts.Cursor

	if cursor.StreamID.IsZero() {
		cursor.StreamID = uuid.UUID{}
	}

	it := &pendingObjectsIterator{
		db: db,

		projectID:             opts.ProjectID,
		bucketName:            []byte(opts.BucketName),
		prefix:                "",
		prefixLimit:           "",
		batchSize:             opts.BatchSize,
		recursive:             true,
		includeCustomMetadata: true,
		includeSystemMetadata: true,

		curIndex: 0,
		cursor: pendingObjectIterateCursor{
			Key:      opts.ObjectKey,
			StreamID: opts.Cursor.StreamID,
		},
		doNextQuery: doNextQueryPendingStreamsByKey,
	}

	return iteratePendingObjects(ctx, it, fn)

}

func doNextQueryPendingStreamsByKey(ctx context.Context, it *pendingObjectsIterator) (_ tagsql.Rows, err error) {
	defer mon.Task()(&ctx)(&err)

	return it.db.db.QueryContext(ctx, `
			SELECT
				object_key, stream_id, encryption,
				created_at, expires_at,
				encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key
			FROM pending_objects
			WHERE
				(project_id, bucket_name, object_key) = ($1, $2, $3) AND
				stream_id > $4::BYTEA
			ORDER BY stream_id ASC
			LIMIT $5
			`, it.projectID, it.bucketName,
		[]byte(it.cursor.Key),
		it.cursor.StreamID,
		it.batchSize,
	)
}
