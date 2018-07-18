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
	//As it is gets encoded to base58 we should properly decode it, 
	//so the bytes dont corrupt
	//Also routing methods expect hex-encoded string
	//return base58.Decode(n.String())
	return []byte(*n)
}

// StringToNodeID trsansforms a string to a NodeID
func StringToNodeID(s string) *NodeID {
	n := NodeID(s)
	return &n
}

// NewID returns a pointer to a newly intialized NodeID
func NewID() (*NodeID, error) {
	b, err := newID()
	if err != nil {
		return nil, err
	}

	bb := NodeID(base58.Encode(b))
	return &bb, nil
}
