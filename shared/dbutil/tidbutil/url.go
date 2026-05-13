// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package tidbutil

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/go-sql-driver/mysql"
)

// defaultSQLMode is the TiDB default sql_mode with NO_ZERO_DATE and
// NO_ZERO_IN_DATE removed, so Go's zero time.Time (which the driver sends as
// 0000-00-00) is accepted — matching Postgres semantics where a zero timestamp
// is stored as 0001-01-01. Every flag below is part of TiDB's stock default;
// the outer single quotes are SQL string-literal syntax for the SET sql_mode
// statement the driver emits at session start.
const defaultSQLMode = "'" +
	// ONLY_FULL_GROUP_BY rejects SELECT / HAVING / ORDER BY references to
	// non-aggregated columns that are neither in the GROUP BY clause nor
	// functionally dependent on it. Catches ambiguous group queries.
	"ONLY_FULL_GROUP_BY," +
	// STRICT_TRANS_TABLES makes transactional tables error on invalid or
	// out-of-range values instead of silently truncating or zero-filling —
	// matching the fail-loud behavior we get from Postgres.
	"STRICT_TRANS_TABLES," +
	// ERROR_FOR_DIVISION_BY_ZERO turns x/0 in INSERT / UPDATE into an error
	// instead of producing NULL.
	"ERROR_FOR_DIVISION_BY_ZERO," +
	// NO_AUTO_CREATE_USER prevents GRANT from implicitly creating user
	// accounts. A no-op on modern MySQL but still recognized by TiDB and
	// kept here so the mode string matches TiDB's documented default.
	"NO_AUTO_CREATE_USER," +
	// NO_ENGINE_SUBSTITUTION errors when CREATE / ALTER TABLE specifies an
	// unavailable storage engine, instead of silently substituting the
	// default engine. Avoids surprise fallbacks during schema changes.
	"NO_ENGINE_SUBSTITUTION" +
	"'"

// defaultParams are appended to a tidb:// URL's query unless the URL already
// sets them.
var defaultParams = []struct{ key, value string }{
	// parseTime makes the driver scan DATE/DATETIME/TIMESTAMP columns into
	// time.Time instead of []byte. Our code expects time.Time everywhere.
	{"parseTime", "true"},

	// multiStatements allows ";"-separated statements in a single Exec/Query.
	// Required by migrations and some batched queries.
	{"multiStatements", "true"},

	// interpolateParams inlines parameters client-side and dispatches via the
	// text protocol (COM_QUERY) instead of prepared statements. TiDB rejects
	// multi-statement prepared statements with "Can not prepare multiple
	// statements", so this is the only way to combine multiStatements with
	// parameters; it also saves the prepare/close round trips for one-shot
	// queries.
	{"interpolateParams", "true"},

	// clientFoundRows makes sql.Result.RowsAffected() return the number of
	// rows matched by the WHERE clause, not just those whose values actually
	// changed. This matches Postgres semantics, which DBX-generated code
	// relies on when it inspects RowsAffected() to decide whether a row
	// existed.
	{"clientFoundRows", "true"},

	// charset pins the connection character set. utf8mb4 is already the
	// TiDB default, but setting it explicitly causes the driver to issue
	// SET NAMES at session start, locking in the encoding regardless of
	// future server-default changes.
	{"charset", "utf8mb4"},

	// loc is the Go-side time.Location used when parsing/formatting timestamp
	// strings. Pinned to UTC so the driver does no implicit conversion.
	{"loc", "UTC"},

	// time_zone is the server session timezone. It controls how TIMESTAMP
	// columns are converted between their internal UTC storage and the
	// client, as well as the values returned by NOW()/CURRENT_TIMESTAMP.
	// Pinned to UTC so the on-disk UTC value matches what Go sees, regardless
	// of the server's default timezone — otherwise round-trips appear correct
	// to this driver while the stored UTC instant silently shifts.
	{"time_zone", "'+00:00'"},

	// sql_mode customizes server-side validation; see defaultSQLMode for why.
	{"sql_mode", defaultSQLMode},
}

// URLToDSN converts a tidb:// URL into a MySQL Go-driver DSN string. It fills
// in TiDB-specific defaults (see defaultParams) for any query parameters the
// URL does not already set.
func URLToDSN(connStr string) (string, error) {
	u, err := url.Parse(connStr)
	if err != nil {
		return "", fmt.Errorf("invalid tidb URL: %w", err)
	}
	if u.Scheme != "tidb" {
		return "", fmt.Errorf("expected tidb scheme, got %q", u.Scheme)
	}

	cfg := mysql.NewConfig()
	cfg.Net = "tcp"
	cfg.Addr = u.Host
	cfg.DBName = strings.TrimPrefix(u.Path, "/")
	if u.User != nil {
		cfg.User = u.User.Username()
		if pw, ok := u.User.Password(); ok {
			cfg.Passwd = pw
		}
	}

	q := u.Query()
	// application_name is a Postgres-flavored URL parameter injected by metabase.Open;
	// the MySQL driver does not accept it, so drop it before building the DSN.
	q.Del("application_name")
	cfg.Params = make(map[string]string, len(q)+len(defaultParams))
	for k, vs := range q {
		if len(vs) > 0 {
			cfg.Params[k] = vs[0]
		}
	}
	for _, p := range defaultParams {
		if _, ok := cfg.Params[p.key]; !ok {
			cfg.Params[p.key] = p.value
		}
	}

	return cfg.FormatDSN(), nil
}
