// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"errors"

	"github.com/zeebo/errs"
	"storj.io/common/uuid"
)

// ListBucket contains arguments necessary for listing a bucket
// about exact object version.
type ListBucket struct {
	ProjectID  uuid.UUID
	BucketName string
	Recursive  bool
}

type ListBucketResult struct {
	Objects []Object
}

// Verify verifies get object reqest fields.
func (opts *ListBucket) Verify() error {
	if opts.BucketName == "" {
		return ErrInvalidRequest.New("BucketName missing")
	}
	if opts.ProjectID.IsZero() {
		return ErrInvalidRequest.New("ProjectID missing")
	}
	// TODO: check BucketName is valid
	// TODO: check projectID exist?
	// TODO: check BucketName for projectID exists
	return nil
}

// ListBucket returns a list of objects within the bucket.
func (db *DB) ListBucket(ctx context.Context, opts ListBucket) (result ListBucketResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return ListBucketResult{}, err
	}

	objects := []Object{}
	// TODO handle encryption column
	rows, err := db.db.Query(ctx, `
		SELECT
			stream_id, object_key, version,
			created_at, expires_at,
			status, segment_count,
			encrypted_metadata_nonce, encrypted_metadata,
			total_encrypted_size, fixed_segment_size,
			encryption
		FROM objects
		WHERE
			project_id  = $1 AND
			bucket_name = $2 -- what about status?
		`, opts.ProjectID, opts.BucketName)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return result, nil
		}
		return ListBucketResult{}, Error.New("list bucket query: %w", err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	for rows.Next() {
		var object Object
		err := rows.Scan(
			&object.StreamID, &object.ObjectKey, &object.Version,
			&object.CreatedAt, &object.ExpiresAt,
			&object.Status, &object.SegmentCount,
			&object.EncryptedMetadataNonce, &object.EncryptedMetadata,
			&object.TotalEncryptedSize, &object.FixedSegmentSize,
			encryptionParameters{&object.Encryption},
		)
		if err != nil {
			return ListBucketResult{}, Error.New("ListBucket scan failed: %w", err)
		}
		object.ProjectID = opts.ProjectID
		object.BucketName = opts.BucketName
		objects = append(objects, object)
	}
	if err := rows.Err(); err != nil {
		return ListBucketResult{}, Error.New("ListBucket scan failed: %w", err)
	}
	result.Objects = objects
	return result, nil
}
