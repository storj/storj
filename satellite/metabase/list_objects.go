// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"storj.io/common/uuid"
	"storj.io/private/tagsql"
)

// ListObjectsCursor is a cursor used during iteration through objects.
type ListObjectsCursor IterateCursor

// ListObjects contains arguments necessary for listing objects.
//
// For Pending = false, the versions are in descending order.
// For pending = true, the versions are in ascending order.
type ListObjects struct {
	ProjectID             uuid.UUID
	BucketName            string
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

	case opts.Pending && !opts.AllVersions:
		return ErrInvalidRequest.New("Not Implemented: Pending && !AllVersions")
	case !opts.Pending && opts.AllVersions:
		return ErrInvalidRequest.New("Not Implemented: !Pending && AllVersions")
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

	if err := opts.Verify(); err != nil {
		return ListObjectsResult{}, err
	}

	ListLimit.Ensure(&opts.Limit)

	var entries []ObjectEntry
	err = withRows(db.db.QueryContext(ctx, opts.getSQLQuery(),
		opts.ProjectID, []byte(opts.BucketName),
		opts.startKey(), opts.Cursor.Version, opts.stopKey(),
		opts.Limit+1, len(opts.Prefix)+1),
	)(func(rows tagsql.Rows) error {
		entries, err = scanListObjectsResult(rows, opts)
		return err
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ListObjectsResult{}, nil
		}
		return ListObjectsResult{}, Error.New("unable to list objects: %w", err)
	}

	if len(entries) > opts.Limit {
		result.More = true
		result.Objects = entries[:opts.Limit]
		return result, nil
	}

	result.Objects = entries
	result.More = false
	return result, nil
}

func (opts *ListObjects) getSQLQuery() string {
	var indexFields string
	if opts.Recursive {
		indexFields = `substring(object_key from $7) AS entry_key, version AS entry_version, FALSE AS is_prefix`
	} else {
		if opts.AllVersions {
			indexFields = `
				DISTINCT ON (entry_key, entry_version)
				(CASE
					WHEN position('/' IN substring(object_key from $7)) <> 0
					THEN substring(substring(object_key from $7) from 0 for (position('/' IN substring(object_key from $7)) +1))
					ELSE substring(object_key from $7)
				END)
				AS entry_key,
				(CASE
					WHEN position('/' IN substring(object_key from $7)) <> 0
					THEN 0
					ELSE version
				END)
				AS entry_version,
				position('/' IN substring(object_key from $7)) <> 0 AS is_prefix`
		} else {
			indexFields = `
				DISTINCT ON (entry_key)
				(CASE
					WHEN position('/' IN substring(object_key from $7)) <> 0
					THEN substring(substring(object_key from $7) from 0 for (position('/' IN substring(object_key from $7)) +1))
					ELSE substring(object_key from $7)
				END)
				AS entry_key,
				version AS entry_version,
				position('/' IN substring(object_key from $7)) <> 0 AS is_prefix`
		}
	}

	switch {
	case opts.Pending && opts.AllVersions:
		return `SELECT ` + indexFields + opts.selectedFields() + `
			FROM objects
			WHERE
				(project_id, bucket_name, object_key, version) > ($1, $2, $3, $4)
				AND ` + opts.stopCondition() + `
				AND status = ` + statusPending + `
				AND (expires_at IS NULL OR expires_at > now())
			ORDER BY ` + opts.orderBy() + `
			LIMIT $6
		`

	case !opts.Pending && !opts.AllVersions:
		// The following subquery for the highest-version can also be implemented via
		//     SELECT MAX(sub.version) FROM objects ...
		// however, that seems slower on benchmarks.

		// query committed objects where the latest is not a delete marker
		return `SELECT ` + indexFields + opts.selectedFields() + `
			FROM objects main
			WHERE
				(project_id, bucket_name, object_key, version) > ($1, $2, $3, $4)
				AND ` + opts.stopCondition() + `
				AND status IN ` + statusesCommitted + `
				AND (expires_at IS NULL OR expires_at > now())
				AND version = (
					SELECT sub.version
					FROM objects sub
					WHERE
						(sub.project_id, sub.bucket_name, sub.object_key) = (main.project_id, main.bucket_name, main.object_key)
						AND status <> ` + statusPending + `
						AND (expires_at IS NULL OR expires_at > now())
					ORDER BY version DESC
					LIMIT 1
				)
			ORDER BY ` + opts.orderBy() + `
			LIMIT $6
		`
	default:
		panic("Not supported configuration, should not happen. Verify should check this.")
	}
}

func (opts *ListObjects) stopKey() []byte {
	if opts.Prefix != "" {
		return []byte(prefixLimit(opts.Prefix))
	}
	return nextBucket([]byte(opts.BucketName))
}

func (opts *ListObjects) stopCondition() string {
	if opts.Prefix != "" {
		return "(project_id, bucket_name, object_key) < ($1, $2, $5)"
	}
	return "(project_id, bucket_name) < ($1, $5)"
}

func (opts *ListObjects) orderBy() string {
	if opts.Pending {
		return "entry_key ASC, entry_version ASC"
	} else {
		return "entry_key ASC, entry_version DESC"
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

// startKey determines what should be the starting key for the given options.
// in the recursive case, or if the cursor key is not in the specified prefix,
// we start at the greatest key between cursor and prefix.
// Otherwise (non-recursive), we start at the prefix after the one in the cursor.
func (opts *ListObjects) startKey() ObjectKey {
	if opts.Prefix == "" && opts.Cursor.Key == "" {
		return ""
	}
	if opts.Recursive || !strings.HasPrefix(string(opts.Cursor.Key), string(opts.Prefix)) {
		if lessKey(opts.Cursor.Key, opts.Prefix) {
			return opts.Prefix
		}
		return opts.Cursor.Key
	}

	// in the recursive case
	// prefix | cursor | startKey
	// a/b/   | a/b/c/d/e | c/d/[0xff] (the first prefix/object key we return )
	key := opts.Cursor.Key
	prefixSize := len(opts.Prefix)
	subPrefix := key[prefixSize:] // c/d/e

	firstDelimiter := strings.Index(string(subPrefix), string(Delimiter))
	if firstDelimiter == -1 {
		return key
	}
	newKey := []byte(key[:prefixSize+firstDelimiter+1]) // c/d/
	newKey = append(newKey, 0xff)
	return ObjectKey(newKey)
}

func scanListObjectsResult(rows tagsql.Rows, opts ListObjects) (entries []ObjectEntry, err error) {

	for rows.Next() {
		var item ObjectEntry

		fields := []interface{}{
			&item.ObjectKey,
			&item.Version,
			&item.IsPrefix,
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
			return entries, err
		}

		if item.IsPrefix {
			item = ObjectEntry{
				IsPrefix:  true,
				ObjectKey: item.ObjectKey,
				Status:    Prefix,
			}
		}

		entries = append(entries, item)
	}

	return entries, nil
}
