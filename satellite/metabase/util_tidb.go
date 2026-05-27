// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"strings"
)

// tidbInsertValuesClause returns the "(<col1>, ...) VALUES (?,?,...),(?,?,...),..."
// portion of a multi-row INSERT statement with rows placeholder tuples.
// rows must be > 0.
func tidbInsertValuesClause(cols []string, rows int) string {
	rowPlaceholder := "(" + tidbPlaceholders(len(cols)) + ")"
	return "(" + strings.Join(cols, ", ") + ") VALUES " +
		strings.Repeat(rowPlaceholder+",", rows-1) + rowPlaceholder
}

// tidbPlaceholders returns a comma-separated list of n "?" placeholders for
// use in IN(...) clauses, VALUES tuples, and similar. n must be > 0.
func tidbPlaceholders(n int) string {
	return strings.Repeat("?,", n-1) + "?"
}

// tidbBatchInsertQuery builds a multi-row INSERT statement of the form:
//
//	INSERT INTO <table> (<col1>, <col2>, ...) VALUES (?,?,...),(?,?,...),...
//
// with rows placeholder tuples. rows must be > 0.
func tidbBatchInsertQuery(table string, cols []string, rows int) string {
	return "INSERT INTO " + table + " " + tidbInsertValuesClause(cols, rows)
}

// tidbBatchInsertIgnoreQuery is like tidbBatchInsertQuery but emits
// INSERT IGNORE, which silently skips rows that would violate a unique or
// primary key constraint. Note that INSERT IGNORE also masks unrelated
// row-level errors (data conversion, NOT NULL violations, etc.) — only use
// where that is acceptable.
func tidbBatchInsertIgnoreQuery(table string, cols []string, rows int) string {
	return "INSERT IGNORE INTO " + table + " " + tidbInsertValuesClause(cols, rows)
}
