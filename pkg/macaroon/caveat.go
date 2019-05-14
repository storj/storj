// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package macaroon

import (
	"encoding/binary"
	"time"
)

// NewCaveat returns a Caveat with a nonce initialized to the current timestamp
// in nanoseconds.
func NewCaveat() Caveat {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(time.Now().UnixNano()))
	return Caveat{Nonce: buf[:]}
}
