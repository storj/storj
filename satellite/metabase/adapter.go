// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"

	"github.com/storj/exp-spanner"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/tagsql"
)

// Adapter is a low level extension point to use datasource related queries.
// TODO: we may need separated adapter for segments/objects/etc.
type Adapter interface {
	BeginObjectNextVersion(context.Context, BeginObjectNextVersion, *Object) error
	GetObjectLastCommitted(ctx context.Context, opts GetObjectLastCommitted, object *Object) error
	IterateLoopSegments(ctx context.Context, aliasCache *NodeAliasCache, opts IterateLoopSegments, fn func(context.Context, LoopSegmentsIterator) error) error
	PendingObjectExists(ctx context.Context, opts BeginSegment) (exists bool, err error)
	CommitPendingObjectSegment(ctx context.Context, opts CommitSegment, aliasPieces AliasPieces) error
	CommitInlineSegment(ctx context.Context, opts CommitInlineSegment) error
	TestingBeginObjectExactVersion(ctx context.Context, opts BeginObjectExactVersion, object *Object) error

	GetTableStats(ctx context.Context, opts GetTableStats) (result TableStats, err error)
	BucketEmpty(ctx context.Context, opts BucketEmpty) (empty bool, err error)

	WithTx(ctx context.Context, f func(context.Context, TransactionAdapter) error) error

	GetSegmentByPosition(ctx context.Context, opts GetSegmentByPosition) (segment Segment, aliasPieces AliasPieces, err error)
	GetObjectExactVersion(ctx context.Context, opts GetObjectExactVersion) (_ Object, err error)
	GetSegmentPositionsAndKeys(ctx context.Context, streamID uuid.UUID) (keysNonces []EncryptedKeyAndNonce, err error)
	GetLatestObjectLastSegment(ctx context.Context, opts GetLatestObjectLastSegment) (segment Segment, aliasPieces AliasPieces, err error)

	ListObjects(ctx context.Context, opts ListObjects) (result ListObjectsResult, err error)
	ListSegments(ctx context.Context, opts ListSegments, aliasCache *NodeAliasCache) (result ListSegmentsResult, err error)
	ListStreamPositions(ctx context.Context, opts ListStreamPositions) (result ListStreamPositionsResult, err error)

	DeleteObjectExactVersion(ctx context.Context, opts DeleteObjectExactVersion) (result DeleteObjectResult, err error)
	DeletePendingObject(ctx context.Context, opts DeletePendingObject) (result DeleteObjectResult, err error)
	DeleteObjectsAllVersions(ctx context.Context, projectID uuid.UUID, bucketName string, objectKeys [][]byte) (result DeleteObjectResult, err error)
	DeleteObjectLastCommittedPlain(ctx context.Context, opts DeleteObjectLastCommitted) (result DeleteObjectResult, err error)
	DeleteObjectLastCommittedSuspended(ctx context.Context, opts DeleteObjectLastCommitted, deleterMarkerStreamID uuid.UUID) (result DeleteObjectResult, err error)
	DeleteObjectLastCommittedVersioned(ctx context.Context, opts DeleteObjectLastCommitted, deleterMarkerStreamID uuid.UUID) (result DeleteObjectResult, err error)

	FindExpiredObjects(ctx context.Context, opts DeleteExpiredObjects, startAfter ObjectStream, batchSize int) (expiredObjects []ObjectStream, err error)
	DeleteObjectsAndSegments(ctx context.Context, objects []ObjectStream) (objectsDeleted, segmentsDeleted int64, err error)
	FindZombieObjects(ctx context.Context, opts DeleteZombieObjects, startAfter ObjectStream, batchSize int) (objects []ObjectStream, err error)
	DeleteInactiveObjectsAndSegments(ctx context.Context, objects []ObjectStream, opts DeleteZombieObjects) (objectsDeleted, segmentsDeleted int64, err error)

	EnsureNodeAliases(ctx context.Context, opts EnsureNodeAliases) error
	ListNodeAliases(ctx context.Context) (_ []NodeAliasEntry, err error)

	TestingBatchInsertSegments(ctx context.Context, aliasCache *NodeAliasCache, segments []RawSegment) (err error)
	TestingGetAllObjects(ctx context.Context) (_ []RawObject, err error)
	TestingGetAllSegments(ctx context.Context, aliasCache *NodeAliasCache) (_ []RawSegment, err error)
	TestingDeleteAll(ctx context.Context) (err error)
	TestingBatchInsertObjects(ctx context.Context, objects []RawObject) (err error)
}

// PostgresAdapter uses Cockroach related SQL queries.
type PostgresAdapter struct {
	log  *zap.Logger
	db   tagsql.DB
	impl dbutil.Implementation
}

var _ Adapter = &PostgresAdapter{}

// CockroachAdapter uses Cockroach related SQL queries.
type CockroachAdapter struct {
	PostgresAdapter
}

var _ Adapter = &CockroachAdapter{}

// TransactionAdapter is a low level extension point to use datasource related queries inside of a transaction.
type TransactionAdapter interface {
	commitObjectTransactionAdapter
	commitObjectWithSegmentsTransactionAdapter
	copyObjectTransactionAdapter
	moveObjectTransactionAdapter
	deleteTransactionAdapter
}

type postgresTransactionAdapter struct {
	postgresAdapter *PostgresAdapter
	tx              tagsql.Tx
}

var _ TransactionAdapter = &postgresTransactionAdapter{}

type spannerTransactionAdapter struct {
	spannerAdapter *SpannerAdapter
	tx             *spanner.ReadWriteTransaction
}

var _ TransactionAdapter = &spannerTransactionAdapter{}
