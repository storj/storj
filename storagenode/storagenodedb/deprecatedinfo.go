// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

const (
	// LegacyInfoDBName represents the database name.
	DeprecatedInfoDBName = "info"
)

// legacyInfoDB represents the database that contains the original legacy sqlite3 database.
type deprecatedInfoDB struct {
	migratableDB
}

func newDeprecatedInfoDB() *deprecatedInfoDB {
	return &deprecatedInfoDB{}
}
