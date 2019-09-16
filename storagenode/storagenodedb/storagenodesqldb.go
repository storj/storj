// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

type storageNodeSQLDB struct {
	SQLDB
}

// Schema returns schema
// These are implemented because the migrate.DB interface requires them.
// Maybe in the future we should untangle those.
func (sn *storageNodeSQLDB) Schema() string {
	return ""
}

// Rebind rebind parameters
// These are implemented because the migrate.DB interface requires them.
// Maybe in the future we should untangle those.
func (sn *storageNodeSQLDB) Rebind(s string) string {
	return s
}

// Configure sets the underlining SQLDB connection.
func (sn *storageNodeSQLDB) Configure(sqlDB SQLDB) {
	sn.SQLDB = sqlDB
}
