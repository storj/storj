// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"cloud.google.com/go/spanner"

	"storj.io/common/storj"
	"storj.io/common/uuid"
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
	// Object indicates whether the entire object should be excluded from read.
	// We want to exclude it during segment commit where pending object was not
	// created at the beginning of upload.
	// Segments are not excluded in this case.
	Object bool
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
	Version       Version          `spanner:"version"`
	StreamID      uuid.UUID        `spanner:"stream_id"`
	RetentionMode RetentionMode    `spanner:"retention_mode"`
	RetainUntil   spanner.NullTime `spanner:"retain_until"`
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
		RetainUntil: spanner.NullTime{
			Time:  obj.Retention.RetainUntil,
			Valid: !obj.Retention.RetainUntil.IsZero(),
		},
	}
}

// PrecommitPendingObject is information about the object to be committed.
type PrecommitPendingObject struct {
	CreatedAt                     time.Time                  `spanner:"created_at"`
	ExpiresAt                     *time.Time                 `spanner:"expires_at"`
	EncryptedMetadata             []byte                     `spanner:"encrypted_metadata"`
	EncryptedMetadataNonce        []byte                     `spanner:"encrypted_metadata_nonce"`
	EncryptedMetadataEncryptedKey []byte                     `spanner:"encrypted_metadata_encrypted_key"`
	EncryptedETag                 []byte                     `spanner:"encrypted_etag"`
	Encryption                    storj.EncryptionParameters `spanner:"encryption"`
	RetentionMode                 RetentionMode              `spanner:"retention_mode"`
	RetainUntil                   spanner.NullTime           `spanner:"retain_until"`
}

// PrecommitQuery queries all information about the object so it can be committed.
func (db *DB) PrecommitQuery(ctx context.Context, opts PrecommitQuery, adapter precommitTransactionAdapter) (result *PrecommitInfo, err error) {
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
	if opts.Pending && !opts.ExcludeFromPending.Object {
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
			additionalColumns += ", encrypted_metadata, encrypted_metadata_nonce, encrypted_metadata_encrypted_key, encrypted_etag"

			values = append(values, &pending.EncryptedMetadata, &pending.EncryptedMetadataNonce, &pending.EncryptedMetadataEncryptedKey, &pending.EncryptedETag)
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
			SELECT version, stream_id, retention_mode, retain_until
			FROM objects
			WHERE (project_id, bucket_name, object_key) = ($1, $2, $3)
				AND version > 0
				AND status IN `+statusesUnversioned+`
		`, opts.ProjectID, opts.BucketName, opts.ObjectKey))(func(rows tagsql.Rows) error {
			for rows.Next() {
				var unversioned PrecommitUnversionedObject
				if err := rows.Scan(&unversioned.Version, &unversioned.StreamID, &unversioned.RetentionMode, &unversioned.RetainUntil); err != nil {
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

func (stx *spannerTransactionAdapter) precommitQuery(ctx context.Context, opts PrecommitQuery) (_ *PrecommitInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	stmt := spanner.Statement{
		SQL: `WITH objects_at_location AS (
			SELECT version, stream_id,
				status,
				retention_mode, retain_until
			FROM objects
			WHERE (project_id, bucket_name, object_key) = (@project_id, @bucket_name, @object_key)
				AND version > 0
		) SELECT
			(` + spannerGenerateTimestampVersion + `),
			(SELECT version FROM objects_at_location  ORDER BY version DESC LIMIT 1)
		`,
		Params: map[string]any{
			"project_id":  opts.ProjectID,
			"bucket_name": opts.BucketName,
			"object_key":  opts.ObjectKey,
		},
	}

	if opts.Pending {
		additionalColumns := ""
		if !opts.ExcludeFromPending.ExpiresAt {
			additionalColumns += ", expires_at"
		}
		if !opts.ExcludeFromPending.EncryptedUserData {
			additionalColumns += ", encrypted_metadata, encrypted_metadata_nonce, encrypted_metadata_encrypted_key, encrypted_etag"
		}

		if !opts.ExcludeFromPending.Object {
			stmt.SQL += `
				,(SELECT ARRAY(
					SELECT AS STRUCT
						created_at,
						encryption,
						retention_mode,
						retain_until
						` + additionalColumns + `
					FROM objects
					WHERE (project_id, bucket_name, object_key, version) = (@project_id, @bucket_name, @object_key, @version)
						AND stream_id = @stream_id
						AND status = ` + statusPending + `
				))				`
			stmt.Params["version"] = opts.Version
		}

		stmt.SQL += `
			,(SELECT ARRAY(
					SELECT AS STRUCT position, encrypted_size, plain_offset, plain_size
					FROM segments
					WHERE stream_id = @stream_id
					ORDER BY position
			))`
		stmt.Params["stream_id"] = opts.StreamID
	}

	if opts.HighestVisible {
		stmt.SQL += `,(SELECT status
				FROM objects_at_location
				WHERE status IN ` + statusesVisible + `
				ORDER BY version DESC
				LIMIT 1
			)`
	}

	if opts.FullUnversioned {
		stmt.SQL += `,(SELECT ARRAY(
				SELECT AS STRUCT ` + spannerObjectColumns() + `
				FROM objects
				WHERE (project_id, bucket_name, object_key) = (@project_id, @bucket_name, @object_key)
					AND status IN ` + statusesUnversioned + `
					AND version > 0
			))`
	} else if opts.Unversioned {
		stmt.SQL += `,(SELECT ARRAY(
				SELECT AS STRUCT version, stream_id, retention_mode, retain_until
				FROM objects_at_location
				WHERE status IN ` + statusesUnversioned + `
			))`
	}

	var result PrecommitInfo
	result.ObjectStream = opts.ObjectStream

	err = stx.tx.QueryWithOptions(ctx, stmt, spanner.QueryOptions{
		RequestTag: `precommit-query`,
	}).Do(func(row *spanner.Row) error {
		if err := row.Column(0, &result.TimestampVersion); err != nil {
			return Error.Wrap(err)
		}

		var highestVersion *int64
		if err := row.Column(1, &highestVersion); err != nil {
			return Error.Wrap(err)
		}
		if highestVersion != nil {
			result.HighestVersion = Version(*highestVersion)
		}

		column := 2
		if opts.Pending {
			if !opts.ExcludeFromPending.Object {
				var pending []*PrecommitPendingObject
				if err := row.Column(column, &pending); err != nil {
					return Error.Wrap(err)
				}
				column++
				if len(pending) > 1 {
					return Error.New("internal error: multiple pending objects with the same key")
				}
				if len(pending) == 0 {
					// TODO: should we return different error when the object is already committed?
					return ErrObjectNotFound.Wrap(Error.New("object with specified version and pending status is missing"))
				}
				result.Pending = pending[0]
			}

			var segments []*struct {
				Position      SegmentPosition `spanner:"position"`
				EncryptedSize int64           `spanner:"encrypted_size"`
				PlainOffset   int64           `spanner:"plain_offset"`
				PlainSize     int64           `spanner:"plain_size"`
			}
			if err := row.Column(column, &segments); err != nil {
				return Error.Wrap(err)
			}
			column++
			result.Segments = make([]PrecommitSegment, len(segments))
			for i, v := range segments {
				if v == nil {
					return Error.New("internal error: null segment returned")
				}
				result.Segments[i] = PrecommitSegment{
					Position:      v.Position,
					EncryptedSize: int32(v.EncryptedSize),
					PlainOffset:   v.PlainOffset,
					PlainSize:     int32(v.PlainSize),
				}
			}
		}

		if opts.HighestVisible {
			var highestVisible *int64
			if err := row.Column(column, &highestVisible); err != nil {
				return Error.Wrap(err)
			}
			column++
			if highestVisible != nil {
				result.HighestVisible = ObjectStatus(*highestVisible)
			}
		}

		if opts.FullUnversioned {
			var unversioned []*precommitUnversionedObjectFull
			if err := row.Column(column, &unversioned); err != nil {
				return Error.Wrap(err)
			}

			if len(unversioned) > 1 {
				logMultipleCommittedVersionsError(stx.spannerAdapter.log, opts.Location())
				return Error.New(multipleCommittedVersionsErrMsg)
			}
			if len(unversioned) == 1 {
				result.FullUnversioned = unversioned[0].toRawObject()
				result.Unversioned = PrecommitUnversionedObjectFromObject(result.FullUnversioned)
			}
		} else if opts.Unversioned {
			var unversioned []*PrecommitUnversionedObject
			if err := row.Column(column, &unversioned); err != nil {
				return Error.Wrap(err)
			}

			if len(unversioned) > 1 {
				logMultipleCommittedVersionsError(stx.spannerAdapter.log, opts.Location())
				return Error.New(multipleCommittedVersionsErrMsg)
			}
			if len(unversioned) == 1 {
				result.Unversioned = unversioned[0]
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// precommitUnversionedObjectFull is used for scanning the result from struct as result.
//
// TODO: unify this with RawObject so we don't need separate type.
type precommitUnversionedObjectFull struct {
	ProjectID  uuid.UUID  `spanner:"project_id"`
	BucketName BucketName `spanner:"bucket_name"`
	ObjectKey  ObjectKey  `spanner:"object_key"`
	Version    Version    `spanner:"version"`
	StreamID   uuid.UUID  `spanner:"stream_id"`

	CreatedAt time.Time        `spanner:"created_at"`
	ExpiresAt spanner.NullTime `spanner:"expires_at"`

	Status       ObjectStatus `spanner:"status"`
	SegmentCount int64        `spanner:"segment_count"`

	EncryptedMetadata             []byte `spanner:"encrypted_metadata_nonce"`
	EncryptedMetadataNonce        []byte `spanner:"encrypted_metadata"`
	EncryptedMetadataEncryptedKey []byte `spanner:"encrypted_metadata_encrypted_key"`
	EncryptedETag                 []byte `spanner:"encrypted_etag"`

	TotalPlainSize     int64 `spanner:"total_plain_size"`
	TotalEncryptedSize int64 `spanner:"total_encrypted_size"`
	FixedSegmentSize   int64 `spanner:"fixed_segment_size"`

	Encryption             storj.EncryptionParameters `spanner:"encryption"`
	ZombieDeletionDeadline spanner.NullTime           `spanner:"zombie_deletion_deadline"`

	RetentionMode RetentionMode    `spanner:"retention_mode"`
	RetainUntil   spanner.NullTime `spanner:"retain_until"`
}

func (obj *precommitUnversionedObjectFull) toRawObject() *RawObject {
	return &RawObject{
		ObjectStream: ObjectStream{
			ProjectID:  obj.ProjectID,
			BucketName: obj.BucketName,
			ObjectKey:  obj.ObjectKey,
			Version:    obj.Version,
			StreamID:   obj.StreamID,
		},
		CreatedAt:    obj.CreatedAt,
		ExpiresAt:    asPtrTime(obj.ExpiresAt),
		Status:       obj.Status,
		SegmentCount: int32(obj.SegmentCount),
		EncryptedUserData: EncryptedUserData{
			EncryptedMetadata:             obj.EncryptedMetadata,
			EncryptedMetadataNonce:        obj.EncryptedMetadataNonce,
			EncryptedMetadataEncryptedKey: obj.EncryptedMetadataEncryptedKey,
			EncryptedETag:                 obj.EncryptedETag,
		},
		TotalPlainSize:         obj.TotalPlainSize,
		TotalEncryptedSize:     obj.TotalEncryptedSize,
		FixedSegmentSize:       int32(obj.FixedSegmentSize),
		Encryption:             obj.Encryption,
		ZombieDeletionDeadline: asPtrTime(obj.ZombieDeletionDeadline),

		Retention: Retention{
			Mode:        obj.RetentionMode.Mode,
			RetainUntil: obj.RetainUntil.Time,
		},
		LegalHold: obj.RetentionMode.LegalHold,
	}
}

func asPtrTime(v spanner.NullTime) *time.Time {
	if !v.Valid {
		return nil
	}
	return &v.Time
}
