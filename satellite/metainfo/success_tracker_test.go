// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/storj/satellite/nodeselection"
)

func TestBitshiftSuccessTracker(t *testing.T) {
	run := func(t *testing.T, do func(func()), wait func()) {
		var tr bitshiftSuccessTracker

		check := func(id storj.NodeID, expect float64) {
			got := tr.Get(&nodeselection.SelectedNode{ID: id})
			require.Equal(t, got, expect)
		}

		// clear out the initial values
		for i := 0; i < 64; i++ {
			tr.Increment(storj.NodeID{0: 1}, false)
			tr.Increment(storj.NodeID{0: 2}, false)
		}

		do(func() { tr.Increment(storj.NodeID{0: 1}, true) })
		do(func() { tr.Increment(storj.NodeID{0: 1}, true) })
		do(func() { tr.Increment(storj.NodeID{0: 1}, false) })

		do(func() { tr.Increment(storj.NodeID{0: 2}, true) })
		do(func() { tr.Increment(storj.NodeID{0: 2}, true) })
		do(func() { tr.Increment(storj.NodeID{0: 2}, true) })

		wait()
		check(storj.NodeID{0: 1}, 2)
		check(storj.NodeID{0: 2}, 3)

		tr.BumpGeneration()

		do(func() { tr.Increment(storj.NodeID{0: 1}, true) })
		do(func() { tr.Increment(storj.NodeID{0: 2}, true) })

		wait()
		check(storj.NodeID{0: 1}, 4)
		check(storj.NodeID{0: 2}, 5)

		tr.BumpGeneration()

		do(func() { tr.Increment(storj.NodeID{0: 1}, false) })
		do(func() { tr.Increment(storj.NodeID{0: 2}, false) })

		wait()
		check(storj.NodeID{0: 1}, 5)
		check(storj.NodeID{0: 2}, 6)

		do(tr.BumpGeneration)
		do(func() { tr.Increment(storj.NodeID{0: 1}, true) })
		do(func() { tr.Increment(storj.NodeID{0: 2}, false) })

		wait()
		check(storj.NodeID{0: 1}, 7)
		check(storj.NodeID{0: 2}, 7)
	}

	t.Run("Serial", func(t *testing.T) {
		run(t,
			func(f func()) {
				f()
			},
			func() {},
		)
	})

	t.Run("Concurrent", func(t *testing.T) {
		var wg sync.WaitGroup
		run(t,
			func(f func()) {
				wg.Add(1)
				go func() {
					defer wg.Done()
					f()
				}()
			},
			wg.Wait,
		)
	})
}

func TestPercentSuccessTracker(t *testing.T) {
	run := func(t *testing.T, do func(func()), wait func()) {
		var tr percentSuccessTracker

		check := func(id storj.NodeID, expect float64) {
			got := tr.Get(&nodeselection.SelectedNode{ID: id})
			require.Equal(t, got, expect)
		}

		do(func() { tr.Increment(storj.NodeID{0: 1}, true) })
		do(func() { tr.Increment(storj.NodeID{0: 1}, true) })
		do(func() { tr.Increment(storj.NodeID{0: 1}, false) })

		do(func() { tr.Increment(storj.NodeID{0: 2}, true) })
		do(func() { tr.Increment(storj.NodeID{0: 2}, true) })
		do(func() { tr.Increment(storj.NodeID{0: 2}, true) })

		wait()
		check(storj.NodeID{0: 1}, 2./3)
		check(storj.NodeID{0: 2}, 3./3)

		tr.BumpGeneration()

		do(func() { tr.Increment(storj.NodeID{0: 1}, true) })
		do(func() { tr.Increment(storj.NodeID{0: 2}, true) })

		wait()
		check(storj.NodeID{0: 1}, 3./4)
		check(storj.NodeID{0: 2}, 4./4)

		tr.BumpGeneration()

		do(func() { tr.Increment(storj.NodeID{0: 1}, false) })
		do(func() { tr.Increment(storj.NodeID{0: 2}, false) })

		wait()
		check(storj.NodeID{0: 1}, 3./5)
		check(storj.NodeID{0: 2}, 4./5)

		do(tr.BumpGeneration) // first generation finally falls out
		do(func() { tr.Increment(storj.NodeID{0: 1}, true) })
		do(func() { tr.Increment(storj.NodeID{0: 2}, false) })

		wait()
		check(storj.NodeID{0: 1}, 2./3)
		check(storj.NodeID{0: 2}, 1./3)
	}

	t.Run("Serial", func(t *testing.T) {
		run(t,
			func(f func()) {
				f()
			},
			func() {},
		)
	})

	t.Run("Concurrent", func(t *testing.T) {
		var wg sync.WaitGroup
		run(t,
			func(f func()) {
				wg.Add(1)
				go func() {
					defer wg.Done()
					f()
				}()
			},
			wg.Wait,
		)
	})
}

func TestBigBitshiftSuccessTracker(t *testing.T) {
	run := func(t *testing.T, do func(func()), wait func()) {
		tr := NewBigBitshiftSuccessTracker(10)

		check := func(id storj.NodeID, expect float64) {
			got := tr.Get(&nodeselection.SelectedNode{ID: id})
			require.Equal(t, got, expect)
		}

		// clear out the initial values
		for i := 0; i < 64; i++ {
			tr.Increment(storj.NodeID{0: 1}, false)
			tr.Increment(storj.NodeID{0: 2}, false)
		}

		do(func() { tr.Increment(storj.NodeID{0: 1}, true) })
		do(func() { tr.Increment(storj.NodeID{0: 1}, true) })
		do(func() { tr.Increment(storj.NodeID{0: 1}, false) })

		do(func() { tr.Increment(storj.NodeID{0: 2}, true) })
		do(func() { tr.Increment(storj.NodeID{0: 2}, true) })
		do(func() { tr.Increment(storj.NodeID{0: 2}, true) })

		wait()
		check(storj.NodeID{0: 1}, 2)
		check(storj.NodeID{0: 2}, 3)

		tr.BumpGeneration()

		do(func() { tr.Increment(storj.NodeID{0: 1}, true) })
		do(func() { tr.Increment(storj.NodeID{0: 2}, true) })

		wait()
		check(storj.NodeID{0: 1}, 4)
		check(storj.NodeID{0: 2}, 5)

		tr.BumpGeneration()

		do(func() { tr.Increment(storj.NodeID{0: 1}, false) })
		do(func() { tr.Increment(storj.NodeID{0: 2}, false) })

		wait()
		check(storj.NodeID{0: 1}, 5)
		check(storj.NodeID{0: 2}, 6)

		do(tr.BumpGeneration)
		do(func() { tr.Increment(storj.NodeID{0: 1}, true) })
		do(func() { tr.Increment(storj.NodeID{0: 2}, false) })

		wait()
		check(storj.NodeID{0: 1}, 7)
		check(storj.NodeID{0: 2}, 7)
	}

	t.Run("Serial", func(t *testing.T) {
		run(t,
			func(f func()) {
				f()
			},
			func() {},
		)
	})

	t.Run("Concurrent", func(t *testing.T) {
		var wg sync.WaitGroup
		run(t,
			func(f func()) {
				wg.Add(1)
				go func() {
					defer wg.Done()
					f()
				}()
			},
			wg.Wait,
		)
	})
}

func TestBigBitList(t *testing.T) {
	b := bigBitList{
		data:   make([]uint64, 1),
		length: 7,
	}
	b.Increment(true)
	require.Equal(t, uint64(1), b.data[0])
	b.Increment(false)
	b.Increment(true)

	// so far 101
	require.Equal(t, uint64(5), b.data[0])

	b.Increment(false)
	b.Increment(false)
	b.Increment(true)

	// so far: 10 0101
	require.Equal(t, uint64(0x25), b.data[0])
	require.Equal(t, 3, b.numberOfOnes)

	b.Increment(true)

	// 7 bit is full (110 0101), overwriting previous values
	b.Increment(true)
	b.Increment(true)
	b.Increment(true)
	b.Increment(false)
	b.Increment(false)
	b.Increment(false)

	// 7 bit is full (100 0111), overwriting previous values
	require.Equal(t, uint64(0x47), b.data[0])
}

func TestBigBitList_small(t *testing.T) {
	b, _ := GetNewSuccessTracker("bitshift3")
	tracker := b()
	for i := 0; i < 10; i++ {
		tracker.Increment(storj.NodeID{}, true)
	}
	require.Equal(t, float64(3), tracker.Get(&nodeselection.SelectedNode{}))
}
