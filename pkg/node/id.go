// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package node

import (
	"context"
	"crypto/rand"

	base58 "github.com/jbenet/go-base58"

	"storj.io/storj/pkg/provider"
)

// ID is the unique identifier of a Node in the overlay network
type ID string

// NewFullIdentity creates a new ID for nodes with difficulty and concurrency params
func NewFullIdentity(ctx context.Context, difficulty uint16, concurrency uint) (*provider.FullIdentity, error) {
	ca, err := provider.NewCA(ctx, difficulty, concurrency)
	if err != nil {
		return nil, err
	}
	identity, err := ca.NewIdentity()
	if err != nil {
		return nil, err
	}
	return identity, err
}

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

// NewID is a deprecated function to generate Kademlia ID's for tests. IT IS BEING REMOVED.
// Use NewFullIdentity instead in the meantime.
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
