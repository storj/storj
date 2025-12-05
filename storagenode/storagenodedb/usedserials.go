// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

// UsedSerialsDBName represents the database name.
const UsedSerialsDBName = "used_serial"

// usedSerialsDB is necessary for previous migration steps, even though the usedserials db is no longer used.
type usedSerialsDB struct {
	dbContainerImpl
}
