// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"io"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/drpc/drpcsignal"
	"storj.io/storj/storagenode/hashstore/platform"
)

var mon = monkit.Package()

// Key is the key space operated on by the store.
type Key = storj.PieceID

func safeDivide(x, y float64) float64 {
	if y == 0 {
		return 0
	}
	return x / y
}

var signalClosed = Error.New("signal closed")

func signalError(sig *drpcsignal.Signal) error {
	if sig == nil {
		return nil
	} else if err, ok := sig.Get(); !ok {
		return nil
	} else if err != nil {
		return err
	}
	return signalClosed
}

func signalChan(sig *drpcsignal.Signal) chan struct{} {
	if sig == nil {
		return nil
	}
	return sig.Signal()
}

type recordStats struct {
	numSet   uint64 // number of set records
	lenSet   uint64 // sum of lengths in set records
	numTrash uint64 // number of set trash records
	lenTrash uint64 // sum of lengths in set trash records
	numTTL   uint64 // number of set records with expiration and not trash
	lenTTL   uint64 // sum of lengths in set records with expiration and not trash
}

func (r *recordStats) Include(rec Record) {
	r.numSet++
	r.lenSet += uint64(rec.Length)

	if rec.Expires.Trash() {
		r.numTrash++
		r.lenTrash += uint64(rec.Length)
	} else if rec.Expires.Set() {
		r.numTTL++
		r.lenTTL += uint64(rec.Length)
	}
}

func zapHumanBytes[T ~int | ~int64 | ~uint | ~uint64](key string, v T) zap.Field {
	return zap.String(key, memory.FormatBytes(int64(v)))
}

//
// date/time helpers
//

// clampDate returns the uint32 value of d, saturating to the maximum if the conversion would
// overflow and returning 1 if it is negative. This is used to put a maximum date on expiration
// times and so that if someone passes in an expiration way in the future it doesn't end up in the
// past, and if someone passes an expiration before 1970, it gets set to the minimum past value in
// 1970.
func clampDate(d int64) uint32 {
	if d < 0 {
		return 1
	} else if uint64(d) >= 1<<23-1 {
		return 1<<23 - 1
	}
	return uint32(d)
}

// TimeToDateDown returns a number of days past the epoch that is less than or equal to t.
func TimeToDateDown(t time.Time) uint32 { return clampDate(t.Unix() / 86400) }

// TimeToDateUp returns a number of days past the epoch that is greater than or equal to t.
func TimeToDateUp(t time.Time) uint32 { return clampDate((t.Unix() + 86400 - 1) / 86400) }

// DateToTime returns the earliest time in the day for the given date.
func DateToTime(d uint32) time.Time { return time.Unix(int64(d)*86400, 0).UTC() }

// NormalizeTTL takes a time and returns a time that is an output of DateToTime that is larger than
// or equal to the input time, for all times before the year 24937. In other words, it rounds up to
// the closest time representable for a TTL, and rounds down if no such time is possible (i.e. times
// after year 24937).
func NormalizeTTL(t time.Time) time.Time { return DateToTime(TimeToDateUp(t)) }

//
// simple boolean flag that can be used to set once
//

type flag bool

func (f flag) get() bool { return bool(f) }
func (f *flag) set() (old bool) {
	old, *f = f.get(), true
	return old
}

//
// generic wrapper around sync.Map
//

type atomicMap[K comparable, V any] struct {
	_ [0]func() (*K, *V) // prevent equality and unsound conversions
	m sync.Map
}

func (a *atomicMap[K, V]) Empty() (empty bool) {
	empty = true
	a.m.Range(func(_, _ any) bool {
		empty = false
		return false
	})
	return empty
}

func (a *atomicMap[K, V]) Clear()       { a.m.Clear() }
func (a *atomicMap[K, V]) Set(k K, v V) { a.m.Store(k, v) }
func (a *atomicMap[K, V]) Range(fn func(K, V) (bool, error)) (err error) {
	a.m.Range(func(k, v any) (ok bool) {
		ok, err = fn(k.(K), v.(V))
		return ok && err == nil
	})
	return err
}

func (a *atomicMap[K, V]) LoadAndDelete(k K) (V, bool) {
	v, ok := a.m.LoadAndDelete(k)
	if !ok {
		return *new(V), false
	}
	return v.(V), true
}

func (a *atomicMap[K, V]) Lookup(k K) (V, bool) {
	v, ok := a.m.Load(k)
	if !ok {
		return *new(V), false
	}
	return v.(V), true
}

//
// filesystem helpers
//

func fileSize(fh *os.File) (int64, error) {
	if fi, err := fh.Stat(); err != nil {
		return 0, Error.Wrap(err)
	} else {
		return fi.Size(), nil
	}
}

func syncDirectory(dir string) {
	if fh, err := os.Open(dir); err == nil {
		_ = fh.Sync()
		_ = fh.Close()
	}
}

//
// atomic file creation helper
//

type atomicFile struct {
	*os.File

	tmp  string
	name string

	mu        sync.Mutex // protects the following fields
	canceled  flag
	committed flag
}

func newAtomicFile(name string) (*atomicFile, error) {
	tmp := createTempName(name)

	fh, err := platform.CreateFile(tmp)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &atomicFile{
		File: fh,

		tmp:  tmp,
		name: name,
	}, nil
}

func (a *atomicFile) Commit() (err error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.committed.set() {
		return nil
	} else if a.canceled.get() {
		return Error.New("atomic file already canceled")
	}

	// attempt to unlink the temporary file if there are any commit errors.
	defer func() {
		if err != nil {
			_ = a.Close()
			_ = os.Remove(a.tmp)
		}
	}()

	if err := a.Sync(); err != nil {
		return Error.Wrap(err)
	}
	if err := platform.Rename(a.tmp, a.name); err != nil {
		return Error.Wrap(err)
	}

	return nil
}

func (a *atomicFile) Cancel() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.canceled.set() || a.committed.get() {
		return
	}

	_ = a.Close()
	_ = os.Remove(a.tmp)
}

//
// rewrittenIndex is a helper to keep track of records rewritten during a compaction.
//

type rewrittenIndex struct {
	records []Record
}

func (ri *rewrittenIndex) add(rec Record) {
	ri.records = append(ri.records, rec)
}

func (ri *rewrittenIndex) sortByLogOff() {
	sort.Slice(ri.records, func(i, j int) bool {
		switch {
		case ri.records[i].Log < ri.records[j].Log:
			return true
		case ri.records[i].Log > ri.records[j].Log:
			return false
		case ri.records[i].Offset < ri.records[j].Offset:
			return true
		case ri.records[i].Offset > ri.records[j].Offset:
			return false
		default:
			return string(ri.records[i].Key[:]) < string(ri.records[j].Key[:])
		}
	})
}

func (ri *rewrittenIndex) sortByKey() {
	sort.Slice(ri.records, func(i, j int) bool {
		return string(ri.records[i].Key[:]) < string(ri.records[j].Key[:])
	})
}

func (ri *rewrittenIndex) findKey(key Key) (int, bool) {
	i := sort.Search(len(ri.records), func(i int) bool {
		return string(ri.records[i].Key[:]) >= string(key[:])
	})
	if i < len(ri.records) && ri.records[i].Key == key {
		return i, true
	}
	return -1, false
}

//
// multi-valued lru cache for read-only file handles so we can ignore close errors.
//

type multiLRUCache[K comparable, V io.Closer] struct {
	cap int

	mu     sync.Mutex
	cached map[K]*linkedList[K, V] // map of path to file handles
	order  linkedList[K, V]        // eviction order (head is next to evict)
}

func newMultiLRUCache[K comparable, V io.Closer](cap int) *multiLRUCache[K, V] {
	return &multiLRUCache[K, V]{
		cap: cap,

		cached: make(map[K]*linkedList[K, V]),
	}
}

func (m *multiLRUCache[K, V]) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for m.order.head != nil {
		_ = m.order.head.value.Close()
		m.order.removeEntry(m.order.head, (*listEntry[K, V]).orderList)
	}
	clear(m.cached)
}

func (m *multiLRUCache[K, V]) Get(key K, mk func(K) (V, error)) (V, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	keyed := m.cached[key]
	if keyed == nil {
		return mk(key)
	}
	ent := keyed.head

	keyed.removeEntry(ent, (*listEntry[K, V]).keyedList)
	m.order.removeEntry(ent, (*listEntry[K, V]).orderList)

	if keyed.count == 0 {
		delete(m.cached, key)
	}

	return ent.value, nil
}

func (m *multiLRUCache[K, V]) Put(key K, fh V) {
	m.mu.Lock()
	defer m.mu.Unlock()

	keyed := m.cached[key]
	if keyed == nil {
		keyed = new(linkedList[K, V])
		m.cached[key] = keyed
	}

	ent := &listEntry[K, V]{key: key, value: fh}
	keyed.appendEntry(ent, (*listEntry[K, V]).keyedList)
	m.order.appendEntry(ent, (*listEntry[K, V]).orderList)

	for m.order.count > m.cap {
		ent := m.order.head
		keyed := m.cached[ent.key]

		keyed.removeEntry(ent, (*listEntry[K, V]).keyedList)
		m.order.removeEntry(ent, (*listEntry[K, V]).orderList)

		if keyed.count == 0 {
			delete(m.cached, ent.key)
		}

		_ = ent.value.Close()
	}
}

//
// doubly-linked double-list
//

type linkedList[K, V any] struct {
	head  *listEntry[K, V]
	tail  *listEntry[K, V]
	count int
}

type listEntry[K, V any] struct {
	key   K
	value V

	order listNode[K, V] // entry in the eviction order list
	keyed listNode[K, V] // entry in the per-key list
}

type listNode[K, V any] struct {
	next *listEntry[K, V]
	prev *listEntry[K, V]
}

func (e *listEntry[K, V]) orderList() *listNode[K, V] { return &e.order }
func (e *listEntry[K, V]) keyedList() *listNode[K, V] { return &e.keyed }

func (l *linkedList[K, V]) appendEntry(ent *listEntry[K, V], node func(*listEntry[K, V]) *listNode[K, V]) {
	if l.head == nil {
		l.head = ent
	}
	if l.tail != nil {
		node(l.tail).next = ent
		node(ent).prev = l.tail
	}
	l.tail = ent
	l.count++
}

func (l *linkedList[K, V]) removeEntry(ent *listEntry[K, V], node func(*listEntry[K, V]) *listNode[K, V]) {
	n := node(ent)
	if l.head == ent {
		l.head = n.next
	}
	if n.next != nil {
		node(n.next).prev = n.prev
	}
	if l.tail == ent {
		l.tail = n.prev
	}
	if n.prev != nil {
		node(n.prev).next = n.next
	}
	l.count--
}
