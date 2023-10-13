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
type ListObjects struct {
	ProjectID             uuid.UUID
	BucketName            string
	Recursive             bool
	Limit                 int
	Prefix                ObjectKey
	Cursor                ListObjectsCursor
	Status                ObjectStatus
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
	case !(opts.Status == Pending || opts.Status == CommittedUnversioned):
		return ErrInvalidRequest.New("Status is invalid")
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
		opts.ProjectID, []byte(opts.BucketName), opts.startKey(), opts.Cursor.Version,
		opts.stopKey(), opts.Status,
		opts.Limit+1, len(opts.Prefix)+1))(func(rows tagsql.Rows) error {
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
	return `
	SELECT ` + opts.selectedFields() + `
	FROM objects
	WHERE
		(project_id, bucket_name, object_key, version) > ($1, $2, $3, $4)
		AND ` + opts.stopCondition() + `
		AND status = $6
		AND (expires_at IS NULL OR expires_at > now())
	ORDER BY ` + opts.orderBy() + `
	LIMIT $7
	`
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
	if !opts.Recursive {
		return "entry_key ASC"
	}

	return "(object_key, version) ASC"
}

func (opts ListObjects) selectedFields() (selectedFields string) {

	if opts.Recursive {
		selectedFields = `
			substring(object_key from $8), FALSE as is_prefix`
	} else {
		selectedFields = `
			DISTINCT ON (entry_key)
			CASE
				WHEN position('/' IN substring(object_key from $8)) <> 0
				THEN substring(substring(object_key from $8) from 0 for (position('/' IN substring(object_key from $8)) +1))
				ELSE substring(object_key from $8)
			END
			AS entry_key,
			position('/' IN substring(object_key from $8)) <> 0 AS is_prefix`
	}

	selectedFields += `
	,stream_id
	,version
	,encryption`

	if opts.IncludeSystemMetadata {
		selectedFields += `
		,status
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
			&item.IsPrefix,
			&item.StreamID,
			&item.Version,
			encryptionParameters{&item.Encryption},
		}

		if opts.IncludeSystemMetadata {
			fields = append(fields,
				&item.Status,
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
				Status:    opts.Status,
			}
		}

		entries = append(entries, item)
	}

	return entries, nil
}
