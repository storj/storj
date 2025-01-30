// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/spanner"
	"cloud.google.com/go/spanner/apiv1/spannerpb"
	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/storj/shared/dbutil/spannerutil"
	"storj.io/storj/shared/tagsql"
)

// DeleteExpiredObjects contains all the information necessary to delete expired objects and segments.
type DeleteExpiredObjects struct {
	ExpiredBefore      time.Time
	AsOfSystemInterval time.Duration
	BatchSize          int
	DeleteConcurrency  int
}

// DeleteExpiredObjects deletes all objects that expired before expiredBefore.
func (db *DB) DeleteExpiredObjects(ctx context.Context, opts DeleteExpiredObjects) (err error) {
	defer mon.Task()(&ctx)(&err)

	if opts.DeleteConcurrency == 0 {
		opts.DeleteConcurrency = 1
	}

	limiter := sync2.NewLimiter(opts.DeleteConcurrency)

	for _, a := range db.adapters {
		err = db.deleteObjectsAndSegmentsBatch(ctx, opts.BatchSize, func(startAfter ObjectStream, batchsize int) (last ObjectStream, err error) {
			expiredObjects, err := a.FindExpiredObjects(ctx, opts, startAfter, batchsize)
			if err != nil {
				return ObjectStream{}, Error.New("unable to select expired objects for deletion: %w", err)
			}

			if len(expiredObjects) == 0 {
				return ObjectStream{}, nil
			}

			ok := limiter.Go(ctx, func() {
				objectsDeleted, segmentsDeleted, err := a.DeleteObjectsAndSegmentsNoVerify(ctx, expiredObjects)
				if err != nil {
					db.log.Error("failed to delete expired objects from DB", zap.Error(err), zap.String("adapter", fmt.Sprintf("%T", a)))
				}

				mon.Meter("expired_object_delete").Mark64(objectsDeleted)
				mon.Meter("expired_segment_delete").Mark64(segmentsDeleted)
			})
			if !ok {
				return ObjectStream{}, Error.New("unable to start delete operation")
			}

			return expiredObjects[len(expiredObjects)-1], err
		})
		if err != nil {
			db.log.Error("failed to find expired objects in DB", zap.Error(err), zap.String("adapter", fmt.Sprintf("%T", a)))
		}
	}
	limiter.Wait()
	return nil
}

// FindExpiredObjects finds up to batchSize objects that expired before opts.ExpiredBefore.
func (p *PostgresAdapter) FindExpiredObjects(ctx context.Context, opts DeleteExpiredObjects, startAfter ObjectStream, batchSize int) (expiredObjects []ObjectStream, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `
		SELECT
			project_id, bucket_name, object_key, version, stream_id,
			expires_at
		FROM objects
		` + p.impl.AsOfSystemInterval(opts.AsOfSystemInterval) + `
		WHERE
			(project_id, bucket_name, object_key, version) > ($1, $2, $3, $4)
			AND expires_at < $5
			ORDER BY project_id, bucket_name, object_key, version
		LIMIT $6;
	`

	expiredObjects = make([]ObjectStream, 0, batchSize)

	err = withRows(p.db.QueryContext(ctx, query,
		startAfter.ProjectID, startAfter.BucketName, []byte(startAfter.ObjectKey), startAfter.Version,
		opts.ExpiredBefore,
		batchSize),
	)(func(rows tagsql.Rows) error {
		var last ObjectStream
		for rows.Next() {
			var expiresAt time.Time
			err = rows.Scan(
				&last.ProjectID, &last.BucketName, &last.ObjectKey, &last.Version, &last.StreamID,
				&expiresAt)
			if err != nil {
				return Error.Wrap(err)
			}

			p.log.Debug("Deleting expired object",
				zap.Stringer("Project", last.ProjectID),
				zap.Stringer("Bucket", last.BucketName),
				zap.String("Object Key", string(last.ObjectKey)),
				zap.Int64("Version", int64(last.Version)),
				zap.Stringer("StreamID", last.StreamID),
				zap.Time("Expired At", expiresAt),
			)
			expiredObjects = append(expiredObjects, last)
		}

		return nil
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return expiredObjects, nil
}

// FindExpiredObjects finds up to batchSize objects that expired before opts.ExpiredBefore.
func (s *SpannerAdapter) FindExpiredObjects(ctx context.Context, opts DeleteExpiredObjects, startAfter ObjectStream, batchSize int) (expiredObjects []ObjectStream, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO: make util for using stale reads
	transaction := s.client.Single()
	if opts.AsOfSystemInterval != 0 {
		// spanner requires non-negative staleness
		staleness := opts.AsOfSystemInterval
		if staleness < 0 {
			staleness *= -1
		}

		transaction = transaction.WithTimestampBound(spanner.MaxStaleness(staleness))
	}

	// TODO(spanner): check whether this query is executed efficiently
	expiredObjects, err = spannerutil.CollectRows(transaction.QueryWithOptions(ctx, spanner.Statement{
		SQL: `
			SELECT
				project_id, bucket_name, object_key, version, stream_id,
				expires_at
			FROM objects
			WHERE
				expires_at < @expires_at
				AND (
					project_id > @project_id
					OR (project_id = @project_id AND bucket_name > @bucket_name)
					OR (project_id = @project_id AND bucket_name = @bucket_name AND object_key > @object_key)
					OR (project_id = @project_id AND bucket_name = @bucket_name AND object_key = @object_key AND version > @version)
				)
				ORDER BY project_id, bucket_name, object_key, version
			LIMIT @batch_size;
		`, Params: map[string]interface{}{
			"project_id":  startAfter.ProjectID,
			"bucket_name": startAfter.BucketName,
			"object_key":  startAfter.ObjectKey,
			"version":     startAfter.Version,
			"expires_at":  opts.ExpiredBefore,
			"batch_size":  batchSize,
		},
	}, spanner.QueryOptions{
		Priority: spannerpb.RequestOptions_PRIORITY_LOW,
	}), func(row *spanner.Row, object *ObjectStream) error {
		var expiresAt time.Time
		err := row.Columns(
			&object.ProjectID, &object.BucketName, &object.ObjectKey, &object.Version, &object.StreamID,
			&expiresAt)
		if err != nil {
			return Error.Wrap(err)
		}

		s.log.Debug("Deleting expired object",
			zap.Stringer("Project", object.ProjectID),
			zap.Stringer("Bucket", object.BucketName),
			zap.String("Object Key", string(object.ObjectKey)),
			zap.Int64("Version", int64(object.Version)),
			zap.Stringer("StreamID", object.StreamID),
			zap.Time("Expired At", expiresAt),
		)

		return nil
	})

	return expiredObjects, Error.Wrap(err)
}

func (db *DB) deleteObjectsAndSegmentsBatch(ctx context.Context, batchsize int, deleteBatch func(startAfter ObjectStream, batchsize int) (last ObjectStream, err error)) (err error) {
	defer mon.Task()(&ctx)(&err)

	deleteBatchsizeLimit.Ensure(&batchsize)

	var startAfter ObjectStream
	for {
		lastDeleted, err := deleteBatch(startAfter, batchsize)
		if err != nil {
			return err
		}
		if lastDeleted.StreamID.IsZero() {
			return nil
		}
		startAfter = lastDeleted
	}
}
