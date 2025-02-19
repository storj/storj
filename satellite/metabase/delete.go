// Copyright (C) 2020 Storj Labs, Inc.
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
	"google.golang.org/api/iterator"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/spannerutil"
	"storj.io/storj/shared/tagsql"
)

const (
	retentionErrMsg                 = "object is protected by a retention period"
	legalHoldErrMsg                 = "object is protected by a legal hold"
	multipleCommittedVersionsErrMsg = "internal error: multiple committed unversioned objects"
)

var (
	// ErrObjectLock is used when an object's Object Lock configuration prevents
	// an operation from succeeding.
	ErrObjectLock = errs.Class("object lock")
)

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
}

// Verify delete object fields.
func (obj *DeleteObjectExactVersion) Verify() error {
	if err := obj.ObjectLocation.Verify(); err != nil {
		return err
	}
	if obj.Version <= 0 {
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

	args := []interface{}{
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
					version, stream_id, created_at, expires_at, status, segment_count, encrypted_metadata_nonce,
					encrypted_metadata, encrypted_metadata_encrypted_key, total_plain_size, total_encrypted_size,
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

	args := []interface{}{
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
				version, stream_id, created_at, expires_at, status, segment_count, encrypted_metadata_nonce,
				encrypted_metadata, encrypted_metadata_encrypted_key, total_plain_size, total_encrypted_size,
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
			&object.EncryptedMetadataNonce, &object.EncryptedMetadata, &object.EncryptedMetadataEncryptedKey,
			&object.TotalPlainSize, &object.TotalEncryptedSize, &object.FixedSegmentSize,
			encryptionParameters{&object.Encryption},
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
			case isRetentionProtected(object.Retention, opts.ObjectLock.BypassGovernance, now):
				return DeleteObjectResult{}, ErrObjectLock.New(retentionErrMsg)
			}
		}
		return DeleteObjectResult{}, Error.New("unable to delete object")
	}

	result.Removed = []Object{*object}
	return result, nil
}

// DeleteObjectExactVersion deletes an exact object version.
func (s *SpannerAdapter) DeleteObjectExactVersion(ctx context.Context, opts DeleteObjectExactVersion) (DeleteObjectResult, error) {
	if opts.ObjectLock.Enabled {
		return s.deleteObjectExactVersionUsingObjectLock(ctx, opts)
	}
	return s.deleteObjectExactVersion(ctx, opts)
}

func (s *SpannerAdapter) deleteObjectExactVersionWithTx(ctx context.Context, tx *spanner.ReadWriteTransaction, opts DeleteObjectExactVersion) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	params := map[string]interface{}{
		"project_id":  opts.ProjectID,
		"bucket_name": opts.BucketName,
		"object_key":  opts.ObjectKey,
		"version":     opts.Version,
	}

	var streamIDFilter string
	if !opts.StreamIDSuffix.IsZero() {
		streamIDFilter = "AND ENDS_WITH(stream_id, @stream_id_suffix)"
		params["stream_id_suffix"] = opts.StreamIDSuffix
	}

	result.Removed, err = collectDeletedObjectsSpanner(ctx, opts.ObjectLocation,
		tx.Query(ctx, spanner.Statement{
			SQL: `
				DELETE FROM objects
				WHERE (project_id, bucket_name, object_key, version) = (@project_id, @bucket_name, @object_key, @version)
				` + streamIDFilter + `
				THEN RETURN` + collectDeletedObjectsSpannerFields,
			Params: params,
		}))
	if err != nil {
		return DeleteObjectResult{}, errs.Wrap(err)
	}

	stmts := make([]spanner.Statement, len(result.Removed))
	for ix, object := range result.Removed {
		stmts[ix] = spanner.Statement{
			SQL: `DELETE FROM segments WHERE @stream_id = stream_id`,
			Params: map[string]interface{}{
				"stream_id": object.StreamID.Bytes(),
			},
		}
	}
	if len(stmts) > 0 {
		var counts []int64
		counts, err = tx.BatchUpdate(ctx, stmts)
		for _, count := range counts {
			result.DeletedSegmentCount += int(count)
		}
	}
	if err != nil {
		return DeleteObjectResult{}, errs.Wrap(err)
	}

	return result, err
}

func (s *SpannerAdapter) deleteObjectExactVersion(ctx context.Context, opts DeleteObjectExactVersion) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = s.client.ReadWriteTransaction(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		result, err = s.deleteObjectExactVersionWithTx(ctx, tx, opts)
		return err
	})
	if err != nil {
		return DeleteObjectResult{}, Error.Wrap(err)
	}
	return result, nil
}

func (s *SpannerAdapter) deleteObjectExactVersionUsingObjectLock(ctx context.Context, opts DeleteObjectExactVersion) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = s.client.ReadWriteTransaction(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		result = DeleteObjectResult{}

		var (
			status    ObjectStatus
			retention Retention
			legalHold bool
		)

		err = tx.Query(ctx, spanner.Statement{
			SQL: `
				SELECT status, retention_mode, retain_until
				FROM objects
				WHERE (project_id, bucket_name, object_key, version) = (@project_id, @bucket_name, @object_key, @version)
			`,
			Params: map[string]interface{}{
				"project_id":  opts.ProjectID,
				"bucket_name": opts.BucketName,
				"object_key":  opts.ObjectKey,
				"version":     opts.Version,
			},
		}).Do(func(row *spanner.Row) error {
			lockMode := lockModeWrapper{
				retentionMode: &retention.Mode,
				legalHold:     &legalHold,
			}
			return errs.Wrap(row.Columns(&status, &lockMode, timeWrapper{&retention.RetainUntil}))
		})
		if err != nil {
			if errs.Is(err, iterator.Done) {
				return nil
			}
			return errs.Wrap(err)
		}

		if status != Pending {
			if err = retention.Verify(); err != nil {
				return errs.Wrap(err)
			}
			switch {
			case legalHold:
				return ErrObjectLock.New(legalHoldErrMsg)
			case isRetentionProtected(retention, opts.ObjectLock.BypassGovernance, time.Now()):
				return ErrObjectLock.New(retentionErrMsg)
			}
		}

		result, err = s.deleteObjectExactVersionWithTx(ctx, tx, opts)
		return errs.Wrap(err)
	})
	if err != nil {
		if ErrObjectLock.Has(err) {
			return DeleteObjectResult{}, errs.Wrap(err)
		}
		return DeleteObjectResult{}, Error.Wrap(err)
	}

	return result, err
}

// DeletePendingObject contains arguments necessary for deleting a pending object.
type DeletePendingObject struct {
	ObjectStream
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

// DeletePendingObject deletes a pending object with specified version and streamID.
func (p *PostgresAdapter) DeletePendingObject(ctx context.Context, opts DeletePendingObject) (result DeleteObjectResult, err error) {
	err = withRows(p.db.QueryContext(ctx, `
			WITH deleted_objects AS (
				DELETE FROM objects
				WHERE
					(project_id, bucket_name, object_key, version, stream_id) = ($1, $2, $3, $4, $5) AND
					status = `+statusPending+`
				RETURNING
					version, stream_id, created_at, expires_at, status, segment_count,
					encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
					total_plain_size, total_encrypted_size, fixed_segment_size, encryption,
					retention_mode, retain_until
			), deleted_segments AS (
				DELETE FROM segments
				WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
				RETURNING segments.stream_id
			)
			SELECT *, (SELECT COUNT(*) FROM deleted_segments) FROM deleted_objects
		`, opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.Version, opts.StreamID))(func(rows tagsql.Rows) error {
		result.Removed, result.DeletedSegmentCount, err = scanObjectDeletionPostgres(ctx, opts.Location(), rows)
		return err
	})
	return result, Error.Wrap(err)
}

// DeletePendingObject deletes a pending object with specified version and streamID.
func (s *SpannerAdapter) DeletePendingObject(ctx context.Context, opts DeletePendingObject) (result DeleteObjectResult, err error) {
	_, err = s.client.ReadWriteTransaction(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		result.Removed, err = collectDeletedObjectsSpanner(ctx, opts.Location(), tx.Query(ctx, spanner.Statement{
			SQL: `
				DELETE FROM objects
				WHERE
					(project_id, bucket_name, object_key, version, stream_id) = (@project_id, @bucket_name, @object_key, @version, @stream_id) AND
					status = ` + statusPending + `
				THEN RETURN` + collectDeletedObjectsSpannerFields,
			Params: map[string]interface{}{
				"project_id":  opts.ProjectID,
				"bucket_name": opts.BucketName,
				"object_key":  opts.ObjectKey,
				"version":     opts.Version,
				"stream_id":   opts.StreamID,
			},
		}))

		stmts := make([]spanner.Statement, len(result.Removed))
		for ix, object := range result.Removed {
			stmts[ix] = spanner.Statement{
				SQL: `DELETE FROM segments WHERE @stream_id = stream_id`,
				Params: map[string]interface{}{
					"stream_id": object.StreamID.Bytes(),
				},
			}
		}
		if len(stmts) > 0 {
			var counts []int64
			counts, err = tx.BatchUpdate(ctx, stmts)
			for _, count := range counts {
				result.DeletedSegmentCount += int(count)
			}
		}
		return errs.Wrap(err)
	})
	if err != nil {
		return DeleteObjectResult{}, Error.Wrap(err)
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
			&object.EncryptedMetadataNonce, &object.EncryptedMetadata, &object.EncryptedMetadataEncryptedKey,
			&object.TotalPlainSize, &object.TotalEncryptedSize, &object.FixedSegmentSize,
			encryptionParameters{&object.Encryption},
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

const collectDeletedObjectsSpannerFields = " " +
	`version, stream_id, created_at, expires_at, status, segment_count, encrypted_metadata_nonce,
	encrypted_metadata, encrypted_metadata_encrypted_key, total_plain_size, total_encrypted_size,
	fixed_segment_size, encryption, retention_mode, retain_until`

// collectDeletedObjectsSpanner reads in the results of an object deletion from the database.
func collectDeletedObjectsSpanner(ctx context.Context, location ObjectLocation, iter *spanner.RowIterator) (objects []Object, err error) {
	defer mon.Task()(&ctx)(&err)

	objects, err = spannerutil.CollectRows(iter,
		func(row *spanner.Row, object *Object) error {
			err := row.Columns(&object.Version, &object.StreamID,
				&object.CreatedAt, &object.ExpiresAt,
				&object.Status, spannerutil.Int(&object.SegmentCount),
				&object.EncryptedMetadataNonce, &object.EncryptedMetadata, &object.EncryptedMetadataEncryptedKey,
				&object.TotalPlainSize, &object.TotalEncryptedSize, spannerutil.Int(&object.FixedSegmentSize),
				encryptionParameters{&object.Encryption},
				lockModeWrapper{
					retentionMode: &object.Retention.Mode,
					legalHold:     &object.LegalHold,
				},
				timeWrapper{&object.Retention.RetainUntil},
			)
			if err != nil {
				return Error.New("unable to delete object: %w", err)
			}
			return nil
		})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	for i := range objects {
		object := &objects[i]
		object.ProjectID = location.ProjectID
		object.BucketName = location.BucketName
		object.ObjectKey = location.ObjectKey
	}

	return objects, nil
}

// DeleteObjectLastCommitted contains arguments necessary for deleting last committed version of object.
type DeleteObjectLastCommitted struct {
	ObjectLocation

	Versioned bool
	Suspended bool

	ObjectLock ObjectLockDeleteOptions
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

		return db.ChooseAdapter(opts.ProjectID).DeleteObjectLastCommittedSuspended(ctx, opts, deleterMarkerStreamID)
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
					encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
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
				version, stream_id, created_at, expires_at, status, segment_count, encrypted_metadata_nonce,
				encrypted_metadata, encrypted_metadata_encrypted_key, total_plain_size, total_encrypted_size,
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
			&object.EncryptedMetadataNonce, &object.EncryptedMetadata, &object.EncryptedMetadataEncryptedKey,
			&object.TotalPlainSize, &object.TotalEncryptedSize, &object.FixedSegmentSize,
			encryptionParameters{&object.Encryption},
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
		case isRetentionProtected(object.Retention, opts.ObjectLock.BypassGovernance, now):
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
func (s *SpannerAdapter) DeleteObjectLastCommittedPlain(ctx context.Context, opts DeleteObjectLastCommitted) (DeleteObjectResult, error) {
	if opts.ObjectLock.Enabled {
		return s.deleteObjectLastCommittedPlainUsingObjectLock(ctx, opts)
	}
	return s.deleteObjectLastCommittedPlain(ctx, opts)
}

func (s *SpannerAdapter) deleteObjectLastCommittedPlain(ctx context.Context, opts DeleteObjectLastCommitted) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)
	// TODO(ver): do we need to pretend here that `expires_at` matters?
	// TODO(ver): should this report an error when the object doesn't exist?
	_, err = s.client.ReadWriteTransaction(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		// TODO(spanner): is there a better way to combine these deletes from different tables?
		result.Removed, err = collectDeletedObjectsSpanner(ctx, opts.ObjectLocation,
			tx.Query(ctx, spanner.Statement{
				SQL: `
					DELETE FROM objects
						WHERE
							(project_id, bucket_name, object_key) = (@project_id, @bucket_name, @object_key) AND
							status = ` + statusCommittedUnversioned + ` AND
							(expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
						THEN RETURN` + collectDeletedObjectsSpannerFields,
				Params: map[string]interface{}{
					"project_id":  opts.ProjectID,
					"bucket_name": opts.BucketName,
					"object_key":  opts.ObjectKey,
				},
			}))
		if err != nil {
			return errs.Wrap(err)
		}

		stmts := make([]spanner.Statement, len(result.Removed))
		for ix, object := range result.Removed {
			stmts[ix] = spanner.Statement{
				SQL: `DELETE FROM segments WHERE @stream_id = stream_id`,
				Params: map[string]interface{}{
					"stream_id": object.StreamID.Bytes(),
				},
			}
		}
		if len(stmts) > 0 {
			var counts []int64
			counts, err = tx.BatchUpdate(ctx, stmts)
			for _, count := range counts {
				result.DeletedSegmentCount += int(count)
			}
		}
		return errs.Wrap(err)
	})
	if err != nil {
		return DeleteObjectResult{}, Error.Wrap(err)
	}
	return result, nil
}

func (s *SpannerAdapter) deleteObjectLastCommittedPlainUsingObjectLock(ctx context.Context, opts DeleteObjectLastCommitted) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	type versionAndLockInfo struct {
		version   Version
		retention Retention
		legalHold bool
	}

	_, err = s.client.ReadWriteTransaction(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		result = DeleteObjectResult{}

		info, err := spannerutil.CollectRow(tx.Query(ctx, spanner.Statement{
			SQL: `
				SELECT version, retention_mode, retain_until
				FROM objects
				WHERE
					(project_id, bucket_name, object_key) = (@project_id, @bucket_name, @object_key)
					AND status = ` + statusCommittedUnversioned + `
					AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
				ORDER BY version DESC LIMIT 1
			`,
			Params: map[string]interface{}{
				"project_id":  opts.ProjectID,
				"bucket_name": opts.BucketName,
				"object_key":  opts.ObjectKey,
			},
		}), func(row *spanner.Row, item *versionAndLockInfo) error {
			return errs.Wrap(row.Columns(
				&item.version,
				lockModeWrapper{
					retentionMode: &item.retention.Mode,
					legalHold:     &item.legalHold,
				},
				timeWrapper{&item.retention.RetainUntil},
			))
		})
		if err != nil {
			if errors.Is(err, iterator.Done) {
				return nil
			}
			return errs.Wrap(err)
		}

		if err = info.retention.Verify(); err != nil {
			return errs.Wrap(err)
		}
		switch {
		case info.legalHold:
			return ErrObjectLock.New(legalHoldErrMsg)
		case isRetentionProtected(info.retention, opts.ObjectLock.BypassGovernance, time.Now()):
			return ErrObjectLock.New(retentionErrMsg)
		}

		result, err = s.deleteObjectExactVersionWithTx(ctx, tx, DeleteObjectExactVersion{
			ObjectLocation: opts.ObjectLocation,
			Version:        info.version,
		})
		return errs.Wrap(err)
	})
	if err != nil {
		if ErrObjectLock.Has(err) {
			return DeleteObjectResult{}, err
		}
		return DeleteObjectResult{}, Error.Wrap(err)
	}

	return result, nil
}

type deleteTransactionAdapter interface {
	PrecommitDeleteUnversionedWithNonPending(ctx context.Context, opts PrecommitDeleteUnversionedWithNonPending) (result PrecommitConstraintWithNonPendingResult, err error)
}

// PrecommitDeleteUnversionedWithNonPending contains arguments necessary for deleting an unversioned object
// at a specified location and returning the highest non-pending version at that location.
type PrecommitDeleteUnversionedWithNonPending struct {
	ObjectLocation

	ObjectLock ObjectLockDeleteOptions
}

// DeleteObjectLastCommittedSuspended deletes an object last committed version when opts.Suspended is true.
func (p *PostgresAdapter) DeleteObjectLastCommittedSuspended(ctx context.Context, opts DeleteObjectLastCommitted, deleterMarkerStreamID uuid.UUID) (result DeleteObjectResult, err error) {
	var precommit PrecommitConstraintWithNonPendingResult

	marker := Object{
		ObjectStream: ObjectStream{
			ProjectID:  opts.ProjectID,
			BucketName: opts.BucketName,
			ObjectKey:  opts.ObjectKey,
			StreamID:   deleterMarkerStreamID,
		},
		Status: DeleteMarkerUnversioned,
	}

	err = p.WithTx(ctx, func(ctx context.Context, tx TransactionAdapter) (err error) {
		result = DeleteObjectResult{}

		precommit, err = tx.PrecommitDeleteUnversionedWithNonPending(ctx, PrecommitDeleteUnversionedWithNonPending{
			ObjectLocation: opts.ObjectLocation,
			ObjectLock:     opts.ObjectLock,
		})
		if err != nil {
			return errs.Wrap(err)
		}
		if precommit.HighestVersion == 0 || precommit.HighestNonPendingVersion == 0 {
			// an object didn't exist in the first place
			return ErrObjectNotFound.New("unable to delete object")
		}
		result.Removed = precommit.Deleted
		result.DeletedSegmentCount = precommit.DeletedSegmentCount

		row := tx.(*postgresTransactionAdapter).tx.QueryRowContext(ctx, `
				INSERT INTO objects (
					project_id, bucket_name, object_key, version, stream_id,
					status,
					zombie_deletion_deadline
				)
				SELECT
					$1, $2, $3, $4, $5,
					`+statusDeleteMarkerUnversioned+`,
					NULL
				RETURNING
					version,
					created_at
			`, opts.ProjectID, opts.BucketName, opts.ObjectKey, precommit.HighestVersion+1, deleterMarkerStreamID)

		return errs.Wrap(row.Scan(&marker.Version, &marker.CreatedAt))
	})
	if err != nil {
		if ErrObjectNotFound.Has(err) || ErrObjectLock.Has(err) {
			return DeleteObjectResult{}, err
		}
		return DeleteObjectResult{}, Error.Wrap(err)
	}

	result.Markers = []Object{marker}

	precommit.submitMetrics()
	return result, nil
}

// DeleteObjectLastCommittedSuspended deletes an object last committed version when opts.Suspended is true.
func (s *SpannerAdapter) DeleteObjectLastCommittedSuspended(ctx context.Context, opts DeleteObjectLastCommitted, deleterMarkerStreamID uuid.UUID) (result DeleteObjectResult, err error) {
	var precommit PrecommitConstraintWithNonPendingResult

	marker := Object{
		ObjectStream: ObjectStream{
			ProjectID:  opts.ProjectID,
			BucketName: opts.BucketName,
			ObjectKey:  opts.ObjectKey,
			StreamID:   deleterMarkerStreamID,
		},
		Status: DeleteMarkerUnversioned,
	}

	err = s.WithTx(ctx, func(ctx context.Context, atx TransactionAdapter) error {
		result = DeleteObjectResult{}
		stx := atx.(*spannerTransactionAdapter)

		precommit, err = stx.PrecommitDeleteUnversionedWithNonPending(ctx, PrecommitDeleteUnversionedWithNonPending{
			ObjectLocation: opts.ObjectLocation,
			ObjectLock:     opts.ObjectLock,
		})
		if err != nil {
			return errs.Wrap(err)
		}
		if precommit.HighestVersion == 0 || precommit.HighestNonPendingVersion == 0 {
			// an object didn't exist in the first place
			return ErrObjectNotFound.New("unable to delete object")
		}
		result.Removed = precommit.Deleted
		result.DeletedSegmentCount = precommit.DeletedSegmentCount

		err = stx.tx.Query(ctx, spanner.Statement{
			SQL: `
				INSERT INTO objects (
					project_id, bucket_name, object_key, version, stream_id,
					status,
					zombie_deletion_deadline
				) VALUES (
					@project_id, @bucket_name, @object_key, @version, @marker,
					` + statusDeleteMarkerUnversioned + `,
					NULL
				)
				THEN RETURN
					version,
					created_at
			`,
			Params: map[string]interface{}{
				"project_id":  opts.ProjectID,
				"bucket_name": opts.BucketName,
				"object_key":  opts.ObjectKey,
				"version":     precommit.HighestVersion + 1,
				"marker":      deleterMarkerStreamID,
			},
		}).Do(func(row *spanner.Row) error {
			return errs.Wrap(row.Columns(&marker.Version, &marker.CreatedAt))
		})
		if err != nil {
			if errors.Is(err, iterator.Done) {
				return errs.New("could not insert deletion marker: %w", err)
			}
			return errs.Wrap(err)
		}
		return nil
	})

	if err != nil {
		if ErrObjectNotFound.Has(err) || ErrObjectLock.Has(err) {
			return DeleteObjectResult{}, err
		}
		return DeleteObjectResult{}, Error.Wrap(err)
	}

	result.Markers = []Object{marker}

	precommit.submitMetrics()
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
				$1, $2, $3,
					coalesce((
						SELECT version + 1
						FROM objects
						WHERE (project_id, bucket_name, object_key) = ($1, $2, $3)
						ORDER BY version DESC
						LIMIT 1
					), 1),
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
func (s *SpannerAdapter) DeleteObjectLastCommittedVersioned(ctx context.Context, opts DeleteObjectLastCommitted, deleterMarkerStreamID uuid.UUID) (result DeleteObjectResult, err error) {
	_, err = s.client.ReadWriteTransaction(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {

		deleted, err := spannerutil.CollectRow(
			tx.Query(ctx, spanner.Statement{
				SQL: `
					INSERT INTO objects (
						project_id, bucket_name, object_key, version, stream_id,
						status,
						zombie_deletion_deadline
					)
					SELECT
						@project_id, @bucket_name, @object_key,
							coalesce((
								SELECT version + 1
								FROM objects
								WHERE (project_id, bucket_name, object_key) = (@project_id, @bucket_name, @object_key)
								ORDER BY version DESC
								LIMIT 1
							), 1),
						@marker,
						` + statusDeleteMarkerVersioned + `,
						NULL
					THEN RETURN version, created_at
				`,
				Params: map[string]interface{}{
					"project_id":  opts.ProjectID,
					"bucket_name": opts.BucketName,
					"object_key":  opts.ObjectKey,
					"marker":      deleterMarkerStreamID,
				},
			}), func(row *spanner.Row, item *Object) error {
				return errs.Wrap(row.Columns(&item.Version, &item.CreatedAt))
			})
		if err != nil {
			if errors.Is(err, iterator.Done) {
				return ErrObjectNotFound.Wrap(Error.New("object does not exist"))
			}
			return errs.Wrap(err)
		}

		deleted.ProjectID = opts.ProjectID
		deleted.BucketName = opts.BucketName
		deleted.ObjectKey = opts.ObjectKey
		deleted.StreamID = deleterMarkerStreamID
		deleted.Status = DeleteMarkerVersioned

		result.Markers = []Object{deleted}

		return nil
	})
	if err != nil {
		if ErrObjectNotFound.Has(err) {
			return DeleteObjectResult{}, err
		}
		return DeleteObjectResult{}, Error.Wrap(err)
	}
	return result, nil
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
		zap.Stringer("Project ID", loc.ProjectID),
		zap.Stringer("Bucket Name", loc.BucketName),
		zap.String("Object Key", hex.EncodeToString([]byte(loc.ObjectKey))),
	)
	mon.Meter("multiple_committed_versions").Mark(1)
}

func isRetentionProtected(retention Retention, bypassGovernance bool, now time.Time) bool {
	return retention.Active(now) && !(bypassGovernance && retention.Mode == storj.GovernanceMode)
}
