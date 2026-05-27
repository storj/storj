// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package tidbutil_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/shared/dbutil/tidbutil"
)

func TestURLToDSN(t *testing.T) {
	defaults := map[string]string{
		"parseTime":         "true",
		"multiStatements":   "true",
		"interpolateParams": "true",
		"clientFoundRows":   "true",
		"charset":           "utf8mb4",
		"loc":               "UTC",
		"time_zone":         "'+00:00'",
		"sql_mode":          "'ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_AUTO_CREATE_USER,NO_ENGINE_SUBSTITUTION'",
	}

	t.Run("basic host and database", func(t *testing.T) {
		dsn, err := tidbutil.URLToDSN("tidb://localhost:4000/mydb")
		require.NoError(t, err)

		prefix, params := splitDSN(t, dsn)
		assert.Equal(t, "tcp(localhost:4000)/mydb", prefix)
		assertParams(t, defaults, params)
	})

	t.Run("with username only", func(t *testing.T) {
		dsn, err := tidbutil.URLToDSN("tidb://root@localhost:4000/mydb")
		require.NoError(t, err)

		prefix, params := splitDSN(t, dsn)
		assert.Equal(t, "root@tcp(localhost:4000)/mydb", prefix)
		assertParams(t, defaults, params)
	})

	t.Run("with username and password", func(t *testing.T) {
		dsn, err := tidbutil.URLToDSN("tidb://root:secret@localhost:4000/mydb")
		require.NoError(t, err)

		prefix, params := splitDSN(t, dsn)
		assert.Equal(t, "root:secret@tcp(localhost:4000)/mydb", prefix)
		assertParams(t, defaults, params)
	})

	t.Run("strips application_name", func(t *testing.T) {
		dsn, err := tidbutil.URLToDSN("tidb://localhost:4000/mydb?application_name=satellite")
		require.NoError(t, err)

		_, params := splitDSN(t, dsn)
		_, hasAppName := params["application_name"]
		assert.False(t, hasAppName, "application_name should be stripped")
		assertParams(t, defaults, params)
	})

	t.Run("preserves overrides", func(t *testing.T) {
		overrides := map[string]string{
			"parseTime":         "false",
			"multiStatements":   "false",
			"interpolateParams": "false",
			"clientFoundRows":   "false",
			"charset":           "latin1",
			"loc":               "Local",
			"time_zone":         "'+02:00'",
			"sql_mode":          "'STRICT_TRANS_TABLES'",
		}
		var qs []string
		for k, v := range overrides {
			qs = append(qs, k+"="+url.QueryEscape(v))
		}
		dsn, err := tidbutil.URLToDSN("tidb://localhost:4000/mydb?" + strings.Join(qs, "&"))
		require.NoError(t, err)

		prefix, params := splitDSN(t, dsn)
		assert.Equal(t, "tcp(localhost:4000)/mydb", prefix)
		assertParams(t, overrides, params)
	})

	t.Run("preserves extra parameters", func(t *testing.T) {
		dsn, err := tidbutil.URLToDSN("tidb://localhost:4000/mydb?tls=true&timeout=5s")
		require.NoError(t, err)

		_, params := splitDSN(t, dsn)
		assert.Equal(t, "true", params["tls"])
		assert.Equal(t, "5s", params["timeout"])
		assertParams(t, defaults, params)
	})

	t.Run("empty database path", func(t *testing.T) {
		dsn, err := tidbutil.URLToDSN("tidb://localhost:4000/")
		require.NoError(t, err)

		prefix, _ := splitDSN(t, dsn)
		assert.Equal(t, "tcp(localhost:4000)/", prefix)
	})

	t.Run("rejects non-tidb scheme", func(t *testing.T) {
		_, err := tidbutil.URLToDSN("mysql://localhost:4000/mydb")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected tidb scheme")
	})

	t.Run("rejects unparseable URL", func(t *testing.T) {
		_, err := tidbutil.URLToDSN("tidb://bad host/mydb")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid tidb URL")
	})
}

// splitDSN splits a MySQL DSN into the prefix portion (before "?") and the
// parsed query parameters.
func splitDSN(t *testing.T, dsn string) (prefix string, params map[string]string) {
	t.Helper()
	params = map[string]string{}
	prefix, query, hasQuery := strings.Cut(dsn, "?")
	if hasQuery {
		values, err := url.ParseQuery(query)
		require.NoError(t, err)
		for k, v := range values {
			require.Len(t, v, 1, "unexpected multi-value parameter %q", k)
			params[k] = v[0]
		}
	}
	return prefix, params
}

// assertParams checks that each expected key/value is present in the DSN params.
func assertParams(t *testing.T, expected, actual map[string]string) {
	t.Helper()
	for k, v := range expected {
		assert.Equal(t, v, actual[k], "param %q", k)
	}
}
