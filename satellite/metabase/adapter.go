// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"time"

	"cloud.google.com/go/spanner"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/tagsql"
)

// TransactionOptions contains options for transaction.
type TransactionOptions struct {
	// supported only by Spanner.
	MaxCommitDelay *time.Duration
	TransactionTag string

	// supported only by Spanner.
	TransmitEvent bool
}

// Adapter is a low level extension point to use datasource related queries.
// TODO: we may need separated adapter for segments/objects/etc.
type Adapter interface {
	Name() string
	Now(ctx context.Context) (time.Time, error)
	Ping(ctx context.Context) error
	MigrateToLatest(ctx context.Context) error
	CheckVersion(ctx context.Context) error
	Implementation() dbutil.Implementation

	BeginObjectNextVersion(context.Context, BeginObjectNextVersion, *Object) error
	GetObjectLastCommitted(ctx context.Context, opts GetObjectLastCommitted) (Object, error)
	IterateLoopSegments(ctx context.Context, aliasCache *NodeAliasCache, opts IterateLoopSegments, fn func(context.Context, LoopSegmentsIterator) error) error
	PendingObjectExists(ctx context.Context, opts BeginSegment) (exists bool, err error)
	CommitPendingObjectSegment(ctx context.Context, opts CommitSegment, aliasPieces AliasPieces) error
	CommitInlineSegment(ctx context.Context, opts CommitInlineSegment) error
	BeginObjectExactVersion(ctx context.Context, opts BeginObjectExactVersion, object *Object) error

	GetObjectExactVersionRetention(ctx context.Context, opts GetObjectExactVersionRetention) (retention Retention, err error)
	GetObjectLastCommittedRetention(ctx context.Context, opts GetObjectLastCommittedRetention) (retention Retention, err error)
	SetObjectExactVersionRetention(ctx context.Context, opts SetObjectExactVersionRetention) error
	SetObjectLastCommittedRetention(ctx context.Context, opts SetObjectLastCommittedRetention) error

	GetObjectExactVersionLegalHold(ctx context.Context, opts GetObjectExactVersionLegalHold) (enabled bool, err error)
	GetObjectLastCommittedLegalHold(ctx context.Context, opts GetObjectLastCommittedLegalHold) (enabled bool, err error)
	SetObjectExactVersionLegalHold(ctx context.Context, opts SetObjectExactVersionLegalHold) error
	SetObjectLastCommittedLegalHold(ctx context.Context, opts SetObjectLastCommittedLegalHold) error

	GetTableStats(ctx context.Context, opts GetTableStats) (result TableStats, err error)
	CountSegments(ctx context.Context, checkTimestamp time.Time) (result int64, err error)
	UpdateTableStats(ctx context.Context) error
	BucketEmpty(ctx context.Context, opts BucketEmpty) (empty bool, err error)

	WithTx(ctx context.Context, opts TransactionOptions, f func(context.Context, TransactionAdapter) error) error

	CollectBucketTallies(ctx context.Context, opts CollectBucketTallies) (result []BucketTally, err error)

	GetSegmentByPosition(ctx context.Context, opts GetSegmentByPosition) (segment Segment, aliasPieces AliasPieces, err error)
	GetSegmentByPositionForAudit(ctx context.Context, opts GetSegmentByPosition) (segment SegmentForAudit, aliasPieces AliasPieces, err error)
	GetSegmentByPositionForRepair(ctx context.Context, opts GetSegmentByPosition) (segment SegmentForRepair, aliasPieces AliasPieces, err error)
	CheckSegmentPiecesAlteration(ctx context.Context, streamID uuid.UUID, position SegmentPosition, aliasPieces AliasPieces) (altered bool, err error)
	GetObjectExactVersion(ctx context.Context, opts GetObjectExactVersion) (_ Object, err error)
	GetSegmentPositionsAndKeys(ctx context.Context, streamID uuid.UUID) (keysNonces []EncryptedKeyAndNonce, err error)
	GetLatestObjectLastSegment(ctx context.Context, opts GetLatestObjectLastSegment) (segment Segment, aliasPieces AliasPieces, err error)

	ListObjects(ctx context.Context, opts ListObjects) (result ListObjectsResult, err error)
	ListSegments(ctx context.Context, opts ListSegments, aliasCache *NodeAliasCache) (result ListSegmentsResult, err error)
	ListStreamPositions(ctx context.Context, opts ListStreamPositions) (result ListStreamPositionsResult, err error)
	ListVerifySegments(ctx context.Context, opts ListVerifySegments) (segments []VerifySegment, err error)
	ListBucketStreamIDs(ctx context.Context, opts ListBucketStreamIDs, process func(ctx context.Context, streamIDs []uuid.UUID) error) (err error)

	UpdateSegmentPieces(ctx context.Context, opts UpdateSegmentPieces, oldPieces, newPieces AliasPieces) (resultPieces AliasPieces, err error)
	UpdateObjectLastCommittedMetadata(ctx context.Context, opts UpdateObjectLastCommittedMetadata) (affected int64, err error)

	DeleteObjectExactVersion(ctx context.Context, opts DeleteObjectExactVersion) (result DeleteObjectResult, err error)
	DeletePendingObject(ctx context.Context, opts DeletePendingObject) (result DeleteObjectResult, err error)

	DeleteObjectLastCommittedPlain(ctx context.Context, opts DeleteObjectLastCommitted) (result DeleteObjectResult, err error)
	DeleteObjectLastCommittedVersioned(ctx context.Context, opts DeleteObjectLastCommitted, deleterMarkerStreamID uuid.UUID) (result DeleteObjectResult, err error)

	IterateExpiredObjects(ctx context.Context, opts DeleteExpiredObjects, process func(context.Context, []ObjectStream) error) (err error)
	DeleteObjectsAndSegmentsNoVerify(ctx context.Context, objects []ObjectStream) (objectsDeleted, segmentsDeleted int64, err error)
	IterateZombieObjects(ctx context.Context, opts DeleteZombieObjects, process func(context.Context, []ObjectStream) error) (err error)
	DeleteInactiveObjectsAndSegments(ctx context.Context, objects []ObjectStream, opts DeleteZombieObjects) (objectsDeleted, segmentsDeleted int64, err error)
	DeleteAllBucketObjects(ctx context.Context, opts DeleteAllBucketObjects) (deletedObjectCount, deletedSegmentCount int64, err error)
	UncoordinatedDeleteAllBucketObjects(ctx context.Context, opts UncoordinatedDeleteAllBucketObjects) (deletedObjectCount, deletedSegmentCount int64, err error)

	EnsureNodeAliases(ctx context.Context, opts EnsureNodeAliases) error
	ListNodeAliases(ctx context.Context) (entries []NodeAliasEntry, err error)
	GetNodeAliasEntries(ctx context.Context, opts GetNodeAliasEntries) (entries []NodeAliasEntry, err error)
	GetStreamPieceCountByAlias(ctx context.Context, opts GetStreamPieceCountByNodeID) (result map[NodeAlias]int64, err error)

	doNextQueryAllVersionsWithStatus(ctx context.Context, it *objectsIterator) (_ tagsql.Rows, err error)
	doNextQueryAllVersionsWithStatusAscending(ctx context.Context, it *objectsIterator) (_ tagsql.Rows, err error)
	doNextQueryPendingObjectsByKey(ctx context.Context, it *objectsIterator) (_ tagsql.Rows, err error)

	TestingBatchInsertSegments(ctx context.Context, aliasCache *NodeAliasCache, segments []RawSegment) (err error)
	TestingGetAllObjects(ctx context.Context) (_ []RawObject, err error)
	TestingGetAllSegments(ctx context.Context, aliasCache *NodeAliasCache) (_ []RawSegment, err error)
	TestingDeleteAll(ctx context.Context) (err error)
	TestingBatchInsertObjects(ctx context.Context, objects []RawObject) (err error)
	TestingSetObjectVersion(ctx context.Context, object ObjectStream, randomVersion Version) (rowsAffected int64, err error)
	TestingSetPlacementAllSegments(ctx context.Context, placement storj.PlacementConstraint) (err error)

	// TestMigrateToLatest creates a database and applies all the migration for test purposes.
	TestMigrateToLatest(ctx context.Context) error

	copyObjectAdapter
}

// PostgresAdapter uses Cockroach related SQL queries.
type PostgresAdapter struct {
	log                        *zap.Logger
	db                         tagsql.DB
	impl                       dbutil.Implementation
	connstr                    string
	testingUniqueUnversioned   bool
	testingTimestampVersioning bool
}

// Name returns the name of the adapter.
func (p *PostgresAdapter) Name() string {
	return "postgres"
}

// UnderlyingDB returns a handle to the underlying DB.
func (p *PostgresAdapter) UnderlyingDB() tagsql.DB {
	return p.db
}

// Implementation returns the dbutil.Implementation code for this adapter.
func (p *PostgresAdapter) Implementation() dbutil.Implementation {
	return p.impl
}

var _ Adapter = &PostgresAdapter{}

// CockroachAdapter uses Cockroach related SQL queries.
type CockroachAdapter struct {
	PostgresAdapter
}

// Name returns the name of the adapter.
func (c *CockroachAdapter) Name() string {
	return "cockroach"
}

var _ Adapter = &CockroachAdapter{}

// TransactionAdapter is a low level extension point to use datasource related queries inside of a transaction.
type TransactionAdapter interface {
	commitObjectTransactionAdapter
	commitObjectWithSegmentsTransactionAdapter
	copyObjectTransactionAdapter
	moveObjectTransactionAdapter
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
