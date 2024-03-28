// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"

	"storj.io/common/tagsql"
)

// Adapter is a low level extension point to use datasource related queries.
// TODO: we may need separated adapter for segments/objects/etc.
type Adapter interface {
	BeginObjectNextVersion(context.Context, BeginObjectNextVersion, *Object) error
	GetObjectLastCommitted(ctx context.Context, opts GetObjectLastCommitted, object *Object) error
	TestingBeginObjectExactVersion(ctx context.Context, opts BeginObjectExactVersion, object *Object) error
}

// PostgresAdapter uses Cockroach related SQL queries.
type PostgresAdapter struct {
	db tagsql.DB
}

var _ Adapter = &PostgresAdapter{}

// CockroachAdapter uses Cockroach related SQL queries.
type CockroachAdapter struct {
	PostgresAdapter
}

var _ Adapter = &CockroachAdapter{}
