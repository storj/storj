// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"errors"

	"cloud.google.com/go/spanner"
	"go.uber.org/zap"
	"google.golang.org/api/iterator"

	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/spannerutil"
)

type precommitTransactionAdapter interface {
	precommitQueryHighest(ctx context.Context, loc ObjectLocation) (highest Version, err error)
	precommitQueryHighestAndUnversioned(ctx context.Context, loc ObjectLocation) (highest Version, unversionedExists bool, err error)
	precommitDeleteUnversioned(ctx context.Context, loc ObjectLocation) (result PrecommitConstraintResult, err error)
	precommitDeleteUnversionedWithSQLCheck(ctx context.Context, loc ObjectLocation) (result PrecommitConstraintResult, err error)
	precommitDeleteUnversionedWithVersionCheck(ctx context.Context, loc ObjectLocation) (result PrecommitConstraintResult, err error)
}

// PrecommitConstraint is arguments to ensure that a single unversioned object or delete marker exists in the
// table per object location.
type PrecommitConstraint struct {
	Location ObjectLocation

	Versioned      bool
	DisallowDelete bool

	PrecommitDeleteMode int
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

const defaultUnversionedPrecommitMode = 0
const withPrecheckSQLUnversionedPrecommitMode = 1
const withVersionPrecheckUnversionedPrecommitMode = 2

// PrecommitDeleteModes is a list of all possible precommit delete modes.
var PrecommitDeleteModes = []int{
	defaultUnversionedPrecommitMode,
	withPrecheckSQLUnversionedPrecommitMode,
	withVersionPrecheckUnversionedPrecommitMode,
}

func (r *PrecommitConstraintResult) submitMetrics() {
	mon.Meter("object_delete").Mark(r.DeletedObjectCount)
	mon.Meter("segment_delete").Mark(r.DeletedSegmentCount)
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

	switch opts.PrecommitDeleteMode {
	case defaultUnversionedPrecommitMode:
		return adapter.precommitDeleteUnversioned(ctx, opts.Location)
	case withPrecheckSQLUnversionedPrecommitMode:
		return adapter.precommitDeleteUnversionedWithSQLCheck(ctx, opts.Location)
	case withVersionPrecheckUnversionedPrecommitMode:
		return adapter.precommitDeleteUnversionedWithVersionCheck(ctx, opts.Location)
	default:
		return PrecommitConstraintResult{}, Error.New("Invalid precommit delete mode version: %d", opts.PrecommitDeleteMode)
	}
}

// precommitQueryHighest queries the highest version for a given object.
func (ptx *postgresTransactionAdapter) precommitQueryHighest(ctx context.Context, loc ObjectLocation) (highest Version, err error) {
	defer mon.Task()(&ctx)(&err)

	err = ptx.tx.QueryRowContext(ctx, `
		SELECT version
		FROM objects
		WHERE (project_id, bucket_name, object_key) = ($1, $2, $3)
		ORDER BY version DESC
		LIMIT 1
	`, loc.ProjectID, loc.BucketName, loc.ObjectKey).Scan(&highest)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, Error.Wrap(err)
	}

	return highest, nil
}

func (stx *spannerTransactionAdapter) precommitQueryHighest(ctx context.Context, loc ObjectLocation) (highest Version, err error) {
	defer mon.Task()(&ctx)(&err)

	highest, err = spannerutil.CollectRow(stx.tx.Query(ctx, spanner.Statement{
		SQL: `
			SELECT version
			FROM objects
			WHERE (project_id, bucket_name, object_key) = (@project_id, @bucket_name, @object_key)
			ORDER BY version DESC
			LIMIT 1
		`,
		Params: map[string]interface{}{
			"project_id":  loc.ProjectID,
			"bucket_name": loc.BucketName,
			"object_key":  loc.ObjectKey,
		},
	}), func(row *spanner.Row, highest *Version) error {
		return Error.Wrap(row.Columns(highest))
	})
	if err != nil {
		if errors.Is(err, iterator.Done) {
			return 0, nil
		}
		return 0, Error.Wrap(err)
	}
	return highest, nil
}

// precommitQueryHighestAndUnversioned queries the highest version for a given object and whether an unversioned object or delete marker exists.
func (ptx *postgresTransactionAdapter) precommitQueryHighestAndUnversioned(ctx context.Context, loc ObjectLocation) (highest Version, unversionedExists bool, err error) {
	defer mon.Task()(&ctx)(&err)

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
	`, loc.ProjectID, loc.BucketName, loc.ObjectKey).Scan(&version, &unversionedExists)
	if err != nil {
		return 0, false, Error.Wrap(err)
	}
	if version.Valid {
		highest = Version(version.Int64)
	}

	return highest, unversionedExists, nil
}

func (stx *spannerTransactionAdapter) precommitQueryHighestAndUnversioned(ctx context.Context, loc ObjectLocation) (highest Version, unversionedExists bool, err error) {
	defer mon.Task()(&ctx)(&err)

	err = stx.tx.Query(ctx, spanner.Statement{
		SQL: `
			SELECT
				(
					SELECT version
					FROM objects
					WHERE (project_id, bucket_name, object_key) = (@project_id, @bucket_name, @object_key)
					ORDER BY version DESC
					LIMIT 1
				),
				(
					SELECT EXISTS (
						SELECT 1
						FROM objects
						WHERE (project_id, bucket_name, object_key) = (@project_id, @bucket_name, @object_key) AND
							status IN ` + statusesUnversioned + `
					)
				)
		`,
		Params: map[string]interface{}{
			"project_id":  loc.ProjectID,
			"bucket_name": loc.BucketName,
			"object_key":  loc.ObjectKey,
		},
	}).Do(func(row *spanner.Row) error {
		var versionOptional *int64
		err := Error.Wrap(row.Columns(&versionOptional, &unversionedExists))
		if err != nil {
			return err
		}

		if versionOptional != nil {
			highest = Version(*versionOptional)
		}
		return nil
	})
	if err != nil {
		return 0, false, Error.Wrap(err)
	}
	return highest, unversionedExists, nil
}

// precommitDeleteUnversioned deletes the unversioned object at loc and also returns the highest version.
func (ptx *postgresTransactionAdapter) precommitDeleteUnversioned(ctx context.Context, loc ObjectLocation) (result PrecommitConstraintResult, err error) {
	defer mon.Task()(&ctx)(&err)

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
	`, loc.ProjectID, loc.BucketName, loc.ObjectKey).
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

	// If there are no objects with the given (project_id, bucket_name, object_key),
	// all of the values queried from deleted_objects will be NULL. We must not
	// dereference the sql.NullX values until we have checked at least one of them.
	if !version.Valid {
		// it looks like the intended behavior here is to return an empty result.Deleted list.
		return result, nil
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
		// It should be impossible to hit this code. Since we use subqueries like "(SELECT version
		// FROM deleted_objects)" in single-valued contexts, we are asserting that there is no more
		// than one row in deleted_objects. If there is more than one, PG or CR should have errored
		// with something like "more than one row returned by a subquery used as an expression".
		// But this is left as a protection against a broken implementation.
		ptx.postgresAdapter.log.Error("object with multiple committed versions were found!",
			zap.Stringer("Project ID", loc.ProjectID), zap.Stringer("Bucket Name", loc.BucketName),
			zap.ByteString("Object Key", []byte(loc.ObjectKey)), zap.Int("deleted", result.DeletedObjectCount))

		mon.Meter("multiple_committed_versions").Mark(1)

		return result, Error.New("internal error: multiple committed unversioned objects")
	}

	if result.DeletedObjectCount > 0 {
		result.Deleted = append(result.Deleted, deleted)
	}

	return result, nil
}

// precommitDeleteUnversionedWithSQLCheck deletes the unversioned object at loc and also returns the highest version. deletes the unversioned object at loc and also returns the highest version.
// it will skip the delete if no deletable objects are found.
func (ptx *postgresTransactionAdapter) precommitDeleteUnversionedWithSQLCheck(ctx context.Context, loc ObjectLocation) (result PrecommitConstraintResult, err error) {
	defer mon.Task()(&ctx)(&err)

	var deleted Object

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
				EXISTS (SELECT * from highest_object)
				AND (project_id, bucket_name, object_key) = ($1, $2, $3)
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
	`, loc.ProjectID, loc.BucketName, loc.ObjectKey).
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
	if errors.Is(err, sql.ErrNoRows) {
		return result, nil
	}
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
			zap.Stringer("Project ID", loc.ProjectID), zap.Stringer("Bucket Name", loc.BucketName),
			zap.ByteString("Object Key", []byte(loc.ObjectKey)), zap.Int("deleted", result.DeletedObjectCount))

		mon.Meter("multiple_committed_versions").Mark(1)

		return result, Error.New("internal error: multiple committed unversioned objects")
	}

	if result.DeletedObjectCount > 0 {
		result.Deleted = append(result.Deleted, deleted)
	}

	return result, nil
}

// precommitDeleteUnversionedWithVersionCheck deletes the unversioned object at loc and also returns the highest version. deletes the unversioned object at loc and also returns the highest version.
// it will skip the delete if no deletable objects are found.
func (ptx *postgresTransactionAdapter) precommitDeleteUnversionedWithVersionCheck(ctx context.Context, loc ObjectLocation) (result PrecommitConstraintResult, err error) {
	defer mon.Task()(&ctx)(&err)

	result.HighestVersion, err = ptx.precommitQueryHighest(ctx, loc)
	if err != nil {
		return PrecommitConstraintResult{}, Error.Wrap(err)
	}

	if result.HighestVersion == 0 {
		// no previous versions, nothing to delete
		return result, nil
	}

	var deleted Object

	var version sql.NullInt64
	var streamID uuid.NullUUID
	var createdAt sql.NullTime
	var segmentCount, fixedSegmentSize sql.NullInt32
	var totalPlainSize, totalEncryptedSize sql.NullInt64
	var status sql.NullByte
	var encryptionParams nullableValue[encryptionParameters]
	encryptionParams.value.EncryptionParameters = &deleted.Encryption

	err = ptx.tx.QueryRowContext(ctx, `
		WITH deleted_objects AS (
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
			(SELECT count(*) FROM deleted_segments)
	`, loc.ProjectID, loc.BucketName, loc.ObjectKey).
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
			zap.Stringer("Project ID", loc.ProjectID), zap.Stringer("Bucket Name", loc.BucketName),
			zap.ByteString("Object Key", []byte(loc.ObjectKey)), zap.Int("deleted", result.DeletedObjectCount))

		mon.Meter("multiple_committed_versions").Mark(1)

		return result, Error.New("internal error: multiple committed unversioned objects")
	}

	if result.DeletedObjectCount > 0 {
		result.Deleted = append(result.Deleted, deleted)
	}

	return result, nil
}

func (stx *spannerTransactionAdapter) precommitDeleteUnversionedWithVersionCheck(ctx context.Context, loc ObjectLocation) (result PrecommitConstraintResult, err error) {
	return stx.precommitDeleteUnversioned(ctx, loc)
}

func (stx *spannerTransactionAdapter) precommitDeleteUnversionedWithSQLCheck(ctx context.Context, loc ObjectLocation) (result PrecommitConstraintResult, err error) {
	return stx.precommitDeleteUnversioned(ctx, loc)
}

func (stx *spannerTransactionAdapter) precommitDeleteUnversioned(ctx context.Context, loc ObjectLocation) (result PrecommitConstraintResult, err error) {
	defer mon.Task()(&ctx)(&err)

	result.HighestVersion, err = spannerutil.CollectRow(stx.tx.Query(ctx, spanner.Statement{
		SQL: `
				SELECT version
				FROM objects
				WHERE (project_id, bucket_name, object_key) = (@project_id, @bucket_name, @object_key)
				ORDER BY version DESC
				LIMIT 1
			`,
		Params: map[string]interface{}{
			"project_id":  loc.ProjectID,
			"bucket_name": loc.BucketName,
			"object_key":  loc.ObjectKey,
		},
	}), func(row *spanner.Row, item *Version) error {
		return Error.Wrap(row.Columns(item))
	})
	if errors.Is(err, iterator.Done) {
		result.HighestVersion = 0
		err = nil
	}
	if err != nil {
		return PrecommitConstraintResult{}, Error.Wrap(err)
	}

	result.Deleted, err = collectDeletedObjectsSpanner(ctx, loc, stx.tx.Query(ctx, spanner.Statement{
		SQL: `
			DELETE FROM objects
			WHERE
				project_id      = @project_id
				AND bucket_name = @bucket_name
				AND object_key  = @object_key
				AND status IN ` + statusesUnversioned + `
			THEN RETURN ` + collectDeletedObjectsSpannerFields,
		Params: map[string]any{
			"project_id":  loc.ProjectID,
			"bucket_name": loc.BucketName,
			"object_key":  loc.ObjectKey,
		},
	}))

	if err != nil {
		return result, Error.Wrap(err)
	}
	result.DeletedObjectCount = len(result.Deleted)

	if len(result.Deleted) > 1 {
		stx.spannerAdapter.log.Error("object with multiple committed versions were found!",
			zap.Stringer("Project ID", loc.ProjectID), zap.Stringer("Bucket Name", loc.BucketName),
			zap.ByteString("Object Key", []byte(loc.ObjectKey)), zap.Int("deleted", len(result.Deleted)))

		mon.Meter("multiple_committed_versions").Mark(1)

		return result, Error.New("internal error: multiple committed unversioned objects")
	}

	if len(result.Deleted) == 1 {
		rowCount, err := stx.tx.Update(ctx, spanner.Statement{
			SQL: `
				DELETE FROM segments
				WHERE segments.stream_id = @stream_id
			`,
			Params: map[string]interface{}{
				"stream_id": result.Deleted[0].StreamID,
			},
		})
		if err != nil {
			return result, Error.Wrap(err)
		}
		result.DeletedSegmentCount = int(rowCount)
	}

	return result, Error.Wrap(err)
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
func (ptx *postgresTransactionAdapter) PrecommitDeleteUnversionedWithNonPending(ctx context.Context, loc ObjectLocation) (result PrecommitConstraintWithNonPendingResult, err error) {
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

	err = ptx.tx.QueryRowContext(ctx, `
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
	`, loc.ProjectID, loc.BucketName, loc.ObjectKey).
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
		ptx.postgresAdapter.log.Error("object with multiple committed versions were found!",
			zap.Stringer("Project ID", loc.ProjectID), zap.Stringer("Bucket Name", loc.BucketName),
			zap.ByteString("Object Key", []byte(loc.ObjectKey)), zap.Int("deleted", result.DeletedObjectCount))

		mon.Meter("multiple_committed_versions").Mark(1)

		return result, Error.New("internal error: multiple committed unversioned objects")
	}

	if result.DeletedObjectCount > 0 {
		result.Deleted = append(result.Deleted, deleted)
	}

	return result, nil
}

// PrecommitDeleteUnversionedWithNonPending deletes the unversioned object at loc and also returns the highest version and highest committed version.
func (stx *spannerTransactionAdapter) PrecommitDeleteUnversionedWithNonPending(ctx context.Context, loc ObjectLocation) (result PrecommitConstraintWithNonPendingResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := loc.Verify(); err != nil {
		return PrecommitConstraintWithNonPendingResult{}, Error.Wrap(err)
	}

	result, err = spannerutil.CollectRow(stx.tx.Query(ctx, spanner.Statement{
		SQL: `
			WITH highest_object AS (
				SELECT version
				FROM objects
				WHERE (project_id, bucket_name, object_key) = (@project_id, @bucket_name, @object_key)
				ORDER BY version DESC
				LIMIT 1
			), highest_non_pending_object AS (
				SELECT version
				FROM objects
				WHERE (project_id, bucket_name, object_key) = (@project_id, @bucket_name, @object_key)
					AND status <> ` + statusPending + `
				ORDER BY version DESC
				LIMIT 1
			)
			SELECT
				COALESCE((SELECT version FROM highest_object), 0) AS highest,
				COALESCE((SELECT version FROM highest_non_pending_object), 0) AS highest_non_pending
		`,
		Params: map[string]interface{}{
			"project_id":  loc.ProjectID,
			"bucket_name": loc.BucketName,
			"object_key":  loc.ObjectKey,
		},
	}), func(row *spanner.Row, result *PrecommitConstraintWithNonPendingResult) error {
		return Error.Wrap(row.Columns(&result.HighestVersion, &result.HighestNonPendingVersion))
	})
	if err != nil {
		return PrecommitConstraintWithNonPendingResult{}, Error.Wrap(err)
	}

	// TODO(spanner): is there a better way to combine these deletes from different tables?
	result.Deleted, err = collectDeletedObjectsSpanner(ctx, loc,
		stx.tx.Query(ctx, spanner.Statement{
			SQL: `
				DELETE FROM objects
				WHERE
					(project_id, bucket_name, object_key) = (@project_id, @bucket_name, @object_key)
					AND status IN ` + statusesUnversioned + `
				THEN RETURN` + collectDeletedObjectsSpannerFields,
			Params: map[string]interface{}{
				"project_id":  loc.ProjectID,
				"bucket_name": loc.BucketName,
				"object_key":  loc.ObjectKey,
			},
		}))
	if err != nil {
		return PrecommitConstraintWithNonPendingResult{}, Error.Wrap(err)
	}

	streamIDs := make([][]byte, 0, len(result.Deleted))
	for _, object := range result.Deleted {
		streamIDs = append(streamIDs, object.StreamID.Bytes())
	}

	// TODO(spanner): make sure this is an efficient query
	segmentDeletion := spanner.Statement{
		SQL: `
			DELETE FROM segments
			WHERE ARRAY_INCLUDES(@stream_ids, stream_id)
		`,
		Params: map[string]interface{}{
			"stream_ids": streamIDs,
		},
	}
	segmentsDeleted, err := stx.tx.Update(ctx, segmentDeletion)
	if err != nil {
		return PrecommitConstraintWithNonPendingResult{}, Error.New("unable to delete segments: %w", err)
	}

	result.DeletedObjectCount = len(result.Deleted)
	result.DeletedSegmentCount = int(segmentsDeleted)

	if len(result.Deleted) > 1 {
		stx.spannerAdapter.log.Error("object with multiple committed versions were found!",
			zap.Stringer("Project ID", loc.ProjectID), zap.Stringer("Bucket Name", loc.BucketName),
			zap.ByteString("Object Key", []byte(loc.ObjectKey)), zap.Int("deleted", result.DeletedObjectCount))

		mon.Meter("multiple_committed_versions").Mark(1)

		return result, Error.New("internal error: multiple committed unversioned objects")
	}

	// match behavior of postgresTransactionAdapter, to appease a test
	if len(result.Deleted) == 0 {
		result.Deleted = nil
	}
	return result, nil
}
