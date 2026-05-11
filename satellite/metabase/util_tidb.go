// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"strings"
)

// tidbBatchInsertQuery builds a multi-row INSERT statement of the form:
//
//	INSERT INTO <table> (<col1>, <col2>, ...) VALUES (?,?,...),(?,?,...),...
//
// with rows placeholder tuples. rows must be > 0.
func tidbBatchInsertQuery(table string, cols []string, rows int) string {
	rowPlaceholder := "(" + strings.Repeat("?,", len(cols)-1) + "?)"
	return "INSERT INTO " + table + " (" + strings.Join(cols, ", ") + ") VALUES " +
		strings.Repeat(rowPlaceholder+",", rows-1) + rowPlaceholder
}
