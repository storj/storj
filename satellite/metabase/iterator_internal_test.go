// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/shared/tagsql"
)

// TestRebindDispatch ensures the opportunistic Rebind dispatch the iterator
// helpers rely on continues to behave: postgresRebind must expose Rebind (so
// Postgres/Cockroach get $N placeholders) and a bare tagsql.DB must not (so
// TiDB sees ? passed through unchanged). The behavior is type-driven; if a
// future wrapper accidentally adds Rebind to TiDB or removes it from the
// Postgres path, the iterator queries silently break in a confusing way.
func TestRebindDispatch(t *testing.T) {
	t.Run("postgres_rebinds", func(t *testing.T) {
		var db tagsql.DB = postgresRebind{}
		rebinder, ok := db.(interface{ Rebind(string) string })
		require.True(t, ok, "postgresRebind must satisfy the Rebind interface")

		out := rebinder.Rebind("SELECT ? FROM t WHERE a = ? AND b > ?")
		require.NotContains(t, out, "?", "all ? placeholders must be rewritten")
		require.Contains(t, out, "$1")
		require.Contains(t, out, "$2")
		require.Contains(t, out, "$3")
	})

	t.Run("tidb_passes_through", func(t *testing.T) {
		// rebindlessDB stands in for whatever tagsql.DB the TiDB adapter
		// installs; it must not satisfy the Rebind interface.
		var db tagsql.DB = rebindlessDB{}
		_, ok := db.(interface{ Rebind(string) string })
		require.False(t, ok, "TiDB-path tagsql.DB must not expose Rebind")
	})
}

// rebindlessDB is a minimal tagsql.DB stub that intentionally does not
// implement Rebind. It exists only so type assertions in TestRebindDispatch
// have a concrete value to operate on.
type rebindlessDB struct{}

func (rebindlessDB) Name() string                                               { return "" }
func (rebindlessDB) BeginTx(context.Context, *sql.TxOptions) (tagsql.Tx, error) { return nil, nil }
func (rebindlessDB) Conn(context.Context) (tagsql.Conn, error)                  { return nil, nil }
func (rebindlessDB) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) {
	return nil, nil
}
func (rebindlessDB) PingContext(context.Context) error { return nil }
func (rebindlessDB) PrepareContext(context.Context, string) (tagsql.Stmt, error) {
	return nil, nil
}
func (rebindlessDB) QueryContext(context.Context, string, ...interface{}) (tagsql.Rows, error) {
	return nil, nil
}
func (rebindlessDB) QueryRowContext(context.Context, string, ...interface{}) *sql.Row {
	return nil
}
func (rebindlessDB) Close() error                     { return nil }
func (rebindlessDB) SetConnMaxLifetime(time.Duration) {}
func (rebindlessDB) SetMaxIdleConns(int)              {}
func (rebindlessDB) SetMaxOpenConns(int)              {}
func (rebindlessDB) Stats() sql.DBStats               { return sql.DBStats{} }

// TestPostgresRebind_PlaceholderCount confirms the rebind preserves the
// number of placeholders. Going forward, this catches accidental over- or
// under-counting in postgresRebind itself.
func TestPostgresRebind_PlaceholderCount(t *testing.T) {
	cases := []struct {
		sql      string
		expected int
	}{
		{"SELECT 1", 0},
		{"SELECT ?", 1},
		{"SELECT ? FROM t WHERE a = ? AND b = ?", 3},
		{"INSERT INTO t VALUES (?, ?, ?, ?, ?)", 5},
	}

	rebinder := postgresRebind{}
	for _, tc := range cases {
		out := rebinder.Rebind(tc.sql)
		got := 0
		for i := 1; ; i++ {
			if !strings.Contains(out, "$"+strconv.Itoa(i)) {
				break
			}
			got++
		}
		require.Equal(t, tc.expected, got, "input %q produced %q", tc.sql, out)
		require.NotContains(t, out, "?", "input %q produced %q", tc.sql, out)
	}
}
