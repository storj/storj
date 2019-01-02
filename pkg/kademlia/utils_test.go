// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
