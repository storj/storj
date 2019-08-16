// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

// versions represents the database that contains the database schema version history.
type versions struct {
	location string
	SQLDB
}

func newVersions(db SQLDB, location string) *versions {
	return &versions{
		location: location,
		SQLDB:    db,
	}
}

// Rebind rebind parameters
func (db *versions) Rebind(s string) string { return s }

// Schema returns schema
func (db *versions) Schema() string { return "" }