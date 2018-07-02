// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storage

import "time"

// Meta info
type Meta struct {
	Modified   time.Time
	Expiration time.Time
	Data       []byte
}
