// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"sync"
	"sync/atomic"

	"github.com/spacemonkeygo/monkit/v3"

	"storj.io/common/storj"
	"storj.io/storj/satellite/nodeselection"
)

const loadTrackerGenerations = 2

type loadCounterArray [loadTrackerGenerations]atomic.Int64

// LoadTracker counts how many times each node was selected for an upload
// recently. The window is between one and two generations long (the current
// one and the previous one), so the value doesn't drop to zero right after
// BumpGeneration.
//
// The counts are local to the process: with multiple api pods each pod only
// sees its own share of the selections, which is fine for comparing nodes.
type LoadTracker struct {
	gen  atomic.Uint64
	data sync.Map // storj.NodeID -> *loadCounterArray
}

var _ nodeselection.ScoreNode = (*LoadTracker)(nil)

// NewLoadTracker creates a new LoadTracker.
func NewLoadTracker() *LoadTracker {
	return &LoadTracker{}
}

// Increment records that the node was selected for an upload.
func (t *LoadTracker) Increment(node storj.NodeID) {
	ctrsI, ok := t.data.Load(node)
	if !ok {
		ctrsI, _ = t.data.LoadOrStore(node, new(loadCounterArray))
	}
	ctrs, _ := ctrsI.(*loadCounterArray)
	ctrs[t.gen.Load()%loadTrackerGenerations].Add(1)
}

// Get implements nodeselection.ScoreNode. It returns the number of recent
// selections of the node, which is the same for all uplinks. Nodes without
// any recent selection have zero load.
func (t *LoadTracker) Get(uplink storj.NodeID) func(node *nodeselection.SelectedNode) float64 {
	return func(node *nodeselection.SelectedNode) float64 {
		ctrsI, ok := t.data.Load(node.ID)
		if !ok {
			return 0
		}
		ctrs, _ := ctrsI.(*loadCounterArray)
		return float64(readLoadCounters(ctrs))
	}
}

// Range iterates over all nodes and calls the function with the actual load.
func (t *LoadTracker) Range(fn func(storj.NodeID, float64)) {
	t.data.Range(func(k, v any) bool {
		nodeID, ok := k.(storj.NodeID)
		ctrs, ok2 := v.(*loadCounterArray)
		if ok && ok2 {
			fn(nodeID, float64(readLoadCounters(ctrs)))
		}
		return true
	})
}

// BumpGeneration starts a new generation, forgetting the selections of the
// oldest one. It must not be called concurrently.
func (t *LoadTracker) BumpGeneration() {
	// clear the slot of the next generation before switching to it, so
	// the previous generation is still counted.
	next := t.gen.Load() + 1
	t.data.Range(func(k, ctrsI any) bool {
		ctrs, _ := ctrsI.(*loadCounterArray)
		ctrs[next%loadTrackerGenerations].Store(0)
		// drop idle nodes, a concurrent Increment may lose one selection.
		if readLoadCounters(ctrs) == 0 {
			t.data.CompareAndDelete(k, ctrsI)
		}
		return true
	})
	t.gen.Store(next)
}

// Stats implements monkit.StatSource.
func (t *LoadTracker) Stats(cb func(monkit.SeriesKey, string, float64)) {
	dist := monkit.NewFloatDist(monkit.NewSeriesKey("load_tracker"))
	t.Range(func(_ storj.NodeID, load float64) {
		dist.Insert(load)
	})
	dist.Stats(cb)
}

func readLoadCounters(ctrs *loadCounterArray) int64 {
	var sum int64
	for i := range ctrs {
		sum += ctrs[i].Load()
	}
	return sum
}
