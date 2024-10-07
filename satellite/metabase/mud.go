// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	_ "embed"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/private/mud"
	"storj.io/storj/shared/dbutil/spannerutil"
)

//go:embed adapter_spanner_scheme.sql
var spannerDDL string
var spannerDDLs = spannerutil.SplitDDL(spannerDDL)

// SpannerTestModule adds all the required dependencies for Spanner migration and adapter.
func SpannerTestModule(ball *mud.Ball, spannerConnection string) {
	mud.Provide[*SpannerAdapter](ball, NewSpannerAdapter)
	mud.Implementation[[]Adapter, *SpannerAdapter](ball)
	mud.RemoveTag[*SpannerAdapter, mud.Optional](ball)
	// Please note that SpannerTestDatabase creates / deletes temporary database via the lifecycle functions.
	mud.Provide[SpannerTestDatabase](ball, func(ctx context.Context, logger *zap.Logger) (SpannerTestDatabase, error) {
		return NewSpannerTestDatabase(ctx, logger, spannerConnection, true)
	})
	mud.Provide[SpannerConfig](ball, NewTestSpannerConfig)
}

// SpannerTestDatabase manages Spanner database and migration for tests.
type SpannerTestDatabase struct {
	ephemeral *spannerutil.EphemeralDB
}

// NewSpannerTestDatabase creates the database (=creates / migrates the database).
func NewSpannerTestDatabase(ctx context.Context, logger *zap.Logger, connstr string, withMigration bool) (SpannerTestDatabase, error) {
	var ddls []string
	if withMigration {
		ddls = spannerDDLs
	}

	ephemeral, err := spannerutil.CreateEphemeralDB(ctx, connstr, "", ddls...)
	if err != nil {
		return SpannerTestDatabase{}, errs.Wrap(err)
	}

	return SpannerTestDatabase{
		ephemeral: ephemeral,
	}, nil
}

// Connection returns with the used connection string (with added unique suffix).
func (d SpannerTestDatabase) Connection() string {
	return d.ephemeral.Params.ConnStr()
}

// Close drops the temporary test database.
func (d SpannerTestDatabase) Close() error {
	// TODO: this should not use context.Background()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return errs.Wrap(d.ephemeral.Close(ctx))
}

// NewTestSpannerConfig creates SpannerConfig for testing.
func NewTestSpannerConfig(database SpannerTestDatabase) SpannerConfig {
	return SpannerConfig{
		Database: database.ephemeral.Params.ConnStr(),
	}
}
