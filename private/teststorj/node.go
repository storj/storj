// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package teststorj

import (
	"storj.io/common/pb"
	"storj.io/common/storj"
)

// NodeIDFromBytes returns a node ID consisting of the bytes
// and padding to the node ID length
func NodeIDFromBytes(b []byte) storj.NodeID {
	id, _ := storj.NodeIDFromBytes(fit(b))
	return id
}

// NodeIDFromString returns node ID consisting of the strings
// and padding to the node ID length
func NodeIDFromString(s string) storj.NodeID {
	return NodeIDFromBytes([]byte(s))
}

// NodeIDsFromBytes returns node IDs consisting of the byte slices
// and padding to the node ID length
func NodeIDsFromBytes(bs ...[]byte) (ids storj.NodeIDList) {
	for _, b := range bs {
		ids = append(ids, NodeIDFromBytes(b))
	}
	return ids
}

// NodeIDsFromStrings returns node IDs consisting of the strings
// and padding to the node ID length
func NodeIDsFromStrings(strs ...string) (ids storj.NodeIDList) {
	for _, s := range strs {
		ids = append(ids, NodeIDFromString(s))
	}
	return ids
}

// used to pad node IDs
func fit(b []byte) []byte {
	l := len(storj.NodeID{})
	if len(b) < l {
		return fit(append(b, 255))
		// return fit(append([]byte{1}, b...))
	}
	return b[:l]
}

// MockNode returns a pb node with an ID consisting of the string
// and padding to the node ID length
func MockNode(s string) *pb.Node {
	id := NodeIDFromString(s)
	var node pb.Node
	node.Id = id
	return &node
}
