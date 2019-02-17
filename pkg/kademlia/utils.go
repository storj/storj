// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"math/bits"
	"sort"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

func cloneNodeIDs(ids storj.NodeIDList) storj.NodeIDList {
	clone := make(storj.NodeIDList, len(ids))
	copy(clone, ids)
	return clone
}

// compareByXor compares left, right xorred by reference
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

func sortByXOR(nodeIDs storj.NodeIDList, ref storj.NodeID) {
	sort.Slice(nodeIDs, func(i, k int) bool {
		return compareByXor(nodeIDs[i], nodeIDs[k], ref) < 0
	})
}

func keysToNodeIDs(keys storage.Keys) (ids storj.NodeIDList, err error) {
	var idErrs []error
	for _, k := range keys {
		id, err := storj.NodeIDFromBytes(k[:])
		if err != nil {
			idErrs = append(idErrs, err)
		}
		ids = append(ids, id)
	}
	if err := errs.Combine(idErrs...); err != nil {
		return nil, err
	}

	return ids, nil
}

func keyToBucketID(key storage.Key) (bID bucketID) {
	copy(bID[:], key)
	return bID
}

func bucketIDToKey(bID bucketID) (key storage.Key) {
	copy(key, bID[:])
	return key
}

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
