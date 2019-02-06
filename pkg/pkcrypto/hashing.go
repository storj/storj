// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pkcrypto

import (
	"crypto/sha256"
)

// SHA256Hash calculates the SHA256 hash of the input data
func SHA256Hash(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}
