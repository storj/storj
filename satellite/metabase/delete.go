// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/dx"
	"storj.io/storj/shared/dbutil/tidbutil"
	"storj.io/storj/shared/s3event"
	"storj.io/storj/shared/tagsql"
)

const (
	retentionErrMsg                 = "object is protected by a retention period"
	legalHoldErrMsg                 = "object is protected by a legal hold"
	multipleCommittedVersionsErrMsg = "internal error: multiple committed unversioned objects"
)

// ErrObjectLock is used when an object's Object Lock configuration prevents
// an operation from succeeding.
var ErrObjectLock = errs.Class("object lock")

// ObjectLockDeleteOptions contains options specifying how objects that may be subject to
// Object Lock restrictions should be deleted.
type ObjectLockDeleteOptions struct {
	// Enabled indicates that locked objects should be protected from deletion.
	Enabled bool

	// BypassGovernance allows governance mode retention restrictions to be bypassed.
	BypassGovernance bool
}

// DeleteObjectExactVersion contains arguments necessary for deleting an exact version of object.
type DeleteObjectExactVersion struct {
	Version        Version
	StreamIDSuffix StreamIDSuffix
	ObjectLocation

	ObjectLock ObjectLockDeleteOptions

	TransmitEvent bool
}

// Verify delete object fields.
func (obj *DeleteObjectExactVersion) Verify() error {
	if err := obj.ObjectLocation.Verify(); err != nil {
		return err
	}
	if obj.Version == 0 {
		return ErrInvalidRequest.New("Version invalid: %v", obj.Version)
	}
	return nil
}

// DeleteObjectResult result of deleting object.
type DeleteObjectResult struct {
	// Removed contains the list of objects that were removed from the metabase.
	Removed []Object
	// Markers contains the delete markers that were added.
	Markers []Object
	// DeletedSegmentCount is the number of segments that were deleted.
	DeletedSegmentCount int
}

// DeleteObjectExactVersion deletes an exact object version.
func (db *DB) DeleteObjectExactVersion(ctx context.Context, opts DeleteObjectExactVersion) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return DeleteObjectResult{}, err
	}
	result, err = db.ChooseAdapter(opts.ProjectID).DeleteObjectExactVersion(ctx, opts)
	if err != nil {
		return DeleteObjectResult{}, err
	}

	mon.Meter("object_delete").Mark(len(result.Removed))
	mon.Meter("segment_delete").Mark(result.DeletedSegmentCount)

	return result, nil
}

// DeleteObjectExactVersion deletes an exact object version.
func (p *PostgresAdapter) DeleteObjectExactVersion(ctx context.Context, opts DeleteObjectExactVersion) (DeleteObjectResult, error) {
	if opts.ObjectLock.Enabled {
		return p.deleteObjectExactVersionUsingObjectLock(ctx, opts)
	}
	return p.deleteObjectExactVersion(ctx, opts)
}

func (p *PostgresAdapter) deleteObjectExactVersion(ctx context.Context, opts DeleteObjectExactVersion) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	args := []any{
		opts.ProjectID,
		opts.BucketName,
		opts.ObjectKey,
		opts.Version,
	}

	var streamIDFilter string
	if !opts.StreamIDSuffix.IsZero() {
		streamIDFilter = "AND SUBSTR(stream_id, 9) = $5"
		args = append(args, opts.StreamIDSuffix)
	}

	err = withRows(
		p.db.QueryContext(ctx, `
			WITH deleted_objects AS (
				DELETE FROM objects
				WHERE (project_id, bucket_name, object_key, version) = ($1, $2, $3, $4)
				`+streamIDFilter+`
				RETURNING
					version, stream_id, created_at, expires_at, status, segment_count,
					encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key, encrypted_etag,
					checksum,
					total_plain_size, total_encrypted_size,
					fixed_segment_size, encryption,
					retention_mode, retain_until
			), deleted_segments AS (
				DELETE FROM segments
				WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
				RETURNING segments.stream_id
			)
			SELECT *, (SELECT COUNT(*) FROM deleted_segments) FROM deleted_objects`,
			args...),
	)(func(rows tagsql.Rows) error {
		result.Removed, result.DeletedSegmentCount, err = scanObjectDeletionPostgres(ctx, opts.ObjectLocation, rows)
		return err
	})
	return result, err
}

func (p *PostgresAdapter) deleteObjectExactVersionUsingObjectLock(ctx context.Context, opts DeleteObjectExactVersion) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	var (
		object  *Object
		deleted bool
	)

	now := time.Now().Truncate(time.Microsecond)

	args := []any{
		opts.ProjectID,
		opts.BucketName,
		opts.ObjectKey,
		opts.Version,
		opts.ObjectLock.BypassGovernance,
		now,
	}

	var streamIDFilter string
	if !opts.StreamIDSuffix.IsZero() {
		streamIDFilter = "AND SUBSTR(stream_id, 9) = $7"
		args = append(args, opts.StreamIDSuffix)
	}

	err = withRows(p.db.QueryContext(ctx, `
		WITH objects_to_delete AS (
			SELECT
				version, stream_id, created_at, expires_at, status, segment_count,
				encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key, encrypted_etag,
				checksum,
				total_plain_size, total_encrypted_size,
				fixed_segment_size, encryption,
				retention_mode, retain_until
			FROM objects
			WHERE (project_id, bucket_name, object_key, version) = ($1, $2, $3, $4)
			`+streamIDFilter+`
		), deleted_objects AS (
			DELETE FROM objects
			WHERE
				(project_id, bucket_name, object_key, version) = ($1, $2, $3, $4)
				`+streamIDFilter+`
				AND CASE
					WHEN status = `+statusPending+` THEN TRUE
					WHEN COALESCE(retention_mode, `+retentionModeNone+`) = 0 THEN TRUE
					WHEN retention_mode & `+retentionModeLegalHold+` != 0 THEN FALSE
					WHEN retain_until IS NULL THEN FALSE -- invalid
					ELSE CASE retention_mode
						WHEN `+retentionModeCompliance+` THEN retain_until <= $6
						WHEN `+retentionModeGovernance+` THEN $5 OR retain_until <= $6
						ELSE FALSE -- invalid
					END
				END
			RETURNING stream_id
		), deleted_segments AS (
			DELETE FROM segments
			WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
			RETURNING segments.stream_id
		)
		SELECT
			*,
			EXISTS(SELECT 1 FROM deleted_objects),
			(SELECT COUNT(*) FROM deleted_segments)
		FROM objects_to_delete
		`, args...,
	))(func(rows tagsql.Rows) error {
		if !rows.Next() {
			return nil
		}

		object = &Object{
			ObjectStream: ObjectStream{
				ProjectID:  opts.ProjectID,
				BucketName: opts.BucketName,
				ObjectKey:  opts.ObjectKey,
			},
		}

		err = rows.Scan(
			&object.Version, &object.StreamID,
			&object.CreatedAt, &object.ExpiresAt,
			&object.Status, &object.SegmentCount,
			&object.EncryptedMetadataNonce, &object.EncryptedMetadata, &object.EncryptedMetadataEncryptedKey, &object.EncryptedETag,
			&object.Checksum,
			&object.TotalPlainSize, &object.TotalEncryptedSize, &object.FixedSegmentSize,
			&object.Encryption,
			lockModeWrapper{
				retentionMode: &object.Retention.Mode,
				legalHold:     &object.LegalHold,
			},
			timeWrapper{&object.Retention.RetainUntil},
			&deleted,
			&result.DeletedSegmentCount,
		)
		if err != nil {
			return errs.New("unable to delete object: %w", err)
		}

		return nil
	})
	if err != nil {
		return DeleteObjectResult{}, Error.Wrap(err)
	}

	if object == nil {
		return DeleteObjectResult{}, nil
	}

	if !deleted {
		if object.Status != Pending {
			if err = object.Retention.Verify(); err != nil {
				return DeleteObjectResult{}, Error.Wrap(err)
			}
			switch {
			case object.LegalHold:
				return DeleteObjectResult{}, ErrObjectLock.New(legalHoldErrMsg)
			case object.Retention.isProtected(opts.ObjectLock.BypassGovernance, now):
				return DeleteObjectResult{}, ErrObjectLock.New(retentionErrMsg)
			}
		}
		return DeleteObjectResult{}, Error.New("unable to delete object")
	}

	result.Removed = []Object{*object}
	return result, nil
}

// DeleteObjectExactVersion deletes an exact object version.
func (t *TiDBAdapter) DeleteObjectExactVersion(ctx context.Context, opts DeleteObjectExactVersion) (DeleteObjectResult, error) {
	if opts.ObjectLock.Enabled {
		return t.deleteObjectExactVersionUsingObjectLock(ctx, opts)
	}
	return t.deleteObjectExactVersion(ctx, opts)
}

func (t *TiDBAdapter) deleteObjectExactVersion(ctx context.Context, opts DeleteObjectExactVersion) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	// The FOR UPDATE select folds BEGIN into its first statement, and the two
	// enqueued DELETEs flush together with COMMIT in one round trip via
	// CommitWithResults, cutting this read-then-write from four round trips to
	// two. CommitWithResults reports the segments DELETE's affected-row count.
	err = tidbutil.WithTx(ctx, t.db, func(ctx context.Context, tx *tidbutil.Tx) error {
		// reset on retry
		result = DeleteObjectResult{}

		args := []any{opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.Version}
		streamIDFilter := ""
		if !opts.StreamIDSuffix.IsZero() {
			streamIDFilter = " AND SUBSTRING(stream_id, 9) = ?"
			args = append(args, opts.StreamIDSuffix)
		}

		var streamIDs [][]byte
		err := dx.WithRows(tx.QueryContext(ctx, `
			SELECT
				version, stream_id, created_at, expires_at, status, segment_count,
				encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key, encrypted_etag,
				checksum,
				total_plain_size, total_encrypted_size,
				fixed_segment_size, encryption,
				retention_mode, retain_until
			FROM objects
			WHERE (project_id, bucket_name, object_key, version) = (?, ?, ?, ?)`+streamIDFilter+`
			FOR UPDATE
		`, args...))(func(rows dx.Rows) error {
			for rows.Next() {
				var object Object
				object.ProjectID = opts.ProjectID
				object.BucketName = opts.BucketName
				object.ObjectKey = opts.ObjectKey
				if err := rows.Scan(
					&object.Version, &object.StreamID,
					&object.CreatedAt, &object.ExpiresAt,
					&object.Status, &object.SegmentCount,
					&object.EncryptedMetadataNonce, &object.EncryptedMetadata, &object.EncryptedMetadataEncryptedKey, &object.EncryptedETag,
					&object.Checksum,
					&object.TotalPlainSize, &object.TotalEncryptedSize, &object.FixedSegmentSize,
					&object.Encryption,
					lockModeWrapper{retentionMode: &object.Retention.Mode, legalHold: &object.LegalHold},
					timeWrapper{&object.Retention.RetainUntil},
				); err != nil {
					return Error.New("unable to delete object: %w", err)
				}
				result.Removed = append(result.Removed, object)
				streamIDs = append(streamIDs, object.StreamID.Bytes())
			}
			return nil
		})
		if err != nil {
			return Error.Wrap(err)
		}

		if len(result.Removed) == 0 {
			return nil
		}

		// Enqueue DELETE objects and DELETE segments; CommitWithResults flushes
		// both with COMMIT in one round trip and returns each statement's
		// affected-row count, so result[1] yields the deleted segment count.
		objArgs := []any{opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.Version}
		if !opts.StreamIDSuffix.IsZero() {
			objArgs = append(objArgs, opts.StreamIDSuffix)
		}
		segArgs := make([]any, 0, len(streamIDs))
		for _, sid := range streamIDs {
			segArgs = append(segArgs, sid)
		}
		tx.EnqueueExec(`DELETE FROM objects WHERE (project_id, bucket_name, object_key, version) = (?, ?, ?, ?)`+streamIDFilter, objArgs...)
		tx.EnqueueExec(`DELETE FROM segments WHERE stream_id IN (`+tidbPlaceholders(len(streamIDs))+`)`, segArgs...)
		if opts.TransmitEvent {
			events := make([]BucketEvent, len(result.Removed))
			for i, object := range result.Removed {
				events[i] = BucketEvent{
					EventName:      s3event.ObjectRemovedDelete.Name(),
					ObjectStream:   object.ObjectStream,
					TotalPlainSize: object.TotalPlainSize,
				}
			}
			tidbEnqueueBucketEvent(tx, events...)
		}
		results, err := tx.CommitWithResults(ctx)
		if err != nil {
			return Error.Wrap(err)
		}
		result.DeletedSegmentCount = int(results[1].RowsAffected)

		return nil
	})
	if err != nil {
		return DeleteObjectResult{}, err
	}
	return result, nil
}

func (t *TiDBAdapter) deleteObjectExactVersionUsingObjectLock(ctx context.Context, opts DeleteObjectExactVersion) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	now := time.Now().Truncate(time.Microsecond)

	// The FOR UPDATE select folds BEGIN into its first statement, and the two
	// enqueued DELETEs flush together with COMMIT in one round trip via
	// CommitWithResults, cutting this read-then-write from four round trips to
	// two. CommitWithResults reports the segments DELETE's affected-row count.
	err = tidbutil.WithTx(ctx, t.db, func(ctx context.Context, tx *tidbutil.Tx) error {
		// reset on retry
		result = DeleteObjectResult{}

		args := []any{opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.Version}
		streamIDFilter := ""
		if !opts.StreamIDSuffix.IsZero() {
			streamIDFilter = " AND SUBSTRING(stream_id, 9) = ?"
			args = append(args, opts.StreamIDSuffix)
		}

		var object Object
		object.ProjectID = opts.ProjectID
		object.BucketName = opts.BucketName
		object.ObjectKey = opts.ObjectKey
		err := tx.QueryRowContext(ctx, `
			SELECT
				version, stream_id, created_at, expires_at, status, segment_count,
				encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key, encrypted_etag,
				checksum,
				total_plain_size, total_encrypted_size,
				fixed_segment_size, encryption,
				retention_mode, retain_until
			FROM objects
			WHERE (project_id, bucket_name, object_key, version) = (?, ?, ?, ?)`+streamIDFilter+`
			FOR UPDATE
		`, args...).Scan(
			&object.Version, &object.StreamID,
			&object.CreatedAt, &object.ExpiresAt,
			&object.Status, &object.SegmentCount,
			&object.EncryptedMetadataNonce, &object.EncryptedMetadata, &object.EncryptedMetadataEncryptedKey, &object.EncryptedETag,
			&object.Checksum,
			&object.TotalPlainSize, &object.TotalEncryptedSize, &object.FixedSegmentSize,
			&object.Encryption,
			lockModeWrapper{retentionMode: &object.Retention.Mode, legalHold: &object.LegalHold},
			timeWrapper{&object.Retention.RetainUntil},
		)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			return Error.Wrap(err)
		}

		if object.Status != Pending {
			if err := object.Retention.Verify(); err != nil {
				return Error.Wrap(err)
			}
			switch {
			case object.LegalHold:
				return ErrObjectLock.New(legalHoldErrMsg)
			case object.Retention.isProtected(opts.ObjectLock.BypassGovernance, now):
				return ErrObjectLock.New(retentionErrMsg)
			}
		}

		// Enqueue DELETE objects and DELETE segments; CommitWithResults flushes
		// both with COMMIT in one round trip and returns each statement's
		// affected-row count, so result[1] yields the deleted segment count.
		objArgs := []any{opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.Version}
		if !opts.StreamIDSuffix.IsZero() {
			objArgs = append(objArgs, opts.StreamIDSuffix)
		}
		tx.EnqueueExec(`DELETE FROM objects WHERE (project_id, bucket_name, object_key, version) = (?, ?, ?, ?)`+streamIDFilter, objArgs...)
		tx.EnqueueExec(`DELETE FROM segments WHERE stream_id = ?`, object.StreamID.Bytes())
		if opts.TransmitEvent {
			tidbEnqueueBucketEvent(tx, BucketEvent{
				EventName:      s3event.ObjectRemovedDelete.Name(),
				ObjectStream:   object.ObjectStream,
				TotalPlainSize: object.TotalPlainSize,
			})
		}
		results, err := tx.CommitWithResults(ctx)
		if err != nil {
			return Error.Wrap(err)
		}
		result.DeletedSegmentCount = int(results[1].RowsAffected)
		result.Removed = []Object{object}

		return nil
	})
	if err != nil {
		return DeleteObjectResult{}, err
	}
	return result, nil
}

// DeletePendingObject contains arguments necessary for deleting a pending object.
type DeletePendingObject struct {
	ObjectStream

	MaxCommitDelay *time.Duration
}

// Verify verifies delete pending object fields validity.
func (opts *DeletePendingObject) Verify() error {
	if err := opts.ObjectStream.Verify(); err != nil {
		return err
	}
	return nil
}

// DeletePendingObject deletes a pending object with specified version and streamID.
func (db *DB) DeletePendingObject(ctx context.Context, opts DeletePendingObject) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return DeleteObjectResult{}, err
	}

	result, err = db.ChooseAdapter(opts.ProjectID).DeletePendingObject(ctx, opts)
	if err != nil {
		return DeleteObjectResult{}, err
	}

	if len(result.Removed) == 0 {
		return DeleteObjectResult{}, ErrObjectNotFound.Wrap(Error.New("no rows deleted"))
	}

	mon.Meter("object_delete").Mark(len(result.Removed))
	mon.Meter("segment_delete").Mark(result.DeletedSegmentCount)

	return result, nil
}

// DeletePendingObject soft-deletes a pending object with specified version and streamID
// by setting expires_at to now() on the object and its segments.
func (p *PostgresAdapter) DeletePendingObject(ctx context.Context, opts DeletePendingObject) (result DeleteObjectResult, err error) {
	// because update is using full primary key we are sure only one object will be updated
	var totalUpdatedObjects int
	err = withRows(p.db.QueryContext(ctx, `
			WITH updated_objects AS (
				UPDATE objects
				SET expires_at = now()
				WHERE
					(project_id, bucket_name, object_key, version, stream_id) = ($1, $2, $3, $4, $5) AND
					status = `+statusPending+` AND (expires_at IS NULL OR expires_at >= now())
				RETURNING stream_id
			), updated_segments AS (
				UPDATE segments
				SET expires_at = now()
				WHERE segments.stream_id IN (SELECT updated_objects.stream_id FROM updated_objects)
				RETURNING 1
			)
			SELECT (SELECT COUNT(*) FROM updated_objects)
		`, opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.Version, opts.StreamID))(func(rows tagsql.Rows) error {
		var updatedObjects int
		for rows.Next() {
			if err := rows.Scan(&updatedObjects); err != nil {
				return err
			}
		}
		totalUpdatedObjects += updatedObjects
		return nil
	})
	if err != nil {
		return DeleteObjectResult{}, Error.Wrap(err)
	}

	if totalUpdatedObjects == 0 {
		return result, nil
	}

	result.Removed = append(result.Removed, Object{
		ObjectStream: opts.ObjectStream,
		Status:       Pending,
	})
	return result, nil
}

// DeletePendingObject soft-deletes a pending object with specified version and streamID
// by setting expires_at to NOW on the object and its segments.
func (t *TiDBAdapter) DeletePendingObject(ctx context.Context, opts DeletePendingObject) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	// The objects UPDATE folds BEGIN into its first statement; its affected-row
	// count decides whether the segments need updating. When it does, the
	// segments UPDATE folds into COMMIT via CommitWithExec, so the two writes
	// cost two round trips instead of four; when it matched nothing, the empty
	// COMMIT still finalizes the transaction in a second round trip.
	err = tidbutil.WithTx(ctx, t.db, func(ctx context.Context, tx *tidbutil.Tx) error {
		// Reset result in case the transaction is retried.
		result = DeleteObjectResult{}

		res, err := tx.ExecContext(ctx, `
			UPDATE objects
			SET expires_at = NOW(6)
			WHERE
				(project_id, bucket_name, object_key, version, stream_id) = (?, ?, ?, ?, ?)
				AND status = `+statusPending+`
				AND (expires_at IS NULL OR expires_at >= NOW(6))
		`, opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.Version, opts.StreamID)
		if err != nil {
			return Error.Wrap(err)
		}
		count, err := res.RowsAffected()
		if err != nil {
			return Error.Wrap(err)
		}
		if count == 0 {
			return nil
		}

		if err := tx.CommitWithExec(ctx, `UPDATE segments SET expires_at = NOW(6) WHERE stream_id = ?`, opts.StreamID); err != nil {
			return Error.Wrap(err)
		}

		result.Removed = append(result.Removed, Object{
			ObjectStream: opts.ObjectStream,
			Status:       Pending,
		})
		return nil
	})
	if err != nil {
		return DeleteObjectResult{}, err
	}
	return result, nil
}

// scanObjectDeletionPostgres reads in the results of an object deletion from the database.
func scanObjectDeletionPostgres(ctx context.Context, location ObjectLocation, rows tagsql.Rows) (objects []Object, deletedSegmentCount int, err error) {
	defer mon.Task()(&ctx)(&err)

	objects = make([]Object, 0, 10)

	var object Object
	for rows.Next() {
		object.ProjectID = location.ProjectID
		object.BucketName = location.BucketName
		object.ObjectKey = location.ObjectKey

		err = rows.Scan(&object.Version, &object.StreamID,
			&object.CreatedAt, &object.ExpiresAt,
			&object.Status, &object.SegmentCount,
			&object.EncryptedMetadataNonce, &object.EncryptedMetadata, &object.EncryptedMetadataEncryptedKey, &object.EncryptedETag,
			&object.Checksum,
			&object.TotalPlainSize, &object.TotalEncryptedSize, &object.FixedSegmentSize,
			&object.Encryption,
			lockModeWrapper{
				retentionMode: &object.Retention.Mode,
				legalHold:     &object.LegalHold,
			},
			timeWrapper{&object.Retention.RetainUntil},
			&deletedSegmentCount,
		)
		if err != nil {
			return objects, deletedSegmentCount, Error.New("unable to delete object: %w", err)
		}

		objects = append(objects, object)
	}

	return objects, deletedSegmentCount, nil
}

// DeleteObjectLastCommitted contains arguments necessary for deleting last committed version of object.
type DeleteObjectLastCommitted struct {
	ObjectLocation

	Versioned bool
	Suspended bool

	ObjectLock ObjectLockDeleteOptions

	TransmitEvent bool
}

// Verify delete object last committed fields.
func (obj *DeleteObjectLastCommitted) Verify() error {
	if obj.Versioned && obj.Suspended {
		return ErrInvalidRequest.New("versioned and suspended cannot be enabled at the same time")
	}
	return obj.ObjectLocation.Verify()
}

// DeleteObjectLastCommitted deletes an object last committed version.
func (db *DB) DeleteObjectLastCommitted(
	ctx context.Context, opts DeleteObjectLastCommitted,
) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return DeleteObjectResult{}, err
	}

	if opts.Suspended {
		deleterMarkerStreamID, err := generateDeleteMarkerStreamID()
		if err != nil {
			return DeleteObjectResult{}, Error.Wrap(err)
		}

		return db.DeleteObjectLastCommittedSuspended(ctx, opts, deleterMarkerStreamID)
	}
	if opts.Versioned {
		// Instead of deleting we insert a deletion marker.
		deleterMarkerStreamID, err := generateDeleteMarkerStreamID()
		if err != nil {
			return DeleteObjectResult{}, Error.Wrap(err)
		}

		return db.ChooseAdapter(opts.ProjectID).DeleteObjectLastCommittedVersioned(ctx, opts, deleterMarkerStreamID)
	}

	result, err = db.ChooseAdapter(opts.ProjectID).DeleteObjectLastCommittedPlain(ctx, opts)
	if err != nil {
		return DeleteObjectResult{}, err
	}

	mon.Meter("object_delete").Mark(len(result.Removed))
	if result.DeletedSegmentCount > 0 {
		mon.Meter("segment_delete").Mark(result.DeletedSegmentCount)
	}

	return result, nil
}

// DeleteObjectLastCommittedPlain deletes an object last committed version when
// opts.Suspended and opts.Versioned are both false.
func (p *PostgresAdapter) DeleteObjectLastCommittedPlain(ctx context.Context, opts DeleteObjectLastCommitted) (result DeleteObjectResult, err error) {
	if opts.ObjectLock.Enabled {
		return p.deleteObjectLastCommittedPlainUsingObjectLock(ctx, opts)
	}
	return p.deleteObjectLastCommittedPlain(ctx, opts)
}

func (p *PostgresAdapter) deleteObjectLastCommittedPlain(ctx context.Context, opts DeleteObjectLastCommitted) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)
	// TODO(ver): do we need to pretend here that `expires_at` matters?
	// TODO(ver): should this report an error when the object doesn't exist?
	err = withRows(
		p.db.QueryContext(ctx, `
			WITH deleted_objects AS (
				DELETE FROM objects
				WHERE
					(project_id, bucket_name, object_key) = ($1, $2, $3) AND
					status = `+statusCommittedUnversioned+` AND
					(expires_at IS NULL OR expires_at > now())
				RETURNING
					version, stream_id,
					created_at, expires_at,
					status, segment_count,
					encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key, encrypted_etag,
					checksum,
					total_plain_size, total_encrypted_size, fixed_segment_size,
					encryption,
					retention_mode, retain_until
			), deleted_segments AS (
				DELETE FROM segments
				WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
				RETURNING segments.stream_id
			)
			SELECT *, (SELECT COUNT(*) FROM deleted_segments) FROM deleted_objects`,
			opts.ProjectID, opts.BucketName, opts.ObjectKey),
	)(func(rows tagsql.Rows) error {
		result.Removed, result.DeletedSegmentCount, err = scanObjectDeletionPostgres(ctx, opts.ObjectLocation, rows)
		return err
	})
	return result, Error.Wrap(err)
}

func (p *PostgresAdapter) deleteObjectLastCommittedPlainUsingObjectLock(ctx context.Context, opts DeleteObjectLastCommitted) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	now := time.Now().Truncate(time.Microsecond)

	var (
		object  *Object
		deleted bool
	)
	err = withRows(p.db.QueryContext(ctx, `
		WITH objects_to_delete AS (
			SELECT
				version, stream_id, created_at, expires_at, status, segment_count,
				encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key, encrypted_etag,
				checksum,
				total_plain_size, total_encrypted_size,
				fixed_segment_size, encryption,
				retention_mode, retain_until
			FROM objects
			WHERE
				(project_id, bucket_name, object_key) = ($1, $2, $3)
				AND status = `+statusCommittedUnversioned+`
				AND (expires_at IS NULL OR expires_at > now())
			ORDER BY version DESC LIMIT 1
		), deleted_objects AS (
			DELETE FROM objects
			WHERE
				(project_id, bucket_name, object_key) = ($1, $2, $3)
				AND version IN (SELECT version FROM objects_to_delete)
				AND CASE
					WHEN COALESCE(retention_mode, `+retentionModeNone+`) = 0 THEN TRUE
					WHEN retention_mode & `+retentionModeLegalHold+` != 0 THEN FALSE
					WHEN retain_until IS NULL THEN FALSE -- invalid
					ELSE CASE retention_mode
						WHEN `+retentionModeCompliance+` THEN retain_until <= $5
						WHEN `+retentionModeGovernance+` THEN $4 OR retain_until <= $5
						ELSE FALSE -- invalid
					END
				END
			RETURNING stream_id
		), deleted_segments AS (
			DELETE FROM segments
			WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
			RETURNING 1
		)
		SELECT
			*,
			EXISTS(SELECT 1 FROM deleted_objects),
			(SELECT COUNT(*) FROM deleted_segments)
		FROM objects_to_delete
		`, opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.ObjectLock.BypassGovernance, now,
	))(func(rows tagsql.Rows) error {
		if !rows.Next() {
			return nil
		}

		object = &Object{
			ObjectStream: ObjectStream{
				ProjectID:  opts.ProjectID,
				BucketName: opts.BucketName,
				ObjectKey:  opts.ObjectKey,
			},
		}

		err = rows.Scan(
			&object.Version, &object.StreamID,
			&object.CreatedAt, &object.ExpiresAt,
			&object.Status, &object.SegmentCount,
			&object.EncryptedMetadataNonce, &object.EncryptedMetadata, &object.EncryptedMetadataEncryptedKey, &object.EncryptedETag,
			&object.Checksum,
			&object.TotalPlainSize, &object.TotalEncryptedSize, &object.FixedSegmentSize,
			&object.Encryption,
			lockModeWrapper{
				retentionMode: &object.Retention.Mode,
				legalHold:     &object.LegalHold,
			}, timeWrapper{&object.Retention.RetainUntil},
			&deleted,
			&result.DeletedSegmentCount,
		)
		if err != nil {
			return errs.New("unable to delete object: %w", err)
		}

		return nil
	})
	if err != nil {
		return DeleteObjectResult{}, Error.Wrap(err)
	}

	if object == nil {
		return result, nil
	}

	if !deleted {
		if err = object.Retention.Verify(); err != nil {
			return DeleteObjectResult{}, Error.Wrap(err)
		}
		switch {
		case object.LegalHold:
			return DeleteObjectResult{}, ErrObjectLock.New(legalHoldErrMsg)
		case object.Retention.isProtected(opts.ObjectLock.BypassGovernance, now):
			return DeleteObjectResult{}, ErrObjectLock.New(retentionErrMsg)
		default:
			return DeleteObjectResult{}, Error.New("unable to delete object")
		}
	}

	result.Removed = []Object{*object}
	return result, nil
}

// DeleteObjectLastCommittedPlain deletes an object last committed version when
// opts.Suspended and opts.Versioned are both false.
func (t *TiDBAdapter) DeleteObjectLastCommittedPlain(ctx context.Context, opts DeleteObjectLastCommitted) (DeleteObjectResult, error) {
	if opts.ObjectLock.Enabled {
		return t.deleteObjectLastCommittedPlainUsingObjectLock(ctx, opts)
	}
	return t.deleteObjectLastCommittedPlain(ctx, opts)
}

func (t *TiDBAdapter) deleteObjectLastCommittedPlain(ctx context.Context, opts DeleteObjectLastCommitted) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	// The FOR UPDATE select folds BEGIN into its first statement and
	// CommitWithExec folds the combined DELETEs with COMMIT, cutting this
	// read-then-write from four round trips to two. The deleted segment count
	// comes from the selected objects, not the DELETE result.
	err = tidbutil.WithTx(ctx, t.db, func(ctx context.Context, tx *tidbutil.Tx) error {
		// Reset result in case the transaction is retried.
		result = DeleteObjectResult{}

		err := dx.WithRows(tx.QueryContext(ctx, `
			SELECT
				version, stream_id,
				created_at, expires_at,
				status, segment_count,
				encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key, encrypted_etag,
				checksum,
				total_plain_size, total_encrypted_size, fixed_segment_size,
				encryption,
				retention_mode, retain_until
			FROM objects
			WHERE (project_id, bucket_name, object_key) = (?, ?, ?)
			  AND status = `+statusCommittedUnversioned+`
			  AND (expires_at IS NULL OR expires_at > NOW(6))
			FOR UPDATE
		`, opts.ProjectID, opts.BucketName, opts.ObjectKey))(func(rows dx.Rows) error {
			for rows.Next() {
				var object Object
				object.ProjectID = opts.ProjectID
				object.BucketName = opts.BucketName
				object.ObjectKey = opts.ObjectKey
				if err := rows.Scan(
					&object.Version, &object.StreamID,
					&object.CreatedAt, &object.ExpiresAt,
					&object.Status, &object.SegmentCount,
					&object.EncryptedMetadataNonce, &object.EncryptedMetadata, &object.EncryptedMetadataEncryptedKey, &object.EncryptedETag,
					&object.Checksum,
					&object.TotalPlainSize, &object.TotalEncryptedSize, &object.FixedSegmentSize,
					&object.Encryption,
					lockModeWrapper{retentionMode: &object.Retention.Mode, legalHold: &object.LegalHold},
					timeWrapper{&object.Retention.RetainUntil},
				); err != nil {
					return Error.New("unable to delete object: %w", err)
				}
				result.Removed = append(result.Removed, object)
			}
			return nil
		})
		if err != nil {
			return Error.Wrap(err)
		}

		if len(result.Removed) == 0 {
			return nil
		}

		// Delete the objects and their segments in a single multi-statement
		// round-trip folded with COMMIT. There is at most one
		// committed-unversioned object per location, so the IN(...) lists stay
		// well under MySQL's uint16 placeholder limit and need no chunking.
		n := len(result.Removed)
		args := make([]any, 0, 3+2*n)
		args = append(args, opts.ProjectID, opts.BucketName, opts.ObjectKey)
		for _, object := range result.Removed {
			args = append(args, object.Version)
		}
		for _, object := range result.Removed {
			args = append(args, object.StreamID.Bytes())
		}

		if opts.TransmitEvent {
			events := make([]BucketEvent, len(result.Removed))
			for i, object := range result.Removed {
				events[i] = BucketEvent{
					EventName:      s3event.ObjectRemovedDelete.Name(),
					ObjectStream:   object.ObjectStream,
					TotalPlainSize: object.TotalPlainSize,
				}
			}
			tidbEnqueueBucketEvent(tx, events...)
		}
		if err = tx.CommitWithExec(ctx, `DELETE FROM objects WHERE (project_id, bucket_name, object_key) = (?, ?, ?) AND version IN (`+
			tidbPlaceholders(n)+`);`+
			`DELETE FROM segments WHERE stream_id IN (`+tidbPlaceholders(n)+`)`, args...); err != nil {
			return Error.New("unable to delete object: %w", err)
		}
		for _, object := range result.Removed {
			result.DeletedSegmentCount += int(object.SegmentCount)
		}

		return nil
	})
	if err != nil {
		return DeleteObjectResult{}, err
	}
	return result, nil
}

func (t *TiDBAdapter) deleteObjectLastCommittedPlainUsingObjectLock(ctx context.Context, opts DeleteObjectLastCommitted) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	now := time.Now().Truncate(time.Microsecond)

	// The FOR UPDATE select folds BEGIN into its first statement and
	// CommitWithExec folds the combined DELETEs with COMMIT, cutting this
	// read-then-write from four round trips to two. The deleted segment count
	// comes from the selected object, not the DELETE result.
	err = tidbutil.WithTx(ctx, t.db, func(ctx context.Context, tx *tidbutil.Tx) error {
		// Reset result in case the transaction is retried.
		result = DeleteObjectResult{}

		var object Object
		object.ProjectID = opts.ProjectID
		object.BucketName = opts.BucketName
		object.ObjectKey = opts.ObjectKey
		err := tx.QueryRowContext(ctx, `
			SELECT
				version, stream_id, created_at, expires_at, status, segment_count,
				encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key, encrypted_etag,
				checksum,
				total_plain_size, total_encrypted_size, fixed_segment_size,
				encryption,
				retention_mode, retain_until
			FROM objects
			WHERE (project_id, bucket_name, object_key) = (?, ?, ?)
			  AND status = `+statusCommittedUnversioned+`
			  AND (expires_at IS NULL OR expires_at > NOW(6))
			ORDER BY version DESC
			LIMIT 1
			FOR UPDATE
		`, opts.ProjectID, opts.BucketName, opts.ObjectKey).Scan(
			&object.Version, &object.StreamID,
			&object.CreatedAt, &object.ExpiresAt,
			&object.Status, &object.SegmentCount,
			&object.EncryptedMetadataNonce, &object.EncryptedMetadata, &object.EncryptedMetadataEncryptedKey, &object.EncryptedETag,
			&object.Checksum,
			&object.TotalPlainSize, &object.TotalEncryptedSize, &object.FixedSegmentSize,
			&object.Encryption,
			lockModeWrapper{retentionMode: &object.Retention.Mode, legalHold: &object.LegalHold},
			timeWrapper{&object.Retention.RetainUntil},
		)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			return Error.Wrap(err)
		}

		if err = object.Retention.Verify(); err != nil {
			return Error.Wrap(err)
		}
		switch {
		case object.LegalHold:
			return ErrObjectLock.New(legalHoldErrMsg)
		case object.Retention.isProtected(opts.ObjectLock.BypassGovernance, now):
			return ErrObjectLock.New(retentionErrMsg)
		}

		if opts.TransmitEvent {
			tidbEnqueueBucketEvent(tx, BucketEvent{
				EventName:      s3event.ObjectRemovedDelete.Name(),
				ObjectStream:   object.ObjectStream,
				TotalPlainSize: object.TotalPlainSize,
			})
		}
		// Combine DELETE objects + DELETE segments into one multi-statement
		// round-trip folded with COMMIT.
		if err = tx.CommitWithExec(ctx, `
			DELETE FROM objects WHERE (project_id, bucket_name, object_key, version) = (?, ?, ?, ?);
			DELETE FROM segments WHERE stream_id = ?`,
			opts.ProjectID, opts.BucketName, opts.ObjectKey, object.Version,
			object.StreamID.Bytes()); err != nil {
			return Error.Wrap(err)
		}
		result.DeletedSegmentCount = int(object.SegmentCount)
		result.Removed = []Object{object}

		return nil
	})
	if err != nil {
		return DeleteObjectResult{}, err
	}
	return result, nil
}

// DeleteObjectLastCommittedSuspended deletes an object last committed version when opts.Suspended is true.
func (db *DB) DeleteObjectLastCommittedSuspended(ctx context.Context, opts DeleteObjectLastCommitted, deleterMarkerStreamID uuid.UUID) (result DeleteObjectResult, err error) {
	var marker Object
	var metrics commitMetrics
	mainAdapter := db.ChooseAdapter(opts.ProjectID)
	txBody := func(ctx context.Context, adapter TransactionAdapter) (err error) {
		// Reset state in case the transaction is retried.
		metrics = commitMetrics{}
		result = DeleteObjectResult{}
		marker = Object{
			ObjectStream: ObjectStream{
				ProjectID:  opts.ProjectID,
				BucketName: opts.BucketName,
				ObjectKey:  opts.ObjectKey,
				StreamID:   deleterMarkerStreamID,
			},
			Status: DeleteMarkerUnversioned,
		}

		query, err := adapter.precommitQuery(ctx, PrecommitQuery{
			ObjectStream:    marker.ObjectStream,
			FullUnversioned: true,
			HighestVisible:  true,
			Pending:         false,
		})
		if err != nil {
			return err
		}

		if query.HighestVersion == 0 || query.HighestVisible == 0 {
			// an object didn't exist in the first place
			return ErrObjectNotFound.New("unable to delete object")
		}

		if query.Unversioned != nil {
			// When committing unversioned objects we need to delete any previous unversioned objects.
			if err := commonPrecommitDeleteUnversioned(ctx, adapter, query, &metrics, precommitDeleteUnversioned{
				DisallowDelete:     false,
				BypassGovernance:   opts.ObjectLock.BypassGovernance,
				DeleteOnlySegments: false,
			}); err != nil {
				return err
			}

			result.Removed = append(result.Removed, Object(*query.FullUnversioned))
			result.DeletedSegmentCount = int(query.FullUnversioned.SegmentCount)
		}

		marker.CreatedAt = time.Now()
		marker.Version = nextVersion(0, query.HighestVersion, query.TimestampVersion, mainAdapter.Config().TestingTimestampVersioning)

		err = adapter.precommitInsertObject(ctx, &marker, nil)
		if err != nil {
			return err
		}

		result.Markers = []Object{marker}

		return nil
	}
	// On TiDB a concurrent writer can take the computed version between the
	// precommit query and the marker insert; retrying the transaction
	// recomputes the version.
	err = retryVersionConflict(ctx, func(ctx context.Context) error {
		return mainAdapter.WithTx(ctx, TransactionOptions{
			TransactionTag: "delete-object-last-committed-suspended",
		}, txBody)
	})
	if err != nil {
		if ErrObjectNotFound.Has(err) || ErrObjectLock.Has(err) {
			return DeleteObjectResult{}, err
		}
		return DeleteObjectResult{}, Error.Wrap(err)
	}

	metrics.submit()

	return result, nil
}

// DeleteObjectLastCommittedVersioned deletes an object last committed version when opts.Versioned is true.
func (p *PostgresAdapter) DeleteObjectLastCommittedVersioned(ctx context.Context, opts DeleteObjectLastCommitted, deleterMarkerStreamID uuid.UUID) (result DeleteObjectResult, err error) {
	row := p.db.QueryRowContext(ctx, `
			INSERT INTO objects (
				project_id, bucket_name, object_key, version, stream_id,
				status,
				zombie_deletion_deadline
			)
			SELECT
				$1, $2, $3, `+p.generateVersion()+`,
				$4,
				`+statusDeleteMarkerVersioned+`,
				NULL
			RETURNING version, created_at
		`, opts.ProjectID, opts.BucketName, opts.ObjectKey, deleterMarkerStreamID)

	var deleted Object
	deleted.ProjectID = opts.ProjectID
	deleted.BucketName = opts.BucketName
	deleted.ObjectKey = opts.ObjectKey
	deleted.StreamID = deleterMarkerStreamID
	deleted.Status = DeleteMarkerVersioned

	err = row.Scan(&deleted.Version, &deleted.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DeleteObjectResult{}, ErrObjectNotFound.Wrap(Error.New("object does not exist"))
		}
		return DeleteObjectResult{}, Error.Wrap(err)
	}
	return DeleteObjectResult{Markers: []Object{deleted}}, nil
}

// DeleteObjectLastCommittedVersioned deletes an object last committed version when opts.Versioned is true.
func (t *TiDBAdapter) DeleteObjectLastCommittedVersioned(ctx context.Context, opts DeleteObjectLastCommitted, deleterMarkerStreamID uuid.UUID) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	deleted := Object{
		ObjectStream: ObjectStream{
			ProjectID:  opts.ProjectID,
			BucketName: opts.BucketName,
			ObjectKey:  opts.ObjectKey,
			StreamID:   deleterMarkerStreamID,
		},
		Status: DeleteMarkerVersioned,
		// Compute created_at client-side to avoid a SELECT round trip.
		CreatedAt: time.Now().Truncate(time.Microsecond),
	}

	versionExpr := "?"
	if !t.config.TestingTimestampVersioning {
		versionExpr = tidbGenerateNextVersionLastInsertID
	}
	insertSQL := `
		INSERT INTO objects (
			project_id, bucket_name, object_key, version, stream_id,
			status, zombie_deletion_deadline, created_at
		) VALUES (
			?, ?, ?, ` + versionExpr + `, ?, ?, NULL, ?
		)`
	commonTail := []any{deleterMarkerStreamID, statusDeleteMarkerVersioned, deleted.CreatedAt}

	insertDeleteMarker := func(ctx context.Context, ex tagsql.ExecQueryer) error {
		if t.config.TestingTimestampVersioning {
			// Compute the version client-side to avoid a SELECT round trip.
			deleted.Version = Version(time.Now().UnixMicro())
			args := append([]any{opts.ProjectID, opts.BucketName, opts.ObjectKey, deleted.Version}, commonTail...)
			if _, err := ex.ExecContext(ctx, insertSQL, args...); err != nil {
				return Error.Wrap(err)
			}
		} else {
			// Non-timestamp mode: the version comes from a subquery on existing rows.
			// LAST_INSERT_ID(expr) wrapped around it makes the chosen value land in
			// the INSERT's OK-packet last_insert_id field, which the driver exposes
			// via sql.Result.LastInsertId() — no follow-up SELECT round trip needed.
			args := append([]any{
				opts.ProjectID, opts.BucketName, opts.ObjectKey,
				opts.ProjectID, opts.BucketName, opts.ObjectKey, // for tidbGenerateNextVersion subquery
			}, commonTail...)
			res, err := ex.ExecContext(ctx, insertSQL, args...)
			if err != nil {
				return Error.Wrap(err)
			}
			version, err := res.LastInsertId()
			if err != nil {
				return Error.Wrap(err)
			}
			deleted.Version = Version(version)
		}
		return nil
	}

	// TODO(tidb): combine transmit and insert delete marker into a single query.
	if opts.TransmitEvent {
		err = tidbRetryVersionConflict(ctx, func(ctx context.Context) error {
			return tidbutil.WithTx(ctx, t.db, func(ctx context.Context, tx *tidbutil.Tx) error {
				if err := insertDeleteMarker(ctx, tx); err != nil {
					return err
				}
				tidbEnqueueBucketEvent(tx, BucketEvent{
					EventName: s3event.ObjectRemovedDeleteMarkerCreated.Name(),
					ObjectStream: ObjectStream{
						ProjectID:  opts.ProjectID,
						BucketName: opts.BucketName,
						ObjectKey:  opts.ObjectKey,
						Version:    deleted.Version,
						StreamID:   deleterMarkerStreamID,
					},
				})
				return nil
			})
		})
	} else {
		err = tidbRetryVersionConflict(ctx, func(ctx context.Context) error {
			return insertDeleteMarker(ctx, t.db)
		})
	}
	if err != nil {
		return DeleteObjectResult{}, err
	}
	return DeleteObjectResult{Markers: []Object{deleted}}, nil
}

// generateDeleteMarkerStreamID returns a uuid that has the first 6 bytes as 0xff.
// Creating a stream id, where the first bytes are 0xff makes it easily recognizable as a delete marker.
func generateDeleteMarkerStreamID() (uuid.UUID, error) {
	v, err := uuid.New()
	if err != nil {
		return v, Error.Wrap(err)
	}

	for i := range v[:6] {
		v[i] = 0xFF
	}
	return v, nil
}

func logMultipleCommittedVersionsError(log *zap.Logger, loc ObjectLocation) {
	log.Error("object with multiple committed versions were found!",
		zap.Stringer("project_id", loc.ProjectID),
		zap.Stringer("bucket_name", loc.BucketName),
		zap.String("object_key", hex.EncodeToString([]byte(loc.ObjectKey))),
	)
	mon.Meter("multiple_committed_versions").Mark(1)
}
