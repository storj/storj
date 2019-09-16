// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

const (
	// LegacyInfoDBName represents the database name.
	LegacyInfoDBName = "info"
)

// legacyInfoDB represents the database that contains the original legacy sqlite3 database.
type legacyInfoDB struct {
	storageNodeSQLDB
}

func newLegacyInfoDB() *legacyInfoDB {
	return &legacyInfoDB{}
}
