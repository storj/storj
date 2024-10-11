// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"cloud.google.com/go/spanner"
	"github.com/zeebo/errs"
	"google.golang.org/api/iterator"

	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/spannerutil"
	"storj.io/storj/shared/tagsql"
)

// DelimiterNext is the string that comes immediately after Delimiter="/".
const DelimiterNext = "0"

// ListObjectsCursor is a cursor used during iteration through objects.
type ListObjectsCursor IterateCursor

// ListObjects contains arguments necessary for listing objects.
//
// For Pending = false, the versions are in descending order.
// For Pending = true, the versions are in ascending order.
type ListObjects struct {
	ProjectID             uuid.UUID
	BucketName            BucketName
	Recursive             bool
	Limit                 int
	Prefix                ObjectKey
	Cursor                ListObjectsCursor
	Pending               bool
	AllVersions           bool
	IncludeCustomMetadata bool
	IncludeSystemMetadata bool
}

// Verify verifies get object request fields.
func (opts *ListObjects) Verify() error {
	switch {
	case opts.ProjectID.IsZero():
		return ErrInvalidRequest.New("ProjectID missing")
	case opts.BucketName == "":
		return ErrInvalidRequest.New("BucketName missing")
	case opts.Limit < 0:
		return ErrInvalidRequest.New("Invalid limit: %d", opts.Limit)
	}

	return nil
}

// ListObjectsResult result of listing objects.
type ListObjectsResult struct {
	Objects []ObjectEntry
	More    bool
}

// ListObjects lists objects.
func (db *DB) ListObjects(ctx context.Context, opts ListObjects) (result ListObjectsResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if db.config.UseListObjectsIterator {
		return db.ListObjectsWithIterator(ctx, opts)
	}

	if err := opts.Verify(); err != nil {
		return ListObjectsResult{}, err
	}

	ListLimit.Ensure(&opts.Limit)

	return db.ChooseAdapter(opts.ProjectID).ListObjects(ctx, opts)
}

// ListObjects lists objects.
func (p *PostgresAdapter) ListObjects(ctx context.Context, opts ListObjects) (result ListObjectsResult, err error) {
	// maxSkipVersionsUntilRequery is the limit on how many versions we query for a single object, until we requery.
	const maxSkipVersionsUntilRequery = 100

	// maxSkipPrefixUntilRequery is the limit on how many entries we scan inside a prefix, until we requery.
	const maxSkipPrefixUntilRequery = 10

	// minQuerySize ensures that we list a more entries, as there's a significant overhead to a single query.
	const minQuerySize = 100

	// requeryLimit is a safety net for invalid implementation.
	requeryLimit := opts.Limit + 10 // we do some extra queries, but, roughly at most we should have one query per entry

	// extraSkipEntries to avoid requerying in the common case of !AllVersions.
	const extraSkipEntries = 10
	// extraEntriesForMore is the additional entry we need for determining whether there are more entries.
	const extraEntriesForMore = 1
	batchSize := opts.Limit + extraEntriesForMore + extraSkipEntries

	if batchSize < minQuerySize {
		batchSize = minQuerySize
	}

	// lastEntry is used to keep track of the last entry put into the result.
	var lastEntry struct {
		Set bool

		ObjectKey ObjectKey
		Version   Version
		IsPrefix  bool
	}

	// skipCounter keeps track on how many entries we have skipped either due to
	// objects of similar version or due to a collapsed non-recursive prefix.
	type skipCounter struct {
		Prefix  int
		Version int
	}
	var skipCount skipCounter

	cursor := opts.StartCursor()

	for repeat := 0; repeat < requeryLimit; repeat++ {
		args := []any{
			opts.ProjectID, opts.BucketName,
			cursor.Key, cursor.Version,
			batchSize, nextBucket(opts.BucketName),
		}
		if opts.Prefix != "" {
			args = append(args, len(opts.Prefix)+1, opts.stopKey())
		}

		var objectKey = `object_key`
		if opts.Prefix != "" {
			objectKey = `substring(object_key from $7) AS object_key`
		}

		var statusCondition = `status != ` + statusPending
		if opts.Pending {
			statusCondition = `status = ` + statusPending
		}

		rows, err := p.db.QueryContext(ctx, `SELECT
			`+objectKey+`,
			version
			`+opts.selectedFields()+`
			FROM objects
			WHERE
				`+opts.boundaryPostgres()+`
				AND (project_id, bucket_name) < ($1, $6)
				AND `+statusCondition+`
				AND (expires_at IS NULL OR expires_at > now())
			ORDER BY `+opts.orderBy()+`
			LIMIT $5
		`, args...)
		if errors.Is(err, sql.ErrNoRows) {
			return result, nil
		}
		if err != nil {
			return result, Error.Wrap(err)
		}

		scannedCount := 0
		skipAhead := false
	read_entries:
		for rows.Next() {
			entry, err := scanListObjectsEntryPostgres(rows, &opts)
			if err != nil {
				return result, Error.Wrap(errs.Combine(err, rows.Err(), rows.Close()))
			}
			scannedCount++

			// skip a duplicate prefix entry, which only happens with opts.Recursive
			// TODO: does this need opts.AllVersions
			skipPrefix := lastEntry.Set && opts.AllVersions && lastEntry.IsPrefix && entry.IsPrefix && lastEntry.ObjectKey == entry.ObjectKey
			// skip duplicate object key with other versions, when !opts.AllVersions
			skipVersion := lastEntry.Set && !opts.AllVersions && lastEntry.IsPrefix == entry.IsPrefix && lastEntry.ObjectKey == entry.ObjectKey

			// we'll need to ensure that when we are iterating only latest objects that we don't
			// emit an object entry when we start iterating from half-way in versions.
			var skipCursorAllVersionsDoubleCheck bool
			if !opts.AllVersions && entryKeyMatchesCursor(opts.Prefix, entry.ObjectKey, opts.Cursor.Key) {
				if opts.VersionAscending() {
					skipCursorAllVersionsDoubleCheck = entry.Version <= opts.Cursor.Version
				} else {
					skipCursorAllVersionsDoubleCheck = entry.Version >= opts.Cursor.Version
				}
			}

			lastEntry.Set = true
			lastEntry.ObjectKey = entry.ObjectKey
			lastEntry.Version = entry.Version
			lastEntry.IsPrefix = entry.IsPrefix

			if skipPrefix || skipVersion || skipCursorAllVersionsDoubleCheck {
				if skipPrefix {
					skipCount.Prefix++
				}
				if skipVersion {
					skipCount.Version++
				}

				if skipCount.Prefix >= maxSkipPrefixUntilRequery || skipCount.Version >= maxSkipVersionsUntilRequery {
					skipAhead = true
					skipCount = skipCounter{}
					// we landed inside a large number of repeated items,
					// either prefixes or versions, let's requery and skip
					break read_entries
				}

				continue
			}

			skipCount = skipCounter{}

			// We don't want to include delete markers in the output, when we are listing only the latest version.
			// We still set "lastEntry" so we skip any objects that are beyond the delete marker.
			if !opts.AllVersions && entry.Status.IsDeleteMarker() {
				continue
			}

			result.Objects = append(result.Objects, entry)
			if len(result.Objects) >= opts.Limit+1 {
				result.More = true
				result.Objects = result.Objects[:opts.Limit]
				return result, Error.Wrap(errs.Combine(err, rows.Err(), rows.Close()))
			}
		}

		if err := errs.Combine(rows.Err(), rows.Close()); err != nil {
			return result, Error.Wrap(err)
		}

		if scannedCount == 0 {
			result.More = false
			return result, nil
		}
		if !skipAhead && scannedCount < batchSize {
			result.More = false
			return result, nil
		}

		switch {
		case lastEntry.IsPrefix: // can only be true if recursive listing
			// skip over the prefix
			cursor.Key = opts.Prefix + lastEntry.ObjectKey[:len(lastEntry.ObjectKey)-1] + DelimiterNext
			cursor.Version = opts.FirstVersion()

		case opts.AllVersions:
			// continue where-ever we left off
			cursor.Key = opts.Prefix + lastEntry.ObjectKey
			cursor.Version = lastEntry.Version

		case !opts.AllVersions:
			// jump to the next object
			cursor.Key = opts.Prefix + lastEntry.ObjectKey
			cursor.Version = opts.lastVersion()
		}
	}

	panic("too many requeries")
}

// ListObjects lists objects.
func (s *SpannerAdapter) ListObjects(ctx context.Context, opts ListObjects) (result ListObjectsResult, err error) {
	// TODO(spanner): retune all of these for Spanner. Also, can we use a smarter query now
	// using some feature that wasn't in Cockroach? (e.g. windowed queries).

	// maxSkipVersionsUntilRequery is the limit on how many versions we query for a single object, until we requery.
	const maxSkipVersionsUntilRequery = 100

	// maxSkipPrefixUntilRequery is the limit on how many entries we scan inside a prefix, until we requery.
	const maxSkipPrefixUntilRequery = 10

	// minQuerySize ensures that we list a more entries, as there's a significant overhead to a single query.
	const minQuerySize = 100

	// requeryLimit is a safety net for invalid implementation.
	requeryLimit := opts.Limit + 10 // we do some extra queries, but, roughly at most we should have one query per entry

	// extraSkipEntries to avoid requerying in the common case of !AllVersions.
	const extraSkipEntries = 10
	// extraEntriesForMore is the additional entry we need for determining whether there are more entries.
	const extraEntriesForMore = 1
	batchSize := opts.Limit + extraEntriesForMore + extraSkipEntries

	if batchSize < minQuerySize {
		batchSize = minQuerySize
	}

	// lastEntry is used to keep track of the last entry put into the result.
	var lastEntry struct {
		Set bool

		ObjectKey ObjectKey
		Version   Version
		IsPrefix  bool
	}

	// skipCounter keeps track on how many entries we have skipped either due to
	// objects of similar version or due to a collapsed non-recursive prefix.
	type skipCounter struct {
		Prefix  int
		Version int
	}
	var skipCount skipCounter

	cursor := opts.StartCursor()

	for repeat := 0; repeat < requeryLimit; repeat++ {
		args := map[string]any{
			"project_id":     opts.ProjectID,
			"bucket_name":    opts.BucketName,
			"cursor_key":     cursor.Key,
			"cursor_version": cursor.Version,
			"limit":          batchSize,
			"next_bucket":    nextBucket(opts.BucketName),
		}
		if opts.Prefix != "" {
			args["prefix_len"] = len(opts.Prefix) + 1
			args["stop_key"] = opts.stopKey()
		}

		var objectKey = `object_key`
		if opts.Prefix != "" {
			objectKey = `substr(object_key, @prefix_len) AS object_key`
		}

		var statusCondition = `status != ` + statusPending
		if opts.Pending {
			statusCondition = `status = ` + statusPending
		}

		stmt := spanner.Statement{
			SQL: `
				SELECT
					` + objectKey + `,
					version
					` + opts.selectedFields() + `
				FROM objects
				WHERE
					` + opts.boundarySpanner() + `
					AND ((project_id < @project_id) OR (project_id = @project_id AND bucket_name < @next_bucket))
					AND ` + statusCondition + `
					AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
				ORDER BY ` + opts.orderBy() + `
				LIMIT @limit
			`,
			Params: args,
		}

		scannedCount := 0
		skipAhead := false
		done := false

		err := func() error {
			rowIterator := s.client.Single().Query(ctx, stmt)
			defer rowIterator.Stop()

		readEntries:
			for {
				row, err := rowIterator.Next()
				if err != nil {
					if errors.Is(err, iterator.Done) {
						done = true
						return nil
					}
					return Error.Wrap(err)
				}

				entry, err := scanListObjectsEntrySpanner(row, &opts)
				if err != nil {
					return Error.Wrap(err)
				}
				scannedCount++

				// skip a duplicate prefix entry, which only happens with opts.Recursive
				// TODO: does this need opts.AllVersions
				skipPrefix := lastEntry.Set && opts.AllVersions && lastEntry.IsPrefix && entry.IsPrefix && lastEntry.ObjectKey == entry.ObjectKey
				// skip duplicate object key with other versions, when !opts.AllVersions
				skipVersion := lastEntry.Set && !opts.AllVersions && lastEntry.IsPrefix == entry.IsPrefix && lastEntry.ObjectKey == entry.ObjectKey

				// we'll need to ensure that when we are iterating only latest objects that we don't
				// emit an object entry when we start iterating from half-way in versions.
				var skipCursorAllVersionsDoubleCheck bool
				if !opts.AllVersions && entryKeyMatchesCursor(opts.Prefix, entry.ObjectKey, opts.Cursor.Key) {
					if opts.VersionAscending() {
						skipCursorAllVersionsDoubleCheck = entry.Version <= opts.Cursor.Version
					} else {
						skipCursorAllVersionsDoubleCheck = entry.Version >= opts.Cursor.Version
					}
				}

				lastEntry.Set = true
				lastEntry.ObjectKey = entry.ObjectKey
				lastEntry.Version = entry.Version
				lastEntry.IsPrefix = entry.IsPrefix

				if skipPrefix || skipVersion || skipCursorAllVersionsDoubleCheck {
					if skipPrefix {
						skipCount.Prefix++
					}
					if skipVersion {
						skipCount.Version++
					}

					if skipCount.Prefix >= maxSkipPrefixUntilRequery || skipCount.Version >= maxSkipVersionsUntilRequery {
						skipAhead = true
						skipCount = skipCounter{}
						// we landed inside a large number of repeated items,
						// either prefixes or versions, let's requery and skip
						break readEntries
					}

					continue
				}

				skipCount = skipCounter{}

				// We don't want to include delete markers in the output, when we are listing only the latest version.
				// We still set "lastEntry" so we skip any objects that are beyond the delete marker.
				if !opts.AllVersions && entry.Status.IsDeleteMarker() {
					continue
				}

				result.Objects = append(result.Objects, entry)
				if len(result.Objects) >= opts.Limit+1 {
					result.More = true
					result.Objects = result.Objects[:opts.Limit]
					done = true
					return nil
				}
			}
			return nil
		}()
		if err != nil {
			return result, Error.Wrap(err)
		}
		if done {
			return result, nil
		}

		if scannedCount == 0 {
			result.More = false
			return result, nil
		}
		if !skipAhead && scannedCount < batchSize {
			result.More = false
			return result, nil
		}

		switch {
		case lastEntry.IsPrefix: // can only be true if recursive listing
			// skip over the prefix
			cursor.Key = opts.Prefix + lastEntry.ObjectKey[:len(lastEntry.ObjectKey)-1] + DelimiterNext
			cursor.Version = opts.FirstVersion()

		case opts.AllVersions:
			// continue where-ever we left off
			cursor.Key = opts.Prefix + lastEntry.ObjectKey
			cursor.Version = lastEntry.Version

		case !opts.AllVersions:
			// jump to the next object
			cursor.Key = opts.Prefix + lastEntry.ObjectKey
			cursor.Version = opts.lastVersion()
		}
	}

	panic("too many requeries")
}

func entryKeyMatchesCursor(prefix, entryKey, cursorKey ObjectKey) bool {
	return len(prefix)+len(entryKey) == len(cursorKey) &&
		prefix == cursorKey[:len(prefix)] &&
		entryKey == cursorKey[len(prefix):]
}

func (opts *ListObjects) stopKey() []byte {
	if opts.Prefix != "" {
		return []byte(PrefixLimit(opts.Prefix))
	}
	return nil
}

func (opts *ListObjects) boundaryPostgres() string {
	const prefixBoundaryCondition = `(project_id, bucket_name, object_key) < ($1, $2, $8)`

	if opts.VersionAscending() {
		const compare = `(project_id, bucket_name, object_key, version) > ($1, $2, $3, $4)`
		if opts.Prefix != "" {
			return compare + " AND " + prefixBoundaryCondition
		}
		return compare
	} else {
		const compare = `((project_id, bucket_name, object_key) > ($1, $2, $3) OR ((project_id, bucket_name, object_key) = ($1, $2, $3) AND version < $4))`
		if opts.Prefix != "" {
			return compare + " AND " + prefixBoundaryCondition
		}
		return compare
	}
}

func (opts *ListObjects) boundarySpanner() string {
	const prefixBoundaryCondition = `(
		(project_id < @project_id)
		OR (project_id = @project_id AND bucket_name < @bucket_name)
		OR (project_id = @project_id AND bucket_name = @bucket_name AND object_key < @stop_key)
	)`

	if opts.VersionAscending() {
		const compare = `(
			project_id > @project_id
			OR (project_id = @project_id AND bucket_name > @bucket_name)
			OR (project_id = @project_id AND bucket_name = @bucket_name AND object_key > @cursor_key)
			OR (project_id = @project_id AND bucket_name = @bucket_name AND object_key = @cursor_key AND version > @cursor_version)
		)`
		if opts.Prefix != "" {
			return compare + " AND " + prefixBoundaryCondition
		}
		return compare
	} else {
		const compare = `(
			(
				project_id > @project_id
				OR (project_id = @project_id AND bucket_name > @bucket_name)
				OR (project_id = @project_id AND bucket_name = @bucket_name AND object_key > @cursor_key)
			)
			OR
			((project_id, bucket_name, object_key) = (@project_id, @bucket_name, @cursor_key) AND version < @cursor_version)
		)`
		if opts.Prefix != "" {
			return compare + " AND " + prefixBoundaryCondition
		}
		return compare
	}
}

// FirstVersion returns the first object version we need to iterate given the list objects logic.
func (opts *ListObjects) FirstVersion() Version {
	if opts.VersionAscending() {
		return 0
	} else {
		return MaxVersion
	}
}

func (opts *ListObjects) lastVersion() Version {
	if opts.VersionAscending() {
		return MaxVersion
	} else {
		return 0
	}
}

// VersionAscending returns whether the versions in the result are in ascending order.
func (opts *ListObjects) VersionAscending() bool {
	return opts.Pending
}

func (opts *ListObjects) orderBy() string {
	if opts.VersionAscending() {
		return "project_id ASC, bucket_name ASC, object_key ASC, version ASC"
	} else {
		return "project_id ASC, bucket_name ASC, object_key ASC, version DESC"
	}
}

func (opts ListObjects) selectedFields() (selectedFields string) {
	selectedFields += `
	,stream_id
	,status
	,encryption`

	if opts.IncludeSystemMetadata {
		selectedFields += `
		,created_at
		,expires_at
		,segment_count
		,total_plain_size
		,total_encrypted_size
		,fixed_segment_size`
	}

	if opts.IncludeCustomMetadata {
		selectedFields += `
		,encrypted_metadata_nonce
		,encrypted_metadata
		,encrypted_metadata_encrypted_key`
	}

	return selectedFields
}

// StartCursor returns the starting object cursor for this listing.
func (opts *ListObjects) StartCursor() ListObjectsCursor {
	if !strings.HasPrefix(string(opts.Cursor.Key), string(opts.Prefix)) {
		// if the starting position is outside of the prefix
		if LessObjectKey(opts.Cursor.Key, opts.Prefix) {
			// If we are before the prefix, then let's start from the prefix.
			return ListObjectsCursor{Key: opts.Prefix, Version: opts.FirstVersion()}
		}

		// Otherwise, we must be after the prefix, and let's leave the cursor as is.
		// We could also entirely skip the query to the database.

		if !opts.AllVersions {
			// We'll do the same behavior of double checking the "versions",
			// however, since the cursor is past prefix, we can entirely skip
			// this logic.
			return ListObjectsCursor{Key: opts.Cursor.Key, Version: opts.FirstVersion()}
		}

		return opts.Cursor
	}

	keyWithoutPrefix := opts.Cursor.Key[len(opts.Prefix):]
	if !opts.Recursive {
		// Check whether we need to skip outside of a prefix.
		firstDelimiter := strings.IndexByte(string(keyWithoutPrefix), '/')
		if firstDelimiter >= 0 {
			firstDelimiter += len(opts.Prefix)
			return ListObjectsCursor{
				Key:     opts.Cursor.Key[:firstDelimiter] + DelimiterNext,
				Version: opts.FirstVersion(),
			}
		}
	}

	if !opts.AllVersions {
		// We need to double check whether the latest entry has been already
		// produced, because we may need to skip it.
		return ListObjectsCursor{Key: opts.Cursor.Key, Version: opts.FirstVersion()}
	}

	return opts.Cursor
}

func scanListObjectsEntryPostgres(rows tagsql.Rows, opts *ListObjects) (item ObjectEntry, err error) {
	fields := []interface{}{
		&item.ObjectKey,
		&item.Version,
		&item.StreamID,
		&item.Status,
		encryptionParameters{&item.Encryption},
	}

	if opts.IncludeSystemMetadata {
		fields = append(fields,
			&item.CreatedAt,
			&item.ExpiresAt,
			&item.SegmentCount,
			&item.TotalPlainSize,
			&item.TotalEncryptedSize,
			&item.FixedSegmentSize,
		)
	}

	if opts.IncludeCustomMetadata {
		fields = append(fields,
			&item.EncryptedMetadataNonce,
			&item.EncryptedMetadata,
			&item.EncryptedMetadataEncryptedKey,
		)
	}

	if err := rows.Scan(fields...); err != nil {
		return item, err
	}

	if !opts.Recursive {
		i := strings.IndexByte(string(item.ObjectKey), Delimiter)
		if i >= 0 {
			item.IsPrefix = true
			item.ObjectKey = item.ObjectKey[:i+1]
		}
	}

	if item.IsPrefix {
		return ObjectEntry{
			IsPrefix:  true,
			ObjectKey: item.ObjectKey,
			Status:    Prefix,
		}, nil
	}

	return item, nil
}
func scanListObjectsEntrySpanner(row *spanner.Row, opts *ListObjects) (item ObjectEntry, err error) {
	fields := []interface{}{
		&item.ObjectKey,
		&item.Version,
		&item.StreamID,
		&item.Status,
		encryptionParameters{&item.Encryption},
	}

	if opts.IncludeSystemMetadata {
		fields = append(fields,
			&item.CreatedAt,
			&item.ExpiresAt,
			spannerutil.Int(&item.SegmentCount),
			&item.TotalPlainSize,
			&item.TotalEncryptedSize,
			spannerutil.Int(&item.FixedSegmentSize),
		)
	}

	if opts.IncludeCustomMetadata {
		fields = append(fields,
			&item.EncryptedMetadataNonce,
			&item.EncryptedMetadata,
			&item.EncryptedMetadataEncryptedKey,
		)
	}

	if err := row.Columns(fields...); err != nil {
		return item, err
	}

	if !opts.Recursive {
		i := strings.IndexByte(string(item.ObjectKey), Delimiter)
		if i >= 0 {
			item.IsPrefix = true
			item.ObjectKey = item.ObjectKey[:i+1]
		}
	}

	if item.IsPrefix {
		return ObjectEntry{
			IsPrefix:  true,
			ObjectKey: item.ObjectKey,
			Status:    Prefix,
		}, nil
	}

	return item, nil
}
