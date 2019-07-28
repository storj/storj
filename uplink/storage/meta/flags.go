// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package meta

// Meta flags for the List method
const (
	// None represents no meta flags
	None = 0
	// Modified meta flag
	Modified = uint32(1 << iota)
	// Expiration meta flag
	Expiration
	// Size meta flags
	Size
	// Checksum meta flag
	Checksum
	// UserDefined meta flag
	UserDefined
	// All represents all the meta flags
	All = ^uint32(0)
)
