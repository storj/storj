// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBitSet(t *testing.T) {
	b := newBitSet(4)
	require.False(t, b.Include(0))
	require.True(t, b.Include(0))
	require.False(t, b.Include(3))
	require.True(t, b.Include(3))
	require.False(t, b.Include(2))
	require.False(t, b.Include(1))
	require.True(t, b.Include(1))
	require.True(t, b.Include(2))
}

func TestBitSetRandom(t *testing.T) {
	for width := 0; width < 66; width++ {
		b := newBitSet(width)
		checks := make([]int, 0, width*2)
		for i := 0; i < width; i++ {
			checks = append(checks, i, i)
		}
		rand.New(rand.NewSource(time.Now().UnixNano())).Shuffle(
			len(checks), func(i, j int) { checks[i], checks[j] = checks[j], checks[i] })

		for i := 0; i < 2; i++ {
			correct := make(map[int]bool, width)
			for _, check := range checks {
				expected := correct[check]
				correct[check] = true
				require.Equal(t, expected, b.Include(check),
					fmt.Sprintf("width: %d, check: %d", width, check))
			}
			b.Clear()
		}
	}
}
