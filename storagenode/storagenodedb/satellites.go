// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"github.com/zeebo/errs"
)

// ErrSatellitesDB represents errors from the satellites database.
var ErrSatellitesDB = errs.Class("satellitesdb error")

const (
	// SatellitesDBName represents the database name.
	SatellitesDBName = "satellites"
)

// reputation works with node reputation DB
type satellitesDB struct {
	SQLDB
}

// newSatellitesDB returns a new instance of satellitesDB initialized with the specified database.
func newSatellitesDB() *satellitesDB {
	return &satellitesDB{}
}

// Configure sets the underlining SQLDB connection.
func (db *satellitesDB) Configure(sqlDB SQLDB) {
	db.SQLDB = sqlDB
}
