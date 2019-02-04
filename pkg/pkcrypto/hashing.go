// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pkcrypto

import (
	"crypto"
)

// SHA256Hash calculates the SHA256 hash of the input data
func SHA256Hash(data []byte) []byte {
	hash := crypto.SHA256.New()
	if _, err := hash.Write(data); err != nil {
		// hash.Write() is documented as never returning an error
		panic(err)
	}
	return hash.Sum(nil)
}
