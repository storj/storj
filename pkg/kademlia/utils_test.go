// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/storj"
)

func TestSortByXOR(t *testing.T) {
	n1 := storj.NodeID{127, 255} //xor 0
	n2 := storj.NodeID{143, 255} //xor 240
	n3 := storj.NodeID{255, 255} //xor 128
	n4 := storj.NodeID{191, 255} //xor 192
	n5 := storj.NodeID{133, 255} //xor 250
	unsorted := storj.NodeIDList{n1, n5, n2, n4, n3}
	sortByXOR(unsorted, n1)
	sorted := storj.NodeIDList{n1, n3, n4, n2, n5}
	assert.Equal(t, sorted, unsorted)
}

func BenchmarkSortByXOR(b *testing.B) {
	nodes := []storj.NodeID{}
	for k := 0; k < 1000; k++ {
		nodes = append(nodes, testrand.NodeID())
	}

	b.ResetTimer()
	for m := 0; m < b.N; m++ {
		rand.Shuffle(len(nodes), func(i, k int) {
			nodes[i], nodes[k] = nodes[k], nodes[i]
		})
		sortByXOR(nodes, testrand.NodeID())
	}
}

func TestDetermineDifferingBitIndex(t *testing.T) {
	filledID := func(a byte) bucketID {
		id := firstBucketID
		id[0] = a
		return id
	}

	cases := []struct {
		bucketID bucketID
		key      bucketID
		expected int
	}{
		{
			bucketID: filledID(191),
			key:      filledID(255),
			expected: 1,
		},
		{
			bucketID: filledID(255),
			key:      filledID(191),
			expected: 1,
		},
		{
			bucketID: filledID(95),
			key:      filledID(127),
			expected: 2,
		},
		{
			bucketID: filledID(95),
			key:      filledID(79),
			expected: 3,
		},
		{
			bucketID: filledID(95),
			key:      filledID(63),
			expected: 2,
		},
		{
			bucketID: filledID(95),
			key:      filledID(79),
			expected: 3,
		},
		{
			bucketID: filledID(255),
			key:      bucketID{},
			expected: -1,
		},
		{
			bucketID: filledID(127),
			key:      bucketID{},
			expected: 0,
		},
		{
			bucketID: filledID(63),
			key:      bucketID{},
			expected: 1,
		},
		{
			bucketID: filledID(31),
			key:      bucketID{},
			expected: 2,
		},
		{
			bucketID: filledID(95),
			key:      filledID(63),
			expected: 2,
		},
	}

	for i, c := range cases {
		t.Logf("#%d. bucketID:%v key:%v\n", i, c.bucketID, c.key)
		diff, err := determineDifferingBitIndex(c.bucketID, c.key)
		assert.NoError(t, err)
		assert.Equal(t, c.expected, diff)
	}

	diff, err := determineDifferingBitIndex(filledID(255), filledID(255))
	assert.True(t, RoutingErr.Has(err))
	assert.Equal(t, diff, -2)
}
