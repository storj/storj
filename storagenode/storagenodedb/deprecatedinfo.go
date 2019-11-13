// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

// DeprecatedInfoDBName represents the database name.
const DeprecatedInfoDBName = "info"

// deprecatedInfoDB represents the database that contains the original legacy sqlite3 database.
type deprecatedInfoDB struct {
	dbContainerImpl
}
