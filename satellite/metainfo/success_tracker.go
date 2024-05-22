// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"sync"
	"sync/atomic"

	"storj.io/common/storj"
)

type successTrackers struct {
	trackers map[storj.NodeID]*successTracker
	global   *successTracker
}

func newSuccessTrackers(approvedUplinks []storj.NodeID) *successTrackers {
	global := new(successTracker)
	trackers := make(map[storj.NodeID]*successTracker, len(approvedUplinks))
	for _, uplink := range approvedUplinks {
		trackers[uplink] = new(successTracker)
	}

	return &successTrackers{
		trackers: trackers,
		global:   global,
	}
}

func (t *successTrackers) BumpGeneration() {
	for _, tracker := range t.trackers {
		tracker.BumpGeneration()
	}
	t.global.BumpGeneration()
}

func (t *successTrackers) GetTracker(uplink storj.NodeID) *successTracker {
	if tracker, ok := t.trackers[uplink]; ok {
		return tracker
	}
	return t.global
}

const nodeSuccessGenerations = 4

type successTracker struct {
	mu   sync.Mutex
	gen  uint64
	data sync.Map
}

func (t *successTracker) Increment(node storj.NodeID, success bool) {
	ctrsI, ok := t.data.Load(node)
	if !ok {
		ctrsI, _ = t.data.LoadOrStore(node, new([nodeSuccessGenerations]uint64))
	}
	ctrs, _ := ctrsI.(*[nodeSuccessGenerations]uint64)

	v := uint64(1)
	if success {
		v |= 1 << 32
	}

	gen := atomic.LoadUint64(&t.gen) % nodeSuccessGenerations
	atomic.AddUint64(&ctrs[gen], v)
}

func (t *successTracker) Get(node storj.NodeID) (success, total uint32) {
	ctrsI, ok := t.data.Load(node)
	if !ok {
		return 0, 0
	}
	ctrs, _ := ctrsI.(*[nodeSuccessGenerations]uint64)

	var sum uint64
	for i := range ctrs {
		sum += atomic.LoadUint64(&ctrs[i])
	}

	return uint32(sum >> 32), uint32(sum)
}

func (t *successTracker) BumpGeneration() {
	t.mu.Lock()
	defer t.mu.Unlock()

	// consider when we have 4 counters, [a b c d] and we have just
	// finished writing to a and so we will start writing to b.
	// when we were writing to a, the valid counters to sum from
	// would be a, c, and d. when we are writing to b, the valid
	// counters to sum from would be b, d and a, and so we have to
	// clear c, which is 2 ahead from a. so we add 2. the atomic
	// call returns the new value, so it adds 1 already.
	gen := (atomic.AddUint64(&t.gen, 1) + 1) % nodeSuccessGenerations
	t.data.Range(func(_, ctrsI any) bool {
		ctrs, _ := ctrsI.(*[nodeSuccessGenerations]uint64)
		atomic.StoreUint64(&ctrs[gen], 0)
		return true
	})
}
