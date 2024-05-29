// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/storj/satellite/metainfo"
)

func TestSuccessTracker(t *testing.T) {
	run := func(t *testing.T, do func(func()), wait func()) {
		var tr metainfo.SuccessTracker

		check := func(id storj.NodeID, success, total uint32) {
			gotsuccess, gottotal := tr.Get(id)
			require.Equal(t, gotsuccess, success)
			require.Equal(t, gottotal, total)
		}

		do(func() { tr.Increment(storj.NodeID{0: 1}, true) })
		do(func() { tr.Increment(storj.NodeID{0: 1}, true) })
		do(func() { tr.Increment(storj.NodeID{0: 1}, false) })

		do(func() { tr.Increment(storj.NodeID{0: 2}, true) })
		do(func() { tr.Increment(storj.NodeID{0: 2}, true) })
		do(func() { tr.Increment(storj.NodeID{0: 2}, true) })

		wait()
		check(storj.NodeID{0: 1}, 2, 3)
		check(storj.NodeID{0: 2}, 3, 3)

		tr.BumpGeneration()

		do(func() { tr.Increment(storj.NodeID{0: 1}, true) })
		do(func() { tr.Increment(storj.NodeID{0: 2}, true) })

		wait()
		check(storj.NodeID{0: 1}, 3, 4)
		check(storj.NodeID{0: 2}, 4, 4)

		tr.BumpGeneration()

		do(func() { tr.Increment(storj.NodeID{0: 1}, false) })
		do(func() { tr.Increment(storj.NodeID{0: 2}, false) })

		wait()
		check(storj.NodeID{0: 1}, 3, 5)
		check(storj.NodeID{0: 2}, 4, 5)

		do(tr.BumpGeneration) // first generation finally falls out
		do(func() { tr.Increment(storj.NodeID{0: 1}, true) })
		do(func() { tr.Increment(storj.NodeID{0: 2}, false) })

		wait()
		check(storj.NodeID{0: 1}, 2, 3)
		check(storj.NodeID{0: 2}, 1, 3)
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
