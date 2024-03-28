// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"errors"

	"storj.io/common/tagsql"
)

// PostgresAdapter uses Cockroach related SQL queries.
type PostgresAdapter struct {
	db tagsql.DB
}

// TestingBeginObjectExactVersion implements Adapter.
func (p *PostgresAdapter) TestingBeginObjectExactVersion(ctx context.Context, opts BeginObjectExactVersion, object *Object) error {
	return p.db.QueryRowContext(ctx, `
		INSERT INTO objects (
			project_id, bucket_name, object_key, version, stream_id,
			expires_at, encryption,
			zombie_deletion_deadline,
			encrypted_metadata, encrypted_metadata_nonce, encrypted_metadata_encrypted_key
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7,
			$8,
			$9, $10, $11
		)
		RETURNING status, created_at
		`, opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey, opts.Version, opts.StreamID,
		opts.ExpiresAt, encryptionParameters{&opts.Encryption},
		opts.ZombieDeletionDeadline,
		opts.EncryptedMetadata, opts.EncryptedMetadataNonce, opts.EncryptedMetadataEncryptedKey,
	).Scan(
		&object.Status, &object.CreatedAt,
	)
}

// BeginObject implements Adapter.
func (p *PostgresAdapter) BeginObject(ctx context.Context, opts BeginObjectNextVersion, object *Object) error {
	return p.db.QueryRowContext(ctx, `
			INSERT INTO objects (
				project_id, bucket_name, object_key, version, stream_id,
				expires_at, encryption,
				zombie_deletion_deadline,
				encrypted_metadata, encrypted_metadata_nonce, encrypted_metadata_encrypted_key
			) VALUES (
				$1, $2, $3,
					coalesce((
						SELECT version + 1
						FROM objects
						WHERE (project_id, bucket_name, object_key) = ($1, $2, $3)
						ORDER BY version DESC
						LIMIT 1
					), 1),
				$4, $5, $6,
				$7,
				$8, $9, $10)
			RETURNING status, version, created_at
		`, opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey, opts.StreamID,
		opts.ExpiresAt, encryptionParameters{&opts.Encryption},
		opts.ZombieDeletionDeadline,
		opts.EncryptedMetadata, opts.EncryptedMetadataNonce, opts.EncryptedMetadataEncryptedKey,
	).Scan(&object.Status, &object.Version, &object.CreatedAt)
}

// GetObjectLastCommitted implements Adapter.
func (p *PostgresAdapter) GetObjectLastCommitted(ctx context.Context, opts GetObjectLastCommitted, object *Object) error {
	row := p.db.QueryRowContext(ctx, `
		SELECT
			stream_id, version, status,
			created_at, expires_at,
			segment_count,
			encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
			total_plain_size, total_encrypted_size, fixed_segment_size,
			encryption
		FROM objects
		WHERE
			(project_id, bucket_name, object_key) = ($1, $2, $3) AND
			status <> `+statusPending+` AND
			(expires_at IS NULL OR expires_at > now())
		ORDER BY version DESC
		LIMIT 1`,
		opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey)

	err := row.Scan(
		&object.StreamID, &object.Version, &object.Status,
		&object.CreatedAt, &object.ExpiresAt,
		&object.SegmentCount,
		&object.EncryptedMetadataNonce, &object.EncryptedMetadata, &object.EncryptedMetadataEncryptedKey,
		&object.TotalPlainSize, &object.TotalEncryptedSize, &object.FixedSegmentSize,
		encryptionParameters{&object.Encryption},
	)

	if errors.Is(err, sql.ErrNoRows) || object.Status.IsDeleteMarker() {
		return ErrObjectNotFound.Wrap(Error.Wrap(sql.ErrNoRows))
	}
	return nil
}

var _ Adapter = &PostgresAdapter{}
