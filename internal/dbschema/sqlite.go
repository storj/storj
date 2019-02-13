// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package dbschema

import (
	"database/sql"
	"errors"
)

// QuerySqlite loads the schema from postgres database.
func QuerySqlite(tx *sql.Tx) (*Schema, error) {
	return nil, errors.New("unimplemented")
}
