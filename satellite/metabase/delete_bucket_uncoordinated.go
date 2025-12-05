// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"time"

	"cloud.google.com/go/spanner"
	"cloud.google.com/go/spanner/apiv1/spannerpb"

	"storj.io/common/uuid"
)

const (
	uncoordinatedDeleteBatchSizeLimit = intLimitRange(10000)
)

// UncoordinatedDeleteAllBucketObjects contains arguments for deleting a whole bucket.
type UncoordinatedDeleteAllBucketObjects struct {
	Bucket    BucketLocation
	BatchSize int

	// supported only by Spanner.
	StalenessTimestampBound spanner.TimestampBound
	MaxCommitDelay          *time.Duration
}

// UncoordinatedDeleteAllBucketObjects deletes all objects in the specified bucket.
//
// This deletion does not force the operations across the tables to be synchronized, speeding up the deletion.
// If there are any ongoing uploads/downloads/deletes it may create zombie segments.
//
// Currently there's no special implementation for Postgres and Cockroach.
func (db *DB) UncoordinatedDeleteAllBucketObjects(ctx context.Context, opts UncoordinatedDeleteAllBucketObjects) (deletedObjects int64, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Bucket.Verify(); err != nil {
		return 0, err
	}

	uncoordinatedDeleteBatchSizeLimit.Ensure(&opts.BatchSize)

	deletedBatchObjectCount, deletedBatchSegmentCount, err := db.ChooseAdapter(opts.Bucket.ProjectID).UncoordinatedDeleteAllBucketObjects(ctx, opts)
	mon.Meter("object_delete").Mark64(deletedBatchObjectCount)
	mon.Meter("segment_delete").Mark64(deletedBatchSegmentCount)

	return deletedBatchObjectCount, err
}

// UncoordinatedDeleteAllBucketObjects deletes objects in the specified bucket in batches of opts.BatchSize number of objects.
func (p *PostgresAdapter) UncoordinatedDeleteAllBucketObjects(ctx context.Context, opts UncoordinatedDeleteAllBucketObjects) (totalDeletedObjects, totalDeletedSegments int64, err error) {
	defer mon.Task()(&ctx)(&err)

	return p.DeleteAllBucketObjects(ctx, DeleteAllBucketObjects{
		Bucket:    opts.Bucket,
		BatchSize: opts.BatchSize,
	})
}

// UncoordinatedDeleteAllBucketObjects deletes objects in the specified bucket in batches of opts.BatchSize number of objects.
func (s *SpannerAdapter) UncoordinatedDeleteAllBucketObjects(ctx context.Context, opts UncoordinatedDeleteAllBucketObjects) (totalDeletedObjects, totalDeletedSegments int64, err error) {
	defer mon.Task()(&ctx)(&err)

	var batchObjectCount, batchSegmentCount int64
	var mutations []*spanner.Mutation

	flushMutationGroups := func() error {
		defer func() {
			mutations = mutations[:0]
			batchObjectCount, batchSegmentCount = 0, 0
		}()

		_, err := s.client.Apply(ctx, mutations,
			spanner.ApplyAtLeastOnce(),
			spanner.Priority(spannerpb.RequestOptions_PRIORITY_MEDIUM),
			spanner.TransactionTag("uncoordinated-delete-all-bucket-objects"),
			spanner.ApplyCommitOptions(spanner.CommitOptions{
				MaxCommitDelay: opts.MaxCommitDelay,
			}),
		)
		if err != nil {
			return Error.New("failed to delete bucket batch: %w", err)
		}
		totalDeletedObjects += batchObjectCount
		totalDeletedSegments += batchSegmentCount

		return nil
	}

	// Note, there's a potential logical race with this approach:
	//
	//   1. delete finds object A with stream id X
	//   2. user deletes that object A#X
	//   3. user uploads object A with stream id Y
	//   4. deletes object A#X, and deletes stream X, leaving stream Y dangling
	err = s.client.Single().WithTimestampBound(opts.StalenessTimestampBound).ReadWithOptions(ctx, "objects", spanner.KeyRange{
		Start: spanner.Key{opts.Bucket.ProjectID, opts.Bucket.BucketName},
		End:   spanner.Key{opts.Bucket.ProjectID, opts.Bucket.BucketName},
		Kind:  spanner.ClosedClosed,
	}, []string{"object_key", "version", "stream_id", "status", "segment_count"},
		&spanner.ReadOptions{
			Priority:   spannerpb.RequestOptions_PRIORITY_MEDIUM,
			RequestTag: "uncoordinated-delete-all-bucket-objects-iterate",
		}).Do(func(r *spanner.Row) error {
		var objectKey ObjectKey
		var version Version
		var streamID []byte
		var status ObjectStatus
		var segmentCount int64

		err := r.Columns(&objectKey, &version, &streamID, &status, &segmentCount)
		if err != nil {
			return Error.Wrap(err)
		}

		if len(streamID) != len(uuid.UUID{}) {
			return Error.New("invalid stream id for object %q version %v", objectKey, version)
		}

		mutations = append(mutations,
			spanner.Delete("objects", spanner.Key{opts.Bucket.ProjectID, opts.Bucket.BucketName, objectKey, int64(version)}),
		)

		if segmentCount > 0 || status.IsPending() {
			mutations = append(mutations,
				spanner.Delete("segments", spanner.KeyRange{
					Start: spanner.Key{streamID},
					End:   spanner.Key{streamID},
					Kind:  spanner.ClosedClosed,
				}))
		}

		batchSegmentCount += segmentCount
		batchObjectCount++

		if len(mutations) >= opts.BatchSize {
			if err := flushMutationGroups(); err != nil {
				return Error.Wrap(err)
			}
		}

		return nil
	})
	if err != nil {
		return totalDeletedObjects, totalDeletedSegments, Error.Wrap(err)
	}

	if len(mutations) > 0 {
		if err := flushMutationGroups(); err != nil {
			return totalDeletedObjects, totalDeletedSegments, Error.Wrap(err)
		}
	}

	return totalDeletedObjects, totalDeletedSegments, nil
}
