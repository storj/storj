// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package migrate

import (
	"storj.io/common/tagsql"
)

// DBX contains additional methods for migrations.
type DBX interface {
	tagsql.DB
	Schema() string
	Rebind(string) string
}

// rebind uses Rebind method when the database has the func.
func rebind(db tagsql.DB, s string) string {
	if dbx, ok := db.(interface{ Rebind(string) string }); ok {
		return dbx.Rebind(s)
	}
	return s
}
