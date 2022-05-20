// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"storj.io/storj/satellite/payments/storjscan"
)

// ensures that *storjscanDB implements storjscan.DB.
var _ storjscan.DB = (*storjscanDB)(nil)

// storjscanDB is storjscan DB.
//
// architecture: Database
type storjscanDB struct {
	db *satelliteDB
}

// Wallets is getter for wallets db.
func (db storjscanDB) Wallets() storjscan.WalletsDB {
	return &storjscanWalletsDB{db: db.db}
}
