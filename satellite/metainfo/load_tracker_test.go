// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/storj/satellite/nodeselection"
)

func TestLoadTracker(t *testing.T) {
	t.Parallel()

	tr := NewLoadTracker()
	node1, node2 := storj.NodeID{0: 1}, storj.NodeID{0: 2}

	check := func(id storj.NodeID, expect float64) {
		t.Helper()
		require.Equal(t, expect, tr.Get(storj.NodeID{})(&nodeselection.SelectedNode{ID: id}))
	}

	// unknown nodes have zero load
	check(node1, 0)

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() { tr.Increment(node1) })
	}
	wg.Wait()
	tr.Increment(node2)

	check(node1, 10)
	check(node2, 1)

	// the previous generation is still counted
	tr.BumpGeneration()
	tr.Increment(node1)
	check(node1, 11)
	check(node2, 1)

	// selections older than the previous generation are forgotten
	tr.BumpGeneration()
	check(node1, 1)
	check(node2, 0)

	tr.BumpGeneration()
	check(node1, 0)

	loads := map[storj.NodeID]float64{}
	tr.Increment(node2)
	tr.Range(func(id storj.NodeID, v float64) {
		loads[id] = v
	})
	// idle nodes are dropped
	require.Equal(t, map[storj.NodeID]float64{node2: 1}, loads)
}

func TestTrackersNodeSelected(t *testing.T) {
	t.Parallel()

	trackers := NewTrackers(Config{}, nil, func(id storj.NodeID) SuccessTracker {
		return NewPercentSuccessTracker()
	}, NewPercentSuccessTracker(), NewPercentSuccessTracker(), nil)

	node := &nodeselection.SelectedNode{ID: storj.NodeID{0: 1}}
	trackers.NodeSelected(node.ID)
	trackers.NodeSelected(node.ID)
	require.Equal(t, float64(2), trackers.GetLoadTracker().Get(storj.NodeID{})(node))

	// the load tracker has its own generation, independent of the success trackers
	trackers.BumpGeneration()
	trackers.BumpGeneration()
	require.Equal(t, float64(2), trackers.GetLoadTracker().Get(storj.NodeID{})(node))

	trackers.BumpLoadGeneration()
	trackers.BumpLoadGeneration()
	require.Equal(t, float64(0), trackers.GetLoadTracker().Get(storj.NodeID{})(node))
}
