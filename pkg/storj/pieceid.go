// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha512"
	"encoding/hex"

	"github.com/zeebo/errs"
)

// ErrPieceID is used when something goes wrong with a piece ID
var ErrPieceID = errs.Class("piece ID error")

// PieceID2 is the unique identifier for pieces
type PieceID2 [32]byte

// NewPieceID creates a piece ID
func NewPieceID() PieceID2 {
	var id PieceID2

	_, err := rand.Read(id[:])
	if err != nil {
		panic(err)
	}

	return id
}

// PieceIDFromString decodes a hex encoded piece ID string
func PieceIDFromString(s string) (PieceID2, error) {
	idBytes, err := hex.DecodeString(s)
	if err != nil {
		return PieceID2{}, ErrNodeID.Wrap(err)
	}
	return PieceIDFromBytes(idBytes)
}

// PieceIDFromBytes converts a byte slice into a piece ID
func PieceIDFromBytes(b []byte) (PieceID2, error) {
	if len(b) != len(PieceID2{}) {
		return PieceID2{}, ErrPieceID.New("not enough bytes to make a piece ID; have %d, need %d", len(b), len(NodeID{}))
	}

	var id PieceID2
	copy(id[:], b[:])
	return id, nil
}

// IsZero returns whether piece ID is unassigned
func (id PieceID2) IsZero() bool {
	return id == PieceID2{}
}

// String representation of the piece ID
func (id PieceID2) String() string { return hex.EncodeToString(id.Bytes()) }

// Bytes returns bytes of the piece ID
func (id PieceID2) Bytes() []byte { return id[:] }

// Derive a new PieceID2 from the current piece ID and the given storage node ID
func (id PieceID2) Derive(storagenodeID NodeID) PieceID2 {
	// TODO: should the secret / content be swapped?
	mac := hmac.New(sha512.New, id.Bytes())
	var derived PieceID2
	copy(derived[:], mac.Sum(storagenodeID.Bytes()))
	return derived
}

// Marshal serializes a piece ID
func (id PieceID2) Marshal() ([]byte, error) {
	return id.Bytes(), nil
}

// MarshalTo serializes a piece ID into the passed byte slice
func (id *PieceID2) MarshalTo(data []byte) (n int, err error) {
	n = copy(data, id.Bytes())
	return n, nil
}

// Unmarshal deserializes a piece ID
func (id *PieceID2) Unmarshal(data []byte) error {
	var err error
	*id, err = PieceIDFromBytes(data)
	return err
}

// Size returns the length of a piece ID (implements gogo's custom type interface)
func (id *PieceID2) Size() int {
	return len(id)
}

// MarshalJSON serializes a piece ID to a json string as bytes
func (id PieceID2) MarshalJSON() ([]byte, error) {
	return []byte(`"` + id.String() + `"`), nil
}

// UnmarshalJSON deserializes a json string (as bytes) to a piece ID
func (id *PieceID2) UnmarshalJSON(data []byte) error {
	var err error
	*id, err = PieceIDFromString(string(data))
	if err != nil {
		return err
	}
	return nil
}
