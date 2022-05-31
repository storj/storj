// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package storjscan

// DB is storjscan DB interface.
//
// architecture: Database
type DB interface {
	// Wallets is getter for wallets db.
	Wallets() WalletsDB
}
