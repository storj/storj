// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"storj.io/storj/satellite/console"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type bucketsDB struct {
	db dbx.Methods
}

// Create is getter a for Users repository
func (db *bucketsDB) Create() console.Users {
	return &users{db.methods}
}

// Get is getter a for Users repository
func (db *bucketsDB) Get() console.Users {
	return &users{db.methods}
}

// Delete is getter a for Users repository
func (db *bucketsDB) Delete() console.Users {
	return &users{db.methods}
}

// List is getter a for Users repository
func (db *bucketsDB) List() console.Users {
	return &users{db.methods}
}
