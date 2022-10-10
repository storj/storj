// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package blockchain

import (
	"encoding/hex"
	"encoding/json"
	"reflect"

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

// Bytes gets the byte representation of the underlying hash.
func (h Hash) Bytes() []byte { return h[:] }

// Hex gets the hex string representation of the underlying hash.
func (h Hash) Hex() string {
	return hex.EncodeToString(h.Bytes())
}

// MarshalJSON implements json marshalling interface.
func (h Hash) MarshalJSON() ([]byte, error) {
	return json.Marshal(h.Hex())
}

// UnmarshalJSON unmarshal JSON into Hash.
func (h *Hash) UnmarshalJSON(bytes []byte) error {
	return unmarshalHexString(h[:], bytes, reflect.TypeOf(Hash{}))
}

// Address is wallet address.
type Address [AddressLength]byte

var _ json.Marshaler = Address{}

// Bytes gets the byte representation of the underlying address.
func (a Address) Bytes() []byte { return a[:] }

// Hex gets string representation of the underlying address.
func (a Address) Hex() string {
	return hex.EncodeToString(a.Bytes())
}

// MarshalJSON implements json marshalling interface.
func (a Address) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.Hex())
}

// UnmarshalJSON unmarshal JSON into Address.
func (a *Address) UnmarshalJSON(bytes []byte) error {
	return unmarshalHexString(a[:], bytes, reflect.TypeOf(Address{}))
}

// unmarshalHexString decodes JSON string containing hex string into bytes.
// Copies result into dst byte slice.
func unmarshalHexString(dst, src []byte, typ reflect.Type) error {
	if !isString(src) {
		return &json.UnmarshalTypeError{Value: "non-string", Type: reflect.TypeOf(typ)}
	}
	src = src[1 : len(src)-1]

	if bytesHave0xPrefix(src) {
		src = src[2:]
	}

	_, err := hex.Decode(dst, src)
	return err
}

// isString checks if JSON value is a string.
func isString(input []byte) bool {
	return len(input) >= 2 && input[0] == '"' && input[len(input)-1] == '"'
}

// bytesHave0xPrefix checks if string bytes representation contains 0x prefix.
func bytesHave0xPrefix(input []byte) bool {
	return len(input) >= 2 && input[0] == '0' && (input[1] == 'x' || input[1] == 'X')
}

// BytesToAddress create a new address from raw bytes.
func BytesToAddress(bytes []byte) (a Address, err error) {
	if len(bytes) != AddressLength {
		return a, errs.New("Invalid address length: %d instead of %d", len(bytes), AddressLength)
	}
	copy(a[:], bytes)
	return a, err
}
