// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"errors"

	"go.uber.org/zap"

	"storj.io/common/uuid"
)

type precommitTransactionAdapter interface {
	precommitQueryHighest(ctx context.Context, loc ObjectLocation) (highest Version, err error)
	precommitQueryHighestAndUnversioned(ctx context.Context, loc ObjectLocation) (highest Version, unversionedExists bool, err error)
	precommitDeleteUnversioned(ctx context.Context, loc ObjectLocation) (result PrecommitConstraintResult, err error)
}

// PrecommitConstraint is arguments to ensure that a single unversioned object or delete marker exists in the
// table per object location.
type PrecommitConstraint struct {
	Location ObjectLocation

	Versioned      bool
	DisallowDelete bool
}

// PrecommitConstraintResult returns the result of enforcing precommit constraint.
type PrecommitConstraintResult struct {
	Deleted []Object

	// DeletedObjectCount returns how many objects were deleted.
	DeletedObjectCount int
	// DeletedSegmentCount returns how many segments were deleted.
	DeletedSegmentCount int

	// HighestVersion returns tha highest version that was present in the table.
	// It returns 0 if there was none.
	HighestVersion Version
}

func (r *PrecommitConstraintResult) submitMetrics() {
	mon.Meter("object_delete").Mark(r.DeletedObjectCount)
	mon.Meter("segment_delete").Mark(r.DeletedSegmentCount)
}

type stmtRow interface {
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

// PrecommitConstraint ensures that only a single uncommitted object exists at the specified location.
func (db *DB) PrecommitConstraint(ctx context.Context, opts PrecommitConstraint, adapter precommitTransactionAdapter) (result PrecommitConstraintResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Location.Verify(); err != nil {
		return result, Error.Wrap(err)
	}

	if opts.Versioned {
		highest, err := adapter.precommitQueryHighest(ctx, opts.Location)
		if err != nil {
			return PrecommitConstraintResult{}, Error.Wrap(err)
		}
		result.HighestVersion = highest
		return result, nil
	}

	if opts.DisallowDelete {
		highest, unversionedExists, err := adapter.precommitQueryHighestAndUnversioned(ctx, opts.Location)
		if err != nil {
			return PrecommitConstraintResult{}, Error.Wrap(err)
		}
		result.HighestVersion = highest
		if unversionedExists {
			return PrecommitConstraintResult{}, ErrPermissionDenied.New("no permissions to delete existing object")
		}
		return result, nil
	}

	return adapter.precommitDeleteUnversioned(ctx, opts.Location)
}

// precommitQueryHighest queries the highest version for a given object.
func (ptx *postgresTransactionAdapter) precommitQueryHighest(ctx context.Context, loc ObjectLocation) (highest Version, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := loc.Verify(); err != nil {
		return 0, Error.Wrap(err)
	}

	err = ptx.tx.QueryRowContext(ctx, `
		SELECT version
		FROM objects
		WHERE (project_id, bucket_name, object_key) = ($1, $2, $3)
		ORDER BY version DESC
		LIMIT 1
	`, loc.ProjectID, []byte(loc.BucketName), loc.ObjectKey).Scan(&highest)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, Error.Wrap(err)
	}

	return highest, nil
}

// precommitQueryHighestAndUnversioned queries the highest version for a given object and whether an unversioned object or delete marker exists.
func (ptx *postgresTransactionAdapter) precommitQueryHighestAndUnversioned(ctx context.Context, loc ObjectLocation) (highest Version, unversionedExists bool, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := loc.Verify(); err != nil {
		return 0, false, Error.Wrap(err)
	}

	var version sql.NullInt64
	err = ptx.tx.QueryRowContext(ctx, `
		SELECT
			(
				SELECT version
				FROM objects
				WHERE (project_id, bucket_name, object_key) = ($1, $2, $3)
				ORDER BY version DESC
				LIMIT 1
			),
			(
				SELECT EXISTS (
					SELECT 1
					FROM objects
					WHERE (project_id, bucket_name, object_key) = ($1, $2, $3) AND
						status IN `+statusesUnversioned+`
				)
			)
	`, loc.ProjectID, []byte(loc.BucketName), loc.ObjectKey).Scan(&version, &unversionedExists)
	if err != nil {
		return 0, false, Error.Wrap(err)
	}
	if version.Valid {
		highest = Version(version.Int64)
	}

	return highest, unversionedExists, nil
}

// precommitDeleteUnversioned deletes the unversioned object at loc and also returns the highest version.
func (ptx *postgresTransactionAdapter) precommitDeleteUnversioned(ctx context.Context, loc ObjectLocation) (result PrecommitConstraintResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := loc.Verify(); err != nil {
		return PrecommitConstraintResult{}, Error.Wrap(err)
	}

	var deleted Object

	// TODO(ver): this scanning can probably simplified somehow.

	var version sql.NullInt64
	var streamID uuid.NullUUID
	var createdAt sql.NullTime
	var segmentCount, fixedSegmentSize sql.NullInt32
	var totalPlainSize, totalEncryptedSize sql.NullInt64
	var status sql.NullByte
	var encryptionParams nullableValue[encryptionParameters]
	encryptionParams.value.EncryptionParameters = &deleted.Encryption

	err = ptx.tx.QueryRowContext(ctx, `
		WITH highest_object AS (
			SELECT version
			FROM objects
			WHERE (project_id, bucket_name, object_key) = ($1, $2, $3)
			ORDER BY version DESC
			LIMIT 1
		), deleted_objects AS (
			DELETE FROM objects
			WHERE
				(project_id, bucket_name, object_key) = ($1, $2, $3)
				AND status IN `+statusesUnversioned+`
			RETURNING
				version, stream_id,
				created_at, expires_at,
				status, segment_count,
				encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
				total_plain_size, total_encrypted_size, fixed_segment_size,
				encryption
		), deleted_segments AS (
			DELETE FROM segments
			WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
			RETURNING segments.stream_id
		)
		SELECT
			(SELECT version FROM deleted_objects),
			(SELECT stream_id FROM deleted_objects),
			(SELECT created_at FROM deleted_objects),
			(SELECT expires_at FROM deleted_objects),
			(SELECT status FROM deleted_objects),
			(SELECT segment_count FROM deleted_objects),
			(SELECT encrypted_metadata_nonce FROM deleted_objects),
			(SELECT encrypted_metadata FROM deleted_objects),
			(SELECT encrypted_metadata_encrypted_key FROM deleted_objects),
			(SELECT total_plain_size FROM deleted_objects),
			(SELECT total_encrypted_size FROM deleted_objects),
			(SELECT fixed_segment_size FROM deleted_objects),
			(SELECT encryption FROM deleted_objects),
			(SELECT count(*) FROM deleted_objects),
			(SELECT count(*) FROM deleted_segments),
			coalesce((SELECT version FROM highest_object), 0)
	`, loc.ProjectID, []byte(loc.BucketName), loc.ObjectKey).
		Scan(
			&version,
			&streamID,
			&createdAt,
			&deleted.ExpiresAt,
			&status,
			&segmentCount,
			&deleted.EncryptedMetadataNonce,
			&deleted.EncryptedMetadata,
			&deleted.EncryptedMetadataEncryptedKey,
			&totalPlainSize,
			&totalEncryptedSize,
			&fixedSegmentSize,
			&encryptionParams,
			&result.DeletedObjectCount,
			&result.DeletedSegmentCount,
			&result.HighestVersion,
		)

	if err != nil {
		return PrecommitConstraintResult{}, Error.Wrap(err)
	}

	deleted.ProjectID = loc.ProjectID
	deleted.BucketName = loc.BucketName
	deleted.ObjectKey = loc.ObjectKey
	deleted.Version = Version(version.Int64)

	deleted.Status = ObjectStatus(status.Byte)
	deleted.StreamID = streamID.UUID
	deleted.CreatedAt = createdAt.Time
	deleted.SegmentCount = segmentCount.Int32

	deleted.TotalPlainSize = totalPlainSize.Int64
	deleted.TotalEncryptedSize = totalEncryptedSize.Int64
	deleted.FixedSegmentSize = fixedSegmentSize.Int32

	if result.DeletedObjectCount > 1 {
		ptx.postgresAdapter.log.Error("object with multiple committed versions were found!",
			zap.Stringer("Project ID", loc.ProjectID), zap.String("Bucket Name", loc.BucketName),
			zap.ByteString("Object Key", []byte(loc.ObjectKey)), zap.Int("deleted", result.DeletedObjectCount))

		mon.Meter("multiple_committed_versions").Mark(1)

		return result, Error.New("internal error: multiple committed unversioned objects")
	}

	if result.DeletedObjectCount > 0 {
		result.Deleted = append(result.Deleted, deleted)
	}

	return result, nil
}

// PrecommitConstraintWithNonPendingResult contains the result for enforcing precommit constraint.
type PrecommitConstraintWithNonPendingResult struct {
	Deleted []Object

	// DeletedObjectCount returns how many objects were deleted.
	DeletedObjectCount int
	// DeletedSegmentCount returns how many segments were deleted.
	DeletedSegmentCount int

	// HighestVersion returns tha highest version that was present in the table.
	// It returns 0 if there was none.
	HighestVersion Version

	// HighestNonPendingVersion returns tha highest non-pending version that was present in the table.
	// It returns 0 if there was none.
	HighestNonPendingVersion Version
}

func (r *PrecommitConstraintWithNonPendingResult) submitMetrics() {
	mon.Meter("object_delete").Mark(r.DeletedObjectCount)
	mon.Meter("segment_delete").Mark(r.DeletedSegmentCount)
}

// PrecommitDeleteUnversionedWithNonPending deletes the unversioned object at loc and also returns the highest version and highest committed version.
func (db *DB) PrecommitDeleteUnversionedWithNonPending(ctx context.Context, loc ObjectLocation, tx stmtRow) (result PrecommitConstraintWithNonPendingResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := loc.Verify(); err != nil {
		return PrecommitConstraintWithNonPendingResult{}, Error.Wrap(err)
	}

	var deleted Object

	// TODO(ver): this scanning can probably simplified somehow.

	var version sql.NullInt64
	var streamID uuid.NullUUID
	var createdAt sql.NullTime
	var segmentCount, fixedSegmentSize sql.NullInt32
	var totalPlainSize, totalEncryptedSize sql.NullInt64
	var status sql.NullByte
	var encryptionParams nullableValue[encryptionParameters]
	encryptionParams.value.EncryptionParameters = &deleted.Encryption

	err = tx.QueryRowContext(ctx, `
		WITH highest_object AS (
			SELECT version
			FROM objects
			WHERE (project_id, bucket_name, object_key) = ($1, $2, $3)
			ORDER BY version DESC
			LIMIT 1
		), highest_non_pending_object AS (
			SELECT version
			FROM objects
			WHERE (project_id, bucket_name, object_key) = ($1, $2, $3)
				AND status <> `+statusPending+`
			ORDER BY version DESC
			LIMIT 1
		), deleted_objects AS (
			DELETE FROM objects
			WHERE
				(project_id, bucket_name, object_key) = ($1, $2, $3)
				AND status IN `+statusesUnversioned+`
			RETURNING
				version, stream_id,
				created_at, expires_at,
				status, segment_count,
				encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
				total_plain_size, total_encrypted_size, fixed_segment_size,
				encryption
		), deleted_segments AS (
			DELETE FROM segments
			WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
			RETURNING segments.stream_id
		)
		SELECT
			(SELECT version FROM deleted_objects),
			(SELECT stream_id FROM deleted_objects),
			(SELECT created_at FROM deleted_objects),
			(SELECT expires_at FROM deleted_objects),
			(SELECT status FROM deleted_objects),
			(SELECT segment_count FROM deleted_objects),
			(SELECT encrypted_metadata_nonce FROM deleted_objects),
			(SELECT encrypted_metadata FROM deleted_objects),
			(SELECT encrypted_metadata_encrypted_key FROM deleted_objects),
			(SELECT total_plain_size FROM deleted_objects),
			(SELECT total_encrypted_size FROM deleted_objects),
			(SELECT fixed_segment_size FROM deleted_objects),
			(SELECT encryption FROM deleted_objects),
			(SELECT count(*) FROM deleted_objects),
			(SELECT count(*) FROM deleted_segments),
			coalesce((SELECT version FROM highest_object), 0),
			coalesce((SELECT version FROM highest_non_pending_object), 0)
	`, loc.ProjectID, []byte(loc.BucketName), loc.ObjectKey).
		Scan(
			&version,
			&streamID,
			&createdAt,
			&deleted.ExpiresAt,
			&status,
			&segmentCount,
			&deleted.EncryptedMetadataNonce,
			&deleted.EncryptedMetadata,
			&deleted.EncryptedMetadataEncryptedKey,
			&totalPlainSize,
			&totalEncryptedSize,
			&fixedSegmentSize,
			&encryptionParams,
			&result.DeletedObjectCount,
			&result.DeletedSegmentCount,
			&result.HighestVersion,
			&result.HighestNonPendingVersion,
		)

	if err != nil {
		return PrecommitConstraintWithNonPendingResult{}, Error.Wrap(err)
	}

	deleted.ProjectID = loc.ProjectID
	deleted.BucketName = loc.BucketName
	deleted.ObjectKey = loc.ObjectKey
	deleted.Version = Version(version.Int64)

	deleted.Status = ObjectStatus(status.Byte)
	deleted.StreamID = streamID.UUID
	deleted.CreatedAt = createdAt.Time
	deleted.SegmentCount = segmentCount.Int32

	deleted.TotalPlainSize = totalPlainSize.Int64
	deleted.TotalEncryptedSize = totalEncryptedSize.Int64
	deleted.FixedSegmentSize = fixedSegmentSize.Int32

	if result.DeletedObjectCount > 1 {
		db.log.Error("object with multiple committed versions were found!",
			zap.Stringer("Project ID", loc.ProjectID), zap.String("Bucket Name", loc.BucketName),
			zap.ByteString("Object Key", []byte(loc.ObjectKey)), zap.Int("deleted", result.DeletedObjectCount))

		mon.Meter("multiple_committed_versions").Mark(1)

		return result, Error.New("internal error: multiple committed unversioned objects")
	}

	if result.DeletedObjectCount > 0 {
		result.Deleted = append(result.Deleted, deleted)
	}

	return result, nil
}
