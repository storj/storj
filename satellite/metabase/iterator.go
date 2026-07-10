// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/tagsql"
)

// iterShape captures the SQL shape of a sqlObjectIterator query so the
// assembled (and optionally rebound) SQL can be cached process-wide. Two
// iterators with the same shape produce byte-identical SQL; only their args
// vary.
type iterShape struct {
	inclusive      bool
	pending        bool
	hasPrefix      bool
	hasPrefixLimit bool
	keyProbe       bool
	sysMeta        bool
	customMeta     bool
	etag           bool
	etagOrCustom   bool
	checksum       bool
}

func shapeOf(it *sqlObjectIterator) iterShape {
	return iterShape{
		inclusive:      it.cursor.Inclusive,
		pending:        it.pending,
		hasPrefix:      it.prefix != "",
		hasPrefixLimit: it.prefix != "" && it.prefixLimit != "",
		keyProbe:       it.keyProbe,
		sysMeta:        it.includeSystemMetadata,
		customMeta:     it.includeCustomMetadata,
		etag:           it.includeETag,
		etagOrCustom:   it.includeETagOrCustomMetadata,
		checksum:       it.includeChecksum,
	}
}

// iterSQLCache memoizes assembled SQL per shape, with separate maps for the
// bare and rebound variants so a single cache serves backends with and
// without postgresRebind.
type iterSQLCache struct {
	bare    sync.Map // iterShape -> string
	rebound sync.Map // iterShape -> string
}

// get returns the cached SQL for shape, building (and optionally rebinding)
// on miss. Once built, neither the SQL nor its rebind state changes for the
// process lifetime — postgresRebind.Rebind is a pure function of its input.
// Concurrent misses for the same shape may redundantly build and store the
// same string; the build is pure so the redundancy is harmless and bounded.
func (c *iterSQLCache) get(shape iterShape, rebinder sqlRebinder, build func() string) string {
	cache := &c.bare
	if rebinder != nil {
		cache = &c.rebound
	}
	if v, ok := cache.Load(shape); ok {
		return v.(string)
	}
	sql := build()
	if rebinder != nil {
		sql = rebinder.Rebind(sql)
	}
	cache.Store(shape, sql)
	return sql
}

// sqlObjectIterator is a SQL-backed ObjectIterator implementation. It
// owns the batch-refill loop that previously lived on objectsIterator.
type sqlObjectIterator struct {
	adapter Adapter

	projectID   uuid.UUID
	bucketName  BucketName
	pending     bool
	prefix      ObjectKey
	prefixLimit ObjectKey
	delimiter   ObjectKey
	batchSize   int
	recursive   bool

	// keyProbe bounds each descending batch scan with a key-probe subquery;
	// set only on TiDB in the key-probe list mode.
	keyProbe bool

	includeCustomMetadata       bool
	includeSystemMetadata       bool
	includeETag                 bool
	includeETagOrCustomMetadata bool
	includeChecksum             bool

	curIndex int
	curRows  tagsql.Rows
	cursor   ObjectsIteratorCursor // not relative to prefix

	doNextQuery func(context.Context, *sqlObjectIterator) (_ tagsql.Rows, err error)

	closed bool
}

// ObjectsIteratorCursor is the current location in an objects iterator.
type ObjectsIteratorCursor struct {
	Key       ObjectKey
	Version   Version
	StreamID  uuid.UUID
	Inclusive bool
}

// objectsIterator wraps a backend ObjectIterator and adds non-recursive
// prefix collapsing on top. All SQL/batch-refill logic lives in
// sqlObjectIterator.
type objectsIterator struct {
	raw       ObjectIterator
	prefix    ObjectKey
	delimiter ObjectKey
	recursive bool

	// ignorePrefix represents the "current" folder that the iterator is in
	// during non-recursive listing. The objects with this prefix need to
	// be skipped. It's relative to the global prefix.
	ignorePrefix ObjectKey

	// err captures errors returned by the underlying ObjectIterator's Next.
	err error
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

	// start from either the cursor or prefix, depending on which is larger
	if LessObjectKey(cursor.Key, opts.Prefix) {
		cursor.Key = opts.Prefix
		cursor.Version = MaxVersion
		cursor.Inclusive = true // TODO: we probably won't need this `Inclusive` handling, if we specify MaxVersion already
	}

	batchSize := opts.BatchSize
	batchsizeLimit.Ensure(&batchSize)

	raw, err := adapter.ObjectIterator(ctx, ObjectIteratorOptions{
		ProjectID:   opts.ProjectID,
		BucketName:  opts.BucketName,
		Prefix:      opts.Prefix,
		PrefixLimit: prefixLimit,
		Cursor:      cursor,
		Delimiter:   opts.Delimiter,
		Recursive:   opts.Recursive,
		BatchSize:   batchSize,
		Mode:        ObjectIteratorModeAllVersionsDescending,
		PendingOnly: opts.Pending,
		ListMode:    opts.listMode,

		IncludeCustomMetadata:       opts.IncludeCustomMetadata,
		IncludeSystemMetadata:       opts.IncludeSystemMetadata,
		IncludeETag:                 opts.IncludeETag,
		IncludeETagOrCustomMetadata: opts.IncludeETagOrCustomMetadata,
		IncludeChecksum:             opts.IncludeChecksum,
	})
	if err != nil {
		return err
	}
	wrap := &objectsIterator{
		raw:       raw,
		prefix:    opts.Prefix,
		delimiter: opts.Delimiter,
		recursive: opts.Recursive,
	}
	defer func() {
		err = errs.Combine(err, wrap.err, raw.Close())
	}()
	return fn(ctx, wrap)
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

	// start from either the cursor or prefix, depending on which is larger
	if LessObjectKey(cursor.Key, opts.Prefix) {
		cursor.Key = opts.Prefix
		cursor.Version = -1
		cursor.Inclusive = true
	}

	batchSize := opts.BatchSize
	batchsizeLimit.Ensure(&batchSize)

	raw, err := adapter.ObjectIterator(ctx, ObjectIteratorOptions{
		ProjectID:   opts.ProjectID,
		BucketName:  opts.BucketName,
		Prefix:      opts.Prefix,
		PrefixLimit: prefixLimit,
		Cursor:      cursor,
		Delimiter:   opts.Delimiter,
		Recursive:   opts.Recursive,
		BatchSize:   batchSize,
		Mode:        ObjectIteratorModeAllVersionsAscending,
		PendingOnly: opts.Pending,

		IncludeCustomMetadata:       opts.IncludeCustomMetadata,
		IncludeSystemMetadata:       opts.IncludeSystemMetadata,
		IncludeETag:                 opts.IncludeETag,
		IncludeETagOrCustomMetadata: opts.IncludeETagOrCustomMetadata,
		IncludeChecksum:             opts.IncludeChecksum,
	})
	if err != nil {
		return err
	}
	wrap := &objectsIterator{
		raw:       raw,
		prefix:    opts.Prefix,
		delimiter: opts.Delimiter,
		recursive: opts.Recursive,
	}
	defer func() {
		err = errs.Combine(err, wrap.err, raw.Close())
	}()
	return fn(ctx, wrap)
}

func iteratePendingObjectsByKey(ctx context.Context, adapter Adapter, opts IteratePendingObjectsByKey, fn func(context.Context, ObjectsIterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	batchSize := opts.BatchSize
	batchsizeLimit.Ensure(&batchSize)

	raw, err := adapter.ObjectIterator(ctx, ObjectIteratorOptions{
		ProjectID:  opts.ProjectID,
		BucketName: opts.BucketName,
		Cursor: ObjectsIteratorCursor{
			Key:      opts.ObjectKey,
			Version:  MaxVersion, // TODO: this needs to come as an argument
			StreamID: opts.Cursor.StreamID,
		},
		BatchSize:   batchSize,
		Recursive:   true,
		Mode:        ObjectIteratorModePendingByKey,
		PendingOnly: true,

		IncludeCustomMetadata:       true,
		IncludeSystemMetadata:       true,
		IncludeETag:                 true,
		IncludeETagOrCustomMetadata: false,
		IncludeChecksum:             true,
	})
	if err != nil {
		return err
	}
	wrap := &objectsIterator{
		raw:       raw,
		prefix:    "",
		delimiter: "",
		recursive: true,
	}
	defer func() {
		err = errs.Combine(err, wrap.err, raw.Close())
	}()
	return fn(ctx, wrap)
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

// next pulls a row from the underlying ObjectIterator into item.
func (it *objectsIterator) next(ctx context.Context, item *ObjectEntry) bool {
	ok, err := it.raw.Next(ctx, item)
	if err != nil {
		it.err = err
		return false
	}
	return ok
}

// newSQLObjectIterator builds an sqlObjectIterator from options. Caller
// must set doNextQuery and prime curRows before returning.
func newSQLObjectIterator(adapter Adapter, opts ObjectIteratorOptions) *sqlObjectIterator {
	return &sqlObjectIterator{
		adapter: adapter,

		projectID:   opts.ProjectID,
		bucketName:  opts.BucketName,
		pending:     opts.PendingOnly,
		prefix:      opts.Prefix,
		prefixLimit: opts.PrefixLimit,
		delimiter:   opts.Delimiter,
		batchSize:   opts.BatchSize,
		recursive:   opts.Recursive,
		cursor:      opts.Cursor,

		includeCustomMetadata:       opts.IncludeCustomMetadata,
		includeSystemMetadata:       opts.IncludeSystemMetadata,
		includeETag:                 opts.IncludeETag,
		includeETagOrCustomMetadata: opts.IncludeETagOrCustomMetadata,
		includeChecksum:             opts.IncludeChecksum,
	}
}

// Next advances the underlying SQL rows, refilling the batch as needed,
// and copies the next row into dst. It returns (true, nil) when dst was
// populated, (false, nil) at end of iteration, and (false, err) on failure.
func (it *sqlObjectIterator) Next(ctx context.Context, dst *ObjectEntry) (bool, error) {
	if !it.curRows.Next() {
		if err := it.curRows.Err(); err != nil {
			return false, err
		}

		if it.curIndex < it.batchSize {
			return false, nil
		}

		// for non-recursive listings, advance the cursor past the
		// current folder before re-querying so we can skip an entire
		// folder in a single round trip rather than fetching every row
		// inside it just to discard them.
		if !it.recursive {
			afterPrefix := it.cursor.Key[len(it.prefix):]
			p := strings.Index(string(afterPrefix), string(it.delimiter))
			if p >= 0 {
				skipPrefix, ok := SkipPrefix(afterPrefix[:p+len(it.delimiter)])
				if !ok {
					// no objects can come after the current folder
					return false, nil
				}
				it.cursor.Key = it.prefix + skipPrefix
				it.cursor.StreamID = uuid.UUID{}
				it.cursor.Version = MaxVersion
			}
		}

		rows, err := it.doNextQuery(ctx, it)
		if err != nil {
			return false, err
		}

		if closeErr := it.curRows.Close(); closeErr != nil {
			return false, errs.Combine(closeErr, rows.Close())
		}

		it.curRows = rows
		it.curIndex = 0
		if !it.curRows.Next() {
			return false, it.curRows.Err()
		}
	}

	if err := it.scanItem(dst); err != nil {
		return false, err
	}

	it.curIndex++
	it.cursor.Key = it.prefix + dst.ObjectKey
	it.cursor.Version = dst.Version
	it.cursor.StreamID = dst.StreamID

	return true, nil
}

// Close releases iterator resources.
func (it *sqlObjectIterator) Close() error {
	if it.closed {
		return nil
	}
	it.closed = true
	if it.curRows == nil {
		return nil
	}
	// Err must be called before Close to satisfy the tagsql leak tracker when
	// the consumer stops iterating before the rows have been fully exhausted.
	return errs.Combine(it.curRows.Err(), it.curRows.Close())
}

// ObjectIterator opens a new SQL-backed object iterator on the
// Postgres/Cockroach adapter.
func (p *PostgresAdapter) ObjectIterator(ctx context.Context, opts ObjectIteratorOptions) (_ ObjectIterator, err error) {
	defer mon.Task()(&ctx)(&err)

	return openTagsqlObjectIterator(ctx, p, opts)
}

// ObjectIterator opens a new SQL-backed object iterator on the TiDB adapter.
func (t *TiDBAdapter) ObjectIterator(ctx context.Context, opts ObjectIteratorOptions) (_ ObjectIterator, err error) {
	defer mon.Task()(&ctx)(&err)

	return openTagsqlObjectIterator(ctx, t, opts)
}

// tagsqlObjectAdapter is satisfied by adapters whose object_iterator queries
// can share SQL — Postgres/Cockroach (after postgresRebind) and TiDB. Both
// halves are required: Adapter for newSQLObjectIterator and tagsqlAdapter for
// the UnderlyingDB() / Implementation() reach-throughs the shared helpers do.
type tagsqlObjectAdapter interface {
	Adapter
	tagsqlAdapter
}

func openTagsqlObjectIterator(ctx context.Context, db tagsqlObjectAdapter, opts ObjectIteratorOptions) (_ ObjectIterator, err error) {
	it := newSQLObjectIterator(db, opts)
	switch opts.Mode {
	case ObjectIteratorModeAllVersionsAscending:
		it.doNextQuery = func(ctx context.Context, it *sqlObjectIterator) (tagsql.Rows, error) {
			return tagsqlDoNextQueryAllVersionsAscending(ctx, db, it)
		}
	case ObjectIteratorModePendingByKey:
		it.doNextQuery = func(ctx context.Context, it *sqlObjectIterator) (tagsql.Rows, error) {
			return tagsqlDoNextQueryPendingObjectsByKey(ctx, db, it)
		}
	default:
		// TiDB cannot stream the mixed-direction descending order from the ascending
		// primary key; the key-probe mode bounds each batch scan with a subquery.
		// The iterator implements only the key-probe strategy, so local-reorder also
		// uses it here rather than regressing to the unbounded plain scan.
		it.keyProbe = (opts.ListMode == ListModeKeyProbe || opts.ListMode == ListModeLocalReorder) &&
			db.Implementation() == dbutil.TiDB
		it.doNextQuery = func(ctx context.Context, it *sqlObjectIterator) (tagsql.Rows, error) {
			return tagsqlDoNextQueryAllVersionsDescending(ctx, db, it)
		}
	}

	it.curRows, err = it.doNextQuery(ctx, it)
	if err != nil {
		return nil, err
	}
	it.cursor.Inclusive = false
	return it, nil
}

// ObjectIterator opens a new SQL-backed object iterator on the
// Spanner adapter.
func (s *SpannerAdapter) ObjectIterator(ctx context.Context, opts ObjectIteratorOptions) (_ ObjectIterator, err error) {
	defer mon.Task()(&ctx)(&err)

	it := newSQLObjectIterator(s, opts)
	switch opts.Mode {
	case ObjectIteratorModeAllVersionsAscending:
		it.doNextQuery = s.doNextQueryAllVersionsWithStatusAscending
	case ObjectIteratorModePendingByKey:
		it.doNextQuery = s.doNextQueryPendingObjectsByKey
	default:
		it.doNextQuery = s.doNextQueryAllVersionsWithStatus
	}

	it.curRows, err = it.doNextQuery(ctx, it)
	if err != nil {
		return nil, err
	}
	it.cursor.Inclusive = false
	return it, nil
}

var iterDescendingCache iterSQLCache

// tagsqlDoNextQueryAllVersionsDescending serves Postgres/Cockroach/TiDB. The
// query uses ? placeholders so the same SQL works for backends that natively
// use them (TiDB / MySQL) and for backends whose tagsql.DB wrapper rebinds to
// $N (Postgres / Cockroach via postgresRebind). expires_at filtering uses a
// time.Now() parameter so the SQL doesn't need a backend-specific now()/NOW(6);
// this means the satellite's clock — not the database's — defines the cutoff,
// so satellite/db skew is observable in object-listing visibility.
func tagsqlDoNextQueryAllVersionsDescending(ctx context.Context, db tagsqlAdapter, it *sqlObjectIterator) (_ tagsql.Rows, err error) {
	defer mon.Task()(&ctx)(&err)

	// boundaryArgs feed the WHERE clause of buildIterDescendingSQL; with the
	// key-probe bound they repeat verbatim for the probe subquery's identical
	// filters, sharing the same expiration cutoff.
	boundaryArgs := []any{
		it.projectID, it.bucketName,
		it.cursor.Key, it.cursor.Key, int64(it.cursor.Version),
	}
	if it.prefix != "" && it.prefixLimit != "" {
		boundaryArgs = append(boundaryArgs, it.prefixLimit)
	}
	boundaryArgs = append(boundaryArgs, time.Now())

	var args []any
	if it.prefix != "" {
		args = append(args, len(it.prefix)+1)
	}
	args = append(args, boundaryArgs...)
	if it.keyProbe {
		args = append(args, boundaryArgs...)
		args = append(args, it.batchSize)
	}
	args = append(args, it.batchSize)

	underlying := db.UnderlyingDB()
	rebinder, _ := underlying.(sqlRebinder)
	sql := iterDescendingCache.get(shapeOf(it), rebinder, func() string {
		return buildIterDescendingSQL(it)
	})
	return underlying.QueryContext(ctx, sql, args...)
}

func buildIterDescendingSQL(it *sqlObjectIterator) string {
	cursorCompare := ">"
	if it.cursor.Inclusive {
		cursorCompare = ">="
	}
	statusFilter := `AND status <> ` + statusPending
	if it.pending {
		statusFilter = `AND status = ` + statusPending
	}
	querySelectFields := querySelectorFields("object_key", it)
	if it.prefix != "" {
		querySelectFields = querySelectorFields("SUBSTRING(object_key FROM ?) AS object_key_suffix", it)
	}
	queryUpperBound := ""
	if it.prefix != "" && it.prefixLimit != "" {
		queryUpperBound = "AND object_key < ?"
	}

	// TiDB cannot stream the mixed-direction ORDER BY (object_key ASC, version DESC)
	// from the ascending primary key, so it plans a TopN that scans and sorts every
	// row from the cursor to the end of the range for each batch. To bound that scan,
	// a probe subquery finds the object_key of the batch's last row using a fully
	// ascending order, which TiDB streams with the LIMIT pushed down. Both orders
	// enumerate whole object_key groups, so the probe key is exactly the last key of
	// the batch, and bounding the outer query by it cannot change the result. See
	// (*TiDBAdapter).ListObjects for the same technique.
	keyBound := ""
	if it.keyProbe {
		keyBound = `AND object_key <= (
				SELECT MAX(object_key) FROM (
					SELECT object_key
					FROM objects
					WHERE
						(project_id, bucket_name) = (?, ?)
						AND (
							object_key > ?
							OR (object_key = ? AND ? ` + cursorCompare + ` version)
						)
						` + queryUpperBound + `
						` + statusFilter + `
						AND (expires_at IS NULL OR expires_at > ?)
					ORDER BY object_key ASC, version ASC
					LIMIT ?
				) AS probe
			)`
	}

	return `
		SELECT
			` + querySelectFields + `
		FROM objects
		WHERE
			(project_id, bucket_name) = (?, ?)
			AND (
				object_key > ?
				OR (object_key = ? AND ? ` + cursorCompare + ` version)
			)
			` + queryUpperBound + `
			` + statusFilter + `
			AND (expires_at IS NULL OR expires_at > ?)
			` + keyBound + `
			ORDER BY project_id ASC, bucket_name ASC, object_key ASC, version DESC
		LIMIT ?
		`
}

func (s *SpannerAdapter) doNextQueryAllVersionsWithStatus(ctx context.Context, it *sqlObjectIterator) (_ tagsql.Rows, err error) {
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

var iterAscendingCache iterSQLCache

func tagsqlDoNextQueryAllVersionsAscending(ctx context.Context, db tagsqlAdapter, it *sqlObjectIterator) (_ tagsql.Rows, err error) {
	defer mon.Task()(&ctx)(&err)

	var args []any
	if it.prefix != "" {
		args = append(args, len(it.prefix)+1)
	}
	args = append(args,
		it.projectID, it.bucketName,
		it.cursor.Key, int64(it.cursor.Version),
	)
	if it.prefix != "" && it.prefixLimit != "" {
		args = append(args, it.prefixLimit)
	}
	args = append(args, time.Now(), it.batchSize)

	underlying := db.UnderlyingDB()
	rebinder, _ := underlying.(sqlRebinder)
	sql := iterAscendingCache.get(shapeOf(it), rebinder, func() string {
		return buildIterAscendingSQL(it)
	})
	return underlying.QueryContext(ctx, sql, args...)
}

func buildIterAscendingSQL(it *sqlObjectIterator) string {
	cursorCompare := ">"
	if it.cursor.Inclusive {
		cursorCompare = ">="
	}
	statusFilter := `AND status <> ` + statusPending
	if it.pending {
		statusFilter = `AND status = ` + statusPending
	}
	querySelectFields := querySelectorFields("object_key", it)
	if it.prefix != "" {
		querySelectFields = querySelectorFields("SUBSTRING(object_key FROM ?) AS object_key_suffix", it)
	}
	queryUpperBound := ""
	if it.prefix != "" && it.prefixLimit != "" {
		queryUpperBound = "AND object_key < ?"
	}
	return `
		SELECT
			` + querySelectFields + `
		FROM objects
		WHERE
			(project_id, bucket_name) = (?, ?)
			AND (object_key, version) ` + cursorCompare + ` (?, ?)
			` + queryUpperBound + `
			` + statusFilter + `
			AND (expires_at IS NULL OR expires_at > ?)
			ORDER BY project_id ASC, bucket_name ASC, object_key ASC, version ASC
		LIMIT ?
		`
}

func (s *SpannerAdapter) doNextQueryAllVersionsWithStatusAscending(ctx context.Context, it *sqlObjectIterator) (_ tagsql.Rows, err error) {
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

func querySelectorFields(objectKeyColumn string, it *sqlObjectIterator) string {
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

	if it.includeCustomMetadata || it.includeETag || it.includeETagOrCustomMetadata || it.includeChecksum {
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

	if it.includeChecksum {
		querySelectFields += `
			, checksum`
	}

	return querySelectFields
}

// nextBucket returns the lexicographically next bucket.
func nextBucket(b BucketName) BucketName {
	return b + "\x00"
}

// pendingObjectsByKeySQL is the constant SQL for the pending-objects-by-key
// page query. The rebound variant is computed lazily and cached.
const pendingObjectsByKeySQL = `
	SELECT
		object_key, stream_id, version, status, encryption,
		created_at, expires_at,
		segment_count,
		total_plain_size, total_encrypted_size, fixed_segment_size,
		encrypted_metadata_nonce, encrypted_metadata_encrypted_key, encrypted_metadata, encrypted_etag,
		checksum
	FROM objects
	WHERE
		(project_id, bucket_name, object_key) = (?, ?, ?)
		AND stream_id > ?
		AND status = ` + statusPending + `
		AND (expires_at IS NULL OR expires_at > ?)
	ORDER BY stream_id ASC
	LIMIT ?
	`

var pendingObjectsByKeyReboundSQL = sync.OnceValue(func() string {
	return postgresRebind{}.Rebind(pendingObjectsByKeySQL)
})

// tagsqlDoNextQueryPendingObjectsByKey executes the pending-objects-by-key
// page query for Postgres/Cockroach/TiDB.
func tagsqlDoNextQueryPendingObjectsByKey(ctx context.Context, db tagsqlAdapter, it *sqlObjectIterator) (_ tagsql.Rows, err error) {
	defer mon.Task()(&ctx)(&err)

	underlying := db.UnderlyingDB()
	sql := pendingObjectsByKeySQL
	if _, ok := underlying.(sqlRebinder); ok {
		sql = pendingObjectsByKeyReboundSQL()
	}
	return underlying.QueryContext(ctx, sql,
		it.projectID, it.bucketName,
		it.cursor.Key,
		it.cursor.StreamID,
		time.Now(),
		it.batchSize,
	)
}

func (s *SpannerAdapter) doNextQueryPendingObjectsByKey(ctx context.Context, it *sqlObjectIterator) (_ tagsql.Rows, err error) {
	defer mon.Task()(&ctx)(&err)

	rowIterator := s.client.Single().QueryWithOptions(ctx, spanner.Statement{
		SQL: `
			SELECT
				object_key, stream_id, version, status, encryption,
				created_at, expires_at,
				segment_count,
				total_plain_size, total_encrypted_size, fixed_segment_size,
				encrypted_metadata_nonce, encrypted_metadata_encrypted_key, encrypted_metadata, encrypted_etag,
				checksum
			FROM objects
			WHERE
				(project_id, bucket_name, object_key) = (@project_id, @bucket_name, @cursor_key)
				AND stream_id > @stream_id
				AND status = ` + statusPending + `
				AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
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

// scanItem scans the current SQL row into ObjectEntry.
func (it *sqlObjectIterator) scanItem(item *ObjectEntry) (err error) {
	item.IsPrefix = false

	fields := []any{
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

	if it.includeCustomMetadata || it.includeETag || it.includeETagOrCustomMetadata || it.includeChecksum {
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

	if it.includeChecksum {
		fields = append(fields, &item.Checksum)
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
