// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package spannerutil

import (
	"context"
	"fmt"
	"strings"

	"storj.io/storj/shared/dbutil/dbschema"
)

// QueryData loads all data from tables.
func QueryData(ctx context.Context, db dbschema.Queryer, schema *dbschema.Schema) (*dbschema.Data, error) {
	return dbschema.QueryData(ctx, db, schema, func(columnName string) string {
		quotedColumnName := QuoteIdentifier(columnName)
		return fmt.Sprintf("TO_JSON_STRING(TO_JSON(%s))", quotedColumnName)
	})
}

// QuoteIdentifier quotes an identifier appropriately for use by Spanner.
func QuoteIdentifier(identifier string) string {
	return "`" + strings.ReplaceAll(strings.ReplaceAll(identifier, "\\", "\\\\"), "`", "\\`") + "`"
}
