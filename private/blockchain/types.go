// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package blockchain

import (
	"encoding/hex"
	"encoding/json"

	"github.com/zeebo/errs"
)

// Error for the package (mainly parsing errors).
var Error = errs.Class("blockchain")

// Lengths of hashes and addresses in bytes.
const (
	// HashLength is the expected length of the hash.
	HashLength = 32
	// AddressLength is the expected length of the address.
	AddressLength = 20
)

// Hash represents the 32 byte Keccak256 hash of arbitrary data.
type Hash [HashLength]byte

var _ json.Marshaler = Hash{}

// MarshalJSON implements json marshalling interface.
func (h Hash) MarshalJSON() ([]byte, error) {
	return json.Marshal(h.Hex())
}

// Bytes gets the byte representation of the underlying hash.
func (h Hash) Bytes() []byte { return h[:] }

// Hex gets the hex string representation of the underlying hash.
func (h Hash) Hex() string {
	return hex.EncodeToString(h.Bytes())
}

// Address is wallet address.
type Address [AddressLength]byte

var _ json.Marshaler = Address{}

// MarshalJSON implements json marshalling interface.
func (a Address) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.Hex())
}

// Bytes gets the byte representation of the underlying address.
func (a Address) Bytes() []byte { return a[:] }

// Hex gets string representation of the underlying address.
func (a Address) Hex() string {
	return hex.EncodeToString(a.Bytes())
}
