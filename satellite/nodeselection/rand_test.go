// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"
)

func TestRand(t *testing.T) {
	// test if we get a full permutation

	t.Run("generate real permutations", func(t *testing.T) {
		var numbers []uint64
		c := NewRandomOrder(20)
		for c.Next() {
			numbers = append(numbers, c.At())
		}
		require.Len(t, numbers, 20)
		slices.Sort(numbers)
		for i := 0; i < len(numbers); i++ {
			require.Equal(t, uint64(i), numbers[i])
		}
	})

	t.Run("next always returns with false at the end", func(t *testing.T) {
		c := NewRandomOrder(3)
		require.True(t, c.Next())
		require.True(t, c.Next())
		require.True(t, c.Next())
		require.False(t, c.Next())
		require.False(t, c.Next())
	})

	t.Run("z  ero size is accepted", func(t *testing.T) {
		c := NewRandomOrder(0)
		require.False(t, c.Next())
	})

}
