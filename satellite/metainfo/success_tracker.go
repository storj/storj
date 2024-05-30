// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"math"
	"math/bits"
	"sync"
	"sync/atomic"

	"storj.io/common/storj"
)

// SuccessTracker describes a type that is told about successes of nodes and
// can be queried for an aggregate value that represents how successful a node
// is expected to be.
type SuccessTracker interface {
	// Increment tells the SuccessTracker if a node was recently successful or
	// not.
	Increment(node storj.NodeID, success bool)

	// Get returns a value that represents how successful a node is expected to
	// be. It can return NaN to indicate that it has no information about the
	// node.
	Get(node storj.NodeID) float64

	// BumpGeneration should be called periodically to clear out stale
	// information.
	BumpGeneration()
}

// GetNewSuccessTracker returns a function that creates a new SuccessTracker
// based on the kind. The bool return value is false if the kind is unknown.
func GetNewSuccessTracker(kind string) (func() SuccessTracker, bool) {
	switch kind {
	case "bitshift":
		return func() SuccessTracker { return new(bitshiftSuccessTracker) }, true
	case "percent":
		return func() SuccessTracker { return new(percentSuccessTracker) }, true
	default:
		return nil, false
	}
}

// SuccessTrackers manages global and uplink level trackers.
type SuccessTrackers struct {
	trackers map[storj.NodeID]SuccessTracker
	global   SuccessTracker
}

// NewSuccessTrackers creates a new success tracker.
func NewSuccessTrackers(approvedUplinks []storj.NodeID, newTracker func() SuccessTracker) *SuccessTrackers {
	global := newTracker()
	trackers := make(map[storj.NodeID]SuccessTracker, len(approvedUplinks))
	for _, uplink := range approvedUplinks {
		trackers[uplink] = newTracker()
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

// GetTracker returns the tracker for the specific uplink. Returns with the
// global tracker, if uplink is not whitelisted.
func (t *SuccessTrackers) GetTracker(uplink storj.NodeID) SuccessTracker {
	if tracker, ok := t.trackers[uplink]; ok {
		return tracker
	}
	return t.global
}

// Get returns a function that can be used to get an estimate of how good a node
// is for a given uplink.
func (t *SuccessTrackers) Get(uplink storj.NodeID) func(node storj.NodeID) float64 {
	return t.GetTracker(uplink).Get
}

//
// percent success tracker
//

const nodeSuccessGenerations = 4

type nodeCounterArray [nodeSuccessGenerations]atomic.Uint64

type percentSuccessTracker struct {
	mu   sync.Mutex
	gen  atomic.Uint64
	data sync.Map // storj.NodeID -> *nodeCounterArray
}

func (t *percentSuccessTracker) Increment(node storj.NodeID, success bool) {
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

func (t *percentSuccessTracker) Get(node storj.NodeID) float64 {
	ctrsI, ok := t.data.Load(node)
	if !ok {
		return math.NaN() // no counter yet means NaN
	}
	ctrs, _ := ctrsI.(*nodeCounterArray)

	var sum uint64
	for i := range ctrs {
		sum += ctrs[i].Load()
	}
	success, total := uint32(sum>>32), uint32(sum)

	return float64(success) / float64(total) // 0/0 == NaN which is ok
}

func (t *percentSuccessTracker) BumpGeneration() {
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

//
// bitshift success tracker
//

// increment does a CAS loop incrementing the value in the counter by sliding
// the bits in it to the left by 1 and adding 1 if there was a success.
func increment(ctr *atomic.Uint64, success bool) {
	for {
		o := ctr.Load()
		v := o << 1
		if success {
			v++
		}
		if ctr.CompareAndSwap(o, v) {
			return
		}
	}
}

type bitshiftSuccessTracker struct {
	mu   sync.Mutex
	data sync.Map // storj.NodeID -> *atomic.Uint64
}

func (t *bitshiftSuccessTracker) Increment(node storj.NodeID, success bool) {
	crtI, ok := t.data.Load(node)
	if !ok {
		v := new(atomic.Uint64)
		v.Store(^uint64(0))
		crtI, _ = t.data.LoadOrStore(node, v)
	}
	ctr, _ := crtI.(*atomic.Uint64)
	increment(ctr, success)
}

func (t *bitshiftSuccessTracker) Get(node storj.NodeID) float64 {
	ctrI, ok := t.data.Load(node)
	if !ok {
		return math.NaN() // no counter yet means NaN
	}
	ctr, _ := ctrI.(*atomic.Uint64)
	return float64(bits.OnesCount64(ctr.Load()))
}

func (t *bitshiftSuccessTracker) BumpGeneration() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.data.Range(func(_, ctrI any) bool {
		ctr, _ := ctrI.(*atomic.Uint64)
		increment(ctr, true)
		return true
	})
}
