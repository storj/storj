// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package intset_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/private/intset"
)

func TestSet(t *testing.T) {

	set := intset.NewSet(5)

	require.Zero(t, set.Count())

	for i := -1; i < 10; i++ {
		set.Include(i)
	}

	require.Equal(t, 5, set.Count())

	for i := 0; i < 5; i++ {
		require.True(t, set.Contains(i))

		set.Exclude(i)
	}

	for i := -1; i < 10; i++ {
		require.False(t, set.Contains(i), "#%d", i)
	}
}

func TestCopySet(t *testing.T) {
	setA := intset.NewSet(10)
	setB := intset.NewSet(10)

	for i := 0; i < 10; i++ {
		if i%2 == 0 {
			setA.Include(i)
		} else {
			setB.Include(i)
		}
	}
	require.Equal(t, 5, setA.Count())
	require.Equal(t, 5, setB.Count())

	setC := intset.NewSet(10)
	setC.Add(setA, setB)
	for i := 0; i < 10; i++ {
		require.True(t, setC.Contains(i))
	}
	require.Equal(t, 10, setC.Count())

	setD := intset.NewSet(3)
	setE := intset.NewSet(3)
	setE.Include(0)
	setE.Include(2)
	// set with different initial size will be ignored while adding
	setF := intset.NewSet(5)
	setF.Include(1)
	setD.Add(setE, setF)

	require.Equal(t, 2, setD.Count())
	for i, contains := range []bool{true, false, true, false, false} {
		require.Equal(t, contains, setD.Contains(i), "#%d", i)
	}
}

func BenchmarkIntSet(b *testing.B) {
	b.Run("create", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = intset.NewSet(1000)
		}
	})
}
