// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"storj.io/storj/pkg/storj"
)

// determineDifferingBitIndex: helper, returns the last bit differs starting from prefix to suffix
func determineDifferingBitIndex(bID, comparisonID bucketID) (int, error) {
	if bID == comparisonID {
		return -2, RoutingErr.New("compared two equivalent k bucket ids")
	}

	if comparisonID == emptyBucketID {
		comparisonID = firstBucketID
	}

	var differingByteIndex int
	var differingByteXor byte
	xorArr := bucketID(xorNodeID(storj.NodeID(bID), storj.NodeID(comparisonID)))

	if xorArr == firstBucketID {
		return -1, nil
	}

	for j, v := range xorArr {
		if v != byte(0) {
			differingByteIndex = j
			differingByteXor = v
			break
		}
	}

	h := 0
	for ; h < 8; h++ {
		toggle := byte(1 << uint(h))
		tempXor := differingByteXor
		tempXor ^= toggle
		if tempXor < differingByteXor {
			break
		}

	}
	bitInByteIndex := 7 - h
	byteIndex := differingByteIndex
	bitIndex := byteIndex*8 + bitInByteIndex

	return bitIndex, nil
}
