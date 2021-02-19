// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/private/tagsql"
)

const fullIteratorBatchSizeLimit = 2500

// FullObjectEntry contains information about and object in metabase.
type FullObjectEntry struct {
	ObjectStream

	CreatedAt time.Time
	ExpiresAt *time.Time

	Status       ObjectStatus
	SegmentCount int32

	EncryptedMetadataNonce        []byte
	EncryptedMetadata             []byte
	EncryptedMetadataEncryptedKey []byte

	TotalPlainSize     int64
	TotalEncryptedSize int64
	FixedSegmentSize   int32

	Encryption storj.EncryptionParameters

	// ZombieDeletionDeadline defines when the pending raw object should be deleted from the database.
	// This is as a safeguard against objects that failed to upload and the client has not indicated
	// whether they want to continue uploading or delete the already uploaded data.
	ZombieDeletionDeadline *time.Time
}

// FullObjectsIterator iterates over a sequence of FullObjectEntry items.
type FullObjectsIterator interface {
	Next(ctx context.Context, item *FullObjectEntry) bool
}

// FullIterateObjects contains arguments necessary for listing objects in metabase.
type FullIterateObjects struct {
	BatchSize int
}

// Verify verifies get object request fields.
func (opts *FullIterateObjects) Verify() error {
	if opts.BatchSize < 0 {
		return ErrInvalidRequest.New("BatchSize is negative")
	}
	return nil
}

// FullIterateObjects iterates through all objects in metabase.
func (db *DB) FullIterateObjects(ctx context.Context, opts FullIterateObjects, fn func(context.Context, FullObjectsIterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return err
	}

	it := &fullIterator{
		db: db,

		batchSize: opts.BatchSize,

		curIndex: 0,
		cursor:   fullIterateCursor{},
	}

	// ensure batch size is reasonable
	if it.batchSize <= 0 || it.batchSize > fullIteratorBatchSizeLimit {
		it.batchSize = fullIteratorBatchSizeLimit
	}

	it.curRows, err = it.doNextQuery(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if rowsErr := it.curRows.Err(); rowsErr != nil {
			err = errs.Combine(err, rowsErr)
		}
		err = errs.Combine(err, it.curRows.Close())
	}()

	return fn(ctx, it)
}

// fullIterator enables iteration of all objects in metabase.
type fullIterator struct {
	db *DB

	batchSize int

	curIndex int
	curRows  tagsql.Rows
	cursor   fullIterateCursor
}

type fullIterateCursor struct {
	ProjectID  uuid.UUID
	BucketName string
	ObjectKey  ObjectKey
	Version    Version
}

// Next returns true if there was another item and copy it in item.
func (it *fullIterator) Next(ctx context.Context, item *FullObjectEntry) bool {
	next := it.curRows.Next()
	if !next {
		if it.curIndex < it.batchSize {
			return false
		}

		if it.curRows.Err() != nil {
			return false
		}

		rows, err := it.doNextQuery(ctx)
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
	it.cursor.ProjectID = item.ProjectID
	it.cursor.BucketName = item.BucketName
	it.cursor.ObjectKey = item.ObjectKey
	it.cursor.Version = item.Version

	return true
}

func (it *fullIterator) doNextQuery(ctx context.Context) (_ tagsql.Rows, err error) {
	defer mon.Task()(&ctx)(&err)

	return it.db.db.Query(ctx, `
		SELECT
			project_id, bucket_name,
			object_key, stream_id, version, status,
			created_at, expires_at,
			segment_count,
			encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
			total_plain_size, total_encrypted_size, fixed_segment_size,
			encryption
		FROM objects
		WHERE (project_id, bucket_name, object_key, version) > ($1, $2, $3, $4)
		ORDER BY project_id ASC, bucket_name ASC, object_key ASC, version ASC
		LIMIT $5
		`, it.cursor.ProjectID, it.cursor.BucketName,
		[]byte(it.cursor.ObjectKey), int(it.cursor.Version),
		it.batchSize,
	)
}

// scanItem scans doNextQuery results into FullObjectEntry.
func (it *fullIterator) scanItem(item *FullObjectEntry) error {
	return it.curRows.Scan(
		&item.ProjectID, &item.BucketName,
		&item.ObjectKey, &item.StreamID, &item.Version, &item.Status,
		&item.CreatedAt, &item.ExpiresAt,
		&item.SegmentCount,
		&item.EncryptedMetadataNonce, &item.EncryptedMetadata, &item.EncryptedMetadataEncryptedKey,
		&item.TotalPlainSize, &item.TotalEncryptedSize, &item.FixedSegmentSize,
		encryptionParameters{&item.Encryption},
	)
}
