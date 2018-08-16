// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestXor(t *testing.T) {
	cases := []struct {
		a        string
		b        string
		expected int
	}{
		{
			a:        "1010",
			b:        "0100",
			expected: 14,
		},
		{
			a:        "0010",
			b:        "0100",
			expected: 6,
		},
		{
			a:        "0000",
			b:        "0000",
			expected: 0,
		},
		{
			a:        "1111",
			b:        "1111",
			expected: 0,
		},
		{
			a:        "111",
			b:        "1111",
			expected: 0,
		},
		{
			a:        "1111",
			b:        "111",
			expected: 0,
		},
	}

	for _, v := range cases {

		actual, err := xor([]byte(v.a), []byte(v.b))
		assert.NoError(t, err)

		assert.Equal(t, v.expected, actual)
	}

}
