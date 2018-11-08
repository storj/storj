// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

// DB used to manage db connections and context through different repositories
type DB interface {
	User() Users

	CreateTables() error
}
