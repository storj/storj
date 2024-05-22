// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
)

func TestSuccessTracker(t *testing.T) {
	var tr successTracker

	check := func(id storj.NodeID, success, total uint32) {
		gotsuccess, gottotal := tr.Get(id)
		require.Equal(t, gotsuccess, success)
		require.Equal(t, gottotal, total)
	}

	tr.Increment(storj.NodeID{0: 1}, true)
	tr.Increment(storj.NodeID{0: 1}, true)
	tr.Increment(storj.NodeID{0: 1}, false)

	tr.Increment(storj.NodeID{0: 2}, true)
	tr.Increment(storj.NodeID{0: 2}, true)
	tr.Increment(storj.NodeID{0: 2}, true)

	check(storj.NodeID{0: 1}, 2, 3)
	check(storj.NodeID{0: 2}, 3, 3)

	tr.BumpGeneration()

	tr.Increment(storj.NodeID{0: 1}, true)
	tr.Increment(storj.NodeID{0: 2}, true)

	check(storj.NodeID{0: 1}, 3, 4)
	check(storj.NodeID{0: 2}, 4, 4)

	tr.BumpGeneration()

	tr.Increment(storj.NodeID{0: 1}, false)
	tr.Increment(storj.NodeID{0: 2}, false)

	check(storj.NodeID{0: 1}, 3, 5)
	check(storj.NodeID{0: 2}, 4, 5)

	tr.BumpGeneration() // first generation finally falls out

	tr.Increment(storj.NodeID{0: 1}, true)
	tr.Increment(storj.NodeID{0: 2}, false)

	check(storj.NodeID{0: 1}, 2, 3)
	check(storj.NodeID{0: 2}, 1, 3)
}
