// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"math/bits"

	"storj.io/storj/pkg/storj"
)

// xorNodeID returns the xor of each byte in NodeID
func xorNodeID(a, b storj.NodeID) storj.NodeID {
	r := storj.NodeID{}
	for i, av := range a {
		r[i] = av ^ b[i]
	}
	return r
}

// xorBucketID returns the xor of each byte in bucketID
func xorBucketID(a, b bucketID) bucketID {
	r := bucketID{}
	for i, av := range a {
		r[i] = av ^ b[i]
	}
	return r
}

// determineDifferingBitIndex: helper, returns the last bit differs starting from prefix to suffix
func determineDifferingBitIndex(bID, comparisonID bucketID) (int, error) {
	if bID == comparisonID {
		return -2, RoutingErr.New("compared two equivalent k bucket ids")
	}

	if comparisonID == emptyBucketID {
		comparisonID = firstBucketID
	}

	xorID := xorBucketID(bID, comparisonID)
	if xorID == firstBucketID {
		return -1, nil
	}

	for i, v := range xorID {
		if v != 0 {
			return i*8 + 7 - bits.TrailingZeros8(v), nil
		}
	}

	return -1, nil
}
