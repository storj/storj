// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"time"

	"cloud.google.com/go/spanner"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"

	"storj.io/common/sync2"
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

	for _, adapter := range db.adapters {
		err := adapter.IterateExpiredObjects(ctx, opts, func(ctx context.Context, objects []ObjectStream) error {
			objects = slices.Clone(objects)

			for _, object := range objects {
				db.log.Debug("Deleting expired object",
					zap.Stringer("Project", object.ProjectID),
					zap.Stringer("Bucket", object.BucketName),
					zap.String("Object Key", string(object.ObjectKey)),
					zap.Int64("Version", int64(object.Version)),
					zap.Stringer("StreamID", object.StreamID),
				)
			}

			ok := limiter.Go(ctx, func() {
				objectsDeleted, segmentsDeleted, err := adapter.DeleteObjectsAndSegmentsNoVerify(ctx, objects)
				if err != nil {
					db.log.Error("failed to delete expired objects from DB", zap.Error(err))
				}

				mon.Meter("expired_object_delete").Mark64(objectsDeleted)
				mon.Meter("expired_segment_delete").Mark64(segmentsDeleted)
			})
			if !ok {
				return Error.New("unable to start delete operation")
			}
			return nil
		})
		if err != nil {
			db.log.Warn("delete from DB zombie objects", zap.Error(err))
		}
	}

	limiter.Wait()

	return nil
}

// IterateExpiredObjects iterates over all expired objects that expired before opts.ExpiredBefore and calls process with at most opts.BatchSize objects.
func (p *PostgresAdapter) IterateExpiredObjects(ctx context.Context, opts DeleteExpiredObjects, process func(context.Context, []ObjectStream) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	return Error.Wrap(p.processObjectStreamBatches(ctx, opts.AsOfSystemInterval, opts.BatchSize, postgresStatement{
		SQL: `
			SELECT project_id, bucket_name, object_key, version, stream_id
			FROM objects
			` + p.impl.AsOfSystemInterval(opts.AsOfSystemInterval) + `
			WHERE expires_at < $1
		`,
		Params: []any{
			opts.ExpiredBefore,
		},
	}, process))
}

// IterateExpiredObjects iterates over all expired objects that expired before opts.ExpiredBefore and calls process with at most opts.BatchSize objects.
func (s *SpannerAdapter) IterateExpiredObjects(ctx context.Context, opts DeleteExpiredObjects, process func(context.Context, []ObjectStream) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	return Error.Wrap(s.processObjectStreamBatches(ctx, opts.AsOfSystemInterval, opts.BatchSize, spanner.Statement{
		SQL: `
			SELECT project_id, bucket_name, object_key, version, stream_id
			FROM objects
			WHERE expires_at < @expires_at
		`,
		Params: map[string]any{
			"expires_at": opts.ExpiredBefore,
		},
	}, process))
}
