// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"

	"go.uber.org/zap"

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
	TestingBeginObjectExactVersion(ctx context.Context, opts BeginObjectExactVersion, object *Object) error

	GetTableStats(ctx context.Context, opts GetTableStats) (result TableStats, err error)

	WithTx(ctx context.Context, f func(context.Context, TransactionAdapter) error) error

	EnsureNodeAliases(ctx context.Context, opts EnsureNodeAliases) error
	ListNodeAliases(ctx context.Context) (_ []NodeAliasEntry, err error)

	TestingBatchInsertSegments(ctx context.Context, aliasCache *NodeAliasCache, segments []RawSegment) (err error)
	TestingGetAllSegments(ctx context.Context, aliasCache *NodeAliasCache) (_ []RawSegment, err error)
	TestingDeleteAll(ctx context.Context) (err error)
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
}

type postgresTransactionAdapter struct {
	postgresAdapter *PostgresAdapter
	tx              tagsql.Tx
}

var _ TransactionAdapter = &postgresTransactionAdapter{}
