// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/spannerutil"
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
func (ptx *postgresTransactionAdapter) PrecommitDeleteUnversionedWithNonPending(ctx context.Context, opts PrecommitDeleteUnversionedWithNonPending) (PrecommitConstraintWithNonPendingResult, error) {
	if err := opts.Verify(); err != nil {
		return PrecommitConstraintWithNonPendingResult{}, Error.Wrap(err)
	}

	if opts.ObjectLock.Enabled {
		return ptx.precommitDeleteUnversionedWithNonPendingUsingObjectLock(ctx, opts)
	}
	return ptx.precommitDeleteUnversionedWithNonPending(ctx, opts.ObjectLocation)
}

func (ptx *postgresTransactionAdapter) precommitDeleteUnversionedWithNonPending(ctx context.Context, loc ObjectLocation) (result PrecommitConstraintWithNonPendingResult, err error) {
	defer mon.Task()(&ctx)(&err)

	var deleted Object

	// TODO(ver): this scanning can probably simplified somehow.

	var version NullableVersion
	var streamID uuid.NullUUID
	var createdAt sql.NullTime
	var segmentCount, fixedSegmentSize sql.NullInt32
	var totalPlainSize, totalEncryptedSize sql.NullInt64
	var status NullableObjectStatus
	var encryptionParams nullableValue[*storj.EncryptionParameters]
	encryptionParams.value = new(storj.EncryptionParameters)

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
				encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key, encrypted_etag,
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
			(SELECT encrypted_etag FROM deleted_objects),
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
			&deleted.EncryptedETag,
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
	deleted.Version = version.Version

	deleted.Status = status.ObjectStatus
	deleted.StreamID = streamID.UUID
	deleted.CreatedAt = createdAt.Time
	deleted.SegmentCount = segmentCount.Int32

	deleted.TotalPlainSize = totalPlainSize.Int64
	deleted.TotalEncryptedSize = totalEncryptedSize.Int64
	deleted.FixedSegmentSize = fixedSegmentSize.Int32

	if !encryptionParams.isnull {
		deleted.Encryption = *encryptionParams.value
	}

	if result.DeletedObjectCount > 1 {
		ptx.postgresAdapter.log.Error("object with multiple committed versions were found!",
			zap.Stringer("Project ID", loc.ProjectID), zap.Stringer("Bucket Name", loc.BucketName),
			zap.String("Object Key", hex.EncodeToString([]byte(loc.ObjectKey))), zap.Int("deleted", result.DeletedObjectCount))

		mon.Meter("multiple_committed_versions").Mark(1)

		return result, Error.New(multipleCommittedVersionsErrMsg)
	}

	if result.DeletedObjectCount > 0 {
		result.Deleted = append(result.Deleted, deleted)
	}

	return result, nil
}

// precommitDeleteUnversionedWithNonPendingUsingObjectLock deletes the unversioned object at loc
// and also returns the highest version and highest committed version. It returns an error if the
// object's Object Lock configuration prohibits its deletion.
func (ptx *postgresTransactionAdapter) precommitDeleteUnversionedWithNonPendingUsingObjectLock(ctx context.Context, opts PrecommitDeleteUnversionedWithNonPending) (result PrecommitConstraintWithNonPendingResult, err error) {
	defer mon.Task()(&ctx)(&err)

	type versionAndLockInfo struct {
		version   Version
		retention Retention
		legalHold bool
	}

	var (
		highestVersionScanned, highestNonPendingVersionScanned bool
		objectToDelete                                         *versionAndLockInfo
	)

	err = withRows(ptx.tx.QueryContext(ctx, `
		SELECT version, status, retention_mode, retain_until
		FROM objects
		WHERE (project_id, bucket_name, object_key) = ($1, $2, $3)
		ORDER BY version DESC
		FOR UPDATE
		`, opts.ProjectID, opts.BucketName, opts.ObjectKey,
	))(func(rows tagsql.Rows) error {
		for rows.Next() {
			var (
				version   Version
				status    ObjectStatus
				retention Retention
				legalHold bool
			)

			lockMode := lockModeWrapper{
				retentionMode: &retention.Mode,
				legalHold:     &legalHold,
			}

			err := rows.Scan(&version, &status, lockMode, timeWrapper{&retention.RetainUntil})
			if err != nil {
				return errs.Wrap(err)
			}

			if !highestVersionScanned {
				result.HighestVersion = version
				highestVersionScanned = true
			}

			if !highestNonPendingVersionScanned && status != Pending {
				result.HighestNonPendingVersion = version
				highestNonPendingVersionScanned = true
			}

			if status.IsUnversioned() {
				if objectToDelete != nil {
					logMultipleCommittedVersionsError(ptx.postgresAdapter.log, opts.ObjectLocation)
					return errs.New(multipleCommittedVersionsErrMsg)
				}
				objectToDelete = &versionAndLockInfo{
					version:   version,
					retention: retention,
					legalHold: legalHold,
				}
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PrecommitConstraintWithNonPendingResult{}, nil
		}
		return PrecommitConstraintWithNonPendingResult{}, Error.Wrap(err)
	}
	if objectToDelete == nil {
		return result, nil
	}

	if err = objectToDelete.retention.Verify(); err != nil {
		return PrecommitConstraintWithNonPendingResult{}, Error.Wrap(err)
	}
	switch {
	case objectToDelete.legalHold:
		return PrecommitConstraintWithNonPendingResult{}, ErrObjectLock.New(legalHoldErrMsg)
	case isRetentionProtected(objectToDelete.retention, opts.ObjectLock.BypassGovernance, time.Now()):
		return PrecommitConstraintWithNonPendingResult{}, ErrObjectLock.New(retentionErrMsg)
	}

	deleted := Object{
		ObjectStream: ObjectStream{
			ProjectID:  opts.ProjectID,
			BucketName: opts.BucketName,
			ObjectKey:  opts.ObjectKey,
			Version:    objectToDelete.version,
		},
	}

	err = ptx.tx.QueryRowContext(ctx, `
		WITH deleted_objects AS (
			DELETE FROM objects
			WHERE
				(project_id, bucket_name, object_key, version) = ($1, $2, $3, $4)
			RETURNING
				stream_id,
				created_at, expires_at,
				status, segment_count,
				encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key, encrypted_etag,
				total_plain_size, total_encrypted_size, fixed_segment_size,
				encryption,
				retention_mode, retain_until
		), deleted_segments AS (
			DELETE FROM segments
			WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
			RETURNING segments.stream_id
		)
		SELECT *, (SELECT count(*) FROM deleted_segments)
		FROM deleted_objects
		`, opts.ProjectID, opts.BucketName, opts.ObjectKey, objectToDelete.version,
	).Scan(
		&deleted.StreamID,
		&deleted.CreatedAt,
		&deleted.ExpiresAt,
		&deleted.Status,
		&deleted.SegmentCount,
		&deleted.EncryptedMetadataNonce,
		&deleted.EncryptedMetadata,
		&deleted.EncryptedMetadataEncryptedKey,
		&deleted.EncryptedETag,
		&deleted.TotalPlainSize,
		&deleted.TotalEncryptedSize,
		&deleted.FixedSegmentSize,
		&deleted.Encryption,
		lockModeWrapper{
			retentionMode: &deleted.Retention.Mode,
			legalHold:     &deleted.LegalHold,
		},
		timeWrapper{&deleted.Retention.RetainUntil},
		&result.DeletedSegmentCount,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// If this query returned no results, then the highest non-pending version
			// was removed since the first query.
			if result.HighestVersion == objectToDelete.version {
				result.HighestVersion = 0
			}
			if result.HighestNonPendingVersion == objectToDelete.version {
				result.HighestNonPendingVersion = 0
			}

			return result, nil
		}
		return PrecommitConstraintWithNonPendingResult{}, Error.Wrap(err)
	}

	result.Deleted = append(result.Deleted, deleted)
	result.DeletedObjectCount = 1

	return result, nil
}

// PrecommitDeleteUnversionedWithNonPending deletes the unversioned object at loc and also returns the highest version and highest committed version.
func (stx *spannerTransactionAdapter) PrecommitDeleteUnversionedWithNonPending(ctx context.Context, opts PrecommitDeleteUnversionedWithNonPending) (PrecommitConstraintWithNonPendingResult, error) {
	if err := opts.Verify(); err != nil {
		return PrecommitConstraintWithNonPendingResult{}, Error.Wrap(err)
	}

	if opts.ObjectLock.Enabled {
		return stx.precommitDeleteUnversionedWithNonPendingUsingObjectLock(ctx, opts)
	}
	return stx.precommitDeleteUnversionedWithNonPending(ctx, opts.ObjectLocation)
}

func (stx *spannerTransactionAdapter) precommitDeleteUnversionedWithNonPending(ctx context.Context, loc ObjectLocation) (result PrecommitConstraintWithNonPendingResult, err error) {
	defer mon.Task()(&ctx)(&err)

	result, err = spannerutil.CollectRow(stx.tx.QueryWithOptions(ctx, spanner.Statement{
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
	}, spanner.QueryOptions{RequestTag: "precommit-delete-unversioned-with-non-pending"}),
		func(row *spanner.Row, result *PrecommitConstraintWithNonPendingResult) error {
			return Error.Wrap(row.Columns(&result.HighestVersion, &result.HighestNonPendingVersion))
		})
	if err != nil {
		return PrecommitConstraintWithNonPendingResult{}, Error.Wrap(err)
	}

	// TODO(spanner): is there a better way to combine these deletes from different tables?
	result.Deleted, err = collectDeletedObjectsSpanner(ctx, loc,
		stx.tx.QueryWithOptions(ctx, spanner.Statement{
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
		}, spanner.QueryOptions{RequestTag: "precommit-delete-unversioned-with-non-pending-objects"}))
	if err != nil {
		return PrecommitConstraintWithNonPendingResult{}, Error.Wrap(err)
	}

	stmts := make([]spanner.Statement, len(result.Deleted))
	for ix, object := range result.Deleted {
		stmts[ix] = spanner.Statement{
			SQL: `DELETE FROM segments WHERE @stream_id = stream_id`,
			Params: map[string]interface{}{
				"stream_id": object.StreamID.Bytes(),
			},
		}
	}

	if len(stmts) > 0 {
		segmentsDeleted, err := stx.tx.BatchUpdateWithOptions(ctx, stmts, spanner.QueryOptions{RequestTag: "precommit-delete-unversioned-with-non-pending-segments"})
		if err != nil {
			return PrecommitConstraintWithNonPendingResult{}, Error.New("unable to delete segments: %w", err)
		}

		for _, v := range segmentsDeleted {
			result.DeletedSegmentCount += int(v)
		}
	}
	result.DeletedObjectCount = len(result.Deleted)

	if len(result.Deleted) > 1 {
		stx.spannerAdapter.log.Error("object with multiple committed versions were found!",
			zap.Stringer("Project ID", loc.ProjectID), zap.Stringer("Bucket Name", loc.BucketName),
			zap.String("Object Key", hex.EncodeToString([]byte(loc.ObjectKey))), zap.Int("deleted", result.DeletedObjectCount))

		mon.Meter("multiple_committed_versions").Mark(1)

		return result, Error.New(multipleCommittedVersionsErrMsg)
	}

	// match behavior of postgresTransactionAdapter, to appease a test
	if len(result.Deleted) == 0 {
		result.Deleted = nil
	}
	return result, nil
}

func (stx *spannerTransactionAdapter) precommitDeleteUnversionedWithNonPendingUsingObjectLock(ctx context.Context, opts PrecommitDeleteUnversionedWithNonPending) (result PrecommitConstraintWithNonPendingResult, err error) {
	defer mon.Task()(&ctx)(&err)

	type versionAndLockInfo struct {
		version   Version
		retention Retention
		legalHold bool
	}

	var (
		highestVersionScanned, highestNonPendingVersionScanned bool
		objectToDelete                                         *versionAndLockInfo
	)

	err = stx.tx.QueryWithOptions(ctx, spanner.Statement{
		SQL: `
			SELECT version, status, retention_mode, retain_until
			FROM objects
			WHERE (project_id, bucket_name, object_key) = (@project_id, @bucket_name, @object_key)
			ORDER BY version DESC
		`,
		Params: map[string]interface{}{
			"project_id":  opts.ProjectID,
			"bucket_name": opts.BucketName,
			"object_key":  opts.ObjectKey,
		},
	}, spanner.QueryOptions{RequestTag: "precommit-delete-unversioned-with-non-pending-using-object-lock-check"}).Do(func(row *spanner.Row) error {
		var (
			version   Version
			status    ObjectStatus
			retention Retention
			legalHold bool
		)

		lockMode := lockModeWrapper{
			retentionMode: &retention.Mode,
			legalHold:     &legalHold,
		}

		err := row.Columns(&version, &status, lockMode, timeWrapper{&retention.RetainUntil})
		if err != nil {
			return errs.Wrap(err)
		}

		if !highestVersionScanned {
			result.HighestVersion = version
			highestVersionScanned = true
		}

		if !highestNonPendingVersionScanned && status != Pending {
			result.HighestNonPendingVersion = version
			highestNonPendingVersionScanned = true
		}

		if status.IsUnversioned() {
			if objectToDelete != nil {
				logMultipleCommittedVersionsError(stx.spannerAdapter.log, opts.ObjectLocation)
				return errs.New(multipleCommittedVersionsErrMsg)
			}
			objectToDelete = &versionAndLockInfo{
				version:   version,
				retention: retention,
				legalHold: legalHold,
			}
		}

		return nil
	})
	if err != nil {
		return PrecommitConstraintWithNonPendingResult{}, Error.Wrap(err)
	}
	if objectToDelete == nil {
		return result, nil
	}

	if err = objectToDelete.retention.Verify(); err != nil {
		return PrecommitConstraintWithNonPendingResult{}, Error.Wrap(err)
	}
	switch {
	case objectToDelete.legalHold:
		return PrecommitConstraintWithNonPendingResult{}, ErrObjectLock.New(legalHoldErrMsg)
	case isRetentionProtected(objectToDelete.retention, opts.ObjectLock.BypassGovernance, time.Now()):
		return PrecommitConstraintWithNonPendingResult{}, ErrObjectLock.New(retentionErrMsg)
	}

	// TODO(spanner): is there a better way to combine these deletes from different tables?
	result.Deleted, err = collectDeletedObjectsSpanner(ctx, opts.ObjectLocation,
		stx.tx.QueryWithOptions(ctx, spanner.Statement{
			SQL: `
				DELETE FROM objects
				WHERE
					(project_id, bucket_name, object_key, version) = (@project_id, @bucket_name, @object_key, @version)
				THEN RETURN ` + collectDeletedObjectsSpannerFields,
			Params: map[string]interface{}{
				"project_id":  opts.ProjectID,
				"bucket_name": opts.BucketName,
				"object_key":  opts.ObjectKey,
				"version":     objectToDelete.version,
			},
		}, spanner.QueryOptions{RequestTag: "precommit-delete-unversioned-with-non-pending-using-object-lock-objects"}),
	)
	if err != nil {
		return PrecommitConstraintWithNonPendingResult{}, Error.Wrap(err)
	}
	if len(result.Deleted) == 0 {
		// If this query returned no results, then the highest non-pending version
		// was removed since the first query.
		if result.HighestVersion == objectToDelete.version {
			result.HighestVersion = 0
		}
		if result.HighestNonPendingVersion == objectToDelete.version {
			result.HighestNonPendingVersion = 0
		}

		// match behavior of postgresTransactionAdapter, to appease a test
		result.Deleted = nil

		return result, nil
	}

	// TODO(spanner): make sure this is an efficient query
	segmentDeletion := spanner.Statement{
		SQL: `
			DELETE FROM segments
			WHERE stream_id = @stream_id
		`,
		Params: map[string]interface{}{
			"stream_id": result.Deleted[0].StreamID,
		},
	}
	segmentsDeleted, err := stx.tx.UpdateWithOptions(ctx, segmentDeletion, spanner.QueryOptions{RequestTag: "precommit-delete-unversioned-with-non-pending-using-object-lock-segments"})
	if err != nil {
		return PrecommitConstraintWithNonPendingResult{}, Error.New("unable to delete segments: %w", err)
	}

	result.DeletedObjectCount = 1
	result.DeletedSegmentCount = int(segmentsDeleted)

	return result, nil
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
	// Unversioned is the unversioned object at the given location. It is only
	// returned when params.Unversioned is true.
	//
	// This is used to delete the previous unversioned object at the location,
	// which ensures that there's only one unversioned object at a given location.
	Unversioned *PrecommitUnversionedObject
}

// PrecommitUnversionedObject is information necessary to delete unversioned object
// at a given location.
type PrecommitUnversionedObject struct {
	Version       Version          `spanner:"version"`
	StreamID      uuid.UUID        `spanner:"stream_id"`
	SegmentCount  int64            `spanner:"segment_count"`
	RetentionMode RetentionMode    `spanner:"retention_mode"`
	RetainUntil   spanner.NullTime `spanner:"retain_until"`
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

		// segments
		err = withRows(ptx.tx.QueryContext(ctx, `
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
	if opts.Unversioned {
		err := withRows(ptx.tx.QueryContext(ctx, `
			SELECT version, stream_id, segment_count, retention_mode, retain_until
			FROM objects
			WHERE (project_id, bucket_name, object_key) = ($1, $2, $3)
				AND version > 0
				AND status IN `+statusesUnversioned+`
		`, opts.ProjectID, opts.BucketName, opts.ObjectKey))(func(rows tagsql.Rows) error {
			for rows.Next() {
				var unversioned PrecommitUnversionedObject
				if err := rows.Scan(&unversioned.Version, &unversioned.StreamID, &unversioned.SegmentCount, &unversioned.RetentionMode, &unversioned.RetainUntil); err != nil {
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
				segment_count,
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

		stmt.SQL += `,(SELECT ARRAY(
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
			)),
			(SELECT ARRAY(
				SELECT AS STRUCT position, encrypted_size, plain_offset, plain_size
				FROM segments
				WHERE stream_id = @stream_id
				ORDER BY position
			))`
		stmt.Params["version"] = opts.Version
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

	if opts.Unversioned {
		stmt.SQL += `,(SELECT ARRAY(
				SELECT AS STRUCT version, stream_id, segment_count, retention_mode, retain_until
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

		if opts.Unversioned {
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
