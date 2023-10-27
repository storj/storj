// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"sort"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/private/dbutil/pgutil"
	"storj.io/private/dbutil/txutil"
	"storj.io/private/tagsql"
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
	result, err = db.deleteObjectExactVersion(ctx, opts, db.db)
	if err != nil {
		return DeleteObjectResult{}, err
	}
	return result, nil
}

type stmt interface {
	QueryContext(ctx context.Context, query string, args ...interface{}) (tagsql.Rows, error)
}
type stmtRow interface {
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

// implementation of DB.DeleteObjectExactVersion for re-use internally in metabase package.
func (db *DB) deleteObjectExactVersion(ctx context.Context, opts DeleteObjectExactVersion, stmt stmt) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return DeleteObjectResult{}, err
	}

	err = withRows(
		stmt.QueryContext(ctx, `
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
		result.Removed, err = db.scanObjectDeletion(ctx, opts.ObjectLocation, rows)
		return err
	})
	if err != nil {
		return DeleteObjectResult{}, err
	}

	mon.Meter("object_delete").Mark(len(result.Removed))
	for _, object := range result.Removed {
		mon.Meter("segment_delete").Mark(int(object.SegmentCount))
	}

	return result, nil
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

	err = withRows(db.db.QueryContext(ctx, `
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
		result.Removed, err = db.scanObjectDeletion(ctx, opts.Location(), rows)
		return err
	})

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

// DeletePendingObjectNew deletes a pending object.
// TODO DeletePendingObjectNew will replace DeletePendingObject when objects table will be free from pending objects.
func (db *DB) DeletePendingObjectNew(ctx context.Context, opts DeletePendingObject) (result DeleteObjectResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return DeleteObjectResult{}, err
	}

	err = withRows(db.db.QueryContext(ctx, `
			WITH deleted_objects AS (
				DELETE FROM pending_objects
				WHERE
					(project_id, bucket_name, object_key, stream_id) = ($1, $2, $3, $4)
				RETURNING
					stream_id, created_at, expires_at,
					encrypted_metadata_nonce, encrypted_metadata, encrypted_metadata_encrypted_key,
					encryption
			), deleted_segments AS (
				DELETE FROM segments
				WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
				RETURNING segments.stream_id
			)
			SELECT * FROM deleted_objects
		`, opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey, opts.StreamID))(func(rows tagsql.Rows) error {
		result.Removed, err = db.scanPendingObjectDeletion(ctx, opts.Location(), rows)
		return err
	})
	if err != nil {
		return DeleteObjectResult{}, err
	}

	if len(result.Removed) == 0 {
		return DeleteObjectResult{}, ErrObjectNotFound.Wrap(Error.New("no rows deleted"))
	}

	mon.Meter("object_delete").Mark(len(result.Removed))

	return result, nil
}

type deleteObjectUnversionedCommittedResult struct {
	Deleted []Object
	// DeletedObjectCount and DeletedSegmentCount return how many elements were deleted.
	DeletedObjectCount  int
	DeletedSegmentCount int
	// MaxVersion returns tha highest version that was present in the table.
	// It returns 0 if there was none.
	MaxVersion Version
}

// deleteObjectUnversionedCommitted deletes the unversioned object at the specified location inside a transaction.
//
// TODO(ver): this should have a better and clearer name.
func (db *DB) deleteObjectUnversionedCommitted(ctx context.Context, loc ObjectLocation, stmt stmtRow) (result deleteObjectUnversionedCommittedResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := loc.Verify(); err != nil {
		return deleteObjectUnversionedCommittedResult{}, Error.Wrap(err)
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

	err = stmt.QueryRowContext(ctx, `
		WITH highest_object AS (
			SELECT MAX(version) as version
			FROM objects
			WHERE (project_id, bucket_name, object_key) = ($1, $2, $3)
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
			&result.MaxVersion,
		)

	if err != nil {
		return deleteObjectUnversionedCommittedResult{}, Error.Wrap(err)
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

	// TODO: this should happen outside of this func
	mon.Meter("object_delete").Mark(result.DeletedObjectCount)
	mon.Meter("segment_delete").Mark(result.DeletedObjectCount)

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

// queryHighestVersion queries the latest version of an object inside an transaction.
//
// TODO(ver): this should have a better and clearer name.
func (db *DB) queryHighestVersion(ctx context.Context, loc ObjectLocation, stmt stmtRow) (highest Version, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := loc.Verify(); err != nil {
		return 0, Error.Wrap(err)
	}

	err = stmt.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version), 0) as version
		FROM objects
		WHERE (project_id, bucket_name, object_key) = ($1, $2, $3)
	`, loc.ProjectID, []byte(loc.BucketName), loc.ObjectKey).Scan(&highest)
	if err != nil {
		return 0, Error.Wrap(err)
	}

	return highest, nil
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

	// It is aleady verified that all object locations are in the same bucket
	projectID := opts.Locations[0].ProjectID
	bucketName := opts.Locations[0].BucketName

	objectKeys := make([][]byte, len(opts.Locations))
	for i := range opts.Locations {
		objectKeys[i] = []byte(opts.Locations[i].ObjectKey)
	}

	// TODO(ver): should this insert delete markers?

	// Sorting the object keys just in case.
	// TODO: Check if this is really necessary for the SQL query.
	sort.Slice(objectKeys, func(i, j int) bool {
		return bytes.Compare(objectKeys[i], objectKeys[j]) < 0
	})
	err = withRows(db.db.QueryContext(ctx, `
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
		result.Removed, err = db.scanMultipleObjectsDeletion(ctx, rows)
		return err
	})

	if err != nil {
		return DeleteObjectResult{}, err
	}

	mon.Meter("object_delete").Mark(len(result.Removed))
	for _, object := range result.Removed {
		mon.Meter("segment_delete").Mark(int(object.SegmentCount))
	}

	return result, nil
}

func (db *DB) scanObjectDeletion(ctx context.Context, location ObjectLocation, rows tagsql.Rows) (objects []Object, err error) {
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

func (db *DB) scanMultipleObjectsDeletion(ctx context.Context, rows tagsql.Rows) (objects []Object, err error) {
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

func (db *DB) scanPendingObjectDeletion(ctx context.Context, location ObjectLocation, rows tagsql.Rows) (objects []Object, err error) {
	defer mon.Task()(&ctx)(&err)

	objects = make([]Object, 0, 10)

	var object Object
	for rows.Next() {
		object.ProjectID = location.ProjectID
		object.BucketName = location.BucketName
		object.ObjectKey = location.ObjectKey

		err = rows.Scan(&object.StreamID,
			&object.CreatedAt, &object.ExpiresAt,
			&object.EncryptedMetadataNonce, &object.EncryptedMetadata, &object.EncryptedMetadataEncryptedKey,
			encryptionParameters{&object.Encryption},
		)
		if err != nil {
			return nil, Error.New("unable to delete pending object: %w", err)
		}

		object.Status = Pending
		objects = append(objects, object)
	}
	return objects, nil
}

// DeleteObjectLastCommitted contains arguments necessary for deleting last committed version of object.
type DeleteObjectLastCommitted struct {
	ObjectLocation

	// TODO(ver): maybe replace these with an enumeration?
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

		err = txutil.WithTx(ctx, db.db, nil, func(ctx context.Context, tx tagsql.Tx) (err error) {
			// TODO(ver) fold deleteObjectUnversionedCommitted into query below using ON CONFLICT
			deleted, err := db.deleteObjectUnversionedCommitted(ctx, opts.ObjectLocation, tx)
			if err != nil {
				return Error.Wrap(err)
			}
			if deleted.MaxVersion == 0 {
				return ErrObjectNotFound.New("unable to delete object")
			}

			row := tx.QueryRowContext(ctx, `
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
			`, opts.ProjectID, []byte(opts.BucketName), opts.ObjectKey, deleted.MaxVersion+1, deleterMarkerStreamID)

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
			result.Removed = deleted.Deleted
			return nil
		})
		return result, err
	}
	if opts.Versioned {
		// Instead of deleting we insert a deletion marker.
		deleterMarkerStreamID, err := generateDeleteMarkerStreamID()
		if err != nil {
			return DeleteObjectResult{}, Error.Wrap(err)
		}

		row := db.db.QueryRowContext(ctx, `
			WITH check_existing_object AS (
				SELECT status
				FROM objects
				WHERE
					(project_id, bucket_name, object_key) = ($1, $2, $3) AND
					status <> `+statusPending+`
				ORDER BY project_id, bucket_name, object_key, stream_id, version DESC, created_at DESC
				LIMIT 1
			),
			added_object AS (
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
				WHERE EXISTS (SELECT 1 FROM check_existing_object)
				RETURNING *
			)
			SELECT version, created_at FROM added_object
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

	// TODO(ver): do we need to pretend here that `expires_at` matters?
	// TODO(ver): should this report an error when the object doesn't exist?
	err = withRows(
		db.db.QueryContext(ctx, `
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
		result.Removed, err = db.scanObjectDeletion(ctx, opts.ObjectLocation, rows)
		return err
	})
	if err != nil {
		return DeleteObjectResult{}, err
	}

	mon.Meter("object_delete").Mark(len(result.Removed))
	for _, object := range result.Removed {
		mon.Meter("segment_delete").Mark(int(object.SegmentCount))
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
