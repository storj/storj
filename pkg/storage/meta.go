// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storage

import (
	"time"
)

// Meta info
type Meta struct {
	Size        int64
	Modified    time.Time
	Expiration  time.Time
	ContentType string
	Checksum    string
	// Redundancy  eestream.RedundancyStrategy
	// EncryptionScheme
	Data []byte
}
