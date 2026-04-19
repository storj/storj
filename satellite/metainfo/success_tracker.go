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
	"github.com/zeebo/mwc"
	"golang.org/x/exp/maps"

	"storj.io/common/storj"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/trust"
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

	// Range iterates over all nodes and calls the function with the actual value.
	Range(fn func(storj.NodeID, float64))

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
		return func() SuccessTracker { return newBitshiftSuccessTracker(0) }, true
	case kind == "congestion":
		return func() SuccessTracker { return newCongestionSuccessTracker() }, true
	case kind == "lag":
		return func() SuccessTracker { return newLagSuccessTracker() }, true
	case strings.HasPrefix(kind, "bitshift-noise-"):
		noiseStr := strings.TrimPrefix(kind, "bitshift-noise-")
		noise, err := strconv.Atoi(noiseStr)
		if err != nil {
			panic("bitshift-noise size should be an integer, not " + noiseStr)
		}

		return func() SuccessTracker {
			return newBitshiftSuccessTracker(noise)
		}, true
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
		return NewPercentSuccessTracker, true
	default:
		return nil, false
	}
}

// Trackers manages global, per-uplink success trackers, and a shared
// failure tracker. It encapsulates the logic for deciding which trackers
// to update when a node is observed succeeding or failing during an upload.
type Trackers struct {
	dedicated      map[storj.NodeID]SuccessTracker
	global         SuccessTracker
	failure        SuccessTracker
	trustedUplinks *trust.TrustedPeersList
	config         Config
}

// NewTrackers creates a new Trackers, managing the global, per-uplink and
// failure trackers together.
func NewTrackers(
	cfg Config,
	approvedUplinks []storj.NodeID,
	newTracker func(id storj.NodeID) SuccessTracker,
	failure SuccessTracker,
	trustedUplinks *trust.TrustedPeersList,
) *Trackers {
	global := newTracker(storj.NodeID{})
	dedicated := make(map[storj.NodeID]SuccessTracker, len(approvedUplinks))
	for _, uplink := range approvedUplinks {
		dedicated[uplink] = newTracker(uplink)
	}
	return &Trackers{
		dedicated:      dedicated,
		global:         global,
		failure:        failure,
		trustedUplinks: trustedUplinks,
		config:         cfg,
	}
}

// BumpGeneration bumps the generation of all dedicated trackers and the
// global tracker.
func (t *Trackers) BumpGeneration() {
	for _, tracker := range t.dedicated {
		tracker.BumpGeneration()
	}
	t.global.BumpGeneration()
}

// BumpFailureGeneration bumps the generation of the failure tracker.
func (t *Trackers) BumpFailureGeneration() {
	t.failure.BumpGeneration()
}

// GetTracker returns the tracker for the specific uplink. Returns with the
// global tracker, if uplink is not whitelisted.
func (t *Trackers) GetTracker(uplink storj.NodeID) SuccessTracker {
	if tracker, ok := t.dedicated[uplink]; ok {
		return tracker
	}
	return t.global
}

// GetDedicatedTracker returns the tracker for the specific uplink. Returns
// nil if the uplink is not whitelisted.
func (t *Trackers) GetDedicatedTracker(uplink storj.NodeID) SuccessTracker {
	if tracker, ok := t.dedicated[uplink]; ok {
		return tracker
	}
	return nil
}

// GetGlobalTracker returns the global tracker.
func (t *Trackers) GetGlobalTracker() SuccessTracker {
	return t.global
}

// GetFailureTracker returns the failure tracker.
func (t *Trackers) GetFailureTracker() SuccessTracker {
	return t.failure
}

// Get returns a function that can be used to get an estimate of how good a
// node is for a given uplink.
func (t *Trackers) Get(uplink storj.NodeID) func(node *nodeselection.SelectedNode) float64 {
	return t.GetTracker(uplink).Get
}

// NodeCommitted records that a node successfully stored a piece as part of
// a committed segment.
func (t *Trackers) NodeCommitted(uplink, node storj.NodeID) {
	t.record(uplink, node, true)
}

// NodeCancelled records that a node was part of the initial order limits for
// a segment but did not end up in the committed set (long-tail cancellation
// or missing upload).
func (t *Trackers) NodeCancelled(uplink, node storj.NodeID) {
	t.record(uplink, node, false)
}

// NodeRetried records that a node's piece upload is being retried with a
// different node, so the original node is considered to have failed.
func (t *Trackers) NodeRetried(uplink, node storj.NodeID) {
	t.record(uplink, node, false)
}

// record implements the shared logic for NodeCommitted, NodeCancelled and
// NodeRetried: it increments the dedicated tracker if one exists for the
// uplink, the global tracker when configured or when no dedicated tracker is
// available, and the failure tracker when the uplink is trusted.
func (t *Trackers) record(uplink, node storj.NodeID, success bool) {
	dedicated, hasDedicated := t.dedicated[uplink]
	if hasDedicated {
		dedicated.Increment(node, success)
	}
	if t.config.AlwaysUpdateGlobalTracker || !hasDedicated {
		t.global.Increment(node, success)
	}
	if t.trustedUplinks != nil && t.trustedUplinks.IsTrusted(uplink) {
		t.failure.Increment(node, success)
	}
}

// RangeAll implements MonitoredTrackers by iterating over all dedicated,
// global, and failure trackers, emitting (seriesKey, nodeID, value) triples.
func (t *Trackers) RangeAll(fn func(key monkit.SeriesKey, nodeID storj.NodeID, value float64)) {
	ids := maps.Keys(t.dedicated)
	sort.Slice(ids, func(i, j int) bool { return ids[i].Less(ids[j]) })

	successKey := monkit.NewSeriesKey("success_tracker")
	for _, id := range ids {
		key := successKey.WithTag("uplink", id.String())
		t.dedicated[id].Range(func(nodeID storj.NodeID, v float64) {
			fn(key, nodeID, v)
		})
	}
	globalKey := successKey.WithTag("uplink", storj.NodeID{}.String())
	t.global.Range(func(nodeID storj.NodeID, v float64) {
		fn(globalKey, nodeID, v)
	})
	failureKey := monkit.NewSeriesKey("failure_tracker")
	t.failure.Range(func(nodeID storj.NodeID, v float64) {
		fn(failureKey, nodeID, v)
	})
}

// Stats reports monkit statistics for all of the per-uplink and global
// trackers. The failure tracker is reported separately by the monkit chain
// wired up at construction.
func (t *Trackers) Stats(cb func(monkit.SeriesKey, string, float64)) {
	ids := maps.Keys(t.dedicated)
	sort.Slice(ids, func(i, j int) bool { return ids[i].Less(ids[j]) })

	for _, id := range ids {
		t.dedicated[id].Stats(func(key monkit.SeriesKey, field string, val float64) {
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

const nodeSuccessGenerations = 8

type nodeCounterArray [nodeSuccessGenerations]atomic.Uint64

type percentSuccessTracker struct {
	mu           sync.Mutex
	gen          atomic.Uint64
	data         sync.Map // storj.NodeID -> *nodeCounterArray
	chanceToSkip float32
}

// NewPercentSuccessTracker creates a new percent-based success tracker.
func NewPercentSuccessTracker() SuccessTracker {
	return new(percentSuccessTracker)
}

// NewStochasticPercentSuccessTracker creates a new percent-based success tracker with a stochastic chance of bumping a node's generation.
func NewStochasticPercentSuccessTracker(chanceToSkip float32) SuccessTracker {
	return &percentSuccessTracker{chanceToSkip: chanceToSkip}
}

// Range implements SuccessTracker.
func (t *percentSuccessTracker) Range(fn func(storj.NodeID, float64)) {
	t.data.Range(func(k, v interface{}) bool {
		nodeID, ok := k.(storj.NodeID)
		value, ok2 := v.(*nodeCounterArray)
		if ok && ok2 {
			fn(nodeID, readCounters(value))
		}
		return true
	})
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
		if t.chanceToSkip == 0 || mwc.Float32() >= t.chanceToSkip {
			ctrs[gen].Store(0)
		}
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

func newBitshiftSuccessTracker(noise int) *parameterizedSuccessTracker {
	addNoise := func() float64 { return 0 }
	if noise > 0 {
		addNoise = func() float64 { return float64(mwc.Intn(noise)) }
	}

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
		score:      func(v uint64) float64 { return float64(bits.OnesCount64(v)) + addNoise() },
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

func newLagSuccessTracker() *parameterizedSuccessTracker {
	return &parameterizedSuccessTracker{
		name: "lag",
		increment: func(ctr *atomic.Uint64, success bool) {

			for {
				old := ctr.Load()
				lag, score := uint32(old>>32), uint32(old)

				if lag < score {
					lag = score
				}

				if success {
					var carry uint32
					score, carry = bits.Add32(lag, score, 0)
					score /= 2
					score++
					if carry > 0 {
						lag = math.MaxUint32 / 2
						score = math.MaxUint32 / 2
					}
				} else {
					const rate = 64 // roughly 46 failures to drop lag by 2x
					lag = uint32(uint64(lag) * (rate - 1) / rate)
					score /= 2
				}

				if ctr.CompareAndSwap(old, uint64(lag)<<32|uint64(score)) {
					return
				}
			}
		},
		defaultVal: 0,
		score:      func(v uint64) float64 { return float64(uint32(v)) },
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

// Range implements SuccessTracker.
func (t *parameterizedSuccessTracker) Range(fn func(storj.NodeID, float64)) {
	t.data.Range(func(k, v interface{}) bool {
		nodeID, ok := k.(storj.NodeID)
		ctr, ok2 := v.(*atomic.Uint64)
		if ok && ok2 {
			fn(nodeID, t.score(ctr.Load()))
		}
		return true
	})
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
		val := t.score(ctr.Load())
		if !math.IsNaN(val) {
			dist.Insert(val)
		}
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

// Range implements SuccessTracker.
func (t *bigBitshiftSuccessTracker) Range(fn func(storj.NodeID, float64)) {
	t.data.Range(func(k, v interface{}) bool {
		nodeID, ok := k.(storj.NodeID)
		ctr, ok2 := v.(*bigBitList)
		if ok && ok2 {
			fn(nodeID, ctr.get())
		}
		return true
	})
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
