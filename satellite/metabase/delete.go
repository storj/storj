// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"sort"

	spanner "github.com/storj/exp-spanner"
	"github.com/zeebo/errs"
	"google.golang.org/api/iterator"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/dbutil/spannerutil"
	"storj.io/storj/shared/tagsql"
)

// DeletedSegmentInfo info about deleted segment.
type DeletedSegmentInfo struct {
	RootPieceID storj.PieceID
	Pieces      Pieces
}

// DeleteObjectExactVersion contains arguments necessary for deleting an exact version of object.
type DeleteObjectExactVersion struct {
	Version Version
	ObjectLocation
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
}

// DeleteObjectsAllVersions contains arguments necessary for deleting all versions of multiple objects from the same bucket.
type DeleteObjectsAllVersions struct {
	Locations []ObjectLocation
}

// Verify delete objects fields.
func (delete *DeleteObjectsAllVersions) Verify() error {
	if len(delete.Locations) == 0 {
		return nil
	}

	if len(delete.Locations) > 1000 {
		return ErrInvalidRequest.New("cannot delete more than 1000 objects in a single request")
	}

	var errGroup errs.Group
	for _, location := range delete.Locations {
		errGroup.Add(location.Verify())
	}

	err := errGroup.Err()
	if err != nil {
		return err
	}

	// Verify if all locations are in the same bucket
	first := delete.Locations[0]
	for _, item := range delete.Locations[1:] {
		if first.ProjectID != item.ProjectID || first.BucketName != item.BucketName {
			return ErrInvalidRequest.New("all objects must be in the same bucket")
		}
	}

	return nil
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
	for _, object := range result.Removed {
		mon.Meter("segment_delete").Mark(int(object.SegmentCount))
	}
	return result, nil
}

// DeleteObjectExactVersion deletes an exact object version.
func (p *PostgresAdapter) DeleteObjectExactVersion(ctx context.Context, opts DeleteObjectExactVersion) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	err = withRows(
		p.db.QueryContext(ctx, `
			WITH deleted_objects AS (
				DELETE FROM objects
				WHERE (project_id, bucket_name, object_key, version) = ($1, $2, $3, $4)
				RETURNING
					version, stream_id, created_at, expires_at, status, segment_count, encrypted_metadata_nonce,
					encrypted_metadata, encrypted_metadata_encrypted_key, total_plain_size, total_encrypted_size,
					fixed_segment_size, encryption
			), deleted_segments AS (
				DELETE FROM segments
				WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
				RETURNING segments.stream_id
			)
			SELECT
				version, stream_id, created_at, expires_at, status, segment_count, encrypted_metadata_nonce,
				encrypted_metadata, encrypted_metadata_encrypted_key, total_plain_size, total_encrypted_size,
				fixed_segment_size, encryption
			FROM deleted_objects`,
			opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey, opts.Version),
	)(func(rows tagsql.Rows) error {
		result.Removed, err = scanObjectDeletionPostgres(ctx, opts.ObjectLocation, rows)
		return err
	})
	return result, err
}

// DeleteObjectExactVersion deletes an exact object version.
func (s *SpannerAdapter) DeleteObjectExactVersion(ctx context.Context, opts DeleteObjectExactVersion) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = s.client.ReadWriteTransaction(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		objectDeletion := spanner.Statement{
			SQL: `
				DELETE FROM objects
				WHERE (project_id, bucket_name, object_key, version) = (@project_id, @bucket_name, @object_key, @version)
				THEN RETURN
					version, stream_id, created_at, expires_at, status, segment_count, encrypted_metadata_nonce,
					encrypted_metadata, encrypted_metadata_encrypted_key, total_plain_size, total_encrypted_size,
					fixed_segment_size, encryption
			`,
			Params: map[string]interface{}{
				"project_id":  opts.ProjectID,
				"bucket_name": opts.BucketName,
				"object_key":  opts.ObjectKey,
				"version":     opts.Version,
			},
		}
		objectsDeleted := tx.Query(ctx, objectDeletion)
		defer objectsDeleted.Stop()

		result.Removed, err = scanObjectDeletionSpanner(ctx, opts.ObjectLocation, objectsDeleted)
		if err != nil {
			return Error.Wrap(err)
		}

		streamIDs := make([][]byte, 0, len(result.Removed))
		for _, object := range result.Removed {
			streamIDs = append(streamIDs, object.StreamID.Bytes())
		}
		segmentDeletion := spanner.Statement{
			SQL: `
				DELETE FROM segments
				WHERE ARRAY_INCLUDES(@stream_ids, stream_id)
			`,
			Params: map[string]interface{}{
				"stream_ids": streamIDs,
			},
		}
		_, err = tx.Update(ctx, segmentDeletion)
		return Error.Wrap(err)
	})
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
	for _, object := range result.Removed {
		mon.Meter("segment_delete").Mark(int(object.SegmentCount))
	}

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
					total_plain_size, total_encrypted_size, fixed_segment_size, encryption
			), deleted_segments AS (
				DELETE FROM segments
				WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
				RETURNING segments.stream_id
			)
			SELECT
				version, stream_id, created_at, expires_at, status, segment_count,
				encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
				total_plain_size, total_encrypted_size, fixed_segment_size, encryption
			FROM deleted_objects
		`, opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey, opts.Version, opts.StreamID))(func(rows tagsql.Rows) error {
		result.Removed, err = scanObjectDeletionPostgres(ctx, opts.Location(), rows)
		return err
	})
	return result, err
}

// DeletePendingObject deletes a pending object with specified version and streamID.
func (s *SpannerAdapter) DeletePendingObject(ctx context.Context, opts DeletePendingObject) (result DeleteObjectResult, err error) {
	_, err = s.client.ReadWriteTransaction(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		objectDeletion := spanner.Statement{
			SQL: `
				DELETE FROM objects
				WHERE
					(project_id, bucket_name, object_key, version, stream_id) = (@project_id, @bucket_name, @object_key, @version, @stream_id) AND
					status = ` + statusPending + `
				THEN RETURN
					version, stream_id, created_at, expires_at, status, segment_count,
					encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
					total_plain_size, total_encrypted_size, fixed_segment_size, encryption
			`,
			Params: map[string]interface{}{
				"project_id":  opts.ProjectID,
				"bucket_name": opts.BucketName,
				"object_key":  opts.ObjectKey,
				"version":     opts.Version,
				"stream_id":   opts.StreamID,
			},
		}
		objectsDeleted := tx.Query(ctx, objectDeletion)
		defer objectsDeleted.Stop()

		result.Removed, err = scanObjectDeletionSpanner(ctx, opts.Location(), objectsDeleted)
		if err != nil {
			return Error.Wrap(err)
		}

		// TODO(spanner): check whether this can be optimized.
		streamIDs := make([][]byte, 0, len(result.Removed))
		for _, object := range result.Removed {
			streamIDs = append(streamIDs, object.StreamID.Bytes())
		}
		segmentDeletion := spanner.Statement{
			SQL: `
				DELETE FROM segments
				WHERE ARRAY_INCLUDES(@stream_ids, stream_id)
			`,
			Params: map[string]interface{}{
				"stream_ids": streamIDs,
			},
		}
		_, err = tx.Update(ctx, segmentDeletion)
		return Error.Wrap(err)
	})
	return result, err
}

// DeleteObjectsAllVersions deletes all versions of multiple objects from the same bucket.
func (db *DB) DeleteObjectsAllVersions(ctx context.Context, opts DeleteObjectsAllVersions) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if db.config.ServerSideCopy {
		return DeleteObjectResult{}, errs.New("method cannot be used when server-side copy is enabled")
	}

	if len(opts.Locations) == 0 {
		// nothing to delete, no error
		return DeleteObjectResult{}, nil
	}

	if err := opts.Verify(); err != nil {
		return DeleteObjectResult{}, err
	}

	// It is already verified that all object locations are in the same bucket
	projectID := opts.Locations[0].ProjectID
	bucketName := opts.Locations[0].BucketName

	objectKeys := make([][]byte, len(opts.Locations))
	for i := range opts.Locations {
		objectKeys[i] = []byte(opts.Locations[i].ObjectKey)
	}

	result, err = db.ChooseAdapter(projectID).DeleteObjectsAllVersions(ctx, projectID, bucketName, objectKeys)
	if err != nil {
		return DeleteObjectResult{}, err
	}

	mon.Meter("object_delete").Mark(len(result.Removed))
	for _, object := range result.Removed {
		mon.Meter("segment_delete").Mark(int(object.SegmentCount))
	}

	return result, nil
}

// DeleteObjectsAllVersions deletes all versions of multiple objects from the same bucket.
func (p *PostgresAdapter) DeleteObjectsAllVersions(ctx context.Context, projectID uuid.UUID, bucketName string, objectKeys [][]byte) (result DeleteObjectResult, err error) {
	// Sorting the object keys just in case.
	sort.Slice(objectKeys, func(i, j int) bool {
		return bytes.Compare(objectKeys[i], objectKeys[j]) < 0
	})

	err = withRows(p.db.QueryContext(ctx, `
		WITH deleted_objects AS (
			DELETE FROM objects
			WHERE
				(project_id, bucket_name) = ($1, $2) AND
				object_key = ANY ($3) AND
				status <> `+statusPending+`
			RETURNING
				project_id, bucket_name, object_key, version, stream_id, created_at, expires_at,
				status, segment_count, encrypted_metadata_nonce, encrypted_metadata,
				encrypted_metadata_encrypted_key, total_plain_size, total_encrypted_size,
				fixed_segment_size, encryption
		), deleted_segments AS (
			DELETE FROM segments
			WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
			RETURNING segments.stream_id
		)
		SELECT
			project_id, bucket_name, object_key, version, stream_id, created_at, expires_at,
			status, segment_count, encrypted_metadata_nonce, encrypted_metadata,
			encrypted_metadata_encrypted_key, total_plain_size, total_encrypted_size,
			fixed_segment_size, encryption
		FROM deleted_objects
	`, projectID, []byte(bucketName), pgutil.ByteaArray(objectKeys)))(func(rows tagsql.Rows) error {
		result.Removed, err = scanMultipleObjectsDeletionPostgres(ctx, rows)
		return err
	})

	return result, nil
}

// DeleteObjectsAllVersions deletes all versions of multiple objects from the same bucket.
func (s *SpannerAdapter) DeleteObjectsAllVersions(ctx context.Context, projectID uuid.UUID, bucketName string, objectKeys [][]byte) (result DeleteObjectResult, err error) {
	// TODO: implement me
	panic("implement me")
}

// scanObjectDeletionPostgres reads in the results of an object deletion from the database.
func scanObjectDeletionPostgres(ctx context.Context, location ObjectLocation, rows tagsql.Rows) (objects []Object, err error) {
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
		)
		if err != nil {
			return nil, Error.New("unable to delete object: %w", err)
		}

		objects = append(objects, object)
	}

	return objects, nil
}

// scanObjectDeletionSpanner reads in the results of an object deletion from the database.
func scanObjectDeletionSpanner(ctx context.Context, location ObjectLocation, resultIter *spanner.RowIterator) (objects []Object, err error) {
	defer mon.Task()(&ctx)(&err)

	objects = make([]Object, 0, 10)

	var object Object
	for {
		row, err := resultIter.Next()
		if err != nil {
			if errors.Is(err, iterator.Done) {
				break
			}
			return nil, Error.New("unable to delete object: %w", err)
		}
		object.ProjectID = location.ProjectID
		object.BucketName = location.BucketName
		object.ObjectKey = location.ObjectKey

		err = row.Columns(&object.Version, &object.StreamID,
			&object.CreatedAt, &object.ExpiresAt,
			&object.Status, spannerutil.Int(&object.SegmentCount),
			&object.EncryptedMetadataNonce, &object.EncryptedMetadata, &object.EncryptedMetadataEncryptedKey,
			&object.TotalPlainSize, &object.TotalEncryptedSize, spannerutil.Int(&object.FixedSegmentSize),
			encryptionParameters{&object.Encryption},
		)
		if err != nil {
			return nil, Error.New("unable to read object deletion result: %w", err)
		}

		objects = append(objects, object)
	}

	return objects, nil
}

// scanMultipleObjectsDeletionPostgres reads in the results of multiple object deletions from the database.
func scanMultipleObjectsDeletionPostgres(ctx context.Context, rows tagsql.Rows) (objects []Object, err error) {
	defer mon.Task()(&ctx)(&err)

	objects = make([]Object, 0, 10)

	var object Object
	for rows.Next() {
		err = rows.Scan(&object.ProjectID, &object.BucketName,
			&object.ObjectKey, &object.Version, &object.StreamID,
			&object.CreatedAt, &object.ExpiresAt,
			&object.Status, &object.SegmentCount,
			&object.EncryptedMetadataNonce, &object.EncryptedMetadata, &object.EncryptedMetadataEncryptedKey,
			&object.TotalPlainSize, &object.TotalEncryptedSize, &object.FixedSegmentSize,
			encryptionParameters{&object.Encryption})
		if err != nil {
			return nil, Error.New("unable to delete object: %w", err)
		}

		objects = append(objects, object)
	}

	if len(objects) == 0 {
		objects = nil
	}

	return objects, nil
}

// DeleteObjectLastCommitted contains arguments necessary for deleting last committed version of object.
type DeleteObjectLastCommitted struct {
	ObjectLocation

	Versioned bool
	Suspended bool
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
	for _, object := range result.Removed {
		mon.Meter("segment_delete").Mark(int(object.SegmentCount))
	}

	return result, nil
}

// DeleteObjectLastCommittedPlain deletes an object last committed version when
// opts.Suspended and opts.Versioned are both false.
func (p *PostgresAdapter) DeleteObjectLastCommittedPlain(ctx context.Context, opts DeleteObjectLastCommitted) (result DeleteObjectResult, err error) {
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
					encryption
			), deleted_segments AS (
				DELETE FROM segments
				WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
				RETURNING segments.stream_id
			)
			SELECT
				version, stream_id, created_at, expires_at, status, segment_count, encrypted_metadata_nonce,
				encrypted_metadata, encrypted_metadata_encrypted_key, total_plain_size, total_encrypted_size,
				fixed_segment_size, encryption
			FROM deleted_objects`,
			opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey),
	)(func(rows tagsql.Rows) error {
		result.Removed, err = scanObjectDeletionPostgres(ctx, opts.ObjectLocation, rows)
		return err
	})
	return result, err
}

// DeleteObjectLastCommittedPlain deletes an object last committed version when
// opts.Suspended and opts.Versioned are both false.
func (s *SpannerAdapter) DeleteObjectLastCommittedPlain(ctx context.Context, opts DeleteObjectLastCommitted) (result DeleteObjectResult, err error) {
	// TODO: implement me
	panic("implement me")
}

type deleteTransactionAdapter interface {
	PrecommitDeleteUnversionedWithNonPending(ctx context.Context, loc ObjectLocation) (result PrecommitConstraintWithNonPendingResult, err error)
}

// DeleteObjectLastCommittedSuspended deletes an object last committed version when opts.Suspended is true.
func (p *PostgresAdapter) DeleteObjectLastCommittedSuspended(ctx context.Context, opts DeleteObjectLastCommitted, deleterMarkerStreamID uuid.UUID) (result DeleteObjectResult, err error) {
	var precommit PrecommitConstraintWithNonPendingResult
	err = p.WithTx(ctx, func(ctx context.Context, tx TransactionAdapter) (err error) {
		precommit, err = tx.PrecommitDeleteUnversionedWithNonPending(ctx, opts.ObjectLocation)
		if err != nil {
			return Error.Wrap(err)
		}
		if precommit.HighestVersion == 0 || precommit.HighestNonPendingVersion == 0 {
			// an object didn't exist in the first place
			return ErrObjectNotFound.New("unable to delete object")
		}

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
			`, opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey, precommit.HighestVersion+1, deleterMarkerStreamID)

		var marker Object
		marker.ProjectID = opts.ProjectID
		marker.BucketName = opts.BucketName
		marker.ObjectKey = opts.ObjectKey
		marker.Status = DeleteMarkerUnversioned
		marker.StreamID = deleterMarkerStreamID

		err = row.Scan(&marker.Version, &marker.CreatedAt)
		if err != nil {
			return Error.Wrap(err)
		}

		result.Markers = append(result.Markers, marker)
		result.Removed = precommit.Deleted
		return nil
	})
	if err != nil {
		return result, err
	}
	precommit.submitMetrics()
	return result, err
}

// DeleteObjectLastCommittedSuspended deletes an object last committed version when opts.Suspended is true.
func (s *SpannerAdapter) DeleteObjectLastCommittedSuspended(ctx context.Context, opts DeleteObjectLastCommitted, deleterMarkerStreamID uuid.UUID) (result DeleteObjectResult, err error) {
	// TODO: implement me
	panic("implement me")
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
		`, opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey, deleterMarkerStreamID)

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
	// TODO: implement me
	panic("implement me")
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
