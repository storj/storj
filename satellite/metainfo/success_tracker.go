// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"sync"
	"sync/atomic"

	"storj.io/common/storj"
	"storj.io/storj/satellite/nodeselection"
)

// SuccessTrackers manages global and uplink level trackers.
type SuccessTrackers struct {
	trackers map[storj.NodeID]*SuccessTracker
	global   *SuccessTracker
}

// NewSuccessTrackers creates a new success tracker.
func NewSuccessTrackers(approvedUplinks []storj.NodeID) *SuccessTrackers {
	global := new(SuccessTracker)
	trackers := make(map[storj.NodeID]*SuccessTracker, len(approvedUplinks))
	for _, uplink := range approvedUplinks {
		trackers[uplink] = new(SuccessTracker)
	}

	return &SuccessTrackers{
		trackers: trackers,
		global:   global,
	}
}

// BumpGeneration will bump all the managed trackers.
func (t *SuccessTrackers) BumpGeneration() {
	for _, tracker := range t.trackers {
		tracker.BumpGeneration()
	}
	t.global.BumpGeneration()
}

// GetTracker returns the tracker for the specific uplink. Returns with the global tracker, if uplink is not whitelisted.
func (t *SuccessTrackers) GetTracker(uplink storj.NodeID) *SuccessTracker {
	if tracker, ok := t.trackers[uplink]; ok {
		return tracker
	}
	return t.global
}

// Get implements nodeselection.UploadSuccessTracker.
func (t *SuccessTrackers) Get(uplink storj.NodeID) func(node storj.NodeID) (success, total uint32) {
	tracker := t.GetTracker(uplink)
	return func(node storj.NodeID) (success, total uint32) {
		return tracker.Get(node)
	}
}

const nodeSuccessGenerations = 4

type nodeCounterArray [nodeSuccessGenerations]atomic.Uint64

// SuccessTracker tracks the success / total uploads per node.
type SuccessTracker struct {
	mu   sync.Mutex
	gen  atomic.Uint64
	data sync.Map // storj.NodeID -> *nodeCounterArray
}

var _ nodeselection.UploadSuccessTracker = &SuccessTrackers{}

// Increment will increment success/total counters.
func (t *SuccessTracker) Increment(node storj.NodeID, success bool) {
	ctrsI, ok := t.data.Load(node)
	if !ok {
		ctrsI, _ = t.data.LoadOrStore(node, new(nodeCounterArray))
	}
	ctrs, _ := ctrsI.(*nodeCounterArray)

	v := uint64(1)
	if success {
		v |= 1 << 32
	}

	gen := t.gen.Load() % nodeSuccessGenerations
	ctrs[gen].Add(v)
}

// Get implements UploadSuccessTracker.
func (t *SuccessTracker) Get(node storj.NodeID) (success, total uint32) {
	ctrsI, ok := t.data.Load(node)
	if !ok {
		return 0, 0
	}
	ctrs, _ := ctrsI.(*nodeCounterArray)

	var sum uint64
	for i := range ctrs {
		sum += ctrs[i].Load()
	}

	return uint32(sum >> 32), uint32(sum)
}

// BumpGeneration bumps the generation. Predefined generation / buckets are used to create sliding-window from the generations / buckets.
func (t *SuccessTracker) BumpGeneration() {
	t.mu.Lock()
	defer t.mu.Unlock()

	// consider when we have 4 counters, [a b c d] and we have just
	// finished writing to a and so we will start writing to b.
	// when we were writing to a, the valid counters to sum from
	// would be a, c, and d. when we are writing to b, the valid
	// counters to sum from would be b, d and a, and so we have to
	// clear c, which is 2 ahead from a. so we add 2. the atomic
	// call returns the new value, so it adds 1 already.
	gen := (t.gen.Add(1) + 1) % nodeSuccessGenerations
	t.data.Range(func(_, ctrsI any) bool {
		ctrs, _ := ctrsI.(*nodeCounterArray)
		ctrs[gen].Store(0)
		return true
	})
}
