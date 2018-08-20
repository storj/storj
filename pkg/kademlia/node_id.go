// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import base58 "github.com/jbenet/go-base58"

// NodeID is the unique identifer of a Node in the overlay network
type NodeID string

// String transforms the nodeID to a string type
func (n *NodeID) String() string {
	return string(*n)
}

// Bytes transforms the nodeID to type []byte
func (n *NodeID) Bytes() []byte {
	return []byte(*n)
}

// StringToNodeID trsansforms a string to a NodeID
func StringToNodeID(s string) *NodeID {
	n := NodeID(s)
	return &n
}

// NewID returns a pointer to a newly intialized NodeID
// TODO@ASK: this should be removed; superseded by `CASetupConfig.Create` / `IdentitySetupConfig.Create`
func NewID() (*NodeID, error) {
	b, err := newID()
	if err != nil {
		return nil, err
	}

	bb := NodeID(base58.Encode(b))
	return &bb, nil
}
