// Copyright (C) 2020 Storj Labs, Inc.
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

// objectIterator enables iteration on objects in a bucket.
type objectsIterator struct {
	db *DB

	projectID             uuid.UUID
	bucketName            []byte
	status                ObjectStatus
	prefix                ObjectKey
	prefixLimit           ObjectKey
	batchSize             int
	recursive             bool
	includeCustomMetadata bool
	includeSystemMetadata bool

	curIndex int
	curRows  tagsql.Rows
	cursor   iterateCursor // not relative to prefix

	skipPrefix  ObjectKey // relative to prefix
	doNextQuery func(context.Context, *objectsIterator) (_ tagsql.Rows, err error)

	// failErr is set when either scan or next query fails during iteration.
	failErr error
}

type iterateCursor struct {
	Key       ObjectKey
	Version   Version
	StreamID  uuid.UUID
	Inclusive bool
}

func iterateAllVersionsWithStatus(ctx context.Context, db *DB, opts IterateObjectsWithStatus, fn func(context.Context, ObjectsIterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	it := &objectsIterator{
		db: db,

		projectID:             opts.ProjectID,
		bucketName:            []byte(opts.BucketName),
		status:                opts.Status,
		prefix:                opts.Prefix,
		prefixLimit:           prefixLimit(opts.Prefix),
		batchSize:             opts.BatchSize,
		recursive:             opts.Recursive,
		includeCustomMetadata: opts.IncludeCustomMetadata,
		includeSystemMetadata: opts.IncludeSystemMetadata,

		curIndex: 0,
		cursor:   firstIterateCursor(opts.Recursive, opts.Cursor, opts.Prefix),

		doNextQuery: doNextQueryAllVersionsWithStatus,
	}

	// start from either the cursor or prefix, depending on which is larger
	if lessKey(it.cursor.Key, opts.Prefix) {
		it.cursor.Key = opts.Prefix
		it.cursor.Version = -1
		it.cursor.Inclusive = true
	}

	return iterate(ctx, it, fn)
}

func iteratePendingObjectsByKey(ctx context.Context, db *DB, opts IteratePendingObjectsByKey, fn func(context.Context, ObjectsIterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	cursor := opts.Cursor

	if cursor.StreamID.IsZero() {
		cursor.StreamID = uuid.UUID{}
	}

	it := &objectsIterator{
		db: db,

		projectID:             opts.ProjectID,
		bucketName:            []byte(opts.BucketName),
		prefix:                "",
		prefixLimit:           "",
		batchSize:             opts.BatchSize,
		recursive:             true,
		includeCustomMetadata: true,
		includeSystemMetadata: true,
		status:                Pending,

		curIndex: 0,
		cursor: iterateCursor{
			Key:      opts.ObjectKey,
			Version:  0,
			StreamID: opts.Cursor.StreamID,
		},
		doNextQuery: doNextQueryStreamsByKey,
	}

	return iterate(ctx, it, fn)

}

func iterate(ctx context.Context, it *objectsIterator, fn func(context.Context, ObjectsIterator) error) (err error) {
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
func (it *objectsIterator) Next(ctx context.Context, item *ObjectEntry) bool {
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
		*item = ObjectEntry{
			IsPrefix:  true,
			ObjectKey: item.ObjectKey[:p+1],
			Status:    it.status,
		}
	}

	return true
}

// next returns true if there was another item and copy it in item.
func (it *objectsIterator) next(ctx context.Context, item *ObjectEntry) bool {
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
				it.cursor.Version = 0
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
	it.cursor.Version = item.Version
	it.cursor.StreamID = item.StreamID

	return true
}

func doNextQueryAllVersionsWithStatus(ctx context.Context, it *objectsIterator) (_ tagsql.Rows, err error) {
	defer mon.Task()(&ctx)(&err)

	cursorCompare := ">"
	if it.cursor.Inclusive {
		cursorCompare = ">="
	}

	if it.prefixLimit == "" {
		querySelectFields := querySelectorFields("object_key", it)
		return it.db.db.QueryContext(ctx, `
			SELECT
				`+querySelectFields+`
			FROM objects
			WHERE
				(project_id, bucket_name, object_key, version) `+cursorCompare+` ($1, $2, $4, $5)
				AND (project_id, bucket_name) < ($1, $7)
				AND status = $3
				AND (expires_at IS NULL OR expires_at > now())
				ORDER BY (project_id, bucket_name, object_key, version) ASC
			LIMIT $6
			`, it.projectID, it.bucketName,
			it.status,
			[]byte(it.cursor.Key), int(it.cursor.Version),
			it.batchSize,
			nextBucket(it.bucketName),
		)
	}

	fromSubstring := 1
	if it.prefix != "" {
		fromSubstring = len(it.prefix) + 1
	}

	querySelectFields := querySelectorFields("SUBSTRING(object_key FROM $8)", it)
	return it.db.db.QueryContext(ctx, `
		SELECT
			`+querySelectFields+`
		FROM objects
		WHERE
			(project_id, bucket_name, object_key, version) `+cursorCompare+` ($1, $2, $4, $5)
			AND (project_id, bucket_name, object_key) < ($1, $2, $6)
			AND status = $3
			AND (expires_at IS NULL OR expires_at > now())
			ORDER BY (project_id, bucket_name, object_key, version) ASC
		LIMIT $7
		`, it.projectID, it.bucketName,
		it.status,
		[]byte(it.cursor.Key), int(it.cursor.Version),
		[]byte(it.prefixLimit),
		it.batchSize,
		fromSubstring,
	)
}

func querySelectorFields(objectKeyColumn string, it *objectsIterator) string {
	querySelectFields := objectKeyColumn + `
		,stream_id
		,version
		,encryption`

	if it.includeSystemMetadata {
		querySelectFields += `
			,created_at
			,expires_at
			,segment_count
			,total_plain_size
			,total_encrypted_size
			,fixed_segment_size`
	}

	if it.includeCustomMetadata {
		querySelectFields += `
			,encrypted_metadata_nonce
			,encrypted_metadata
			,encrypted_metadata_encrypted_key`
	}

	return querySelectFields
}

// nextBucket returns the lexicographically next bucket.
func nextBucket(b []byte) []byte {
	xs := make([]byte, len(b)+1)
	copy(xs, b)
	return xs
}

// doNextQuery executes query to fetch the next batch returning the rows.
func doNextQueryStreamsByKey(ctx context.Context, it *objectsIterator) (_ tagsql.Rows, err error) {
	defer mon.Task()(&ctx)(&err)

	return it.db.db.QueryContext(ctx, `
			SELECT
				object_key, stream_id, version, encryption,
				created_at, expires_at,
				segment_count,
				total_plain_size, total_encrypted_size, fixed_segment_size,
				encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key
			FROM objects
			WHERE
				project_id = $1 AND bucket_name = $2
				AND object_key = $3
				AND stream_id > $4::BYTEA
				AND status = `+statusPending+`
			ORDER BY stream_id ASC
			LIMIT $5
			`, it.projectID, it.bucketName,
		[]byte(it.cursor.Key),
		it.cursor.StreamID,
		it.batchSize,
	)
}

// scanItem scans doNextQuery results into ObjectEntry.
func (it *objectsIterator) scanItem(item *ObjectEntry) (err error) {
	item.IsPrefix = false
	item.Status = it.status

	fields := []interface{}{
		&item.ObjectKey,
		&item.StreamID,
		&item.Version,
		encryptionParameters{&item.Encryption},
	}

	if it.includeSystemMetadata {
		fields = append(fields,
			&item.CreatedAt,
			&item.ExpiresAt,
			&item.SegmentCount,
			&item.TotalPlainSize,
			&item.TotalEncryptedSize,
			&item.FixedSegmentSize,
		)
	}

	if it.includeCustomMetadata {
		fields = append(fields,
			&item.EncryptedMetadataNonce,
			&item.EncryptedMetadata,
			&item.EncryptedMetadataEncryptedKey,
		)
	}

	err = it.curRows.Scan(fields...)

	if err != nil {
		return err
	}
	return nil
}

func prefixLimit(a ObjectKey) ObjectKey {
	if a == "" {
		return ""
	}
	if a[len(a)-1] == 0xFF {
		return a + "\x00"
	}

	key := []byte(a)
	key[len(key)-1]++
	return ObjectKey(key)
}

// lessKey returns whether a < b.
func lessKey(a, b ObjectKey) bool {
	return bytes.Compare([]byte(a), []byte(b)) < 0
}

// firstIterateCursor adjust the cursor for a non-recursive iteration.
// The cursor is non-inclusive and we need to adjust to handle prefix as cursor properly.
// We return the next possible key from the prefix.
func firstIterateCursor(recursive bool, cursor IterateCursor, prefix ObjectKey) iterateCursor {
	if recursive {
		return iterateCursor{
			Key:     cursor.Key,
			Version: cursor.Version,
		}
	}

	// when the cursor does not match the prefix, we'll return the original cursor.
	if !strings.HasPrefix(string(cursor.Key), string(prefix)) {
		return iterateCursor{
			Key:     cursor.Key,
			Version: cursor.Version,
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
		return iterateCursor{
			Key:     cursor.Key,
			Version: cursor.Version,
		}
	}

	// return the next prefix given a scoped path
	return iterateCursor{
		Key:       cursor.Key[:len(prefix)+p] + ObjectKey(Delimiter+1),
		Version:   -1,
		Inclusive: true,
	}
}
