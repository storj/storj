// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"github.com/zeebo/errs"
)

// ErrSatellitesDB represents errors from the satellites database.
var ErrSatellitesDB = errs.Class("satellitesdb error")

// reputation works with node reputation DB
type satellitesDB struct {
	location string
	SQLDB
}

// newSatellitesDB returns a new instance of satellitesDB initialized with the specified database.
func newSatellitesDB(db SQLDB, location string) *satellitesDB {
	return &satellitesDB{
		location: location,
		SQLDB:    db,
	}
}
