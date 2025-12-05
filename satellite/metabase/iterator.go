// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"bytes"
	"context"
	"strings"

	"cloud.google.com/go/spanner"
	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/shared/tagsql"
)

// objectIterator enables iteration on objects in a bucket.
type objectsIterator struct {
	adapter Adapter

	projectID   uuid.UUID
	bucketName  BucketName
	pending     bool
	prefix      ObjectKey
	prefixLimit ObjectKey
	delimiter   ObjectKey
	batchSize   int
	recursive   bool

	includeCustomMetadata       bool
	includeSystemMetadata       bool
	includeETag                 bool
	includeETagOrCustomMetadata bool

	curIndex int
	curRows  tagsql.Rows
	cursor   ObjectsIteratorCursor // not relative to prefix

	// ignorePrefix represents the "current" folder that the iterator is in during non-recursive listing.
	// The objects with this prefix needs to be skipped.
	// It's relative to the global prefix.
	ignorePrefix ObjectKey
	doNextQuery  func(context.Context, *objectsIterator) (_ tagsql.Rows, err error)

	// failErr is set when either scan or next query fails during iteration.
	failErr error
}

// ObjectsIteratorCursor is the current location in an objects iterator.
type ObjectsIteratorCursor struct {
	Key       ObjectKey
	Version   Version
	StreamID  uuid.UUID
	Inclusive bool
}

func iterateAllVersionsWithStatusDescending(ctx context.Context, adapter Adapter, opts IterateObjectsWithStatus, fn func(context.Context, ObjectsIterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	prefixLimit, _ := SkipPrefix(opts.Prefix)

	if opts.Delimiter == "" {
		opts.Delimiter = Delimiter
	}

	cursor, ok := FirstIterateCursor(opts.Recursive, opts.Cursor, opts.Prefix, opts.Delimiter)
	if !ok {
		// the prefix and cursor combination does not match any objects
		return nil
	}

	it := &objectsIterator{
		adapter: adapter,

		projectID:   opts.ProjectID,
		bucketName:  opts.BucketName,
		pending:     opts.Pending,
		prefix:      opts.Prefix,
		prefixLimit: prefixLimit,
		delimiter:   opts.Delimiter,
		batchSize:   opts.BatchSize,
		recursive:   opts.Recursive,

		includeCustomMetadata:       opts.IncludeCustomMetadata,
		includeSystemMetadata:       opts.IncludeSystemMetadata,
		includeETag:                 opts.IncludeETag,
		includeETagOrCustomMetadata: opts.IncludeETagOrCustomMetadata,

		curIndex: 0,
		cursor:   cursor,

		doNextQuery: adapter.doNextQueryAllVersionsWithStatus,
	}

	// start from either the cursor or prefix, depending on which is larger
	if LessObjectKey(it.cursor.Key, opts.Prefix) {
		it.cursor.Key = opts.Prefix
		it.cursor.Version = MaxVersion
		it.cursor.Inclusive = true // TODO: we probably won't need this `Inclusive` handling, if we specify MaxVersion already
	}

	return iterate(ctx, it, fn)
}

func iterateAllVersionsWithStatusAscending(ctx context.Context, adapter Adapter, opts IterateObjectsWithStatus, fn func(context.Context, ObjectsIterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	prefixLimit, _ := SkipPrefix(opts.Prefix)

	if opts.Delimiter == "" {
		opts.Delimiter = Delimiter
	}

	cursor, ok := FirstIterateCursor(opts.Recursive, opts.Cursor, opts.Prefix, opts.Delimiter)
	if !ok {
		// the prefix and cursor combination does not match any objects
		return nil
	}

	it := &objectsIterator{
		adapter: adapter,

		projectID:   opts.ProjectID,
		bucketName:  opts.BucketName,
		pending:     opts.Pending,
		prefix:      opts.Prefix,
		prefixLimit: prefixLimit,
		delimiter:   opts.Delimiter,
		batchSize:   opts.BatchSize,
		recursive:   opts.Recursive,

		includeCustomMetadata:       opts.IncludeCustomMetadata,
		includeSystemMetadata:       opts.IncludeSystemMetadata,
		includeETag:                 opts.IncludeETag,
		includeETagOrCustomMetadata: opts.IncludeETagOrCustomMetadata,

		curIndex: 0,
		cursor:   cursor,

		doNextQuery: adapter.doNextQueryAllVersionsWithStatusAscending,
	}

	// start from either the cursor or prefix, depending on which is larger
	if LessObjectKey(it.cursor.Key, opts.Prefix) {
		it.cursor.Key = opts.Prefix
		it.cursor.Version = -1
		it.cursor.Inclusive = true
	}

	return iterate(ctx, it, fn)
}

func iteratePendingObjectsByKey(ctx context.Context, adapter Adapter, opts IteratePendingObjectsByKey, fn func(context.Context, ObjectsIterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	it := &objectsIterator{
		adapter: adapter,

		projectID:   opts.ProjectID,
		bucketName:  opts.BucketName,
		prefix:      "",
		prefixLimit: "",
		delimiter:   "",
		batchSize:   opts.BatchSize,
		recursive:   true,

		includeCustomMetadata:       true,
		includeSystemMetadata:       true,
		includeETag:                 true,
		includeETagOrCustomMetadata: false,

		pending: true,

		curIndex: 0,
		cursor: ObjectsIteratorCursor{
			Key:      opts.ObjectKey,
			Version:  MaxVersion, // TODO: this needs to come as an argument
			StreamID: opts.Cursor.StreamID,
		},
		doNextQuery: adapter.doNextQueryPendingObjectsByKey,
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
	if it.ignorePrefix != "" {
		for strings.HasPrefix(string(item.ObjectKey), string(it.ignorePrefix)) {
			if !it.next(ctx, item) {
				return false
			}
		}
		it.ignorePrefix = ""
	} else {
		ok := it.next(ctx, item)
		if !ok {
			return false
		}
	}

	// should this be treated as a prefix?
	p := strings.Index(string(item.ObjectKey), string(it.delimiter))
	if p >= 0 {
		prefix := item.ObjectKey[:p+len(it.delimiter)]
		it.ignorePrefix = prefix
		*item = ObjectEntry{
			IsPrefix:  true,
			ObjectKey: prefix,
			Status:    Prefix,
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
			p := strings.Index(string(afterPrefix), string(it.delimiter))
			if p >= 0 {
				skipPrefix, ok := SkipPrefix(afterPrefix[:p+len(it.delimiter)])
				if !ok {
					// there are no more objects with it.prefix and the "folder"
					// we currently are in, is the last one.
					return false
				}
				// otherwise query the first possible object after the prefix
				it.cursor.Key = it.prefix + skipPrefix
				it.cursor.StreamID = uuid.UUID{}
				it.cursor.Version = MaxVersion
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

func (p *PostgresAdapter) doNextQueryAllVersionsWithStatus(ctx context.Context, it *objectsIterator) (_ tagsql.Rows, err error) {
	defer mon.Task()(&ctx)(&err)

	cursorCompare := ">"
	if it.cursor.Inclusive {
		cursorCompare = ">="
	}

	statusFilter := `AND status <> ` + statusPending
	if it.pending {
		statusFilter = `AND status = ` + statusPending
	}

	args := []any{
		it.projectID, it.bucketName,
		it.cursor.Key, int(it.cursor.Version),
		it.batchSize,
	}

	var querySelectFields string
	var queryUpperBound string
	if it.prefix == "" {
		querySelectFields = querySelectorFields("object_key", it)
	} else {
		args = append(args, len(it.prefix)+1)
		querySelectFields = querySelectorFields("SUBSTRING(object_key FROM $6) AS object_key_suffix", it)

		if it.prefixLimit != "" {
			args = append(args, it.prefixLimit)
			queryUpperBound = "AND object_key < $7"
		}
	}

	return p.db.QueryContext(ctx, `
		SELECT
			`+querySelectFields+`
		FROM objects
		WHERE
			(project_id, bucket_name) = ($1, $2)
			AND (
				object_key > $3
				OR (object_key = $3 AND $4::INT8 `+cursorCompare+` version)
			)
			`+queryUpperBound+`
			`+statusFilter+`
			AND (expires_at IS NULL OR expires_at > now())
			ORDER BY project_id ASC, bucket_name ASC, object_key ASC, version DESC
		LIMIT $5
		`, args...,
	)
}

func (s *SpannerAdapter) doNextQueryAllVersionsWithStatus(ctx context.Context, it *objectsIterator) (_ tagsql.Rows, err error) {
	defer mon.Task()(&ctx)(&err)

	cursorCompare := ">"
	if it.cursor.Inclusive {
		cursorCompare = ">="
	}

	statusFilter := `AND status <> ` + statusPending
	if it.pending {
		statusFilter = `AND status = ` + statusPending
	}

	args := map[string]any{
		"project_id":     it.projectID,
		"bucket_name":    it.bucketName,
		"cursor_key":     it.cursor.Key,
		"cursor_version": it.cursor.Version,
		"batch_size":     int64(it.batchSize),
	}

	var querySelectFields string
	var queryUpperBound string
	if it.prefix == "" {
		querySelectFields = querySelectorFields("object_key", it)
	} else {
		args["from_substring"] = len(it.prefix) + 1
		querySelectFields = querySelectorFields("SUBSTR(object_key, @from_substring) AS object_key_suffix", it)

		if it.prefixLimit != "" {
			args["prefix_limit"] = it.prefixLimit
			queryUpperBound = `AND object_key < @prefix_limit`
		}
	}

	rowIterator := s.client.Single().QueryWithOptions(ctx, spanner.Statement{
		SQL: `
			SELECT
				` + querySelectFields + `
			FROM objects
			WHERE
				(project_id, bucket_name) = (@project_id, @bucket_name)
				AND (
					object_key > @cursor_key
					OR (object_key = @cursor_key AND @cursor_version ` + cursorCompare + ` version)
				)
				` + queryUpperBound + `
				` + statusFilter + `
				AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
			ORDER BY project_id ASC, bucket_name ASC, object_key ASC, version DESC
			LIMIT @batch_size
		`,
		Params: args,
	}, spanner.QueryOptions{RequestTag: "do-next-query-all-versions-with-status"})
	return newSpannerRows(rowIterator), nil
}

func (p *PostgresAdapter) doNextQueryAllVersionsWithStatusAscending(ctx context.Context, it *objectsIterator) (_ tagsql.Rows, err error) {
	defer mon.Task()(&ctx)(&err)

	cursorCompare := ">"
	if it.cursor.Inclusive {
		cursorCompare = ">="
	}

	statusFilter := `AND status <> ` + statusPending
	if it.pending {
		statusFilter = `AND status = ` + statusPending
	}

	args := []any{
		it.projectID, it.bucketName,
		it.cursor.Key, int(it.cursor.Version),
		it.batchSize,
	}

	var querySelectFields string
	var queryUpperBound string
	if it.prefix == "" {
		querySelectFields = querySelectorFields("object_key", it)
	} else {
		args = append(args, len(it.prefix)+1)
		querySelectFields = querySelectorFields("SUBSTRING(object_key FROM $6) AS object_key_suffix", it)
		if it.prefixLimit != "" {
			args = append(args, it.prefixLimit)
			queryUpperBound = "AND object_key < $7"
		}
	}

	return p.db.QueryContext(ctx, `
		SELECT
			`+querySelectFields+`
		FROM objects
		WHERE
			(project_id, bucket_name) = ($1, $2)
			AND (object_key, version) `+cursorCompare+` ($3, $4)
			`+queryUpperBound+`
			`+statusFilter+`
			AND (expires_at IS NULL OR expires_at > now())
			ORDER BY (project_id, bucket_name, object_key, version) ASC
		LIMIT $5
		`, args...,
	)
}

func (s *SpannerAdapter) doNextQueryAllVersionsWithStatusAscending(ctx context.Context, it *objectsIterator) (_ tagsql.Rows, err error) {
	defer mon.Task()(&ctx)(&err)

	cursorCompare := ">"
	if it.cursor.Inclusive {
		cursorCompare = ">="
	}

	statusFilter := `AND status <> ` + statusPending
	if it.pending {
		statusFilter = `AND status = ` + statusPending
	}

	args := map[string]any{
		"project_id":     it.projectID,
		"bucket_name":    it.bucketName,
		"cursor_key":     it.cursor.Key,
		"cursor_version": it.cursor.Version,
		"batch_size":     int64(it.batchSize),
	}

	var querySelectFields string
	var queryUpperBound string
	if it.prefix == "" {
		querySelectFields = querySelectorFields("object_key", it)
	} else {
		args["from_substring"] = len(it.prefix) + 1
		querySelectFields = querySelectorFields("SUBSTR(object_key, @from_substring) AS object_key_suffix", it)

		if it.prefixLimit != "" {
			args["prefix_limit"] = it.prefixLimit
			queryUpperBound = `AND object_key < @prefix_limit`
		}
	}

	rowIterator := s.client.Single().QueryWithOptions(ctx, spanner.Statement{
		SQL: `
			SELECT
				` + querySelectFields + `
			FROM objects
			WHERE
				(project_id, bucket_name) = (@project_id, @bucket_name)
				AND (
					(object_key > @cursor_key)
					OR (object_key = @cursor_key AND version ` + cursorCompare + ` @cursor_version)
				)
				` + queryUpperBound + `
				` + statusFilter + `
				AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
			ORDER BY project_id ASC, bucket_name ASC, object_key ASC, version ASC
			LIMIT @batch_size
		`,
		Params: args,
	}, spanner.QueryOptions{RequestTag: "do-next-query-all-versions-with-status-ascending"})
	return newSpannerRows(rowIterator), nil
}

func querySelectorFields(objectKeyColumn string, it *objectsIterator) string {
	querySelectFields := objectKeyColumn + `
		,stream_id
		,version
		,status
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

	if it.includeCustomMetadata || it.includeETag || it.includeETagOrCustomMetadata {
		querySelectFields += `
			,encrypted_metadata_nonce
			,encrypted_metadata_encrypted_key`
	}

	if it.includeCustomMetadata {
		querySelectFields += `
			,encrypted_metadata`
	}

	if it.includeETag {
		querySelectFields += `
			,encrypted_etag`
	}

	if it.includeETagOrCustomMetadata {
		querySelectFields += `
			, encrypted_etag IS NOT NULL AS is_encrypted_etag
			, COALESCE(encrypted_etag, encrypted_metadata) AS etag_or_metadata`
	}

	return querySelectFields
}

// nextBucket returns the lexicographically next bucket.
func nextBucket(b BucketName) BucketName {
	return b + "\x00"
}

// doNextQuery executes query to fetch the next batch returning the rows.
func (p *PostgresAdapter) doNextQueryPendingObjectsByKey(ctx context.Context, it *objectsIterator) (_ tagsql.Rows, err error) {
	defer mon.Task()(&ctx)(&err)

	return p.db.QueryContext(ctx, `
			SELECT
				object_key, stream_id, version, status, encryption,
				created_at, expires_at,
				segment_count,
				total_plain_size, total_encrypted_size, fixed_segment_size,
				encrypted_metadata_nonce, encrypted_metadata_encrypted_key, encrypted_metadata, encrypted_etag
			FROM objects
			WHERE
				(project_id, bucket_name, object_key) = ($1, $2, $3)
				AND stream_id > $4::BYTEA
				AND status = `+statusPending+`
			ORDER BY stream_id ASC
			LIMIT $5
			`, it.projectID, it.bucketName,
		it.cursor.Key,
		it.cursor.StreamID,
		it.batchSize,
	)
}

func (s *SpannerAdapter) doNextQueryPendingObjectsByKey(ctx context.Context, it *objectsIterator) (_ tagsql.Rows, err error) {
	defer mon.Task()(&ctx)(&err)

	rowIterator := s.client.Single().QueryWithOptions(ctx, spanner.Statement{
		SQL: `
			SELECT
				object_key, stream_id, version, status, encryption,
				created_at, expires_at,
				segment_count,
				total_plain_size, total_encrypted_size, fixed_segment_size,
				encrypted_metadata_nonce, encrypted_metadata_encrypted_key, encrypted_metadata, encrypted_etag
			FROM objects
			WHERE
				(project_id, bucket_name, object_key) = (@project_id, @bucket_name, @cursor_key)
				AND stream_id > @stream_id
				AND status = ` + statusPending + `
			ORDER BY stream_id ASC
			LIMIT @batch_size
		`,
		Params: map[string]any{
			"project_id":  it.projectID,
			"bucket_name": it.bucketName,
			"cursor_key":  it.cursor.Key,
			"stream_id":   it.cursor.StreamID,
			"batch_size":  int64(it.batchSize),
		},
	}, spanner.QueryOptions{RequestTag: "do-next-query-pending-objects-by-key"})
	return newSpannerRows(rowIterator), nil
}

// scanItem scans doNextQuery results into ObjectEntry.
func (it *objectsIterator) scanItem(item *ObjectEntry) (err error) {
	item.IsPrefix = false

	fields := []interface{}{
		&item.ObjectKey,
		&item.StreamID,
		&item.Version,
		&item.Status,
		&item.Encryption,
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

	if it.includeCustomMetadata || it.includeETag || it.includeETagOrCustomMetadata {
		fields = append(fields,
			&item.EncryptedMetadataNonce,
			&item.EncryptedMetadataEncryptedKey,
		)
	}

	if it.includeCustomMetadata {
		fields = append(fields,
			&item.EncryptedMetadata,
		)
	}

	if it.includeETag {
		fields = append(fields,
			&item.EncryptedETag,
		)
	}

	var isEncryptedETag bool
	var etagOrMetadata []byte

	if it.includeETagOrCustomMetadata {
		fields = append(fields,
			&isEncryptedETag,
			&etagOrMetadata,
		)
	}

	err = it.curRows.Scan(fields...)
	if err != nil {
		return err
	}

	if it.includeETagOrCustomMetadata {
		if isEncryptedETag {
			item.EncryptedETag = etagOrMetadata
			if !it.includeCustomMetadata {
				item.EncryptedMetadata = nil
			}
		} else {
			item.EncryptedMetadata = etagOrMetadata
			if !it.includeETag {
				item.EncryptedETag = nil
			}
		}
	}

	return nil
}

// LessObjectKey returns whether a < b.
func LessObjectKey(a, b ObjectKey) bool {
	return bytes.Compare([]byte(a), []byte(b)) < 0
}

// FirstIterateCursor adjust the cursor for a non-recursive iteration.
// The cursor is non-inclusive and we need to adjust to handle prefix as cursor properly.
// We return the next possible key from the prefix.
func FirstIterateCursor(recursive bool, cursor IterateCursor, prefix, delimiter ObjectKey) (_ ObjectsIteratorCursor, ok bool) {
	if recursive {
		return ObjectsIteratorCursor{
			Key:     cursor.Key,
			Version: cursor.Version,
		}, true
	}

	// when the cursor does not match the prefix, we'll return the original cursor.
	if !strings.HasPrefix(string(cursor.Key), string(prefix)) {
		return ObjectsIteratorCursor{
			Key:     cursor.Key,
			Version: cursor.Version,
		}, true
	}

	// handle case where:
	//   prefix: x/y/
	//   cursor: x/y/z/w
	// In this case, we want the skip prefix to be `x/y/z` + string('/' + 1).

	cursorWithoutPrefix := cursor.Key[len(prefix):]
	p := strings.Index(string(cursorWithoutPrefix), string(delimiter))
	if p < 0 {
		// The cursor is not a prefix, but instead a path inside the prefix,
		// so we can use it directly.
		return ObjectsIteratorCursor{
			Key:     cursor.Key,
			Version: cursor.Version,
		}, true
	}

	afterPrefix, ok := SkipPrefix(cursorWithoutPrefix[:p+len(delimiter)])
	if !ok {
		// the cursor is inside a final prefix, so there are no objects after this object,
		// let's just return the original cursor
		return ObjectsIteratorCursor{}, false
	}

	// return the next prefix given a scoped path
	return ObjectsIteratorCursor{
		Key:       cursor.Key[:len(prefix)] + afterPrefix,
		Version:   MaxVersion,
		Inclusive: true,
	}, true
}
