// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"time"
)

const (
	uncoordinatedDeleteBatchSizeLimit = intLimitRange(10000)
)

// UncoordinatedDeleteAllBucketObjects contains arguments for deleting a whole bucket.
type UncoordinatedDeleteAllBucketObjects struct {
	Bucket    BucketLocation
	BatchSize int

	MaxCommitDelay *time.Duration

	// OnObjectsDeleted is called per batch with object info for deleted objects in that batch.
	// When nil, object info is not collected.
	OnObjectsDeleted func([]DeleteObjectsInfo)
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
		Bucket:           opts.Bucket,
		BatchSize:        opts.BatchSize,
		OnObjectsDeleted: opts.OnObjectsDeleted,
	})
}

// UncoordinatedDeleteAllBucketObjects deletes objects in the specified bucket in batches of opts.BatchSize number of objects.
func (t *TiDBAdapter) UncoordinatedDeleteAllBucketObjects(ctx context.Context, opts UncoordinatedDeleteAllBucketObjects) (totalDeletedObjects, totalDeletedSegments int64, err error) {
	defer mon.Task()(&ctx)(&err)

	return t.DeleteAllBucketObjects(ctx, DeleteAllBucketObjects{
		Bucket:           opts.Bucket,
		BatchSize:        opts.BatchSize,
		OnObjectsDeleted: opts.OnObjectsDeleted,
	})
}
