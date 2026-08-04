// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/dx"
	"storj.io/storj/shared/tagsql"
)

type precommitTransactionAdapter interface {
	precommitQuery(ctx context.Context, params PrecommitQuery) (*PrecommitInfo, error)
}

type commitMetrics struct {
	// DeletedObjectCount returns how many objects were deleted.
	DeletedObjectCount int
	// DeletedSegmentCount returns how many segments were deleted.
	DeletedSegmentCount int
}

func (r *commitMetrics) submit() {
	mon.Meter("object_delete").Mark(r.DeletedObjectCount)
	mon.Meter("segment_delete").Mark(r.DeletedSegmentCount)
}

// ExcludeFromPending contains fields to exclude from the pending object.
type ExcludeFromPending struct {
	// ExpiresAt indicates whether the expires_at field should be excluded from read
	// We want to exclude it during object commit where we know expiration value but
	// don't want to exclude it for copy/move operations.
	ExpiresAt bool
	// EncryptedUserData indicates whether encrypted user data fields should be excluded from read.
	// We want to exclude it during object commit when data is provided explicitly but
	// don't want to exclude it for copy/move operations.
	EncryptedUserData bool
}

// PrecommitQuery is used for querying precommit info.
type PrecommitQuery struct {
	ObjectStream
	// Pending returns the pending object and segments at the location. Precommit returns an error when it does not exist.
	Pending bool
	// ExcludeFromPending contains fields to exclude from the pending object.
	ExcludeFromPending ExcludeFromPending
	// Unversioned returns the unversioned object at the location.
	Unversioned bool
	// FullUnversioned returns all properties of the unversioned object at the location.
	FullUnversioned bool
	// HighestVisible returns the highest committed object or delete marker at the location.
	HighestVisible bool
}

// PrecommitInfo is the information necessary for committing objects.
type PrecommitInfo struct {
	ObjectStream

	// TimestampVersion is used for timestamp versioning.
	//
	// This is used when timestamp versioning is enabled and we need to change version.
	// We request it from the database to have a consistent source of time.
	TimestampVersion Version
	// HighestVersion is the highest object version in the database.
	//
	// This is needed to determine whether the current pending object is the
	// latest and we can avoid changing the primary key. If it's not the newest
	// we can use it to generate the new version, when not using timestamp versioning.
	HighestVersion Version
	// Pending contains all the fields for the object to be committed.
	// This is used to reinsert the object when primary key cannot be changed.
	//
	// Encrypted fields are also necessary to verify when updating encrypted metadata.
	//
	// TODO: the amount of data transferred can probably reduced by doing a conditional
	// query.
	Pending *PrecommitPendingObject
	// Segments contains all the segments for the given object.
	Segments []PrecommitSegment
	// HighestVisible returns the status of the highest version that's either committed
	// or a delete marker.
	//
	// This is used to handle "IfNoneMatch" query. We need to know whether
	// the we consider the object to exist or not.
	HighestVisible ObjectStatus
	// Unversioned is the unversioned object at the given location. It is
	// returned when params.Unversioned or params.FullUnversioned is true.
	//
	// This is used to delete the previous unversioned object at the location,
	// which ensures that there's only one unversioned object at a given location.
	Unversioned *PrecommitUnversionedObject

	// FullUnversioned is the unversioned object at the given location.
	// It is returned when params.FullUnversioned is true.
	FullUnversioned *RawObject
}

// PrecommitUnversionedObject is information necessary to delete unversioned object
// at a given location.
type PrecommitUnversionedObject struct {
	Version            Version
	StreamID           uuid.UUID
	RetentionMode      RetentionMode
	RetainUntil        sql.NullTime
	CreatedAt          time.Time
	Status             ObjectStatus
	TotalEncryptedSize int64
}

// PrecommitUnversionedObjectFromObject creates a unversioned object from raw object.
func PrecommitUnversionedObjectFromObject(obj *RawObject) *PrecommitUnversionedObject {
	return &PrecommitUnversionedObject{
		Version:  obj.Version,
		StreamID: obj.StreamID,
		RetentionMode: RetentionMode{
			Mode:      obj.Retention.Mode,
			LegalHold: obj.LegalHold,
		},
		RetainUntil: sql.NullTime{
			Time:  obj.Retention.RetainUntil,
			Valid: !obj.Retention.RetainUntil.IsZero(),
		},
		CreatedAt:          obj.CreatedAt,
		Status:             obj.Status,
		TotalEncryptedSize: obj.TotalEncryptedSize,
	}
}

// PrecommitPendingObject is information about the object to be committed.
type PrecommitPendingObject struct {
	CreatedAt                     time.Time
	ExpiresAt                     *time.Time
	EncryptedMetadata             []byte
	EncryptedMetadataNonce        []byte
	EncryptedMetadataEncryptedKey []byte
	EncryptedETag                 []byte
	Checksum                      Checksum
	Encryption                    storj.EncryptionParameters
	RetentionMode                 RetentionMode
	RetainUntil                   sql.NullTime
}

// PrecommitQuery queries all information about the object so it can be committed.
func (db *DB) PrecommitQuery(ctx context.Context, opts PrecommitQuery, adapter precommitTransactionAdapter) (result *PrecommitInfo, err error) {
	return precommitQuery(ctx, opts, adapter)
}

func precommitQuery(ctx context.Context, opts PrecommitQuery, adapter precommitTransactionAdapter) (result *PrecommitInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.ObjectStream.Verify(); err != nil {
		return nil, Error.Wrap(err)
	}

	return adapter.precommitQuery(ctx, opts)
}

func (ptx *postgresTransactionAdapter) precommitQuery(ctx context.Context, opts PrecommitQuery) (*PrecommitInfo, error) {
	var info PrecommitInfo
	info.ObjectStream = opts.ObjectStream

	// database timestamp
	{
		err := ptx.tx.QueryRowContext(ctx, "SELECT "+postgresGenerateTimestampVersion).Scan(&info.TimestampVersion)
		if err != nil {
			return nil, Error.Wrap(err)
		}
	}

	// highest version
	{
		err := ptx.tx.QueryRowContext(ctx, `
			SELECT version
			FROM objects
			WHERE (project_id, bucket_name, object_key) = ($1, $2, $3)
				AND version > 0
			ORDER BY version DESC
			LIMIT 1
		`, opts.ProjectID, opts.BucketName, opts.ObjectKey).Scan(&info.HighestVersion)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, Error.Wrap(err)
		}
	}

	// pending object
	if opts.Pending {
		var pending PrecommitPendingObject
		values := []any{
			&pending.CreatedAt,
			&pending.Encryption, &pending.RetentionMode, &pending.RetainUntil,
		}

		additionalColumns := ""
		if !opts.ExcludeFromPending.ExpiresAt {
			additionalColumns = ", expires_at"

			values = append(values, &pending.ExpiresAt)
		}
		if !opts.ExcludeFromPending.EncryptedUserData {
			additionalColumns += `,
				encrypted_metadata,
				encrypted_metadata_nonce,
				encrypted_metadata_encrypted_key,
				encrypted_etag,
				checksum`

			values = append(values,
				&pending.EncryptedMetadata,
				&pending.EncryptedMetadataNonce,
				&pending.EncryptedMetadataEncryptedKey,
				&pending.EncryptedETag,
				&pending.Checksum,
			)
		}

		err := ptx.tx.QueryRowContext(ctx, `
			SELECT created_at,
				encryption,
				retention_mode,
				retain_until
				`+additionalColumns+`
			FROM objects
			WHERE (project_id, bucket_name, object_key, version) = ($1, $2, $3, $4)
				AND stream_id = $5
				AND status = `+statusPending+`
			ORDER BY version DESC
			LIMIT 1
		`, opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.Version, opts.StreamID).
			Scan(values...)
		if errors.Is(err, sql.ErrNoRows) {
			// TODO: should we return different error when the object is already committed?
			return nil, ErrObjectNotFound.Wrap(Error.New("object with specified version and pending status is missing"))
		}
		if err != nil {
			return nil, Error.Wrap(err)
		}

		info.Pending = &pending
	}

	// segments - query segments regardless of whether pending object was queried or excluded
	if opts.Pending {
		err := withRows(ptx.tx.QueryContext(ctx, `
			SELECT position, encrypted_size, plain_offset, plain_size
			FROM segments
			WHERE stream_id = $1
			ORDER BY position
		`, opts.StreamID))(func(rows tagsql.Rows) error {
			info.Segments = []PrecommitSegment{}
			for rows.Next() {
				var segment PrecommitSegment
				if err := rows.Scan(&segment.Position, &segment.EncryptedSize, &segment.PlainOffset, &segment.PlainSize); err != nil {
					return Error.Wrap(err)
				}
				info.Segments = append(info.Segments, segment)
			}
			return nil
		})
		if err != nil {
			return nil, Error.Wrap(err)
		}
	}

	// highest visible
	if opts.HighestVisible {
		err := ptx.tx.QueryRowContext(ctx, `
			SELECT status
			FROM objects
			WHERE (project_id, bucket_name, object_key) = ($1, $2, $3)
				AND version > 0
				AND status IN `+statusesVisible+`
			ORDER BY version DESC
			LIMIT 1
		`, opts.ProjectID, opts.BucketName, opts.ObjectKey).Scan(&info.HighestVisible)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, Error.Wrap(err)
		}
	}

	// unversioned
	if opts.FullUnversioned {
		err := withRows(ptx.tx.QueryContext(ctx, `
			SELECT `+postgresObjectColumns()+`
			FROM objects
			WHERE (project_id, bucket_name, object_key) = ($1, $2, $3)
				AND version > 0
				AND status IN `+statusesUnversioned+`
		`, opts.ProjectID, opts.BucketName, opts.ObjectKey))(func(rows tagsql.Rows) error {
			for rows.Next() {
				var unversioned RawObject
				if err := rows.Scan(postgresObjectScan(&unversioned)...); err != nil {
					return Error.Wrap(err)
				}
				if info.FullUnversioned != nil {
					logMultipleCommittedVersionsError(ptx.postgresAdapter.log, opts.ObjectStream.Location())
					return Error.New(multipleCommittedVersionsErrMsg)
				}
				info.FullUnversioned = &unversioned
				info.Unversioned = PrecommitUnversionedObjectFromObject(&unversioned)
			}
			return nil
		})
		if err != nil {
			return nil, Error.Wrap(err)
		}
	} else if opts.Unversioned {
		err := withRows(ptx.tx.QueryContext(ctx, `
			SELECT version, stream_id, retention_mode, retain_until,
				created_at, status, total_encrypted_size
			FROM objects
			WHERE (project_id, bucket_name, object_key) = ($1, $2, $3)
				AND version > 0
				AND status IN `+statusesUnversioned+`
		`, opts.ProjectID, opts.BucketName, opts.ObjectKey))(func(rows tagsql.Rows) error {
			for rows.Next() {
				var unversioned PrecommitUnversionedObject
				if err := rows.Scan(&unversioned.Version, &unversioned.StreamID, &unversioned.RetentionMode, &unversioned.RetainUntil,
					&unversioned.CreatedAt, &unversioned.Status, &unversioned.TotalEncryptedSize); err != nil {
					return Error.Wrap(err)
				}
				if info.Unversioned != nil {
					logMultipleCommittedVersionsError(ptx.postgresAdapter.log, opts.ObjectStream.Location())
					return Error.New(multipleCommittedVersionsErrMsg)
				}
				info.Unversioned = &unversioned
			}
			return nil
		})
		if err != nil {
			return nil, Error.Wrap(err)
		}
	}

	return &info, nil
}

func (tx *tidbTransactionAdapter) precommitQuery(ctx context.Context, opts PrecommitQuery) (_ *PrecommitInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	var info PrecommitInfo
	info.ObjectStream = opts.ObjectStream

	queryTimestamp := dx.Query{
		Statement: `SELECT ` + tidbGenerateTimestampVersion,
		Do:        dx.ScanRow(&info.TimestampVersion),
	}

	// FOR UPDATE serializes concurrent commits to the same object location:
	// without it two transactions can read the same highest version, both
	// derive the same next version and the loser fails at COMMIT with a
	// non-retryable duplicate primary key error. Locking the highest row makes
	// the second transaction wait and re-read after the first one commits.
	// TiDB pessimistic transactions take no gap locks, so this does not
	// serialize commits to a location that has no rows yet.
	queryHighest := dx.Query{
		Statement: `
			SELECT version
			FROM objects
			WHERE (project_id, bucket_name, object_key) = (?, ?, ?)
				AND version > 0
			ORDER BY version DESC
			LIMIT 1
			FOR UPDATE`,
		Args: []any{opts.ProjectID, opts.BucketName, opts.ObjectKey},
		Do:   dx.ScanRowOptional(&info.HighestVersion),
	}

	var pending PrecommitPendingObject
	var queryPending, querySegments dx.Query
	if opts.Pending {
		pendingValues := []any{
			&pending.CreatedAt,
			&pending.Encryption,
			&pending.RetentionMode,
			&pending.RetainUntil,
		}

		additionalColumns := ""
		if !opts.ExcludeFromPending.ExpiresAt {
			additionalColumns = ", expires_at"
			pendingValues = append(pendingValues, &pending.ExpiresAt)
		}
		if !opts.ExcludeFromPending.EncryptedUserData {
			additionalColumns += `,
				encrypted_metadata,
				encrypted_metadata_nonce,
				encrypted_metadata_encrypted_key,
				encrypted_etag,
				checksum`
			pendingValues = append(pendingValues,
				&pending.EncryptedMetadata,
				&pending.EncryptedMetadataNonce,
				&pending.EncryptedMetadataEncryptedKey,
				&pending.EncryptedETag,
				&pending.Checksum,
			)
		}

		queryPending = dx.Query{
			Statement: `
				SELECT created_at,
					encryption,
					retention_mode,
					retain_until
					` + additionalColumns + `
				FROM objects
				WHERE (project_id, bucket_name, object_key, version, stream_id) = (?, ?, ?, ?, ?)
					AND status = ` + statusPending + `
				ORDER BY version DESC
				LIMIT 1`,
			Args: []any{opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.Version, opts.StreamID},
			Do: func(rows dx.Rows) error {
				err := dx.ScanRow(pendingValues...)(rows)
				if errors.Is(err, sql.ErrNoRows) {
					// TODO: should we return different error when the object is already committed?
					return ErrObjectNotFound.Wrap(Error.New("object with specified version and pending status is missing"))
				}
				return err
			},
		}

		querySegments = dx.Query{
			Statement: `
				SELECT position, encrypted_size, plain_offset, plain_size
				FROM segments
				WHERE stream_id = ?
				ORDER BY position`,
			Args: []any{opts.StreamID},
			Do: func(rows dx.Rows) error {
				info.Segments = []PrecommitSegment{}
				for rows.Next() {
					var segment PrecommitSegment
					if err := rows.Scan(&segment.Position, &segment.EncryptedSize, &segment.PlainOffset, &segment.PlainSize); err != nil {
						return Error.Wrap(err)
					}
					info.Segments = append(info.Segments, segment)
				}
				return nil
			},
		}
	}

	var queryHighestVisible dx.Query
	if opts.HighestVisible {
		queryHighestVisible = dx.Query{
			Statement: `
				SELECT status
				FROM objects
				WHERE (project_id, bucket_name, object_key) = (?, ?, ?)
					AND version > 0
					AND status IN ` + statusesVisible + `
				ORDER BY version DESC
				LIMIT 1`,
			Args: []any{opts.ProjectID, opts.BucketName, opts.ObjectKey},
			Do:   dx.ScanRowOptional(&info.HighestVisible),
		}
	}

	var queryUnversioned dx.Query
	if opts.FullUnversioned {
		queryUnversioned = dx.Query{
			Statement: `
				SELECT ` + postgresObjectColumns() + `
				FROM objects
				WHERE (project_id, bucket_name, object_key) = (?, ?, ?)
					AND version > 0
					AND status IN ` + statusesUnversioned + `
				FOR UPDATE`,
			Args: []any{opts.ProjectID, opts.BucketName, opts.ObjectKey},
			Do: func(rows dx.Rows) error {
				for rows.Next() {
					var unversioned RawObject
					if err := rows.Scan(postgresObjectScan(&unversioned)...); err != nil {
						return Error.Wrap(err)
					}
					if info.FullUnversioned != nil {
						logMultipleCommittedVersionsError(tx.tidbAdapter.log, opts.ObjectStream.Location())
						return Error.New(multipleCommittedVersionsErrMsg)
					}
					info.FullUnversioned = &unversioned
					info.Unversioned = PrecommitUnversionedObjectFromObject(&unversioned)
				}
				return nil
			},
		}
	} else if opts.Unversioned {
		queryUnversioned = dx.Query{
			Statement: `
				SELECT version, stream_id, retention_mode, retain_until,
					created_at, status, total_encrypted_size
				FROM objects
				WHERE (project_id, bucket_name, object_key) = (?, ?, ?)
					AND version > 0
					AND status IN ` + statusesUnversioned + `
				FOR UPDATE`,
			Args: []any{opts.ProjectID, opts.BucketName, opts.ObjectKey},
			Do: func(rows dx.Rows) error {
				for rows.Next() {
					var unversioned PrecommitUnversionedObject
					if err := rows.Scan(
						&unversioned.Version, &unversioned.StreamID,
						&unversioned.RetentionMode, &unversioned.RetainUntil,
						&unversioned.CreatedAt, &unversioned.Status, &unversioned.TotalEncryptedSize,
					); err != nil {
						return Error.Wrap(err)
					}
					if info.Unversioned != nil {
						logMultipleCommittedVersionsError(tx.tidbAdapter.log, opts.ObjectStream.Location())
						return Error.New(multipleCommittedVersionsErrMsg)
					}
					info.Unversioned = &unversioned
				}
				return nil
			},
		}
	}

	err = dx.Do(ctx, tx.tx,
		queryTimestamp,
		queryHighest,
		queryPending,
		querySegments,
		queryHighestVisible,
		queryUnversioned,
	)
	if err != nil {
		return nil, err
	}

	if opts.Pending {
		info.Pending = &pending
	}
	return &info, nil
}
