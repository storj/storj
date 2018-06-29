// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

// NodeID is the unique identifer of a Node in the overlay network
type NodeID string

// String transforms the nodeID to a string type
func (n NodeID) String() string {
	return string(n)
}
