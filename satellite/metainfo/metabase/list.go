// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"

	"storj.io/common/uuid"
	"storj.io/storj/private/tagsql"
)

// ListBucket contains arguments necessary for listing a bucket
// about exact object version.
type ListBucket struct {
	ProjectID  uuid.UUID
	BucketName string
	Recursive  bool
	limit      int
}

type ListBucketItem Object

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

//Iterator iterates.
type Iterator interface {
	Next(ctx context.Context, item *ListBucketItem) bool
}

//ListBucketIterator enables iteration on objects in a bucket.
type ListBucketIterator struct {
	ProjectID  uuid.UUID
	BucketName string
	db         *DB
	curRows    tagsql.Rows
	curIndex   int
}

//Iterate iterates
func (db *DB) Iterate(ctx context.Context, opts ListBucket, fn func(context.Context, Iterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err = opts.Verify(); err != nil {
		return err
	}
	it := &ListBucketIterator{db: db,
		ProjectID:  opts.ProjectID,
		BucketName: opts.BucketName}
	it.curRows, err = it.doNextQuery(ctx)

	if err != nil {
		return err
	}
	return fn(ctx, it)
}

//Next returns next object
func (it *ListBucketIterator) Next(ctx context.Context, item *ListBucketItem) bool {
	next := it.curRows.Next()
	if next {
		err := it.curRows.Scan(
			&item.StreamID, &item.ObjectKey, &item.Version,
			&item.CreatedAt, &item.ExpiresAt,
			&item.Status, &item.SegmentCount,
			&item.EncryptedMetadataNonce, &item.EncryptedMetadata,
			&item.TotalEncryptedSize, &item.FixedSegmentSize,
			encryptionParameters{&item.Encryption},
		)
		if err != nil {
			return false
		}
		return true
	}
	return false
}

func (it *ListBucketIterator) doNextQuery(ctx context.Context) (_ tagsql.Rows, err error) {
	defer mon.Task()(&ctx)(&err)

	return it.db.db.Query(ctx, `
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
		`, it.ProjectID, it.BucketName)
}
