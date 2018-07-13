// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storage

import (
	"time"

	"storj.io/storj/protos/meta"
)

// Meta flags for the List method
const (
	// MetaNone represents no meta flags
	MetaNone = uint64(1 << iota)
	// MetaModified meta flag
	MetaModified
	// MetaExpiration meta flag
	MetaExpiration
	// MetaSize meta flags
	MetaSize
	// MetaChecksum meta flag
	MetaChecksum
	// MetaContentType meta flag
	MetaContentType
	// MetaUserDefined meta flag
	MetaUserDefined
	// MetaAll represents all the meta flags
	MetaAll = ^uint64(0)
)

// Meta is the full object metadata
type Meta struct {
	meta.Serializable
	Modified   time.Time
	Expiration time.Time
	Size       int64
	Checksum   string
	// Redundancy []eestream.RedundancyStrategy
	// EncryptionScheme
}
