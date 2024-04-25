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
	TestingBeginObjectExactVersion(ctx context.Context, opts BeginObjectExactVersion, object *Object) error

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
