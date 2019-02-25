// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package psclient

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha512"

	"github.com/mr-tron/base58/base58"
)

// PieceID is the unique identifier for pieces
type PieceID string

// NewPieceID creates a PieceID
func NewPieceID() PieceID {
	b := make([]byte, 32)

	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}

	return PieceID(base58.Encode(b))
}

// String representation of the PieceID
func (id PieceID) String() string {
	return string(id)
}

// IsValid checks if the current PieceID is valid
func (id PieceID) IsValid() bool {
	return len(id) >= 20
}

// Derive a new PieceID from the current PieceID and the given secret
func (id PieceID) Derive(secret []byte) (derived PieceID, err error) {
	mac := hmac.New(sha512.New, secret)
	_, err = mac.Write([]byte(id))
	if err != nil {
		return "", err
	}
	h := mac.Sum(nil)
	// Trim the hash if greater than 32 bytes
	if len(h) > 32 {
		h = h[:32]
	}
	return PieceID(base58.Encode(h)), nil
}
