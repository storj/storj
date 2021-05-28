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

	projectID       uuid.UUID
	bucketName      []byte
	status          ObjectStatus
	prefix          ObjectKey
	prefixLimit     ObjectKey
	batchSize       int
	recursive       bool
	includePrefixes bool

	curIndex        int
	curRows         tagsql.Rows
	cursor          iterateCursor
	inclusiveCursor bool

	skipPrefix  ObjectKey
	doNextQuery func(context.Context, *objectsIterator) (_ tagsql.Rows, err error)
}

type iterateCursor struct {
	Key      ObjectKey
	Version  Version
	StreamID uuid.UUID
}

func iterateAllVersions(ctx context.Context, db *DB, opts IterateObjects, fn func(context.Context, ObjectsIterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	it := &objectsIterator{
		db: db,

		projectID:       opts.ProjectID,
		bucketName:      []byte(opts.BucketName),
		prefix:          opts.Prefix,
		prefixLimit:     prefixLimit(opts.Prefix),
		batchSize:       opts.BatchSize,
		recursive:       true,
		includePrefixes: true,
		inclusiveCursor: false,

		curIndex: 0,
		cursor: iterateCursor{
			Key:     opts.Cursor.Key,
			Version: opts.Cursor.Version,
		},
		doNextQuery: doNextQueryAllVersionsWithoutStatus,
	}

	// start from either the cursor or prefix, depending on which is larger
	if lessKey(it.cursor.Key, opts.Prefix) {
		it.cursor.Key = opts.Prefix
		it.cursor.Version = -1
		it.inclusiveCursor = true
	}

	return iterate(ctx, it, fn)
}

func iterateAllVersionsWithStatus(ctx context.Context, db *DB, opts IterateObjectsWithStatus, fn func(context.Context, ObjectsIterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	it := &objectsIterator{
		db: db,

		projectID:       opts.ProjectID,
		bucketName:      []byte(opts.BucketName),
		status:          opts.Status,
		prefix:          opts.Prefix,
		prefixLimit:     prefixLimit(opts.Prefix),
		batchSize:       opts.BatchSize,
		recursive:       opts.Recursive,
		includePrefixes: true,

		curIndex: 0,
		cursor: iterateCursor{
			Key:     opts.Cursor.Key,
			Version: opts.Cursor.Version,
		},
		doNextQuery: doNextQueryAllVersionsWithStatus,
	}

	// start from either the cursor or prefix, depending on which is larger
	if lessKey(it.cursor.Key, opts.Prefix) {
		it.cursor.Key = opts.Prefix
		it.cursor.Version = -1
		it.inclusiveCursor = true
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

		projectID:       opts.ProjectID,
		bucketName:      []byte(opts.BucketName),
		prefix:          "",
		prefixLimit:     "",
		batchSize:       opts.BatchSize,
		recursive:       false,
		includePrefixes: false,

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
	// ensure batch size is reasonable
	if it.batchSize <= 0 || it.batchSize > batchsizeLimit {
		it.batchSize = batchsizeLimit
	}

	it.curRows, err = it.doNextQuery(ctx, it)
	if err != nil {
		return err
	}
	it.inclusiveCursor = false

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
	if it.recursive {
		return it.next(ctx, item)
	}

	// TODO: implement this on the database side

	ok := it.next(ctx, item)
	if !ok {
		return false
	}

	// skip until we are past the prefix we returned before.
	if it.skipPrefix != "" {
		for strings.HasPrefix(string(item.ObjectKey), string(it.skipPrefix)) {
			if !it.next(ctx, item) {
				return false
			}
		}
		it.skipPrefix = ""
	}

	if it.includePrefixes {
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

		rows, err := it.doNextQuery(ctx, it)
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
	it.cursor.Key = item.ObjectKey
	it.cursor.Version = item.Version
	it.cursor.StreamID = item.StreamID

	if it.prefix != "" {
		if !strings.HasPrefix(string(item.ObjectKey), string(it.prefix)) {
			return false
		}
	}

	// TODO this should be done with SQL query
	item.ObjectKey = item.ObjectKey[len(it.prefix):]

	return true
}

func doNextQueryAllVersionsWithoutStatus(ctx context.Context, it *objectsIterator) (_ tagsql.Rows, err error) {
	defer mon.Task()(&ctx)(&err)

	cursorCompare := ">"
	if it.inclusiveCursor {
		cursorCompare = ">="
	}

	if it.prefixLimit == "" {
		return it.db.db.Query(ctx, `
			SELECT
				object_key, stream_id, version, status,
				created_at, expires_at,
				segment_count,
				encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
				total_plain_size, total_encrypted_size, fixed_segment_size,
				encryption
			FROM objects
			WHERE
				project_id = $1 AND bucket_name = $2
				AND (object_key, version) `+cursorCompare+` ($3, $4)
			ORDER BY object_key ASC, version ASC
			LIMIT $5
			`, it.projectID, it.bucketName,
			[]byte(it.cursor.Key), int(it.cursor.Version),
			it.batchSize,
		)
	}

	// TODO this query should use SUBSTRING(object_key from $8) but there is a problem how it
	// works with CRDB.
	return it.db.db.Query(ctx, `
		SELECT
			object_key, stream_id, version, status,
			created_at, expires_at,
			segment_count,
			encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
			total_plain_size, total_encrypted_size, fixed_segment_size,
			encryption
		FROM objects
		WHERE
			project_id = $1 AND bucket_name = $2
			AND (object_key, version) `+cursorCompare+` ($3, $4)
			AND object_key < $5
		ORDER BY object_key ASC, version ASC
		LIMIT $6
	`, it.projectID, it.bucketName,
		[]byte(it.cursor.Key), int(it.cursor.Version),
		[]byte(it.prefixLimit),
		it.batchSize,

		// len(it.prefix)+1, // TODO uncomment when CRDB issue will be fixed
	)
}

func doNextQueryAllVersionsWithStatus(ctx context.Context, it *objectsIterator) (_ tagsql.Rows, err error) {
	defer mon.Task()(&ctx)(&err)

	cursorCompare := ">"
	if it.inclusiveCursor {
		cursorCompare = ">="
	}

	if it.prefixLimit == "" {
		return it.db.db.Query(ctx, `
			SELECT
				object_key, stream_id, version, status,
				created_at, expires_at,
				segment_count,
				encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
				total_plain_size, total_encrypted_size, fixed_segment_size,
				encryption
			FROM objects
			WHERE
				(project_id, bucket_name, object_key, version) `+cursorCompare+` ($1, $2, $4, $5)
				AND (project_id, bucket_name) < ($1, $7)
				AND status = $3
				ORDER BY (project_id, bucket_name, object_key, version) ASC
			LIMIT $6
			`, it.projectID, it.bucketName,
			it.status,
			[]byte(it.cursor.Key), int(it.cursor.Version),
			it.batchSize,
			nextBucket(it.bucketName),
		)
	}

	// TODO this query should use SUBSTRING(object_key from $8) but there is a problem how it
	// works with CRDB.
	return it.db.db.Query(ctx, `
		SELECT
			object_key, stream_id, version, status,
			created_at, expires_at,
			segment_count,
			encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
			total_plain_size, total_encrypted_size, fixed_segment_size,
			encryption
		FROM objects
		WHERE
			(project_id, bucket_name, object_key, version) `+cursorCompare+` ($1, $2, $4, $5)
			AND (project_id, bucket_name, object_key) < ($1, $2, $6)
			AND status = $3
			ORDER BY (project_id, bucket_name, object_key, version) ASC
		LIMIT $7
	`, it.projectID, it.bucketName,
		it.status,
		[]byte(it.cursor.Key), int(it.cursor.Version),
		[]byte(it.prefixLimit),
		it.batchSize,
		// len(it.prefix)+1, // TODO uncomment when CRDB issue will be fixed
	)
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

	return it.db.db.Query(ctx, `
			SELECT
				object_key, stream_id, version, status,
				created_at, expires_at,
				segment_count,
				encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
				total_plain_size, total_encrypted_size, fixed_segment_size,
				encryption
			FROM objects
			WHERE
				project_id = $1 AND bucket_name = $2
				AND object_key = $3
				AND stream_id > $4::BYTEA
				AND status = `+pendingStatus+`
			ORDER BY stream_id ASC
			LIMIT $5
			`, it.projectID, it.bucketName,
		[]byte(it.cursor.Key),
		it.cursor.StreamID,
		it.batchSize,
	)
}

// scanItem scans doNextQuery results into ObjectEntry.
func (it *objectsIterator) scanItem(item *ObjectEntry) error {
	item.IsPrefix = false
	err := it.curRows.Scan(
		&item.ObjectKey, &item.StreamID, &item.Version, &item.Status,
		&item.CreatedAt, &item.ExpiresAt,
		&item.SegmentCount,
		&item.EncryptedMetadataNonce, &item.EncryptedMetadata, &item.EncryptedMetadataEncryptedKey,
		&item.TotalPlainSize, &item.TotalEncryptedSize, &item.FixedSegmentSize,
		encryptionParameters{&item.Encryption},
	)
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
