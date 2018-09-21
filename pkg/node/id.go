// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package node

import (
	"crypto/rand"

	base58 "github.com/jbenet/go-base58"
)

// ID is the unique identifier of a Node in the overlay network
type ID string

// String transforms the ID to a string type
func (n *ID) String() string {
	return string(*n)
}

// Bytes transforms the ID to type []byte
func (n *ID) Bytes() []byte {
	return []byte(*n)
}

// IDFromString trsansforms a string to a ID
func IDFromString(s string) *ID {
	n := ID(s)
	return &n
}

// NewID returns a pointer to a newly intialized ID
// TODO@ASK: this should be removed; superseded by `CASetupConfig.Create` / `IdentitySetupConfig.Create`
func NewID() (*ID, error) {
	b, err := newID()
	if err != nil {
		return nil, err
	}

	bb := ID(base58.Encode(b))
	return &bb, nil
}

// newID generates a new random ID.
// This purely to get things working. We shouldn't use this as the ID in the actual network
func newID() ([]byte, error) {
	result := make([]byte, 20)
	_, err := rand.Read(result)
	return result, err
}
