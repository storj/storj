// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"math"
	"math/bits"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/spacemonkeygo/monkit/v3"
	"golang.org/x/exp/maps"

	"storj.io/common/storj"
	"storj.io/storj/satellite/nodeselection"
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
	Get(node *nodeselection.SelectedNode) float64

	// BumpGeneration should be called periodically to clear out stale
	// information.
	BumpGeneration()

	monkit.StatSource
}

// GetNewSuccessTracker returns a function that creates a new SuccessTracker
// based on the kind. The bool return value is false if the kind is unknown.
func GetNewSuccessTracker(kind string) (func() SuccessTracker, bool) {

	switch {
	case kind == "bitshift":
		return func() SuccessTracker { return newBitshiftSuccessTracker() }, true
	case kind == "congestion":
		return func() SuccessTracker { return newCongestionSuccessTracker() }, true
	case strings.HasPrefix(kind, "bitshift"):
		lengthDef := strings.TrimPrefix(kind, "bitshift")
		length, err := strconv.Atoi(lengthDef)
		if err != nil {
			panic("bitshift size should be an integer, not " + lengthDef)
		}

		return func() SuccessTracker {
			return &bigBitshiftSuccessTracker{
				length: length,
			}
		}, true
	case kind == "percent":
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
func (t *SuccessTrackers) Get(uplink storj.NodeID) func(node *nodeselection.SelectedNode) float64 {
	return t.GetTracker(uplink).Get
}

// Stats reports monkit statistics for all of the trackers.
func (t *SuccessTrackers) Stats(cb func(monkit.SeriesKey, string, float64)) {
	ids := maps.Keys(t.trackers)
	sort.Slice(ids, func(i, j int) bool { return ids[i].Less(ids[j]) })

	for _, id := range ids {
		t.trackers[id].Stats(func(key monkit.SeriesKey, field string, val float64) {
			cb(key.WithTag("uplink_id", id.String()), field, val)
		})
	}
	t.global.Stats(func(key monkit.SeriesKey, field string, val float64) {
		cb(key.WithTag("uplink_id", "global"), field, val)
	})
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

func readCounters(ctrs *nodeCounterArray) float64 {
	var sum uint64
	for i := range ctrs {
		sum += ctrs[i].Load()
	}
	success, total := uint32(sum>>32), uint32(sum)
	return float64(success) / float64(total) // 0/0 == NaN which is ok
}

func (t *percentSuccessTracker) Get(node *nodeselection.SelectedNode) float64 {
	ctrsI, ok := t.data.Load(node.ID)
	if !ok {
		return math.NaN() // no counter yet means NaN
	}
	ctrs, _ := ctrsI.(*nodeCounterArray)
	return readCounters(ctrs)
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

func (t *percentSuccessTracker) Stats(cb func(monkit.SeriesKey, string, float64)) {
	dist := monkit.NewFloatDist(monkit.NewSeriesKey("percent_tracker"))

	t.data.Range(func(_, ctrsI any) bool {
		ctrs, _ := ctrsI.(*nodeCounterArray)
		dist.Insert(readCounters(ctrs))
		return true
	})

	dist.Stats(cb)
}

//
// different success trackers
//

func newBitshiftSuccessTracker() *parameterizedSuccessTracker {
	return &parameterizedSuccessTracker{
		name: "bitshift",
		increment: func(ctr *atomic.Uint64, success bool) {
			// increment does a CAS loop incrementing the value in the counter by sliding
			// the bits in it to the left by 1 and adding 1 if there was a success.
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
		},
		defaultVal: ^uint64(0),
		score:      func(v uint64) float64 { return float64(bits.OnesCount64(v)) },
	}
}

func newCongestionSuccessTracker() *parameterizedSuccessTracker {
	return &parameterizedSuccessTracker{
		name: "congestion",
		increment: func(ctr *atomic.Uint64, success bool) {
			if success {
				ctr.Add(1)
				return
			}
			for {
				o := ctr.Load()
				if ctr.CompareAndSwap(o, o>>1) {
					return
				}
			}
		},
		defaultVal: 0,
		score:      func(v uint64) float64 { return float64(v) },
	}
}

//
// parameterized success tracker implementation
//

type parameterizedSuccessTracker struct {
	mu         sync.Mutex
	data       sync.Map // storj.NodeID -> *atomic.Uint64
	name       string
	increment  func(ctr *atomic.Uint64, success bool)
	defaultVal uint64
	score      func(uint64) float64
}

func (t *parameterizedSuccessTracker) Increment(node storj.NodeID, success bool) {
	crtI, ok := t.data.Load(node)
	if !ok {
		v := new(atomic.Uint64)
		v.Store(t.defaultVal)
		crtI, _ = t.data.LoadOrStore(node, v)
	}
	ctr, _ := crtI.(*atomic.Uint64)
	t.increment(ctr, success)
}

func (t *parameterizedSuccessTracker) Get(node *nodeselection.SelectedNode) float64 {
	ctrI, ok := t.data.Load(node.ID)
	if !ok {
		return math.NaN() // no counter yet means NaN
	}
	ctr, _ := ctrI.(*atomic.Uint64)
	return t.score(ctr.Load())
}

func (t *parameterizedSuccessTracker) BumpGeneration() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.data.Range(func(_, ctrI any) bool {
		ctr, _ := ctrI.(*atomic.Uint64)
		t.increment(ctr, true)
		return true
	})
}

func (t *parameterizedSuccessTracker) Stats(cb func(monkit.SeriesKey, string, float64)) {
	dist := monkit.NewFloatDist(monkit.NewSeriesKey(t.name + "_tracker"))

	t.data.Range(func(_, ctrI any) bool {
		ctr, _ := ctrI.(*atomic.Uint64)
		dist.Insert(t.score(ctr.Load()))
		return true
	})

	dist.Stats(cb)
}

type bigBitList struct {
	mu           sync.Mutex
	position     int
	numberOfOnes int
	data         []uint64
	length       int
}

func (l *bigBitList) Increment(success bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	byteIndex := l.position / 64
	bitIndex := l.position % 64
	oldValue := (l.data[byteIndex] >> bitIndex) & 1
	if success {
		l.data[byteIndex] |= 1 << bitIndex
		if oldValue == 0 {
			l.numberOfOnes++
		}
	} else {
		l.data[byteIndex] &^= 1 << bitIndex
		if oldValue == 1 {
			l.numberOfOnes--
		}
	}
	l.position++
	if l.position >= l.length {
		l.position = 0
	}
}

func (l *bigBitList) get() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return float64(l.numberOfOnes)
}

type bigBitshiftSuccessTracker struct {
	data   sync.Map // storj.NodeID -> bigBitList
	length int
}

// NewBigBitshiftSuccessTracker creates a new BigBitshiftSuccessTracker.
func NewBigBitshiftSuccessTracker(length int) SuccessTracker {
	return &bigBitshiftSuccessTracker{
		length: length,
	}
}

// Increment implements SuccessTracker.
func (t *bigBitshiftSuccessTracker) Increment(node storj.NodeID, success bool) {
	crtI, ok := t.data.Load(node)
	if !ok {
		v := &bigBitList{
			data:   make([]uint64, int(math.Ceil(float64(t.length)/64))),
			length: t.length,
		}
		crtI, _ = t.data.LoadOrStore(node, v)
	}
	ctr, _ := crtI.(*bigBitList)
	ctr.Increment(success)
}

// Get implements SuccessTracker.
func (t *bigBitshiftSuccessTracker) Get(node *nodeselection.SelectedNode) float64 {
	ctrI, ok := t.data.Load(node.ID)
	if !ok {
		return math.NaN() // no counter yet means NaN
	}
	ctr, _ := ctrI.(*bigBitList)
	return ctr.get()
}

// BumpGeneration implements SuccessTracker.
func (t *bigBitshiftSuccessTracker) BumpGeneration() {
	t.data.Range(func(_, ctrI any) bool {
		ctr, _ := ctrI.(*bigBitList)
		ctr.Increment(true)
		return true
	})
}

func (t *bigBitshiftSuccessTracker) Stats(cb func(monkit.SeriesKey, string, float64)) {
	dist := monkit.NewFloatDist(monkit.NewSeriesKey("big_bitshift_tracker"))

	t.data.Range(func(_, ctrI any) bool {
		ctr, _ := ctrI.(*bigBitList)
		dist.Insert(ctr.get())
		return true
	})

	dist.Stats(cb)
}
