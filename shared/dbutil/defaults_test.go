// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package dbutil_test

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/stretchr/testify/require"

	"storj.io/storj/shared/dbutil"
)

// fakeDB is a ConfigurableDB whose Stats are whatever the test says they are.
type fakeDB struct {
	stats sql.DBStats
}

func (f *fakeDB) SetMaxIdleConns(int)              {}
func (f *fakeDB) SetMaxOpenConns(int)              {}
func (f *fakeDB) SetConnMaxLifetime(time.Duration) {}
func (f *fakeDB) Stats() sql.DBStats               { return f.stats }

// collect returns the WaitCount reported for every db_stats series in scope.
func collect(scope *monkit.Scope) map[string]float64 {
	out := map[string]float64{}
	scope.Stats(func(key monkit.SeriesKey, field string, val float64) {
		if key.Measurement == "db_stats" && field == "WaitCount" {
			out[key.String()] = val
		}
	})
	return out
}

// TestConfigureParameters_DistinctTagsKeepPoolsApart ensures pools sharing a
// dbName still emit one series each, so their samples don't collide.
func TestConfigureParameters_DistinctTagsKeepPoolsApart(t *testing.T) {
	registry := monkit.NewRegistry()
	scope := registry.ScopeNamed("test")

	busy := &fakeDB{stats: sql.DBStats{WaitCount: 19336}}
	idle := &fakeDB{stats: sql.DBStats{WaitCount: 0}}

	params := &dbutil.ConnParams{MaxIdleConns: -1, MaxOpenConns: -1, ConnMaxLifetime: -1}
	dbutil.ConfigureParameters(busy, params, "metabase", scope, monkit.NewSeriesTag("adapter", "tidb"))
	dbutil.ConfigureParameters(idle, params, "metabase", scope, monkit.NewSeriesTag("adapter", "crdb"))

	series := collect(scope)
	require.Len(t, series, 2, "each pool must get its own series: %v", series)

	byAdapter := map[string]float64{}
	for key, val := range series {
		require.Contains(t, key, "db_name=metabase")
		switch {
		case strings.Contains(key, "adapter=tidb"):
			byAdapter["tidb"] = val
		case strings.Contains(key, "adapter=crdb"):
			byAdapter["crdb"] = val
		default:
			t.Fatalf("unexpected series %q", key)
		}
	}
	require.Equal(t, map[string]float64{"tidb": 19336, "crdb": 0}, byAdapter)
}

// TestConfigureParameters_SameTagsCollide documents the failure this guards
// against: without a distinguishing tag the pools share one series key.
func TestConfigureParameters_SameTagsCollide(t *testing.T) {
	registry := monkit.NewRegistry()
	scope := registry.ScopeNamed("test")

	params := &dbutil.ConnParams{MaxIdleConns: -1, MaxOpenConns: -1, ConnMaxLifetime: -1}
	dbutil.ConfigureParameters(&fakeDB{stats: sql.DBStats{WaitCount: 19336}}, params, "metabase", scope)
	dbutil.ConfigureParameters(&fakeDB{stats: sql.DBStats{WaitCount: 0}}, params, "metabase", scope)

	require.Len(t, collect(scope), 1, "untagged pools are expected to share a key")
}
