// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testrouting

import (
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

type nodeDataTimeSorter []*nodeData

func (s nodeDataTimeSorter) Len() int      { return len(s) }
func (s nodeDataTimeSorter) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

func (s nodeDataTimeSorter) Less(i, j int) bool {
	return s[i].ordering < s[j].ordering
}

type nodeDataDistanceSorter struct {
	self  storj.NodeID
	nodes []*nodeData
}

func (s nodeDataDistanceSorter) Len() int { return len(s.nodes) }

func (s nodeDataDistanceSorter) Swap(i, j int) {
	s.nodes[i], s.nodes[j] = s.nodes[j], s.nodes[i]
}

func (s nodeDataDistanceSorter) Less(i, j int) bool {
	return compareByXor(s.nodes[i].node.Id, s.nodes[j].node.Id, s.self) < 0
}

func compareByXor(left, right, reference storj.NodeID) int {
	for i, r := range reference {
		a, b := left[i]^r, right[i]^r
		if a != b {
			if a < b {
				return -1
			}
			return 1
		}
	}
	return 0
}

func bitAtDepth(id storj.NodeID, bitDepth int) bool {
	// we could make this a fun one-liner but this is more understandable
	byteDepth := bitDepth / 8
	bitOffset := bitDepth % 8
	power := uint(7 - bitOffset)
	bitMask := byte(1 << power)
	byte_ := id[byteDepth]
	if byte_&bitMask > 0 {
		return true
	}
	return false
}

func addressEqual(a1, a2 *pb.NodeAddress) bool {
	return a1.Transport == a2.Transport &&
		a1.Address == a2.Address
}
