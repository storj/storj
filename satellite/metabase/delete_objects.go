// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/shared/tagsql"
)

const (
	deleteBatchsizeLimit  = intLimitRange(1000)
	deleteObjectsMaxItems = 1000
)

// DeleteObjects contains options for deleting multiple committed objects from a bucket.
type DeleteObjects struct {
	ProjectID  uuid.UUID
	BucketName BucketName
	Items      []DeleteObjectsItem

	Versioned bool
	Suspended bool

	ObjectLock ObjectLockDeleteOptions
}

// DeleteObjectsItem describes the location of an object in a bucket to be deleted.
type DeleteObjectsItem struct {
	ObjectKey       ObjectKey
	StreamVersionID StreamVersionID
}

// Verify verifies bucket object deletion request fields.
func (opts DeleteObjects) Verify() error {
	itemCount := len(opts.Items)
	switch {
	case opts.Suspended:
		return ErrInvalidRequest.New("deletion from buckets with versioning suspended is not yet supported")
	case opts.ObjectLock.Enabled:
		return ErrInvalidRequest.New("deletion from buckets with Object Lock enabled is not yet supported")
	case opts.ProjectID.IsZero():
		return ErrInvalidRequest.New("ProjectID missing")
	case opts.BucketName == "":
		return ErrInvalidRequest.New("BucketName missing")
	case itemCount == 0:
		return ErrInvalidRequest.New("Items missing")
	case itemCount > deleteObjectsMaxItems:
		return ErrInvalidRequest.New("Items is too long; expected <= %d, but got %d", deleteObjectsMaxItems, itemCount)
	}
	for i, item := range opts.Items {
		if item.ObjectKey == "" {
			return ErrInvalidRequest.New("Items[%d].ObjectKey missing", i)
		}
		version := item.StreamVersionID.Version()
		if !item.StreamVersionID.IsZero() && version <= 0 {
			return ErrInvalidRequest.New("Items[%d].StreamVersionID invalid: version is %v", i, version)
		}
	}
	return nil
}

// DeleteObjectsResult contains the results of an attempt to delete specific objects from a bucket.
type DeleteObjectsResult struct {
	Items               []DeleteObjectsResultItem
	DeletedSegmentCount int64
}

// DeleteObjectsStatus represents the success or failure status of an individual DeleteObjects deletion.
type DeleteObjectsStatus int

const (
	// DeleteStatusNotFound indicates that the object could not be deleted because it didn't exist.
	DeleteStatusNotFound DeleteObjectsStatus = iota
	// DeleteStatusOK indicates that the object was successfully deleted.
	DeleteStatusOK
	// DeleteStatusInternalError indicates that an internal error occurred when attempting to delete the object.
	DeleteStatusInternalError
)

// DeleteObjectsResultItem contains the result of an attempt to delete a specific object from a bucket.
type DeleteObjectsResultItem struct {
	ObjectKey                ObjectKey
	RequestedStreamVersionID StreamVersionID

	Removed *DeleteObjectsInfo
	Marker  *DeleteObjectsInfo

	Status DeleteObjectsStatus
}

// DeleteObjectsInfo contains information about an object that was deleted or a delete marker that was inserted
// as a result of processing a DeleteObjects request item.
type DeleteObjectsInfo struct {
	StreamVersionID StreamVersionID
	Status          ObjectStatus
}

// DeleteObjects deletes specific objects from a bucket.
//
// TODO: Support Object Lock and properly handle buckets with versioning suspended.
func (db *DB) DeleteObjects(ctx context.Context, opts DeleteObjects) (result DeleteObjectsResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return DeleteObjectsResult{}, errs.Wrap(err)
	}

	if opts.Versioned {
		result, err = db.ChooseAdapter(opts.ProjectID).DeleteObjectsVersioned(ctx, opts)
	} else {
		result, err = db.ChooseAdapter(opts.ProjectID).DeleteObjectsPlain(ctx, opts)
	}
	if err != nil {
		return DeleteObjectsResult{}, errs.Wrap(err)
	}

	var deletedObjects int
	for _, item := range result.Items {
		if item.Status == DeleteStatusOK && item.Removed != nil {
			deletedObjects++
		}
	}
	if deletedObjects > 0 {
		mon.Meter("object_delete").Mark(deletedObjects)
	}
	if result.DeletedSegmentCount > 0 {
		mon.Meter("segment_delete").Mark64(result.DeletedSegmentCount)
	}

	return result, nil
}

type deleteObjectsSetupInfo struct {
	results        []DeleteObjectsResultItem
	resultsIndices map[DeleteObjectsItem]int
}

// processResults returns data that (*Adapter).DeleteObjects implementations require for executing database queries.
func (opts DeleteObjects) processResults() (info deleteObjectsSetupInfo) {
	info.resultsIndices = make(map[DeleteObjectsItem]int, len(opts.Items))
	i := 0
	for _, item := range opts.Items {
		if _, exists := info.resultsIndices[item]; !exists {
			info.resultsIndices[item] = i
			i++
		}
	}

	info.results = make([]DeleteObjectsResultItem, len(info.resultsIndices))
	for item, resultsIdx := range info.resultsIndices {
		info.results[resultsIdx] = DeleteObjectsResultItem{
			ObjectKey:                item.ObjectKey,
			RequestedStreamVersionID: item.StreamVersionID,
		}
	}

	return info
}

// DeleteObjectsPlain deletes specific objects from an unversioned bucket.
func (p *PostgresAdapter) DeleteObjectsPlain(ctx context.Context, opts DeleteObjects) (result DeleteObjectsResult, err error) {
	defer mon.Task()(&ctx)(&err)

	processedOpts := opts.processResults()
	result.Items = processedOpts.results

	now := time.Now().Truncate(time.Microsecond)

	for i := 0; i < len(processedOpts.results); i++ {
		resultItem := &processedOpts.results[i]

		if resultItem.RequestedStreamVersionID.IsZero() {
			err = Error.Wrap(withRows(
				p.db.QueryContext(ctx, `
					WITH deleted_objects AS (
						DELETE FROM objects
						WHERE
							(project_id, bucket_name, object_key) = ($1, $2, $3)
							AND status = `+statusCommittedUnversioned+`
							AND (expires_at IS NULL OR expires_at > $4)
						RETURNING version, stream_id
					), deleted_segments AS (
						DELETE FROM segments
						WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
						RETURNING 1
					)
					SELECT version, stream_id, (SELECT COUNT(*) FROM deleted_segments) FROM deleted_objects`,
					opts.ProjectID,
					opts.BucketName,
					resultItem.ObjectKey,
					now,
				),
			)(func(rows tagsql.Rows) error {
				if !rows.Next() {
					return nil
				}

				var (
					version      Version
					streamID     uuid.UUID
					segmentCount int64
				)
				if err := rows.Scan(&version, &streamID, &segmentCount); err != nil {
					return errs.Wrap(err)
				}

				result.DeletedSegmentCount += segmentCount

				sv := NewStreamVersionID(version, streamID)
				deleteInfo := &DeleteObjectsInfo{
					StreamVersionID: sv,
					Status:          CommittedUnversioned,
				}
				resultItem.Removed = deleteInfo
				resultItem.Status = DeleteStatusOK

				// Handle the case where an object was specified twice in the deletion request:
				// once with a version omitted and once with a version set. We must ensure that
				// when the object is deleted, both result items that reference it are updated.
				if i, ok := processedOpts.resultsIndices[DeleteObjectsItem{
					ObjectKey:       resultItem.ObjectKey,
					StreamVersionID: sv,
				}]; ok {
					processedOpts.results[i].Removed = deleteInfo
					processedOpts.results[i].Status = DeleteStatusOK
				}

				if rows.Next() {
					logMultipleCommittedVersionsError(p.log, ObjectLocation{
						ProjectID:  opts.ProjectID,
						BucketName: opts.BucketName,
						ObjectKey:  resultItem.ObjectKey,
					})
				}

				return nil
			}))
		} else {
			if resultItem.Status == DeleteStatusOK {
				continue
			}

			err = Error.Wrap(withRows(
				p.db.QueryContext(ctx, `
					WITH deleted_objects AS (
						DELETE FROM objects
						WHERE
							(project_id, bucket_name, object_key, version) = ($1, $2, $3, $4)
							AND SUBSTR(stream_id, 9) = $5
							AND (expires_at IS NULL OR expires_at > $6)
						RETURNING status, stream_id
					), deleted_segments AS (
						DELETE FROM segments
						WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
						RETURNING 1
					)
					SELECT status, (SELECT COUNT(*) FROM deleted_segments) FROM deleted_objects`,
					opts.ProjectID,
					opts.BucketName,
					resultItem.ObjectKey,
					resultItem.RequestedStreamVersionID.Version(),
					resultItem.RequestedStreamVersionID.StreamIDSuffix(),
					now,
				),
			)(func(rows tagsql.Rows) error {
				if !rows.Next() {
					return nil
				}

				var (
					status       ObjectStatus
					segmentCount int64
				)
				if err := rows.Scan(&status, &segmentCount); err != nil {
					return errs.Wrap(err)
				}

				result.DeletedSegmentCount += segmentCount

				deleteInfo := &DeleteObjectsInfo{
					StreamVersionID: resultItem.RequestedStreamVersionID,
					Status:          status,
				}
				resultItem.Removed = deleteInfo
				resultItem.Status = DeleteStatusOK

				if status == CommittedUnversioned {
					if i, ok := processedOpts.resultsIndices[DeleteObjectsItem{
						ObjectKey: resultItem.ObjectKey,
					}]; ok {
						processedOpts.results[i].Removed = deleteInfo
						processedOpts.results[i].Status = DeleteStatusOK
					}
				}

				return nil
			}))
		}

		if err != nil {
			for j := i; j < len(processedOpts.results); j++ {
				processedOpts.results[j].Status = DeleteStatusInternalError
			}
			break
		}
	}

	return result, err
}

func spannerDeleteSegmentsByStreamID(ctx context.Context, tx *spanner.ReadWriteTransaction, streamIDs [][]byte) (count int64, err error) {
	if len(streamIDs) == 0 {
		return 0, nil
	}
	count, err = tx.Update(ctx, spanner.Statement{
		SQL: `
			DELETE FROM segments
			WHERE stream_id IN UNNEST(@stream_ids)
		`,
		Params: map[string]interface{}{
			"stream_ids": streamIDs,
		},
	})
	return count, errs.Wrap(err)
}

// DeleteObjectsPlain deletes the specified objects from an unversioned bucket.
func (s *SpannerAdapter) DeleteObjectsPlain(ctx context.Context, opts DeleteObjects) (result DeleteObjectsResult, err error) {
	defer mon.Task()(&ctx)(&err)

	processedOpts := opts.processResults()
	result.Items = processedOpts.results

	now := time.Now().Truncate(time.Microsecond)

	for i := 0; i < len(processedOpts.results); i++ {
		resultItem := &processedOpts.results[i]

		var (
			deletedSegmentCount int64
			linkedResultItem    *DeleteObjectsResultItem
		)

		if resultItem.RequestedStreamVersionID.IsZero() {
			var multipleCommittedVersions bool
			_, err = s.client.ReadWriteTransaction(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) (err error) {
				deletedSegmentCount = 0
				multipleCommittedVersions = false
				linkedResultItem = nil

				var streamIDsToDelete [][]byte

				err = errs.Wrap(tx.Query(ctx, spanner.Statement{
					SQL: `
						DELETE FROM objects
						WHERE
							(project_id, bucket_name, object_key) = (@project_id, @bucket_name, @object_key)
							AND status = ` + statusCommittedUnversioned + `
							AND (expires_at IS NULL OR expires_at > @now)
						THEN RETURN version, stream_id
					`,
					Params: map[string]interface{}{
						"project_id":  opts.ProjectID,
						"bucket_name": opts.BucketName,
						"object_key":  resultItem.ObjectKey,
						"now":         now,
					},
				}).Do(func(row *spanner.Row) error {
					var (
						version       Version
						streamIDBytes []byte
					)
					if err := row.Columns(&version, &streamIDBytes); err != nil {
						return errs.Wrap(err)
					}

					streamIDsToDelete = append(streamIDsToDelete, streamIDBytes)

					if resultItem.Removed != nil {
						multipleCommittedVersions = true
						return nil
					}

					streamID, err := uuid.FromBytes(streamIDBytes)
					if err != nil {
						return errs.Wrap(err)
					}

					sv := NewStreamVersionID(version, streamID)
					deleteInfo := &DeleteObjectsInfo{
						StreamVersionID: sv,
						Status:          CommittedUnversioned,
					}
					resultItem.Removed = deleteInfo
					resultItem.Status = DeleteStatusOK

					// Handle the case where an object was specified twice in the deletion request:
					// once with a version omitted and once with a version set. We must ensure that
					// when the object is deleted, both deletion results that reference it are updated.
					if i, ok := processedOpts.resultsIndices[DeleteObjectsItem{
						ObjectKey:       resultItem.ObjectKey,
						StreamVersionID: sv,
					}]; ok {
						linkedResultItem = &processedOpts.results[i]
						linkedResultItem.Removed = deleteInfo
						linkedResultItem.Status = DeleteStatusOK
					}

					return nil
				}))
				if err != nil {
					return err
				}

				deletedSegmentCount, err = spannerDeleteSegmentsByStreamID(ctx, tx, streamIDsToDelete)
				return err
			})
			if err == nil && multipleCommittedVersions {
				logMultipleCommittedVersionsError(s.log, ObjectLocation{
					ProjectID:  opts.ProjectID,
					BucketName: opts.BucketName,
					ObjectKey:  resultItem.ObjectKey,
				})
			}
		} else {
			if resultItem.Status == DeleteStatusOK {
				continue
			}

			_, err = s.client.ReadWriteTransaction(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) (err error) {
				deletedSegmentCount = 0
				linkedResultItem = nil

				var streamID []byte

				err = errs.Wrap(tx.Query(ctx, spanner.Statement{
					SQL: `
						DELETE FROM objects
						WHERE
							(project_id, bucket_name, object_key, version) = (@project_id, @bucket_name, @object_key, @version)
							AND SUBSTR(stream_id, 9) = @stream_id_suffix
							AND (expires_at IS NULL OR expires_at > @now)
						THEN RETURN status, stream_id
					`,
					Params: map[string]interface{}{
						"project_id":       opts.ProjectID,
						"bucket_name":      opts.BucketName,
						"object_key":       resultItem.ObjectKey,
						"version":          resultItem.RequestedStreamVersionID.Version(),
						"stream_id_suffix": resultItem.RequestedStreamVersionID.StreamIDSuffix(),
						"now":              now,
					},
				}).Do(func(row *spanner.Row) error {
					var status ObjectStatus
					if err := row.Columns(&status, &streamID); err != nil {
						return errs.Wrap(err)
					}

					deleteInfo := &DeleteObjectsInfo{
						StreamVersionID: resultItem.RequestedStreamVersionID,
						Status:          status,
					}
					resultItem.Removed = deleteInfo
					resultItem.Status = DeleteStatusOK

					if status == CommittedUnversioned {
						if i, ok := processedOpts.resultsIndices[DeleteObjectsItem{
							ObjectKey: resultItem.ObjectKey,
						}]; ok {
							linkedResultItem = &processedOpts.results[i]
							linkedResultItem.Removed = deleteInfo
							linkedResultItem.Status = DeleteStatusOK
						}
					}

					return nil
				}))
				if err != nil || resultItem.Removed == nil {
					return err
				}

				deletedSegmentCount, err = spannerDeleteSegmentsByStreamID(ctx, tx, [][]byte{streamID})
				return err
			})
		}

		if err == nil {
			result.DeletedSegmentCount += deletedSegmentCount
		} else {
			resultItem.Removed = nil
			if linkedResultItem != nil {
				linkedResultItem.Removed = nil
			}
			for j := i; j < len(processedOpts.results); j++ {
				processedOpts.results[j].Status = DeleteStatusInternalError
			}
			break
		}
	}

	return result, Error.Wrap(err)
}

// DeleteObjectsVersioned deletes specific objects from a bucket with versioning enabled.
func (p *PostgresAdapter) DeleteObjectsVersioned(ctx context.Context, opts DeleteObjects) (result DeleteObjectsResult, err error) {
	defer mon.Task()(&ctx)(&err)

	processedOpts := opts.processResults()
	result.Items = processedOpts.results

	now := time.Now()
	for i := 0; i < len(processedOpts.results); i++ {
		resultItem := &processedOpts.results[i]
		if resultItem.RequestedStreamVersionID.IsZero() {
			var streamID uuid.UUID
			streamID, err = generateDeleteMarkerStreamID()
			if err != nil {
				break
			}

			err = Error.Wrap(withRows(
				p.db.QueryContext(ctx, `
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
					RETURNING version`,
					opts.ProjectID,
					opts.BucketName,
					resultItem.ObjectKey,
					streamID,
				),
			)(func(rows tagsql.Rows) error {
				if !rows.Next() {
					return errs.New("could not insert delete marker")
				}

				var version Version
				if err := rows.Scan(&version); err != nil {
					return errs.Wrap(err)
				}

				resultItem.Status = DeleteStatusOK
				resultItem.Marker = &DeleteObjectsInfo{
					StreamVersionID: NewStreamVersionID(version, streamID),
					Status:          DeleteMarkerVersioned,
				}

				return nil
			}))
		} else {
			// Prevent the removal of a delete marker that was added in a previous iteration.
			if i, ok := processedOpts.resultsIndices[DeleteObjectsItem{
				ObjectKey: resultItem.ObjectKey,
			}]; ok {
				marker := processedOpts.results[i].Marker
				if marker != nil && marker.StreamVersionID == resultItem.RequestedStreamVersionID {
					continue
				}
			}

			err = Error.Wrap(withRows(
				p.db.QueryContext(ctx, `
					WITH deleted_objects AS (
						DELETE FROM objects
						WHERE
							(project_id, bucket_name, object_key, version) = ($1, $2, $3, $4)
							AND SUBSTR(stream_id, 9) = $5
							AND (expires_at IS NULL OR expires_at > $6)
						RETURNING status, stream_id
					), deleted_segments AS (
						DELETE FROM segments
						WHERE segments.stream_id IN (SELECT deleted_objects.stream_id FROM deleted_objects)
						RETURNING 1
					)
					SELECT status, (SELECT COUNT(*) FROM deleted_segments) FROM deleted_objects`,
					opts.ProjectID,
					opts.BucketName,
					resultItem.ObjectKey,
					resultItem.RequestedStreamVersionID.Version(),
					resultItem.RequestedStreamVersionID.StreamIDSuffix(),
					now,
				),
			)(func(rows tagsql.Rows) error {
				if !rows.Next() {
					return nil
				}

				var (
					status       ObjectStatus
					segmentCount int64
				)
				if err := rows.Scan(&status, &segmentCount); err != nil {
					return errs.Wrap(err)
				}
				result.DeletedSegmentCount += segmentCount

				resultItem.Status = DeleteStatusOK
				resultItem.Removed = &DeleteObjectsInfo{
					StreamVersionID: resultItem.RequestedStreamVersionID,
					Status:          status,
				}

				return nil
			}))
		}
		if err != nil {
			for j := i; j < len(processedOpts.results); j++ {
				processedOpts.results[j].Status = DeleteStatusInternalError
			}
			break
		}
	}

	return result, err
}

// DeleteObjectsVersioned deletes specific objects from a bucket with versioning enabled.
func (s *SpannerAdapter) DeleteObjectsVersioned(ctx context.Context, opts DeleteObjects) (result DeleteObjectsResult, err error) {
	defer mon.Task()(&ctx)(&err)

	processedOpts := opts.processResults()
	result.Items = processedOpts.results

	now := time.Now()
	for i := 0; i < len(processedOpts.results); i++ {
		resultItem := &processedOpts.results[i]
		if resultItem.RequestedStreamVersionID.IsZero() {
			_, err = s.client.ReadWriteTransaction(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) (err error) {
				resultItem.Marker = nil
				resultItem.Status = DeleteStatusNotFound

				var streamID uuid.UUID
				streamID, err = generateDeleteMarkerStreamID()
				if err != nil {
					return err
				}

				return errs.Wrap(tx.Query(ctx, spanner.Statement{
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
							@stream_id,
							` + statusDeleteMarkerVersioned + `,
							NULL
						THEN RETURN version`,
					Params: map[string]interface{}{
						"project_id":  opts.ProjectID,
						"bucket_name": opts.BucketName,
						"object_key":  resultItem.ObjectKey,
						"stream_id":   streamID,
					},
				}).Do(func(row *spanner.Row) error {
					var version Version
					if err := row.Columns(&version); err != nil {
						return errs.Wrap(err)
					}

					resultItem.Marker = &DeleteObjectsInfo{
						StreamVersionID: NewStreamVersionID(version, streamID),
						Status:          DeleteMarkerVersioned,
					}
					resultItem.Status = DeleteStatusOK

					return nil
				}))
			})
			if err != nil {
				resultItem.Marker = nil
			}
		} else {
			// Prevent the removal of a delete marker that was added in a previous iteration.
			if i, ok := processedOpts.resultsIndices[DeleteObjectsItem{
				ObjectKey: resultItem.ObjectKey,
			}]; ok {
				marker := processedOpts.results[i].Marker
				if marker != nil && marker.StreamVersionID == resultItem.RequestedStreamVersionID {
					continue
				}
			}

			var deletedSegmentCount int64
			_, err = s.client.ReadWriteTransaction(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) (err error) {
				resultItem.Status = DeleteStatusNotFound
				deletedSegmentCount = 0

				var streamID []byte

				err = errs.Wrap(tx.Query(ctx, spanner.Statement{
					SQL: `
						DELETE FROM objects
						WHERE
							(project_id, bucket_name, object_key, version) = (@project_id, @bucket_name, @object_key, @version)
							AND SUBSTR(stream_id, 9) = @stream_id_suffix
							AND (expires_at IS NULL OR expires_at > @now)
						THEN RETURN status, stream_id`,
					Params: map[string]interface{}{
						"project_id":       opts.ProjectID,
						"bucket_name":      opts.BucketName,
						"object_key":       resultItem.ObjectKey,
						"version":          resultItem.RequestedStreamVersionID.Version(),
						"stream_id_suffix": resultItem.RequestedStreamVersionID.StreamIDSuffix(),
						"now":              now,
					},
				}).Do(func(row *spanner.Row) error {
					var status ObjectStatus
					if err := row.Columns(&status, &streamID); err != nil {
						return errs.Wrap(err)
					}

					resultItem.Status = DeleteStatusOK
					resultItem.Removed = &DeleteObjectsInfo{
						StreamVersionID: resultItem.RequestedStreamVersionID,
						Status:          status,
					}

					return nil
				}))
				if err != nil {
					return err
				}

				deletedSegmentCount, err = spannerDeleteSegmentsByStreamID(ctx, tx, [][]byte{streamID})
				return err
			})
			if err == nil {
				result.DeletedSegmentCount += deletedSegmentCount
			} else {
				resultItem.Removed = nil
			}
		}
		if err != nil {
			for j := i; j < len(processedOpts.results); j++ {
				processedOpts.results[j].Status = DeleteStatusInternalError
			}
			break
		}
	}

	return result, Error.Wrap(err)
}
